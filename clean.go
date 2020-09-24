package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/dep"
	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/stringset"
	"github.com/Jguer/yay/v10/pkg/text"
)

// CleanDependencies removes all dangling dependencies in system
func cleanDependencies(cmdArgs *settings.Arguments, dbExecutor db.Executor, removeOptional bool) error {
	hanging := hangingPackages(removeOptional, dbExecutor)
	if len(hanging) != 0 {
		return cleanRemove(cmdArgs, hanging)
	}

	return nil
}

// CleanRemove sends a full removal command to pacman with the pkgName slice
func cleanRemove(cmdArgs *settings.Arguments, pkgNames []string) error {
	if len(pkgNames) == 0 {
		return nil
	}

	arguments := cmdArgs.CopyGlobal()
	_ = arguments.AddArg("R")
	arguments.AddTarget(pkgNames...)

	return config.Runtime.CmdRunner.Show(passToPacman(arguments))
}

func syncClean(cmdArgs *settings.Arguments, dbExecutor db.Executor) error {
	keepInstalled := false
	keepCurrent := false

	_, removeAll, _ := cmdArgs.GetArg("c", "clean")

	for _, v := range config.Runtime.PacmanConf.CleanMethod {
		if v == "KeepInstalled" {
			keepInstalled = true
		} else if v == "KeepCurrent" {
			keepCurrent = true
		}
	}

	if config.Runtime.Mode == settings.ModeRepo || config.Runtime.Mode == settings.ModeAny {
		if err := config.Runtime.CmdRunner.Show(passToPacman(cmdArgs)); err != nil {
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

	if text.ContinueTask(question, true, settings.NoConfirm) {
		if err := cleanAUR(keepInstalled, keepCurrent, removeAll, dbExecutor); err != nil {
			return err
		}
	}

	if removeAll {
		return nil
	}

	if text.ContinueTask(gotext.Get("Do you want to remove ALL untracked AUR files?"), true, settings.NoConfirm) {
		return cleanUntracked()
	}

	return nil
}

func cleanAUR(keepInstalled, keepCurrent, removeAll bool, dbExecutor db.Executor) error {
	fmt.Println(gotext.Get("removing AUR packages from cache..."))

	installedBases := make(stringset.StringSet)
	inAURBases := make(stringset.StringSet)

	remotePackages, _ := query.GetRemotePackages(dbExecutor)

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
		info, errInfo := query.AURInfo(cachedPackages, &query.AURWarnings{}, config.RequestSplitN)
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
			if err := config.Runtime.CmdRunner.Show(config.Runtime.CmdBuilder.BuildGitCmd(dir, "clean", "-fx")); err != nil {
				text.Warnln(gotext.Get("Unable to clean:"), dir)
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

func cleanAfter(bases []dep.Base) {
	fmt.Println(gotext.Get("removing untracked AUR files from cache..."))

	for i, base := range bases {
		dir := filepath.Join(config.BuildDir, base.Pkgbase())
		if !isGitRepository(dir) {
			continue
		}

		text.OperationInfoln(gotext.Get("Cleaning (%d/%d): %s", i+1, len(bases), text.Cyan(dir)))

		_, stderr, err := config.Runtime.CmdRunner.Capture(config.Runtime.CmdBuilder.BuildGitCmd(dir, "reset", "--hard", "HEAD"), 0)
		if err != nil {
			text.Errorln(gotext.Get("error resetting %s: %s", base.String(), stderr))
		}

		if err := config.Runtime.CmdRunner.Show(config.Runtime.CmdBuilder.BuildGitCmd(dir, "clean", "-fx", "--exclude='*.pkg.*'")); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func cleanBuilds(bases []dep.Base) {
	for i, base := range bases {
		dir := filepath.Join(config.BuildDir, base.Pkgbase())
		text.OperationInfoln(gotext.Get("Deleting (%d/%d): %s", i+1, len(bases), text.Cyan(dir)))
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}
