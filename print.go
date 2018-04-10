package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	rpc "github.com/mikkeloscar/aur"
)

const arrow = "==>"

// human method returns results in human readable format.
func human(size int64) string {
	floatsize := float32(size)
	units := [...]string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi", "Yi"}
	for _, unit := range units {
		if floatsize < 1024 {
			return fmt.Sprintf("%.1f %sB", floatsize, unit)
		}
		floatsize /= 1024
	}
	return fmt.Sprintf("%d%s", size, "B")
}

// PrintSearch handles printing search results in a given format
func (q aurQuery) printSearch(start int) {
	localDb, _ := alpmHandle.LocalDb()

	for i, res := range q {
		var toprint string
		if config.SearchMode == NumberMenu {
			if config.SortMode == BottomUp {
				toprint += magenta(strconv.Itoa(len(q)+start-i-1) + " ")
			} else {
				toprint += magenta(strconv.Itoa(start+i) + " ")
			}
		} else if config.SearchMode == Minimal {
			fmt.Println(res.Name)
			continue
		}

		toprint += bold(colourHash("aur")) + "/" + bold(res.Name) +
			" " + cyan(res.Version) +
			bold(" (+"+strconv.Itoa(res.NumVotes)) +
			" " + bold(strconv.FormatFloat(res.Popularity, 'f', 2, 64)+"%) ")

		if res.Maintainer == "" {
			toprint += bold(red("(Orphaned)")) + " "
		}

		if res.OutOfDate != 0 {
			toprint += bold(red("(Out-of-date "+formatTime(res.OutOfDate)+")")) + " "
		}

		if pkg, err := localDb.PkgByName(res.Name); err == nil {
			if pkg.Version() != res.Version {
				toprint += bold(green("(Installed: " + pkg.Version() + ")"))
			} else {
				toprint += bold(green("(Installed)"))
			}
		}
		toprint += "\n    " + res.Description
		fmt.Println(toprint)
	}
}

// PrintSearch receives a RepoSearch type and outputs pretty text.
func (s repoQuery) printSearch() {
	for i, res := range s {
		var toprint string
		if config.SearchMode == NumberMenu {
			if config.SortMode == BottomUp {
				toprint += magenta(strconv.Itoa(len(s)-i) + " ")
			} else {
				toprint += magenta(strconv.Itoa(i+1) + " ")
			}
		} else if config.SearchMode == Minimal {
			fmt.Println(res.Name())
			continue
		}

		toprint += bold(colourHash(res.DB().Name())) + "/" + bold(res.Name()) +
			" " + cyan(res.Version()) +
			bold(" ("+human(res.Size())+
				" "+human(res.ISize())+") ")

		if len(res.Groups().Slice()) != 0 {
			toprint += fmt.Sprint(res.Groups().Slice(), " ")
		}

		localDb, err := alpmHandle.LocalDb()
		if err == nil {
			if pkg, err := localDb.PkgByName(res.Name()); err == nil {
				if pkg.Version() != res.Version() {
					toprint += bold(green("(Installed: " + pkg.Version() + ")"))
				} else {
					toprint += bold(green("(Installed)"))
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
func formatPkgbase(pkg *rpc.Pkg, bases map[string][]*rpc.Pkg) string {
	str := pkg.PackageBase
	if len(bases[pkg.PackageBase]) > 1 || pkg.PackageBase != pkg.Name {
		str2 := " ("
		for _, split := range bases[pkg.PackageBase] {
			str2 += split.Name + " "
		}
		str2 = str2[:len(str2)-1] + ")"

		str += str2
	}

	return str
}

// Print prints the details of the packages to upgrade.
func (u upSlice) Print(start int) {
	for k, i := range u {
		left, right := getVersionDiff(i.LocalVersion, i.RemoteVersion)

		fmt.Print(magenta(fmt.Sprintf("%3d ", len(u)+start-k-1)))
		fmt.Print(bold(colourHash(i.Repository)), "/", bold(i.Name))

		w := 70 - len(i.Repository) - len(i.Name)
		padding := fmt.Sprintf("%%%ds", w)
		fmt.Printf(padding, left)
		fmt.Printf(" -> %s\n", right)
	}
}

// printDownloadsFromRepo prints repository packages to be downloaded
func printDepCatagories(dc *depCatagories) {
	repo := ""
	repoMake := ""
	aur := ""
	aurMake := ""

	repoLen := 0
	repoMakeLen := 0
	aurLen := 0
	aurMakeLen := 0

	for _, pkg := range dc.Repo {
		if dc.MakeOnly.get(pkg.Name()) {
			repoMake += "  " + pkg.Name() + "-" + pkg.Version()
			repoMakeLen++
		} else {
			repo += "  " + pkg.Name() + "-" + pkg.Version()
			repoLen++
		}
	}

	for _, pkg := range dc.Aur {
		pkgStr := "  " + pkg.PackageBase + "-" + pkg.Version
		pkgStrMake := pkgStr

		push := false
		pushMake := false

		if len(dc.Bases[pkg.PackageBase]) > 1 || pkg.PackageBase != pkg.Name {
			pkgStr += " ("
			pkgStrMake += " ("

			for _, split := range dc.Bases[pkg.PackageBase] {
				if dc.MakeOnly.get(split.Name) {
					pkgStrMake += split.Name + " "
					aurMakeLen++
					pushMake = true
				} else {
					pkgStr += split.Name + " "
					aurLen++
					push = true
				}
			}

			pkgStr = pkgStr[:len(pkgStr)-1] + ")"
			pkgStrMake = pkgStrMake[:len(pkgStrMake)-1] + ")"
		} else if dc.MakeOnly.get(pkg.Name) {
			aurMakeLen++
			pushMake = true
		} else {
			aurLen++
			push = true
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
func PrintInfo(a *rpc.Pkg) {
	fmt.Println(bold("Repository      :"), "aur")
	fmt.Println(bold("Name            :"), a.Name)
	fmt.Println(bold("Version         :"), a.Version)
	fmt.Println(bold("Description     :"), a.Description)
	fmt.Println(bold("URL             :"), a.URL)
	fmt.Println(bold("Licenses        :"), strings.Join(a.License, "  "))
	fmt.Println(bold("Provides        :"), strings.Join(a.Provides, "  "))
	fmt.Println(bold("Depends On      :"), strings.Join(a.Depends, "  "))
	fmt.Println(bold("Make Deps       :"), strings.Join(a.MakeDepends, "  "))
	fmt.Println(bold("Check Deps      :"), strings.Join(a.CheckDepends, "  "))
	fmt.Println(bold("Optional Deps   :"), strings.Join(a.OptDepends, "  "))
	fmt.Println(bold("Conflicts With  :"), strings.Join(a.Conflicts, "  "))
	fmt.Println(bold("Maintainer      :"), a.Maintainer)
	fmt.Println(bold("Votes           :"), a.NumVotes)
	fmt.Println(bold("Popularity      :"), a.Popularity)
	if a.OutOfDate != 0 {
		fmt.Println(bold("Out-of-date     :"), "Yes", "["+formatTime(a.OutOfDate)+"]")
	}

	fmt.Println()
}

// BiggestPackages prints the name of the ten biggest packages in the system.
func biggestPackages() {
	localDb, err := alpmHandle.LocalDb()
	if err != nil {
		return
	}

	pkgCache := localDb.PkgCache()
	pkgS := pkgCache.SortBySize().Slice()

	if len(pkgS) < 10 {
		return
	}

	for i := 0; i < 10; i++ {
		fmt.Println(bold(pkgS[i].Name()) + ": " + cyan(human(pkgS[i].ISize())))
	}
	// Could implement size here as well, but we just want the general idea
}

// localStatistics prints installed packages statistics.
func localStatistics() error {
	info, err := statistics()
	if err != nil {
		return err
	}

	_, _, _, remoteNames, err := filterPackages()
	if err != nil {
		return err
	}

	fmt.Printf(bold("Yay version v%s\n"), version)
	fmt.Println(bold(cyan("===========================================")))
	fmt.Println(bold(green("Total installed packages: ")) + cyan(strconv.Itoa(info.Totaln)))
	fmt.Println(bold(green("Total foreign installed packages: ")) + cyan(strconv.Itoa(len(remoteNames))))
	fmt.Println(bold(green("Explicitly installed packages: ")) + cyan(strconv.Itoa(info.Expln)))
	fmt.Println(bold(green("Total Size occupied by packages: ")) + cyan(human(info.TotalSize)))
	fmt.Println(bold(cyan("===========================================")))
	fmt.Println(bold(green("Ten biggest packages:")))
	biggestPackages()
	fmt.Println(bold(cyan("===========================================")))

	aurInfo(remoteNames)

	return nil
}

//TODO: Make it less hacky
func printNumberOfUpdates() error {
	//todo
	old := os.Stdout // keep backup of the real stdout
	os.Stdout = nil
	aurUp, repoUp, err := upList()
	os.Stdout = old // restoring the real stdout
	if err != nil {
		return err
	}
	fmt.Println(len(aurUp) + len(repoUp))

	return nil
}

//TODO: Make it less hacky
func printUpdateList(parser *arguments) error {
	old := os.Stdout // keep backup of the real stdout
	os.Stdout = nil
	_, _, localNames, remoteNames, err := filterPackages()
	aurUp, repoUp, err := upList()
	os.Stdout = old // restoring the real stdout
	if err != nil {
		return err
	}

	noTargets := len(parser.targets) == 0

	if !parser.existsArg("m", "foreigne") {
		for _, pkg := range repoUp {
			if noTargets || parser.targets.get(pkg.Name) {
				fmt.Printf("%s %s -> %s\n", bold(pkg.Name), green(pkg.LocalVersion), green(pkg.RemoteVersion))
				delete(parser.targets, pkg.Name)
			}
		}
	}

	if !parser.existsArg("n", "native") {
		for _, pkg := range aurUp {
			if noTargets || parser.targets.get(pkg.Name) {
				fmt.Printf("%s %s -> %s\n", bold(pkg.Name), green(pkg.LocalVersion), green(pkg.RemoteVersion))
				delete(parser.targets, pkg.Name)
			}
		}
	}

	missing := false

outer:
	for pkg := range parser.targets {
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

		fmt.Println(red(bold("error:")), "package '"+pkg+"' was not found")
		missing = true
	}

	if missing {
		return fmt.Errorf("")
	}

	return nil
}

// Formats a unix timestamp to yyyy/mm/dd
func formatTime(i int) string {
	t := time.Unix(int64(i), 0)
	return fmt.Sprintf("%d/%02d/%02d", t.Year(), int(t.Month()), t.Day())
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
	if useColor {
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

// Colours text using a hashing algorithm. The same text will always produce the
// same colour while different text will produce a different colour.
func colourHash(name string) (output string) {
	if !useColor {
		return name
	}
	var hash = 5381
	for i := 0; i < len(name); i++ {
		hash = int(name[i]) + ((hash << 5) + (hash))
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", hash%6+31, name)
}
