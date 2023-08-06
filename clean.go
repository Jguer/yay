package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Jguer/aur"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
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

func syncClean(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	keepInstalled := false
	keepCurrent := false

	_, removeAll, _ := cmdArgs.GetArg("c", "clean")

	for _, v := range run.PacmanConf.CleanMethod {
		if v == "KeepInstalled" {
			keepInstalled = true
		} else if v == "KeepCurrent" {
			keepCurrent = true
		}
	}

	if run.Cfg.Mode.AtLeastRepo() {
		if err := run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, run.Cfg.Mode, settings.NoConfirm)); err != nil {
			return err
		}
	}

	if !run.Cfg.Mode.AtLeastAUR() {
		return nil
	}

	var question string
	if removeAll {
		question = gotext.Get("Do you want to remove ALL AUR packages from cache?")
	} else {
		question = gotext.Get("Do you want to remove all other AUR packages from cache?")
	}

	run.Logger.Println(gotext.Get("\nBuild directory:"), run.Cfg.BuildDir)

	if run.Logger.ContinueTask(question, true, settings.NoConfirm) {
		if err := cleanAUR(ctx, run, keepInstalled, keepCurrent, removeAll, dbExecutor); err != nil {
			return err
		}
	}

	if removeAll {
		return nil
	}

	if run.Logger.ContinueTask(gotext.Get("Do you want to remove ALL untracked AUR files?"), true, settings.NoConfirm) {
		return cleanUntracked(ctx, run)
	}

	return nil
}

func cleanAUR(ctx context.Context, run *runtime.Runtime,
	keepInstalled, keepCurrent, removeAll bool, dbExecutor db.Executor,
) error {
	run.Logger.Println(gotext.Get("removing AUR packages from cache..."))

	installedBases := mapset.NewThreadUnsafeSet[string]()
	inAURBases := mapset.NewThreadUnsafeSet[string]()

	remotePackages := dbExecutor.InstalledRemotePackages()

	files, err := os.ReadDir(run.Cfg.BuildDir)
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
		info, errInfo := run.AURClient.Get(ctx, &aur.Query{
			Needles: cachedPackages,
		})
		if errInfo != nil {
			return errInfo
		}

		for i := range info {
			inAURBases.Add(info[i].PackageBase)
		}
	}

	for _, pkg := range remotePackages {
		if pkg.Base() != "" {
			installedBases.Add(pkg.Base())
		} else {
			installedBases.Add(pkg.Name())
		}
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		if !removeAll {
			if keepInstalled && installedBases.Contains(file.Name()) {
				continue
			}

			if keepCurrent && inAURBases.Contains(file.Name()) {
				continue
			}
		}

		dir := filepath.Join(run.Cfg.BuildDir, file.Name())
		run.Logger.Debugln("removing", dir)
		if err = os.RemoveAll(dir); err != nil {
			run.Logger.Warnln(gotext.Get("Unable to remove %s: %s", dir, err))
		}
	}

	return nil
}

func cleanUntracked(ctx context.Context, run *runtime.Runtime) error {
	run.Logger.Println(gotext.Get("removing untracked AUR files from cache..."))

	files, err := os.ReadDir(run.Cfg.BuildDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		dir := filepath.Join(run.Cfg.BuildDir, file.Name())
		run.Logger.Debugln("cleaning", dir)
		if isGitRepository(dir) {
			if err := run.CmdBuilder.Show(run.CmdBuilder.BuildGitCmd(ctx, dir, "clean", "-fx")); err != nil {
				run.Logger.Warnln(gotext.Get("Unable to clean:"), dir)
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
