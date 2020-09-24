package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/leonelquinteros/gotext"
	rpc "github.com/mikkeloscar/aur"

	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/stringset"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/multierror"
)

// PrintSearch handles printing search results in a given format
func (q aurQuery) printSearch(start int, dbExecutor db.Executor) {
	for i := range q {
		var toprint string
		if config.SearchMode == numberMenu {
			switch config.SortMode {
			case settings.TopDown:
				toprint += text.Magenta(strconv.Itoa(start+i) + " ")
			case settings.BottomUp:
				toprint += text.Magenta(strconv.Itoa(len(q)+start-i-1) + " ")
			default:
				text.Warnln(gotext.Get("invalid sort mode. Fix with yay -Y --bottomup --save"))
			}
		} else if config.SearchMode == minimal {
			fmt.Println(q[i].Name)
			continue
		}

		toprint += text.Bold(text.ColorHash("aur")) + "/" + text.Bold(q[i].Name) +
			" " + text.Cyan(q[i].Version) +
			text.Bold(" (+"+strconv.Itoa(q[i].NumVotes)) +
			" " + text.Bold(strconv.FormatFloat(q[i].Popularity, 'f', 2, 64)+") ")

		if q[i].Maintainer == "" {
			toprint += text.Bold(text.Red(gotext.Get("(Orphaned)"))) + " "
		}

		if q[i].OutOfDate != 0 {
			toprint += text.Bold(text.Red(gotext.Get("(Out-of-date: %s)", text.FormatTime(q[i].OutOfDate)))) + " "
		}

		if pkg := dbExecutor.LocalPackage(q[i].Name); pkg != nil {
			if pkg.Version() != q[i].Version {
				toprint += text.Bold(text.Green(gotext.Get("(Installed: %s)", pkg.Version())))
			} else {
				toprint += text.Bold(text.Green(gotext.Get("(Installed)")))
			}
		}
		toprint += "\n    " + q[i].Description
		fmt.Println(toprint)
	}
}

// PrintSearch receives a RepoSearch type and outputs pretty text.
func (s repoQuery) printSearch(dbExecutor db.Executor) {
	for i, res := range s {
		var toprint string
		if config.SearchMode == numberMenu {
			switch config.SortMode {
			case settings.TopDown:
				toprint += text.Magenta(strconv.Itoa(i+1) + " ")
			case settings.BottomUp:
				toprint += text.Magenta(strconv.Itoa(len(s)-i) + " ")
			default:
				text.Warnln(gotext.Get("invalid sort mode. Fix with yay -Y --bottomup --save"))
			}
		} else if config.SearchMode == minimal {
			fmt.Println(res.Name())
			continue
		}

		toprint += text.Bold(text.ColorHash(res.DB().Name())) + "/" + text.Bold(res.Name()) +
			" " + text.Cyan(res.Version()) +
			text.Bold(" ("+text.Human(res.Size())+
				" "+text.Human(res.ISize())+") ")

		packageGroups := dbExecutor.PackageGroups(res)
		if len(packageGroups) != 0 {
			toprint += fmt.Sprint(packageGroups, " ")
		}

		if pkg := dbExecutor.LocalPackage(res.Name()); pkg != nil {
			if pkg.Version() != res.Version() {
				toprint += text.Bold(text.Green(gotext.Get("(Installed: %s)", pkg.Version())))
			} else {
				toprint += text.Bold(text.Green(gotext.Get("(Installed)")))
			}
		}

		toprint += "\n    " + res.Description()
		fmt.Println(toprint)
	}
}

// Pretty print a set of packages from the same package base.

// PrintInfo prints package info like pacman -Si.
func PrintInfo(a *rpc.Pkg, extendedInfo bool) {
	text.PrintInfoValue(gotext.Get("Repository"), "aur")
	text.PrintInfoValue(gotext.Get("Name"), a.Name)
	text.PrintInfoValue(gotext.Get("Keywords"), a.Keywords...)
	text.PrintInfoValue(gotext.Get("Version"), a.Version)
	text.PrintInfoValue(gotext.Get("Description"), a.Description)
	text.PrintInfoValue(gotext.Get("URL"), a.URL)
	text.PrintInfoValue(gotext.Get("AUR URL"), config.AURURL+"/packages/"+a.Name)
	text.PrintInfoValue(gotext.Get("Groups"), a.Groups...)
	text.PrintInfoValue(gotext.Get("Licenses"), a.License...)
	text.PrintInfoValue(gotext.Get("Provides"), a.Provides...)
	text.PrintInfoValue(gotext.Get("Depends On"), a.Depends...)
	text.PrintInfoValue(gotext.Get("Make Deps"), a.MakeDepends...)
	text.PrintInfoValue(gotext.Get("Check Deps"), a.CheckDepends...)
	text.PrintInfoValue(gotext.Get("Optional Deps"), a.OptDepends...)
	text.PrintInfoValue(gotext.Get("Conflicts With"), a.Conflicts...)
	text.PrintInfoValue(gotext.Get("Maintainer"), a.Maintainer)
	text.PrintInfoValue(gotext.Get("Votes"), fmt.Sprintf("%d", a.NumVotes))
	text.PrintInfoValue(gotext.Get("Popularity"), fmt.Sprintf("%f", a.Popularity))
	text.PrintInfoValue(gotext.Get("First Submitted"), text.FormatTimeQuery(a.FirstSubmitted))
	text.PrintInfoValue(gotext.Get("Last Modified"), text.FormatTimeQuery(a.LastModified))

	if a.OutOfDate != 0 {
		text.PrintInfoValue(gotext.Get("Out-of-date"), text.FormatTimeQuery(a.OutOfDate))
	} else {
		text.PrintInfoValue(gotext.Get("Out-of-date"), "No")
	}

	if extendedInfo {
		text.PrintInfoValue("ID", fmt.Sprintf("%d", a.ID))
		text.PrintInfoValue(gotext.Get("Package Base ID"), fmt.Sprintf("%d", a.PackageBaseID))
		text.PrintInfoValue(gotext.Get("Package Base"), a.PackageBase)
		text.PrintInfoValue(gotext.Get("Snapshot URL"), config.AURURL+a.URLPath)
	}

	fmt.Println()
}

// BiggestPackages prints the name of the ten biggest packages in the system.
func biggestPackages(dbExecutor db.Executor) {
	pkgS := dbExecutor.BiggestPackages()

	if len(pkgS) < 10 {
		return
	}

	for i := 0; i < 10; i++ {
		fmt.Printf("%s: %s\n", text.Bold(pkgS[i].Name()), text.Cyan(text.Human(pkgS[i].ISize())))
	}
	// Could implement size here as well, but we just want the general idea
}

// localStatistics prints installed packages statistics.
func localStatistics(dbExecutor db.Executor) error {
	info := statistics(dbExecutor)

	_, remoteNames, err := query.GetPackageNamesBySource(dbExecutor)
	if err != nil {
		return err
	}

	text.Infoln(gotext.Get("Yay version v%s", yayVersion))
	fmt.Println(text.Bold(text.Cyan("===========================================")))
	text.Infoln(gotext.Get("Total installed packages: %s", text.Cyan(strconv.Itoa(info.Totaln))))
	text.Infoln(gotext.Get("Total foreign installed packages: %s", text.Cyan(strconv.Itoa(len(remoteNames)))))
	text.Infoln(gotext.Get("Explicitly installed packages: %s", text.Cyan(strconv.Itoa(info.Expln))))
	text.Infoln(gotext.Get("Total Size occupied by packages: %s", text.Cyan(text.Human(info.TotalSize))))
	fmt.Println(text.Bold(text.Cyan("===========================================")))
	text.Infoln(gotext.Get("Ten biggest packages:"))
	biggestPackages(dbExecutor)
	fmt.Println(text.Bold(text.Cyan("===========================================")))

	query.AURInfoPrint(remoteNames, config.RequestSplitN)

	return nil
}

// TODO: Make it less hacky
func printNumberOfUpdates(dbExecutor db.Executor, enableDowngrade bool) error {
	warnings := query.NewWarnings()
	old := os.Stdout // keep backup of the real stdout
	os.Stdout = nil
	aurUp, repoUp, err := upList(warnings, dbExecutor, enableDowngrade)
	os.Stdout = old // restoring the real stdout
	if err != nil {
		return err
	}
	fmt.Println(len(aurUp) + len(repoUp))

	return nil
}

// TODO: Make it less hacky
func printUpdateList(cmdArgs *settings.Arguments, dbExecutor db.Executor, enableDowngrade bool) error {
	targets := stringset.FromSlice(cmdArgs.Targets)
	warnings := query.NewWarnings()
	old := os.Stdout // keep backup of the real stdout
	os.Stdout = nil
	localNames, remoteNames, err := query.GetPackageNamesBySource(dbExecutor)
	if err != nil {
		return err
	}

	aurUp, repoUp, err := upList(warnings, dbExecutor, enableDowngrade)
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

		text.Errorln(gotext.Get("package '%s' was not found", pkg))
		missing = true
	}

	if missing {
		return fmt.Errorf("")
	}

	return nil
}

func printPkgbuilds(dbExecutor db.Executor, pkgS []string) error {
	var pkgbuilds []string
	var localPkgbuilds []string
	missing := false
	pkgS = query.RemoveInvalidTargets(pkgS, config.Runtime.Mode)
	aurS, repoS := packageSlices(pkgS, dbExecutor)
	var err error
	var errs multierror.MultiError

	if len(aurS) != 0 {
		noDB := make([]string, 0, len(aurS))
		for _, pkg := range aurS {
			_, name := text.SplitDBFromName(pkg)
			noDB = append(noDB, name)
		}
		localPkgbuilds, err = aurPkgbuilds(noDB)
		pkgbuilds = append(pkgbuilds, localPkgbuilds...)
		errs.Add(err)
	}

	if len(repoS) != 0 {
		localPkgbuilds, err = repoPkgbuilds(dbExecutor, repoS)
		pkgbuilds = append(pkgbuilds, localPkgbuilds...)
		errs.Add(err)
	}

	if len(aurS) != len(pkgbuilds) {
		missing = true
	}

	if len(pkgbuilds) != 0 {
		for _, pkgbuild := range pkgbuilds {
			fmt.Print(pkgbuild)
		}
	}

	if missing {
		err = fmt.Errorf("")
	}

	return err
}
