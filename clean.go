package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/stringset"
	"github.com/Jguer/yay/v10/pkg/text"
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

	arguments := cmdArgs.CopyGlobal()
	_ = arguments.AddArg("R")
	arguments.AddTarget(pkgNames...)

	return show(passToPacman(arguments))
}

func syncClean(parser *settings.Arguments) error {
	keepInstalled := false
	keepCurrent := false

	_, removeAll, _ := parser.GetArg("c", "clean")

	for _, v := range pacmanConf.CleanMethod {
		if v == "KeepInstalled" {
			keepInstalled = true
		} else if v == "KeepCurrent" {
			keepCurrent = true
		}
	}

	if config.Runtime.Mode == settings.ModeRepo || config.Runtime.Mode == settings.ModeAny {
		if err := show(passToPacman(parser)); err != nil {
			return err
		}
	}

	if !(config.Runtime.Mode == settings.ModeAUR || config.Runtime.Mode == settings.ModeAny) {
		return nil
	}

	var question string
	if removeAll {
		question = gotext.Get("Do you want to remove ALL AUR packages from cache?")
	} else {
		question = gotext.Get("Do you want to remove all other AUR packages from cache?")
	}

	fmt.Println(gotext.Get("\nBuild directory:"), config.BuildDir)

	if continueTask(question, true) {
		if err := cleanAUR(keepInstalled, keepCurrent, removeAll); err != nil {
			return err
		}
	}

	if removeAll {
		return nil
	}

	if continueTask(gotext.Get("Do you want to remove ALL untracked AUR files?"), true) {
		return cleanUntracked()
	}

	return nil
}

func cleanAUR(keepInstalled, keepCurrent, removeAll bool) error {
	fmt.Println(gotext.Get("removing AUR packages from cache..."))

	installedBases := make(stringset.StringSet)
	inAURBases := make(stringset.StringSet)

	_, remotePackages, _, _, err := query.FilterPackages(alpmHandle)
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
		info, errInfo := aurInfo(cachedPackages, &aurWarnings{})
		if errInfo != nil {
			return errInfo
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
	fmt.Println(gotext.Get("removing untracked AUR files from cache..."))

	files, err := ioutil.ReadDir(config.BuildDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		dir := filepath.Join(config.BuildDir, file.Name())
		if isGitRepository(dir) {
			if err := show(passToGit(dir, "clean", "-fx")); err != nil {
				return err
			}
		}
	}
	return nil
}

func isGitRepository(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return !os.IsNotExist(err)
}

func cleanAfter(bases []Base) {
	fmt.Println(gotext.Get("removing untracked AUR files from cache..."))

	for i, base := range bases {
		dir := filepath.Join(config.BuildDir, base.Pkgbase())
		if !isGitRepository(dir) {
			continue
		}

		text.OperationInfoln(gotext.Get("Cleaning (%d/%d): %s", i+1, len(bases), cyan(dir)))

		_, stderr, err := capture(passToGit(dir, "reset", "--hard", "HEAD"))
		if err != nil {
			text.Errorln(gotext.Get("error resetting %s: %s", base.String(), stderr))
		}

		if err := show(passToGit(dir, "clean", "-fx", "--exclude='*.pkg.*'")); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func cleanBuilds(bases []Base) {
	for i, base := range bases {
		dir := filepath.Join(config.BuildDir, base.Pkgbase())
		text.OperationInfoln(gotext.Get("Deleting (%d/%d): %s", i+1, len(bases), cyan(dir)))
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}
