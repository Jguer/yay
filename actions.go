package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	aur "github.com/jguer/yay/aur"
	"github.com/jguer/yay/config"
	pac "github.com/jguer/yay/pacman"
	"github.com/jguer/yay/upgrade"
)

// Install handles package installs
func install(pkgs []string, flags []string) error {
	aurs, repos, _ := pac.PackageSlices(pkgs)

	if len(repos) != 0 {
		err := config.PassToPacman("-S", repos, flags)
		if err != nil {
			fmt.Println("Error installing repo packages.")
		}
	}

	if len(aurs) != 0 {
		err := aur.Install(aurs, flags)
		if err != nil {
			fmt.Println("Error installing aur packages.")
		}
	}
	return nil
}

// Upgrade handles updating the cache and installing updates.
func upgradePkgs(flags []string) error {
	aurUp, repoUp, err := upgrade.List()
	if err != nil {
		return err
	}

	if len(aurUp)+len(repoUp) > 0 {
		sort.Sort(repoUp)
		fmt.Printf("\x1b[1;34;1m:: \x1b[0m\x1b[1m%d Packages to upgrade.\x1b[0m\n", len(aurUp)+len(repoUp))
		upgrade.Print(len(aurUp), repoUp)
		upgrade.Print(0, aurUp)
		fmt.Print("\x1b[32mEnter packages you don't want to upgrade.\x1b[0m\nNumbers: ")
	}
	reader := bufio.NewReader(os.Stdin)

	numberBuf, overflow, err := reader.ReadLine()
	if err != nil || overflow {
		fmt.Println(err)
		return err
	}

	result := strings.Fields(string(numberBuf))
	var repoNums []int
	var aurNums []int
	for _, numS := range result {
		num, err := strconv.Atoi(numS)
		if err != nil {
			continue
		}
		if num > len(aurUp)+len(repoUp)-1 || num < 0 {
			continue
		} else if num < len(aurUp) {
			num = len(aurUp) - num - 1
			aurNums = append(aurNums, num)
		} else {
			num = len(aurUp) + len(repoUp) - num - 1
			repoNums = append(repoNums, num)
		}
	}

	if len(repoUp) != 0 {
		var repoNames []string
	repoloop:
		for i, k := range repoUp {
			for _, j := range repoNums {
				if j == i {
					continue repoloop
				}
			}
			repoNames = append(repoNames, k.Name)
		}

		err := config.PassToPacman("-S", repoNames, flags)
		if err != nil {
			fmt.Println("Error upgrading repo packages.")
		}
	}

	if len(aurUp) != 0 {
		var aurNames []string
	aurloop:
		for i, k := range aurUp {
			for _, j := range aurNums {
				if j == i {
					continue aurloop
				}
			}
			aurNames = append(aurNames, k.Name)
		}
		aur.Install(aurNames, flags)
	}
	return nil
}

// CleanDependencies removels all dangling dependencies in system
func cleanDependencies(pkgs []string) error {
	hanging, err := pac.HangingPackages()
	if err != nil {
		return err
	}

	if len(hanging) != 0 {
		if !config.ContinueTask("Confirm Removal?", "nN") {
			return nil
		}
		err = pac.CleanRemove(hanging)
	}

	return err
}

// GetPkgbuild gets the pkgbuild of the package 'pkg' trying the ABS first and then the AUR trying the ABS first and then the AUR.
func getPkgbuild(pkg string) (err error) {
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	wd = wd + "/"

	err = pac.GetPkgbuild(pkg, wd)
	if err == nil {
		return
	}

	err = aur.GetPkgbuild(pkg, wd)
	return
}
