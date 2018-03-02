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
				toprint += yellow(strconv.Itoa(len(q)+start-i-1) + " ")
			} else {
				toprint += yellow(strconv.Itoa(start+i) + " ")
			}
		} else if config.SearchMode == Minimal {
			fmt.Println(res.Name)
			continue
		}
		toprint += bold(colourHash("aur")) + "/" + bold(yellow(res.Name)) +
			" " + bold(cyan(res.Version)) +
			" (" + strconv.Itoa(res.NumVotes) + ") "

		if res.Maintainer == "" {
			toprint += red(blackBg("(Orphaned)")) + " "
		}

		if res.OutOfDate != 0 {
			toprint += red(blackBg("(Out-of-date)")) + " "
		}

		if _, err := localDb.PkgByName(res.Name); err == nil {
			toprint += green(blackBg("Installed"))
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
				toprint += yellow(strconv.Itoa(len(s)-i) + " ")
			} else {
				toprint += yellow(strconv.Itoa(i+1) + " ")
			}
		} else if config.SearchMode == Minimal {
			fmt.Println(res.Name())
			continue
		}
		toprint += colourHash(res.DB().Name()) + "/" + bold(yellow(res.Name())) +
			" " + bold(cyan(res.Version())) + " "

		if len(res.Groups().Slice()) != 0 {
			toprint += fmt.Sprint(res.Groups().Slice(), " ")
		}

		localDb, err := alpmHandle.LocalDb()
		if err == nil {
			if _, err = localDb.PkgByName(res.Name()); err == nil {
				toprint += green(blackBg("Installed"))
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

	repoInfo := bold(blue(
		"[" + repoName + ": " + strconv.Itoa(length) + "]"))
	fmt.Println(repoInfo + yellow(packages))
}

// PrintInfo prints package info like pacman -Si.
func PrintInfo(a *rpc.Pkg) {
	fmt.Println(bold(white("Repository      :")), "aur")
	fmt.Println(bold(white("Name            :")), a.Name)
	fmt.Println(bold(white("Version         :")), a.Version)
	fmt.Println(bold(white("Description     :")), a.Description)
	fmt.Println(bold(white("URL             :")), a.URL)
	fmt.Println(bold(white("Licenses        :")), strings.Join(a.License, "  "))
	fmt.Println(bold(white("Depends On      :")), strings.Join(a.Depends, "  "))
	fmt.Println(bold(white("Make Deps       :")), strings.Join(a.MakeDepends, "  "))
	fmt.Println(bold(white("Check Deps      :")), strings.Join(a.CheckDepends, "  "))
	fmt.Println(bold(white("Optional Deps   :")), strings.Join(a.OptDepends, "  "))
	fmt.Println(bold(white("Conflicts With  :")), strings.Join(a.Conflicts, "  "))
	fmt.Println(bold(white("Maintainer      :")), a.Maintainer)
	fmt.Println(bold(white("Votes           :")), a.NumVotes)
	fmt.Println(bold(white("Popularity      :")), a.Popularity)
	if a.OutOfDate != 0 {
		fmt.Println(bold(white("Out-of-date     :")), "Yes")
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
		fmt.Println(pkgS[i].Name() + ": " + yellow(human(pkgS[i].ISize())))
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
	fmt.Println(bold(cyan("===========================================")))
	fmt.Println(bold(green("Total installed packages: ")) + yellow(strconv.Itoa(info.Totaln)))
	fmt.Println(bold(green("Total foreign installed packages: ")) + yellow(strconv.Itoa(len(remoteNames))))
	fmt.Println(bold(green("Explicitly installed packages: ")) + yellow(strconv.Itoa(info.Expln)))
	fmt.Println(bold(green("Total Size occupied by packages: ")) + yellow(human(info.TotalSize)))
	fmt.Println(bold(cyan("===========================================")))
	fmt.Println(bold(green("Ten biggest packages")))
	biggestPackages()
	fmt.Println(bold(cyan("===========================================")))

	aurInfo(remoteNames)

	return nil
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

func red(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[31m" + in + "\x1b[0m"
	}

	return in
}

func green(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[32m" + in + "\x1b[0m"
	}

	return in
}

func yellow(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[33m" + in + "\x1b[0m"
	}

	return in
}

func blue(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[34m" + in + "\x1b[0m"
	}

	return in
}

func cyan(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[36m" + in + "\x1b[0m"
	}

	return in
}

func white(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[37m" + in + "\x1b[0m"
	}

	return in
}

func blackBg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[40m" + in + "\x1b[0m"
	}

	return in
}

func bold(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[1m" + in + "\x1b[0m"
	}

	return in
}

func colourHash(name string) (output string) {
	if alpmConf.Options&alpm.ConfColor == 0 {
		return name
	}
	var hash = 5381
	for i := 0; i < len(name); i++ {
		hash = int(name[i]) + ((hash << 5) + (hash))
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", hash%6+31, name)
}
