package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	alpm "github.com/jguer/go-alpm"
	rpc "github.com/mikkeloscar/aur"
)

const warning = "\x1b[33mWarning:\x1b[0m "
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

	var q aurQuery
	var j int
	for i := len(remoteNames); i != 0; i = j {
		j = i - config.RequestSplitN
		if j < 0 {
			j = 0
		}
		qtemp, err := rpc.Info(remoteNames[j:i])
		q = append(q, qtemp...)
		if err != nil {
			return err
		}
	}

	var outcast []string
	for _, s := range remoteNames {
		found := false
		for _, i := range q {
			if s == i.Name {
				found = true
				break
			}
		}
		if !found {
			outcast = append(outcast, s)
		}
	}

	if err != nil {
		return err
	}

	for _, res := range q {
		if res.Maintainer == "" {
			fmt.Println(boldRedFgBlackBg(arrow+"Warning:"),
				boldYellowFgBlackBg(res.Name), whiteFgBlackBg("is orphaned"))
		}
		if res.OutOfDate != 0 {
			fmt.Println(boldRedFgBlackBg(arrow+"Warning:"),
				boldYellowFgBlackBg(res.Name), whiteFgBlackBg("is out-of-date in AUR"))
		}
	}

	for _, res := range outcast {
		fmt.Println(boldRedFgBlackBg(arrow+"Warning:"),
			boldYellowFgBlackBg(res), whiteFgBlackBg("is not available in AUR"))
	}

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

//todo make it less hacky
func printUpdateList() error {
	old := os.Stdout // keep backup of the real stdout
	os.Stdout = nil
	aurUp, repoUp, err := upList()
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

func yellowFg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[0;33m" + in + "\x1b[0m"
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
		return "\x1b[31;40m" + in + "\x1b[0m"
	}

	return in
}

func greenFgBlackBg(in string) string {
	if alpmConf.Options&alpm.ConfColor > 0 {
		return "\x1b[32;40m" + in + "\x1b[0m"
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
