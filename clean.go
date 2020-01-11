package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/Jguer/yay/v9/pkg/stringset"
)

// GetPkgbuild gets the pkgbuild of the package 'pkg' trying the ABS first and then the AUR trying the ABS first and then the AUR.

// RemovePackage removes package from VCS information
func removeVCSPackage(pkgs []string) {
	updated := false

	for _, pkgName := range pkgs {
		if _, ok := savedInfo[pkgName]; ok {
			delete(savedInfo, pkgName)
			updated = true
		}
	}

	if updated {
		err := saveVCSInfo()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

// CleanDependencies removes all dangling dependencies in system
func cleanDependencies(removeOptional bool) error {
	hanging, err := hangingPackages(removeOptional)
	if err != nil {
		return err
	}

	if len(hanging) != 0 {
		return cleanRemove(hanging)
	}

	return nil
}

// CleanRemove sends a full removal command to pacman with the pkgName slice
func cleanRemove(pkgNames []string) error {
	if len(pkgNames) == 0 {
		return nil
	}

	arguments := cmdArgs.copyGlobal()
	_ = arguments.addArg("R")
	_ = arguments.addArg("n")
	arguments.addTarget(pkgNames...)

	return show(passToPacman(arguments))
}

func syncClean(parser *arguments) error {
	var err error
	keepInstalled := false
	keepCurrent := false

	_, removeAll, _ := parser.getArg("c", "clean")

	for _, v := range pacmanConf.CleanMethod {
		if v == "KeepInstalled" {
			keepInstalled = true
		} else if v == "KeepCurrent" {
			keepCurrent = true
		}
	}

	if mode == modeRepo || mode == modeAny {
		if err := show(passToPacman(parser)); err != nil {
			return err
		}
	}

	if !(mode == modeAUR || mode == modeAny) {
		return nil
	}

	var question string
	if removeAll {
		question = "Do you want to remove ALL AUR packages from cache?"
	} else {
		question = "Do you want to remove all other AUR packages from cache?"
	}

	fmt.Printf("\nBuild directory: %s\n", config.BuildDir)

	if continueTask(question, true) {
		err = cleanAUR(keepInstalled, keepCurrent, removeAll)
	}

	if err != nil || removeAll {
		return err
	}

	if continueTask("Do you want to remove ALL untracked AUR files?", true) {
		return cleanUntracked()
	}

	return nil
}

func cleanAUR(keepInstalled, keepCurrent, removeAll bool) error {
	fmt.Println("removing AUR packages from cache...")

	installedBases := make(stringset.StringSet)
	inAURBases := make(stringset.StringSet)

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
			inAURBases.Set(pkg.PackageBase)
		}
	}

	for _, pkg := range remotePackages {
		if pkg.Base() != "" {
			installedBases.Set(pkg.Base())
		} else {
			installedBases.Set(pkg.Name())
		}
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		if !removeAll {
			if keepInstalled && installedBases.Get(file.Name()) {
				continue
			}

			if keepCurrent && inAURBases.Get(file.Name()) {
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
		if err := show(passToGit(dir, "clean", "-fx")); err != nil {
			return err
		}
	}

	return nil
}

func cleanAfter(bases []Base) {
	fmt.Println("removing Untracked AUR files from cache...")

	for i, base := range bases {
		dir := filepath.Join(config.BuildDir, base.Pkgbase())

		fmt.Printf(bold(cyan("::")+" Cleaning (%d/%d): %s\n"), i+1, len(bases), cyan(dir))
		_, stderr, err := capture(passToGit(dir, "reset", "--hard", "HEAD"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error resetting %s: %s", base.String(), stderr)
		}

		if err := show(passToGit(dir, "clean", "-fx")); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func cleanBuilds(bases []Base) {
	for i, base := range bases {
		dir := filepath.Join(config.BuildDir, base.Pkgbase())
		fmt.Printf(bold(cyan("::")+" Deleting (%d/%d): %s\n"), i+1, len(bases), cyan(dir))
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}
