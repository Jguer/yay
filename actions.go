package yay

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"

	aur "github.com/jguer/yay/aur"
	pac "github.com/jguer/yay/pacman"
	"github.com/jguer/yay/util"
)

// NarrowSearch removes terms that don't contain narrow terms in the description or name.
func narrowSearch(aq aur.Query, pq pac.Query, narrow []string) (raq aur.Query, rpq pac.Query) {
	for _, pr := range pq {
		match := false
		for _, narrowS := range narrow {
			if strings.Contains(strings.ToUpper(pr.Name), strings.ToUpper(narrowS)) || strings.Contains(strings.ToUpper(pr.Description), strings.ToUpper(narrowS)) {
				match = true
			} else {
				match = false
			}
		}

		if match {
			rpq = append(rpq, pr)
		}

	}

	for _, ar := range aq {
		match := false
		for _, narrowS := range narrow {
			if strings.Contains(strings.ToUpper(ar.Name), strings.ToUpper(narrowS)) || strings.Contains(strings.ToUpper(ar.Description), strings.ToUpper(narrowS)) {
				match = true
			} else {
				match = false
			}
		}

		if match {
			raq = append(raq, ar)
		}
	}

	return
}

// NumberMenu presents a CLI for selecting packages to install.
func NumberMenu(pkgName string, narrow []string, flags []string) (err error) {
	var num int
	var numberString string

	aq, numaq, err := aur.Search(pkgName, true)
	if err != nil {
		fmt.Println("Error during AUR search:", err)
	}
	pq, numpq, err := pac.Search(pkgName)
	if err != nil {
		return
	}

	if numpq == 0 && numaq == 0 {
		return fmt.Errorf("no packages match search")
	}

	if len(narrow) != 0 {
		aq, pq = narrowSearch(aq, pq, narrow)
		numaq = len(aq)
		numpq = len(pq)
	}

	if util.SortMode == util.BottomUp {
		aq.PrintSearch(numpq)
		pq.PrintSearch()
	} else {
		pq.PrintSearch()
		aq.PrintSearch(numpq)
	}

	fmt.Printf("\x1b[32m%s\x1b[0m\nNumbers:", "Type numbers to install. Separate each number with a space.")
	reader := bufio.NewReader(os.Stdin)
	numberString, err = reader.ReadString('\n')
	if err != nil {
		fmt.Println(err)
		return
	}

	var aurInstall []string
	var repoInstall []string
	result := strings.Fields(numberString)
	for _, numS := range result {
		num, err = strconv.Atoi(numS)
		if err != nil {
			continue
		}

		// Install package
		if num > numaq+numpq-1 || num < 0 {
			continue
		} else if num > numpq-1 {
			if util.SortMode == util.BottomUp {
				aurInstall = append(aurInstall, aq[numaq+numpq-num-1].Name)
			} else {
				aurInstall = append(aurInstall, aq[num-numpq].Name)
			}
		} else {
			if util.SortMode == util.BottomUp {
				repoInstall = append(repoInstall, pq[numpq-num-1].Name)
			} else {
				repoInstall = append(repoInstall, pq[num].Name)
			}
		}
	}

	if len(repoInstall) != 0 {
		pac.Install(repoInstall, flags)
	}

	if len(aurInstall) != 0 {
		q, n, err := aur.MultiInfo(aurInstall)
		if err != nil {
			return err
		} else if n != len(aurInstall) {
			q.MissingPackage(aurInstall)
		}

		var finalrm []string
		for _, aurpkg := range q {
			finalmdeps, err := aurpkg.Install(flags)
			finalrm = append(finalrm, finalmdeps...)
			if err != nil {
				// Do not abandon program, we might still be able to install the rest
				fmt.Println(err)
			}
		}

		if len(finalrm) != 0 {
			aur.RemoveMakeDeps(finalrm)
		}
	}

	return nil
}

// Install handles package installs
func Install(pkgs []string, flags []string) error {
	aurs, repos, _ := pac.PackageSlices(pkgs)

	err := pac.Install(repos, flags)
	if err != nil {
		fmt.Println("Error installing repo packages.")
	}

	q, n, err := aur.MultiInfo(aurs)
	if len(aurs) != n || err != nil {
		fmt.Println("Unable to get info on some packages")
	}

	var finalrm []string
	for _, aurpkg := range q {
		finalmdeps, err := aurpkg.Install(flags)
		finalrm = append(finalrm, finalmdeps...)
		if err != nil {
			fmt.Println("Error installing", aurpkg.Name, ":", err)
		}
	}

	if len(finalrm) != 0 {
		aur.RemoveMakeDeps(finalrm)
	}

	return nil
}

// Upgrade handles updating the cache and installing updates.
func Upgrade(flags []string) error {
	errp := pac.UpdatePackages(flags)
	erra := aur.Upgrade(flags)

	if errp != nil {
		return errp
	}

	return erra
}

// Search presents a query to the local repos and to the AUR.
func Search(pkg string, narrow []string) (err error) {
	aq, _, err := aur.Search(pkg, true)
	if err != nil {
		return err
	}
	pq, _, err := pac.Search(pkg)
	if err != nil {
		return err
	}

	if len(narrow) != 0 {
		aq, pq = narrowSearch(aq, pq, narrow)
	}

	if util.SortMode == util.BottomUp {
		aq.PrintSearch(0)
		pq.PrintSearch()
	} else {
		pq.PrintSearch()
		aq.PrintSearch(0)
	}

	return nil
}

// SingleSearch serves as a pacman -Si for repo packages and AUR packages.
func SingleSearch(pkgS []string, flags []string) (err error) {
	aurS, repoS, err := pac.PackageSlices(pkgS)
	if err != nil {
		return
	}

	q, _, err := aur.MultiInfo(aurS)
	if err != nil {
		fmt.Println(err)
	}

	for _, aurP := range q {
		aurP.PrintInfo()
	}

	if len(repoS) != 0 {
		err = PassToPacman("-Si", repoS, flags)
	}

	return
}

// LocalStatistics returns installed packages statistics.
func LocalStatistics(version string) error {
	info, err := pac.Statistics()
	if err != nil {
		return err
	}

	foreignS, foreign, _ := pac.ForeignPackages()

	fmt.Printf("\n Yay version r%s\n", version)
	fmt.Println("\x1B[1;34m===========================================\x1B[0m")
	fmt.Printf("\x1B[1;32mTotal installed packages: \x1B[0;33m%d\x1B[0m\n", info.Totaln)
	fmt.Printf("\x1B[1;32mTotal foreign installed packages: \x1B[0;33m%d\x1B[0m\n", foreign)
	fmt.Printf("\x1B[1;32mExplicitly installed packages: \x1B[0;33m%d\x1B[0m\n", info.Expln)
	fmt.Printf("\x1B[1;32mTotal Size occupied by packages: \x1B[0;33m%s\x1B[0m\n", size(info.TotalSize))
	fmt.Println("\x1B[1;34m===========================================\x1B[0m")
	fmt.Println("\x1B[1;32mTen biggest packages\x1B[0m")
	pac.BiggestPackages()
	fmt.Println("\x1B[1;34m===========================================\x1B[0m")

	keys := make([]string, len(foreignS))
	i := 0
	for k := range foreignS {
		keys[i] = k
		i++
	}
	q, _, err := aur.MultiInfo(keys)
	if err != nil {
		return err
	}

	for _, res := range q {
		if res.Maintainer == "" {
			fmt.Printf("\x1b[1;31;40mWarning: \x1B[1;33;40m%s\x1b[0;;40m is orphaned.\x1b[0m\n", res.Name)
		}
		if res.OutOfDate != 0 {
			fmt.Printf("\x1b[1;31;40mWarning: \x1B[1;33;40m%s\x1b[0;;40m is out-of-date in AUR.\x1b[0m\n", res.Name)
		}
	}

	return nil
}

// Function by pyk https://github.com/pyk/byten
func index(s int64) float64 {
	x := math.Log(float64(s)) / math.Log(1024)
	return math.Floor(x)
}

// Function by pyk https://github.com/pyk/byten
func countSize(s int64, i float64) float64 {
	return float64(s) / math.Pow(1024, math.Floor(i))
}

// Size return a formated string from file size
// Function by pyk https://github.com/pyk/byten
func size(s int64) string {

	symbols := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	i := index(s)
	if s < 10 {
		return fmt.Sprintf("%dB", s)
	}
	size := countSize(s, i)
	format := "%.0f"
	if size < 10 {
		format = "%.1f"
	}

	return fmt.Sprintf(format+"%s", size, symbols[int(i)])
}

// PassToPacman outsorces execution to pacman binary without modifications.
func PassToPacman(op string, pkgs []string, flags []string) error {
	var cmd *exec.Cmd
	var args []string

	args = append(args, op)
	if len(pkgs) != 0 {
		args = append(args, pkgs...)
	}

	if len(flags) != 0 {
		args = append(args, flags...)
	}

	if strings.Contains(op, "-Q") || op == "-Si" {
		cmd = exec.Command("pacman", args...)
	} else {
		args = append([]string{"pacman"}, args...)
		cmd = exec.Command("sudo", args...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}

// CleanDependencies removels all dangling dependencies in system
func CleanDependencies(pkgs []string) error {
	hanging, err := pac.HangingPackages()
	if err != nil {
		return err
	}

	if len(hanging) != 0 {
		if !util.ContinueTask("Confirm Removal?", "nN") {
			return nil
		}
		err = pac.CleanRemove(hanging)
	}

	return err
}
