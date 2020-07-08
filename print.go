package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/leonelquinteros/gotext"
	rpc "github.com/mikkeloscar/aur"

	"github.com/Jguer/go-alpm"

	"github.com/Jguer/yay/v10/pkg/intrange"
	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/stringset"
	"github.com/Jguer/yay/v10/pkg/text"
)

const arrow = "==>"
const smallArrow = " ->"

func (warnings *aurWarnings) print() {
	if len(warnings.Missing) > 0 {
		text.Warn(gotext.Get("Missing AUR Packages:"))
		for _, name := range warnings.Missing {
			fmt.Print("  " + cyan(name))
		}
		fmt.Println()
	}

	if len(warnings.Orphans) > 0 {
		text.Warn(gotext.Get("Orphaned AUR Packages:"))
		for _, name := range warnings.Orphans {
			fmt.Print("  " + cyan(name))
		}
		fmt.Println()
	}

	if len(warnings.OutOfDate) > 0 {
		text.Warn(gotext.Get("Flagged Out Of Date AUR Packages:"))
		for _, name := range warnings.OutOfDate {
			fmt.Print("  " + cyan(name))
		}
		fmt.Println()
	}
}

// PrintSearch handles printing search results in a given format
func (q aurQuery) printSearch(start int, alpmHandle *alpm.Handle) {
	localDB, _ := alpmHandle.LocalDB()

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

		if pkg := localDB.Pkg(q[i].Name); pkg != nil {
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
func (s repoQuery) printSearch(alpmHandle *alpm.Handle) {
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

		if len(res.Groups().Slice()) != 0 {
			toprint += fmt.Sprint(res.Groups().Slice(), " ")
		}

		localDB, err := alpmHandle.LocalDB()
		if err == nil {
			if pkg := localDB.Pkg(res.Name()); pkg != nil {
				if pkg.Version() != res.Version() {
					toprint += bold(green(gotext.Get("(Installed: %s)", pkg.Version())))
				} else {
					toprint += bold(green(gotext.Get("(Installed)")))
				}
			}
		}

		toprint += "\n    " + res.Description()
		fmt.Println(toprint)
	}
}

// Pretty print a set of packages from the same package base.
// Packages foo and bar from a pkgbase named base would print like so:
// base (foo bar)
func (b Base) String() string {
	pkg := b[0]
	str := pkg.PackageBase
	if len(b) > 1 || pkg.PackageBase != pkg.Name {
		str2 := " ("
		for _, split := range b {
			str2 += split.Name + " "
		}
		str2 = str2[:len(str2)-1] + ")"

		str += str2
	}

	return str
}

func (u *upgrade) StylizedNameWithRepository() string {
	return bold(text.ColorHash(u.Repository)) + "/" + bold(u.Name)
}

// Print prints the details of the packages to upgrade.
func (u upSlice) print() {
	longestName, longestVersion := 0, 0
	for _, pack := range u {
		packNameLen := len(pack.StylizedNameWithRepository())
		packVersion, _ := getVersionDiff(pack.LocalVersion, pack.RemoteVersion)
		packVersionLen := len(packVersion)
		longestName = intrange.Max(packNameLen, longestName)
		longestVersion = intrange.Max(packVersionLen, longestVersion)
	}

	namePadding := fmt.Sprintf("%%-%ds  ", longestName)
	versionPadding := fmt.Sprintf("%%-%ds", longestVersion)
	numberPadding := fmt.Sprintf("%%%dd  ", len(fmt.Sprintf("%v", len(u))))

	for k, i := range u {
		left, right := getVersionDiff(i.LocalVersion, i.RemoteVersion)

		fmt.Print(magenta(fmt.Sprintf(numberPadding, len(u)-k)))

		fmt.Printf(namePadding, i.StylizedNameWithRepository())

		fmt.Printf("%s -> %s\n", fmt.Sprintf(versionPadding, left), right)
	}
}

// Print prints repository packages to be downloaded
func (do *depOrder) Print() {
	repo := ""
	repoMake := ""
	aur := ""
	aurMake := ""

	repoLen := 0
	repoMakeLen := 0
	aurLen := 0
	aurMakeLen := 0

	for _, pkg := range do.Repo {
		if do.Runtime.Get(pkg.Name()) {
			repo += "  " + pkg.Name() + "-" + pkg.Version()
			repoLen++
		} else {
			repoMake += "  " + pkg.Name() + "-" + pkg.Version()
			repoMakeLen++
		}
	}

	for _, base := range do.Aur {
		pkg := base.Pkgbase()
		pkgStr := "  " + pkg + "-" + base[0].Version
		pkgStrMake := pkgStr

		push := false
		pushMake := false

		switch {
		case len(base) > 1, pkg != base[0].Name:
			pkgStr += " ("
			pkgStrMake += " ("

			for _, split := range base {
				if do.Runtime.Get(split.Name) {
					pkgStr += split.Name + " "
					aurLen++
					push = true
				} else {
					pkgStrMake += split.Name + " "
					aurMakeLen++
					pushMake = true
				}
			}

			pkgStr = pkgStr[:len(pkgStr)-1] + ")"
			pkgStrMake = pkgStrMake[:len(pkgStrMake)-1] + ")"
		case do.Runtime.Get(base[0].Name):
			aurLen++
			push = true
		default:
			aurMakeLen++
			pushMake = true
		}

		if push {
			aur += pkgStr
		}
		if pushMake {
			aurMake += pkgStrMake
		}
	}

	printDownloads("Repo", repoLen, repo)
	printDownloads("Repo Make", repoMakeLen, repoMake)
	printDownloads("Aur", aurLen, aur)
	printDownloads("Aur Make", aurMakeLen, aurMake)
}

func printDownloads(repoName string, length int, packages string) {
	if length < 1 {
		return
	}

	repoInfo := bold(blue(
		"[" + repoName + ": " + strconv.Itoa(length) + "]"))
	fmt.Println(repoInfo + cyan(packages))
}

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
		fmt.Printf("%s: %s\n", bold(pkgS[i].Name()), cyan(text.Human(pkgS[i].ISize())))
	}
	// Could implement size here as well, but we just want the general idea
}

// localStatistics prints installed packages statistics.
func localStatistics(alpmHandle *alpm.Handle) error {
	info, err := statistics(alpmHandle)
	if err != nil {
		return err
	}

	_, _, _, remoteNames, err := query.FilterPackages(alpmHandle)
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
	biggestPackages(alpmHandle)
	fmt.Println(bold(cyan("===========================================")))

	aurInfoPrint(remoteNames)

	return nil
}

//TODO: Make it less hacky
func printNumberOfUpdates(alpmHandle *alpm.Handle, enableDowngrade bool) error {
	warnings := makeWarnings()
	old := os.Stdout // keep backup of the real stdout
	os.Stdout = nil
	aurUp, repoUp, err := upList(warnings, alpmHandle, enableDowngrade)
	os.Stdout = old // restoring the real stdout
	if err != nil {
		return err
	}
	fmt.Println(len(aurUp) + len(repoUp))

	return nil
}

//TODO: Make it less hacky
func printUpdateList(cmdArgs *settings.Arguments, alpmHandle *alpm.Handle, enableDowngrade bool) error {
	targets := stringset.FromSlice(cmdArgs.Targets)
	warnings := makeWarnings()
	old := os.Stdout // keep backup of the real stdout
	os.Stdout = nil
	_, _, localNames, remoteNames, err := query.FilterPackages(alpmHandle)
	if err != nil {
		return err
	}

	aurUp, repoUp, err := upList(warnings, alpmHandle, enableDowngrade)
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
	yellowCode  = "\x1b[33m"
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

func yellow(in string) string {
	return stylize(yellowCode, in)
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

func providerMenu(dep string, providers providers) *rpc.Pkg {
	size := providers.Len()

	str := bold(gotext.Get("There are %d providers available for %s:", size, dep))

	size = 1
	str += bold(cyan("\n:: ")) + bold(gotext.Get("Repository AUR")) + "\n    "

	for _, pkg := range providers.Pkgs {
		str += fmt.Sprintf("%d) %s ", size, pkg.Name)
		size++
	}

	text.OperationInfoln(str)

	for {
		fmt.Print(gotext.Get("\nEnter a number (default=1): "))

		if config.NoConfirm {
			fmt.Println("1")
			return providers.Pkgs[0]
		}

		reader := bufio.NewReader(os.Stdin)
		numberBuf, overflow, err := reader.ReadLine()

		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			break
		}

		if overflow {
			text.Errorln(gotext.Get("input too long"))
			continue
		}

		if string(numberBuf) == "" {
			return providers.Pkgs[0]
		}

		num, err := strconv.Atoi(string(numberBuf))
		if err != nil {
			text.Errorln(gotext.Get("invalid number: %s", string(numberBuf)))
			continue
		}

		if num < 1 || num >= size {
			text.Errorln(gotext.Get("invalid value: %d is not between %d and %d", num, 1, size-1))
			continue
		}

		return providers.Pkgs[num-1]
	}

	return nil
}
