package main

import (
	"fmt"
	"os"

	alpm "github.com/jguer/go-alpm"
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

// Upgrade handles updating the cache and installing updates.
func upgrade(flags []string) error {
	errp := config.PassToPacman("-Sy", nil, flags)
	if errp != nil {
		return errp
	}

	pacC := make(chan []alpm.Package)
	aurC := make(chan []aur.Upgrade)
	errC := make(chan error)
	var pacUp []alpm.Package
	var aurUp []aur.Upgrade

	go func() {
		pacUpList, err := pac.UpgradeList()
		errC <- err
		pacC <- pacUpList
	}()

	go func() {
		aurUpList, err := aur.UpgradeList()
		errC <- err
		aurC <- aurUpList
	}()

	var i = 0
loop:
	for {
		select {
		case pacUp = <-pacC:
			i++
		case aurUp = <-aurC:
			i++
		case err := <-errC:
			if err != nil {
				fmt.Println(err)
			}
		default:
			if i == 2 {
				break loop
			}
		}
	}

	fmt.Printf("%+v\n", aurUp)
	fmt.Printf("%+v\n", pacUp)
	// erra := aur.Upgrade(flags)

	// if errp != nil {
	// 	return errp
	// }

	// var buffer bytes.Buffer
	// buffer.WriteString("\n")

	// fmt.Println("\x1b[1;36;1m::\x1b[0m\x1b[1m Starting AUR upgrade...\x1b[0m")
	// return erra
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
