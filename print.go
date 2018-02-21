package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	alpm "github.com/jguer/go-alpm"
	rpc "github.com/mikkeloscar/aur"
)

const arrow = "==>"

// Human returns results in Human readable format.
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
				toprint += yellowFg(strconv.Itoa(len(q)+start-i-1) + " ")
			} else {
				toprint += yellowFg(strconv.Itoa(start+i) + " ")
			}
		} else if config.SearchMode == Minimal {
			fmt.Println(res.Name)
			continue
		}
		toprint += boldWhiteFg("aur/") + boldYellowFg(res.Name) +
			" " + boldCyanFg(res.Version) +
			" (" + strconv.Itoa(res.NumVotes) + ") "

		if res.Maintainer == "" {
			toprint += redFgBlackBg("(Orphaned)") + " "
		}

		if res.OutOfDate != 0 {
			toprint += redFgBlackBg("(Out-of-date)") + " "
		}

		if _, err := localDb.PkgByName(res.Name); err == nil {
			toprint += greenFgBlackBg("Installed")
		}
		toprint += "\n    " + res.Description
		fmt.Println(toprint)
	}
}

//PrintSearch receives a RepoSearch type and outputs pretty text.
func (s repoQuery) printSearch() {
	for i, res := range s {
		var toprint string
		if config.SearchMode == NumberMenu {
			if config.SortMode == BottomUp {
				toprint += yellowFg(strconv.Itoa(len(s)-i) + " ")
			} else {
				toprint += yellowFg(strconv.Itoa(i+1) + " ")
			}
		} else if config.SearchMode == Minimal {
			fmt.Println(res.Name())
			continue
		}
		toprint += boldWhiteFg(res.DB().Name()+"/") + boldYellowFg(res.Name()) +
			" " + boldCyanFg(res.Version()) + " "

		if len(res.Groups().Slice()) != 0 {
			toprint += fmt.Sprint(res.Groups().Slice(), " ")
		}

		localDb, err := alpmHandle.LocalDb()
		if err == nil {
			if _, err = localDb.PkgByName(res.Name()); err == nil {
				toprint += greenFgBlackBg("Installed")
			}
		}

		toprint += "\n    " + res.Description()
		fmt.Println(toprint)
	}
}

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
			repoMake += "  " + pkg.Name()
			repoMakeLen++
		} else {
			repo += "  " + pkg.Name()
			repoLen++
		}
	}

	for _, pkg := range dc.Aur {
		pkgStr := "  " + pkg.PackageBase
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

	repoInfo := boldBlueFg(
		"[" + repoName + ": " + strconv.Itoa(length) + "]")
	fmt.Println(repoInfo + yellowFg(packages))
}

func printDeps(repoDeps []string, aurDeps []string) {
	if len(repoDeps) != 0 {
		fmt.Print(boldGreenFg(arrow + " Repository dependencies: "))
		for _, repoD := range repoDeps {
			fmt.Print(yellowFg(repoD) + " ")
		}
		fmt.Print("\n")

	}
	if len(aurDeps) != 0 {
		fmt.Print(boldGreenFg(arrow + " AUR dependencies: "))
		for _, aurD := range aurDeps {
			fmt.Print(yellowFg(aurD) + " ")
		}
		fmt.Print("\n")
	}
}

// PrintInfo prints package info like pacman -Si.
func PrintInfo(a *rpc.Pkg) {
	fmt.Println(boldWhiteFg("Repository      :"), "aur")
	fmt.Println(boldWhiteFg("Name            :"), a.Name)
	fmt.Println(boldWhiteFg("Version         :"), a.Version)
	fmt.Println(boldWhiteFg("Description     :"), a.Description)
	fmt.Println(boldWhiteFg("URL             :"), a.URL)
	fmt.Println(boldWhiteFg("Licenses        :"), strings.Join(a.License, "  "))
	fmt.Println(boldWhiteFg("Depends On      :"), strings.Join(a.Depends, "  "))
	fmt.Println(boldWhiteFg("Make Deps       :"), strings.Join(a.MakeDepends, "  "))
	fmt.Println(boldWhiteFg("Check Deps      :"), strings.Join(a.CheckDepends, "  "))
	fmt.Println(boldWhiteFg("Optional Deps   :"), strings.Join(a.OptDepends, "  "))
	fmt.Println(boldWhiteFg("Conflicts With  :"), strings.Join(a.Conflicts, "  "))
	fmt.Println(boldWhiteFg("Maintainer      :"), a.Maintainer)
	fmt.Println(boldWhiteFg("Votes           :"), a.NumVotes)
	fmt.Println(boldWhiteFg("Popularity      :"), a.Popularity)
	if a.OutOfDate != 0 {
		fmt.Println(boldWhiteFg("Out-of-date     :"), "Yes")
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
		fmt.Println(pkgS[i].Name() + ": " + yellowFg(human(pkgS[i].ISize())))
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

	fmt.Printf("\n Yay version r%s\n", version)
	fmt.Println(boldCyanFg("==========================================="))
	fmt.Println(boldGreenFg("Total installed packages: ") + yellowFg(strconv.Itoa(info.Totaln)))
	fmt.Println(boldGreenFg("Total foreign installed packages: ") + yellowFg(strconv.Itoa(len(remoteNames))))
	fmt.Println(boldGreenFg("Explicitly installed packages: ") + yellowFg(strconv.Itoa(info.Expln)))
	fmt.Println(boldGreenFg("Total Size occupied by packages: ") + yellowFg(human(info.TotalSize)))
	fmt.Println(boldCyanFg("==========================================="))
	fmt.Println(boldGreenFg("Ten biggest packages"))
	biggestPackages()
	fmt.Println(boldCyanFg("==========================================="))

	aurInfo(remoteNames)

	return nil
}

//todo make pretty
func printMissing(missing stringSet) {
	fmt.Print("Packages not found in repos or aur:")
	for pkg := range missing {
		fmt.Print(" ", pkg)
	}
	fmt.Println()
}

//todo make it less hacky
func printNumberOfUpdates() error {
	//todo
	old := os.Stdout // keep backup of the real stdout
	os.Stdout = nil
	_, _, localNames, remoteNames, err := filterPackages()
	dt, _ := getDepTree(append(localNames, remoteNames...))
	aurUp, repoUp, err := upList(dt)
	os.Stdout = old // restoring the real stdout
	if err != nil {
		return err
	}
	fmt.Println(len(aurUp) + len(repoUp))

	return nil
}

//todo make it less hacky
func printUpdateList() error {
	old := os.Stdout // keep backup of the real stdout
	os.Stdout = nil
	_, _, localNames, remoteNames, err := filterPackages()
	dt, _ := getDepTree(append(localNames, remoteNames...))
	aurUp, repoUp, err := upList(dt)

	os.Stdout = old // restoring the real stdout
	if err != nil {
		return err
	}
	for _, pkg := range repoUp {
		fmt.Println(pkg.Name)
	}

	for _, pkg := range aurUp {
		fmt.Println(pkg.Name)
	}

	return nil
}

func blackBg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[0;;40m" + in + "\x1b[0m"
	}

	return in
}

func redFg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[0;31m" + in + "\x1b[0m"
	}

	return in
}

func greenFg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[0;32m" + in + "\x1b[0m"
	}

	return in
}

func yellowFg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[0;33m" + in + "\x1b[0m"
	}

	return in
}

func boldFg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[1m" + in + "\x1b[0m"
	}

	return in
}
func boldGreenFg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[1;32m" + in + "\x1b[0m"
	}

	return in
}

func boldYellowFg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[1;33m" + in + "\x1b[0m"
	}

	return in
}

func boldBlueFg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[1;34m" + in + "\x1b[0m"
	}

	return in
}

func boldCyanFg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[1;36m" + in + "\x1b[0m"
	}

	return in
}

func boldWhiteFg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[1;37m" + in + "\x1b[0m"
	}

	return in
}

func redFgBlackBg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[0;31;40m" + in + "\x1b[0m"
	}

	return in
}

func greenFgBlackBg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[0;32;40m" + in + "\x1b[0m"
	}

	return in
}

func whiteFgBlackBg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[0;37;40m" + in + "\x1b[0m"
	}

	return in
}

func boldRedFgBlackBg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[1;31;40m" + in + "\x1b[0m"
	}

	return in
}

func boldYellowFgBlackBg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[1;33;40m" + in + "\x1b[0m"
	}

	return in
}
