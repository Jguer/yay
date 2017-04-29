package main

import (
	"fmt"
	"os"

	aur "github.com/jguer/yay/aur"
	pac "github.com/jguer/yay/pacman"
	"github.com/jguer/yay/util"
)

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

// GetPkgbuild gets the pkgbuild of the package 'pkg' trying the ABS first and then the AUR trying the ABS first and then the AUR.
func GetPkgbuild(pkg string) (err error) {
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
