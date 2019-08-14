package lookup

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Jguer/go-alpm"
	rpc "github.com/mikkeloscar/aur"

	"github.com/Jguer/yay/v10/pkg/lookup/query"
	"github.com/Jguer/yay/v10/pkg/lookup/upgrade"
	"github.com/Jguer/yay/v10/pkg/runtime"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"
	"github.com/Jguer/yay/v10/pkg/vcs"
)

const smallArrow = " ->"
const arrow = "==>"

// PrintUpdateList prints available updates
//TODO: Make it less hacky
func PrintUpdateList(cmdArgs *types.Arguments, alpmHandle *alpm.Handle, config *runtime.Configuration, savedInfo vcs.InfoStore) error {
	targets := types.SliceToStringSet(cmdArgs.Targets)
	warnings := &types.AURWarnings{}
	old := os.Stdout // keep backup of the real stdout
	os.Stdout = nil
	_, _, localNames, remoteNames, err := query.FilterPackages(alpmHandle)
	if err != nil {
		return err
	}

	aurUp, repoUp, err := upgrade.UpList(config, alpmHandle, cmdArgs, savedInfo, warnings)
	os.Stdout = old // restoring the real stdout
	if err != nil {
		return err
	}

	noTargets := len(targets) == 0

	if !cmdArgs.ExistsArg("m", "foreign") {
		for _, pkg := range repoUp {
			if noTargets || targets.Get(pkg.Name) {
				if cmdArgs.ExistsArg("q", "quiet") {
					fmt.Printf("%s\n", pkg.Name)
				} else {
					fmt.Printf("%s %s -> %s\n", text.Bold(pkg.Name), text.Green(pkg.LocalVersion), text.Green(pkg.RemoteVersion))
				}
				delete(targets, pkg.Name)
			}
		}
	}

	if !cmdArgs.ExistsArg("n", "native") {
		for _, pkg := range aurUp {
			if noTargets || targets.Get(pkg.Name) {
				if cmdArgs.ExistsArg("q", "quiet") {
					fmt.Printf("%s\n", pkg.Name)
				} else {
					fmt.Printf("%s %s -> %s\n", text.Bold(pkg.Name), text.Green(pkg.LocalVersion), text.Green(pkg.RemoteVersion))
				}
				delete(targets, pkg.Name)
			}
		}
	}

	missing := false

outer:
	for pkg := range targets {
		for _, name := range localNames {
			if name == pkg {
				continue outer
			}
		}

		for _, name := range remoteNames {
			if name == pkg {
				continue outer
			}
		}

		fmt.Fprintln(os.Stderr, text.Red(text.Bold("error:")), "package '"+pkg+"' was not found")
		missing = true
	}

	if missing {
		return fmt.Errorf("")
	}

	return nil
}

// PrintNumberOfUpdates returns count of available updates
//TODO: Make it less hacky
func PrintNumberOfUpdates(cmdArgs *types.Arguments, alpmHandle *alpm.Handle, config *runtime.Configuration, savedInfo vcs.InfoStore) error {
	//todo
	warnings := &types.AURWarnings{}
	old := os.Stdout // keep backup of the real stdout
	os.Stdout = nil
	aurUp, repoUp, err := upgrade.UpList(config, alpmHandle, cmdArgs, savedInfo, warnings)
	os.Stdout = old // restoring the real stdout
	if err != nil {
		return err
	}
	fmt.Println(len(aurUp) + len(repoUp))

	return nil
}

func printInfoValue(str, value string) {
	if value == "" {
		value = "None"
	}

	fmt.Printf(text.Bold("%-16s%s")+" %s\n", str, ":", value)
}

// PrintInfo prints package info like pacman -Si.
func printInfo(args *types.Arguments, aurURL string, a *rpc.Pkg) {
	printInfoValue("Repository", "aur")
	printInfoValue("Name", a.Name)
	printInfoValue("Keywords", strings.Join(a.Keywords, "  "))
	printInfoValue("Version", a.Version)
	printInfoValue("Description", a.Description)
	printInfoValue("URL", a.URL)
	printInfoValue("AUR URL", aurURL+"/packages/"+a.Name)
	printInfoValue("Groups", strings.Join(a.Groups, "  "))
	printInfoValue("Licenses", strings.Join(a.License, "  "))
	printInfoValue("Provides", strings.Join(a.Provides, "  "))
	printInfoValue("Depends On", strings.Join(a.Depends, "  "))
	printInfoValue("Make Deps", strings.Join(a.MakeDepends, "  "))
	printInfoValue("Check Deps", strings.Join(a.CheckDepends, "  "))
	printInfoValue("Optional Deps", strings.Join(a.OptDepends, "  "))
	printInfoValue("Conflicts With", strings.Join(a.Conflicts, "  "))
	printInfoValue("Maintainer", a.Maintainer)
	printInfoValue("Votes", fmt.Sprintf("%d", a.NumVotes))
	printInfoValue("Popularity", fmt.Sprintf("%f", a.Popularity))
	printInfoValue("First Submitted", text.FormatTimeQuery(a.FirstSubmitted))
	printInfoValue("Last Modified", text.FormatTimeQuery(a.LastModified))

	if a.OutOfDate != 0 {
		printInfoValue("Out-of-date", text.FormatTimeQuery(a.OutOfDate))
	} else {
		printInfoValue("Out-of-date", "No")
	}

	if args.ExistsDouble("i") {
		printInfoValue("ID", fmt.Sprintf("%d", a.ID))
		printInfoValue("Package Base ID", fmt.Sprintf("%d", a.PackageBaseID))
		printInfoValue("Package Base", a.PackageBase)
		printInfoValue("Snapshot URL", aurURL+a.URLPath)
	}

	fmt.Println()
}

// Statistics returns statistics about packages installed in system
func statistics(alpmHandle *alpm.Handle) (info struct {
	Totaln    int
	Expln     int
	TotalSize int64
}, err error) {
	var tS int64 // TotalSize
	var nPkg int
	var ePkg int

	localDB, err := alpmHandle.LocalDB()
	if err != nil {
		return
	}

	for _, pkg := range localDB.PkgCache().Slice() {
		tS += pkg.ISize()
		nPkg++
		if pkg.Reason() == 0 {
			ePkg++
		}
	}

	info = struct {
		Totaln    int
		Expln     int
		TotalSize int64
	}{
		nPkg, ePkg, tS,
	}

	return
}

// BiggestPackages prints the name of the ten biggest packages in the system.
func biggestPackages(alpmHandle *alpm.Handle) {
	localDB, err := alpmHandle.LocalDB()
	if err != nil {
		return
	}

	pkgCache := localDB.PkgCache()
	pkgS := pkgCache.SortBySize().Slice()

	if len(pkgS) < 10 {
		return
	}

	for i := 0; i < 10; i++ {
		fmt.Println(text.Bold(pkgS[i].Name()) + ": " + text.Cyan(text.Human(pkgS[i].ISize())))
	}
	// Could implement size here as well, but we just want the general idea
}

// LocalStatistics prints installed packages statistics.
// TODO: printing is done here, not in children.
func LocalStatistics(config *runtime.Configuration, alpmHandle *alpm.Handle) error {
	info, err := statistics(alpmHandle)
	if err != nil {
		return err
	}

	_, _, _, remoteNames, err := query.FilterPackages(alpmHandle)
	if err != nil {
		return err
	}

	fmt.Printf(text.Bold("Yay version v%s\n"), runtime.Version)
	fmt.Println(text.Bold(text.Cyan("===========================================")))
	fmt.Println(text.Bold(text.Green("Total installed packages: ")) + text.Cyan(strconv.Itoa(info.Totaln)))
	fmt.Println(text.Bold(text.Green("Total foreign installed packages: ")) + text.Cyan(strconv.Itoa(len(remoteNames))))
	fmt.Println(text.Bold(text.Green("Explicitly installed packages: ")) + text.Cyan(strconv.Itoa(info.Expln)))
	fmt.Println(text.Bold(text.Green("Total Size occupied by packages: ")) + text.Cyan(text.Human(info.TotalSize)))
	fmt.Println(text.Bold(text.Cyan("===========================================")))
	fmt.Println(text.Bold(text.Green("Ten biggest packages:")))
	biggestPackages(alpmHandle)
	fmt.Println(text.Bold(text.Cyan("===========================================")))

	query.AURInfoPrint(config, remoteNames)

	return nil
}
