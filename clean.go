package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// GetPkgbuild gets the pkgbuild of the package 'pkg' trying the ABS first and then the AUR trying the ABS first and then the AUR.

// RemovePackage removes package from VCS information
func removeVCSPackage(pkgs []string) {
	updated := false

	for _, pkgName := range pkgs {
		_, ok := savedInfo[pkgName]
		if ok {
			delete(savedInfo, pkgName)
			updated = true
		}
	}

	if updated {
		saveVCSInfo()
	}
}

// CleanDependencies removes all dangling dependencies in system
func cleanDependencies(removeOptional bool) error {
	hanging, err := hangingPackages(removeOptional)
	if err != nil {
		return err
	}

	if len(hanging) != 0 {
		err = cleanRemove(hanging)
	}

	return err
}

// CleanRemove sends a full removal command to pacman with the pkgName slice
func cleanRemove(pkgNames []string) (err error) {
	if len(pkgNames) == 0 {
		return nil
	}

	arguments := makeArguments()
	arguments.addArg("R")
	arguments.addTarget(pkgNames...)
	err = show(passToPacman(arguments))
	return err
}

func syncClean(parser *arguments) error {
	keepInstalled := false
	keepCurrent := false

	_, removeAll, _ := parser.getArg("c", "clean")

	for _, v := range alpmConf.CleanMethod {
		if v == "KeepInstalled" {
			keepInstalled = true
		} else if v == "KeepCurrent" {
			keepCurrent = true
		}
	}

	err := show(passToPacman(parser))
	if err != nil {
		return err
	}

	var question string
	if removeAll {
		question = "Do you want to remove ALL AUR packages from cache?"
	} else {
		question = "Do you want to remove all other AUR packages from cache?"
	}

	fmt.Println()
	fmt.Printf("Build directory: %s\n", config.BuildDir)

	if continueTask(question, "nN") {
		err = cleanAUR(keepInstalled, keepCurrent, removeAll)
	}

	if err != nil || removeAll {
		return err
	}

	if continueTask("Do you want to remove ALL untracked AUR files?", "nN") {
		err = cleanUntracked()
	}

	return err
}

func cleanAUR(keepInstalled, keepCurrent, removeAll bool) error {
	fmt.Println("removing AUR packages from cache...")

	installedBases := make(stringSet)
	inAURBases := make(stringSet)

	_, remotePackages, _, _, err := filterPackages()
	if err != nil {
		return err
	}

	files, err := ioutil.ReadDir(config.BuildDir)
	if err != nil {
		return err
	}

	cachedPackages := make([]string, 0, len(files))
	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		cachedPackages = append(cachedPackages, file.Name())
	}

	// Most people probably don't use keep current and that is the only
	// case where this is needed.
	// Querying the AUR is slow and needs internet so don't do it if we
	// don't need to.
	if keepCurrent {
		info, err := aurInfo(cachedPackages, &aurWarnings{})
		if err != nil {
			return err
		}

		for _, pkg := range info {
			inAURBases.set(pkg.PackageBase)
		}
	}

	for _, pkg := range remotePackages {
		if pkg.Base() != "" {
			installedBases.set(pkg.Base())
		} else {
			installedBases.set(pkg.Name())
		}
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		if !removeAll {
			if keepInstalled && installedBases.get(file.Name()) {
				continue
			}

			if keepCurrent && inAURBases.get(file.Name()) {
				continue
			}
		}

		err = os.RemoveAll(filepath.Join(config.BuildDir, file.Name()))
		if err != nil {
			return nil
		}
	}

	return nil
}

func cleanUntracked() error {
	fmt.Println("removing Untracked AUR files from cache...")

	files, err := ioutil.ReadDir(config.BuildDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		dir := filepath.Join(config.BuildDir, file.Name())

		if shouldUseGit(dir) {
			err = show(passToGit(dir, "clean", "-fx"))
			if err != nil {
				return err
			}
		}
	}

	return nil
}
