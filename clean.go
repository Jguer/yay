package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/stringset"
	"github.com/Jguer/yay/v12/pkg/text"
)

// CleanDependencies removes all dangling dependencies in system.
func cleanDependencies(ctx context.Context, cfg *settings.Configuration,
	cmdBuilder exe.ICmdBuilder, cmdArgs *parser.Arguments, dbExecutor db.Executor,
	removeOptional bool,
) error {
	hanging := hangingPackages(removeOptional, dbExecutor)
	if len(hanging) != 0 {
		return cleanRemove(ctx, cfg, cmdBuilder, cmdArgs, hanging)
	}

	return nil
}

// CleanRemove sends a full removal command to pacman with the pkgName slice.
func cleanRemove(ctx context.Context, cfg *settings.Configuration,
	cmdBuilder exe.ICmdBuilder, cmdArgs *parser.Arguments, pkgNames []string,
) error {
	if len(pkgNames) == 0 {
		return nil
	}

	arguments := cmdArgs.CopyGlobal()
	if err := arguments.AddArg("R", "s", "u"); err != nil {
		return err
	}
	arguments.AddTarget(pkgNames...)

	return cmdBuilder.Show(
		cmdBuilder.BuildPacmanCmd(ctx,
			arguments, cfg.Mode, settings.NoConfirm))
}

func syncClean(ctx context.Context, cfg *settings.Configuration, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	keepInstalled := false
	keepCurrent := false

	_, removeAll, _ := cmdArgs.GetArg("c", "clean")

	for _, v := range cfg.Runtime.PacmanConf.CleanMethod {
		if v == "KeepInstalled" {
			keepInstalled = true
		} else if v == "KeepCurrent" {
			keepCurrent = true
		}
	}

	if cfg.Mode.AtLeastRepo() {
		if err := cfg.Runtime.CmdBuilder.Show(cfg.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, cfg.Mode, settings.NoConfirm)); err != nil {
			return err
		}
	}

	if !cfg.Mode.AtLeastAUR() {
		return nil
	}

	var question string
	if removeAll {
		question = gotext.Get("Do you want to remove ALL AUR packages from cache?")
	} else {
		question = gotext.Get("Do you want to remove all other AUR packages from cache?")
	}

	fmt.Println(gotext.Get("\nBuild directory:"), cfg.BuildDir)

	if text.ContinueTask(os.Stdin, question, true, settings.NoConfirm) {
		if err := cleanAUR(ctx, cfg, keepInstalled, keepCurrent, removeAll, dbExecutor); err != nil {
			return err
		}
	}

	if removeAll {
		return nil
	}

	if text.ContinueTask(os.Stdin, gotext.Get("Do you want to remove ALL untracked AUR files?"), true, settings.NoConfirm) {
		return cleanUntracked(ctx, cfg)
	}

	return nil
}

func cleanAUR(ctx context.Context, config *settings.Configuration,
	keepInstalled, keepCurrent, removeAll bool, dbExecutor db.Executor,
) error {
	fmt.Println(gotext.Get("removing AUR packages from cache..."))

	installedBases := make(stringset.StringSet)
	inAURBases := make(stringset.StringSet)

	remotePackages := dbExecutor.InstalledRemotePackages()

	files, err := os.ReadDir(config.BuildDir)
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
		info, errInfo := query.AURInfo(ctx, config.Runtime.AURClient, cachedPackages, query.NewWarnings(nil), config.RequestSplitN)
		if errInfo != nil {
			return errInfo
		}

		for i := range info {
			inAURBases.Set(info[i].PackageBase)
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

		dir := filepath.Join(config.BuildDir, file.Name())
		err = os.RemoveAll(dir)
		if err != nil {
			text.Warnln(gotext.Get("Unable to remove %s: %s", dir, err))
		}
	}

	return nil
}

func cleanUntracked(ctx context.Context, cfg *settings.Configuration) error {
	fmt.Println(gotext.Get("removing untracked AUR files from cache..."))

	files, err := os.ReadDir(cfg.BuildDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		dir := filepath.Join(cfg.BuildDir, file.Name())
		if isGitRepository(dir) {
			if err := cfg.Runtime.CmdBuilder.Show(cfg.Runtime.CmdBuilder.BuildGitCmd(ctx, dir, "clean", "-fx")); err != nil {
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

func cleanAfter(ctx context.Context, config *settings.Configuration,
	cmdBuilder exe.ICmdBuilder, pkgbuildDirs map[string]string,
) {
	fmt.Println(gotext.Get("removing untracked AUR files from cache..."))

	i := 0
	for _, dir := range pkgbuildDirs {
		text.OperationInfoln(gotext.Get("Cleaning (%d/%d): %s", i+1, len(pkgbuildDirs), text.Cyan(dir)))

		_, stderr, err := cmdBuilder.Capture(
			cmdBuilder.BuildGitCmd(
				ctx, dir, "reset", "--hard", "HEAD"))
		if err != nil {
			text.Errorln(gotext.Get("error resetting %s: %s", dir, stderr))
		}

		if err := config.Runtime.CmdBuilder.Show(
			config.Runtime.CmdBuilder.BuildGitCmd(
				ctx, dir, "clean", "-fx", "--exclude", "*.pkg.*")); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}

		i++
	}
}
