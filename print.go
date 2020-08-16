package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/leonelquinteros/gotext"
	rpc "github.com/mikkeloscar/aur"

	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/stringset"
	"github.com/Jguer/yay/v10/pkg/text"
)

// PrintSearch handles printing search results in a given format
func (q aurQuery) printSearch(start int, dbExecutor db.Executor) {
	for i := range q {
		var toprint string
		if config.SearchMode == numberMenu {
			switch config.SortMode {
			case settings.TopDown:
				toprint += magenta(strconv.Itoa(start+i) + " ")
			case settings.BottomUp:
				toprint += magenta(strconv.Itoa(len(q)+start-i-1) + " ")
			default:
				text.Warnln(gotext.Get("invalid sort mode. Fix with yay -Y --bottomup --save"))
			}
		} else if config.SearchMode == minimal {
			fmt.Println(q[i].Name)
			continue
		}

		toprint += bold(text.ColorHash("aur")) + "/" + bold(q[i].Name) +
			" " + cyan(q[i].Version) +
			bold(" (+"+strconv.Itoa(q[i].NumVotes)) +
			" " + bold(strconv.FormatFloat(q[i].Popularity, 'f', 2, 64)+") ")

		if q[i].Maintainer == "" {
			toprint += bold(red(gotext.Get("(Orphaned)"))) + " "
		}

		if q[i].OutOfDate != 0 {
			toprint += bold(red(gotext.Get("(Out-of-date: %s)", text.FormatTime(q[i].OutOfDate)))) + " "
		}

		if pkg := dbExecutor.LocalPackage(q[i].Name); pkg != nil {
			if pkg.Version() != q[i].Version {
				toprint += bold(green(gotext.Get("(Installed: %s)", pkg.Version())))
			} else {
				toprint += bold(green(gotext.Get("(Installed)")))
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
				toprint += magenta(strconv.Itoa(i+1) + " ")
			case settings.BottomUp:
				toprint += magenta(strconv.Itoa(len(s)-i) + " ")
			default:
				text.Warnln(gotext.Get("invalid sort mode. Fix with yay -Y --bottomup --save"))
			}
		} else if config.SearchMode == minimal {
			fmt.Println(res.Name())
			continue
		}

		toprint += bold(text.ColorHash(res.DB().Name())) + "/" + bold(res.Name()) +
			" " + cyan(res.Version()) +
			bold(" ("+text.Human(res.Size())+
				" "+text.Human(res.ISize())+") ")

		packageGroups := dbExecutor.PackageGroups(res)
		if len(packageGroups) != 0 {
			toprint += fmt.Sprint(packageGroups, " ")
		}

		if pkg := dbExecutor.LocalPackage(res.Name()); pkg != nil {
			if pkg.Version() != res.Version() {
				toprint += bold(green(gotext.Get("(Installed: %s)", pkg.Version())))
			} else {
				toprint += bold(green(gotext.Get("(Installed)")))
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
	text.PrintInfoValue(gotext.Get("Keywords"), strings.Join(a.Keywords, "  "))
	text.PrintInfoValue(gotext.Get("Version"), a.Version)
	text.PrintInfoValue(gotext.Get("Description"), a.Description)
	text.PrintInfoValue(gotext.Get("URL"), a.URL)
	text.PrintInfoValue(gotext.Get("AUR URL"), config.AURURL+"/packages/"+a.Name)
	text.PrintInfoValue(gotext.Get("Groups"), strings.Join(a.Groups, "  "))
	text.PrintInfoValue(gotext.Get("Licenses"), strings.Join(a.License, "  "))
	text.PrintInfoValue(gotext.Get("Provides"), strings.Join(a.Provides, "  "))
	text.PrintInfoValue(gotext.Get("Depends On"), strings.Join(a.Depends, "  "))
	text.PrintInfoValue(gotext.Get("Make Deps"), strings.Join(a.MakeDepends, "  "))
	text.PrintInfoValue(gotext.Get("Check Deps"), strings.Join(a.CheckDepends, "  "))
	text.PrintInfoValue(gotext.Get("Optional Deps"), strings.Join(a.OptDepends, "  "))
	text.PrintInfoValue(gotext.Get("Conflicts With"), strings.Join(a.Conflicts, "  "))
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
		fmt.Printf("%s: %s\n", bold(pkgS[i].Name()), cyan(text.Human(pkgS[i].ISize())))
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
	fmt.Println(bold(cyan("===========================================")))
	text.Infoln(gotext.Get("Total installed packages: %s", cyan(strconv.Itoa(info.Totaln))))
	text.Infoln(gotext.Get("Total foreign installed packages: %s", cyan(strconv.Itoa(len(remoteNames)))))
	text.Infoln(gotext.Get("Explicitly installed packages: %s", cyan(strconv.Itoa(info.Expln))))
	text.Infoln(gotext.Get("Total Size occupied by packages: %s", cyan(text.Human(info.TotalSize))))
	fmt.Println(bold(cyan("===========================================")))
	text.Infoln(gotext.Get("Ten biggest packages:"))
	biggestPackages(dbExecutor)
	fmt.Println(bold(cyan("===========================================")))

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
					fmt.Printf("%s %s -> %s\n", bold(pkg.Name), green(pkg.LocalVersion), green(pkg.RemoteVersion))
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
					fmt.Printf("%s %s -> %s\n", bold(pkg.Name), green(pkg.LocalVersion), green(pkg.RemoteVersion))
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

const (
	redCode     = "\x1b[31m"
	greenCode   = "\x1b[32m"
	blueCode    = "\x1b[34m"
	magentaCode = "\x1b[35m"
	cyanCode    = "\x1b[36m"
	boldCode    = "\x1b[1m"

	resetCode = "\x1b[0m"
)

func stylize(startCode, in string) string {
	if text.UseColor {
		return startCode + in + resetCode
	}

	return in
}

func red(in string) string {
	return stylize(redCode, in)
}

func green(in string) string {
	return stylize(greenCode, in)
}

func blue(in string) string {
	return stylize(blueCode, in)
}

func cyan(in string) string {
	return stylize(cyanCode, in)
}

func magenta(in string) string {
	return stylize(magentaCode, in)
}

func bold(in string) string {
	return stylize(boldCode, in)
}
