package main

import (
	"fmt"
	"os"

	aur "github.com/jguer/yay/aur"
	"github.com/jguer/yay/config"
	pac "github.com/jguer/yay/pacman"
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
