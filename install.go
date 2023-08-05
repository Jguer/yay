package main

import (
	"context"
	"errors"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
)

func removeMake(ctx context.Context, config *settings.Configuration,
	cmdBuilder exe.ICmdBuilder, makeDeps []string, cmdArgs *parser.Arguments,
) error {
	removeArguments := cmdArgs.CopyGlobal()

	err := removeArguments.AddArg("R", "s", "u")
	if err != nil {
		return err
	}

	for _, pkg := range makeDeps {
		removeArguments.AddTarget(pkg)
	}

	oldValue := settings.NoConfirm
	settings.NoConfirm = true
	err = cmdBuilder.Show(cmdBuilder.BuildPacmanCmd(ctx,
		removeArguments, config.Mode, settings.NoConfirm))
	settings.NoConfirm = oldValue

	return err
}

func earlyRefresh(ctx context.Context, cfg *settings.Configuration, cmdBuilder exe.ICmdBuilder, cmdArgs *parser.Arguments) error {
	arguments := cmdArgs.Copy()
	if cfg.CombinedUpgrade {
		arguments.DelArg("u", "sysupgrade")
	}
	arguments.DelArg("s", "search")
	arguments.DelArg("i", "info")
	arguments.DelArg("l", "list")
	arguments.ClearTargets()

	return cmdBuilder.Show(cmdBuilder.BuildPacmanCmd(ctx,
		arguments, cfg.Mode, settings.NoConfirm))
}

func gitMerge(ctx context.Context, cmdBuilder exe.ICmdBuilder, dir string) error {
	_, stderr, err := cmdBuilder.Capture(
		cmdBuilder.BuildGitCmd(ctx,
			dir, "reset", "--hard", "HEAD"))
	if err != nil {
		return errors.New(gotext.Get("error resetting %s: %s", dir, stderr))
	}

	_, stderr, err = cmdBuilder.Capture(
		cmdBuilder.BuildGitCmd(ctx,
			dir, "merge", "--no-edit", "--ff"))
	if err != nil {
		return errors.New(gotext.Get("error merging %s: %s", dir, stderr))
	}

	return nil
}

func mergePkgbuilds(ctx context.Context, cmdBuilder exe.ICmdBuilder, pkgbuildDirs map[string]string) error {
	for _, dir := range pkgbuildDirs {
		err := gitMerge(ctx, cmdBuilder, dir)
		if err != nil {
			return err
		}
	}

	return nil
}
