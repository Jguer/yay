package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/vcs"
)

func setPkgReason(ctx context.Context,
	cmdBuilder exe.ICmdBuilder,
	mode parser.TargetMode,
	cmdArgs *parser.Arguments, pkgs []string, exp bool,
) error {
	if len(pkgs) == 0 {
		return nil
	}

	cmdArgs = cmdArgs.CopyGlobal()
	if exp {
		if err := cmdArgs.AddArg("q", "D", "asexplicit"); err != nil {
			return err
		}
	} else {
		if err := cmdArgs.AddArg("q", "D", "asdeps"); err != nil {
			return err
		}
	}

	for _, compositePkgName := range pkgs {
		pkgSplit := strings.Split(compositePkgName, "/")

		pkgName := pkgSplit[0]
		if len(pkgSplit) > 1 {
			pkgName = pkgSplit[1]
		}

		cmdArgs.AddTarget(pkgName)
	}

	if err := cmdBuilder.Show(cmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, mode, settings.NoConfirm)); err != nil {
		return &SetPkgReasonError{exp: exp}
	}

	return nil
}

func asdeps(ctx context.Context,
	cmdBuilder exe.ICmdBuilder,
	mode parser.TargetMode, cmdArgs *parser.Arguments, pkgs []string,
) error {
	return setPkgReason(ctx, cmdBuilder, mode, cmdArgs, pkgs, false)
}

func asexp(ctx context.Context,
	cmdBuilder exe.ICmdBuilder,
	mode parser.TargetMode, cmdArgs *parser.Arguments, pkgs []string,
) error {
	return setPkgReason(ctx, cmdBuilder, mode, cmdArgs, pkgs, true)
}

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

func parsePackageList(ctx context.Context, cmdBuilder exe.ICmdBuilder,
	dir string,
) (pkgdests map[string]string, pkgVersion string, err error) {
	stdout, stderr, err := cmdBuilder.Capture(
		cmdBuilder.BuildMakepkgCmd(ctx, dir, "--packagelist"))
	if err != nil {
		return nil, "", fmt.Errorf("%s %w", stderr, err)
	}

	lines := strings.Split(stdout, "\n")
	pkgdests = make(map[string]string)

	for _, line := range lines {
		if line == "" {
			continue
		}

		fileName := filepath.Base(line)
		split := strings.Split(fileName, "-")

		if len(split) < 4 {
			return nil, "", errors.New(gotext.Get("cannot find package name: %v", split))
		}

		// pkgname-pkgver-pkgrel-arch.pkgext
		// This assumes 3 dashes after the pkgname, Will cause an error
		// if the PKGEXT contains a dash. Please no one do that.
		pkgName := strings.Join(split[:len(split)-3], "-")
		pkgVersion = strings.Join(split[len(split)-3:len(split)-1], "-")
		pkgdests[pkgName] = line
	}

	if len(pkgdests) == 0 {
		return nil, "", &NoPkgDestsFoundError{dir}
	}

	return pkgdests, pkgVersion, nil
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

func installPkgArchive(ctx context.Context,
	cmdBuilder exe.ICmdBuilder,
	mode parser.TargetMode,
	vcsStore vcs.Store,
	cmdArgs *parser.Arguments,
	pkgArchives []string,
	noConfirm bool,
) error {
	if len(pkgArchives) == 0 {
		return nil
	}

	arguments := cmdArgs.Copy()
	arguments.ClearTargets()
	arguments.Op = "U"
	arguments.DelArg("confirm")
	arguments.DelArg("noconfirm")
	arguments.DelArg("c", "clean")
	arguments.DelArg("i", "install")
	arguments.DelArg("q", "quiet")
	arguments.DelArg("y", "refresh")
	arguments.DelArg("u", "sysupgrade")
	arguments.DelArg("w", "downloadonly")
	arguments.DelArg("asdeps", "asdep")
	arguments.DelArg("asexplicit", "asexp")

	arguments.AddTarget(pkgArchives...)

	if errShow := cmdBuilder.Show(cmdBuilder.BuildPacmanCmd(ctx,
		arguments, mode, noConfirm)); errShow != nil {
		return errShow
	}

	if errStore := vcsStore.Save(); errStore != nil {
		fmt.Fprintln(os.Stderr, errStore)
	}

	return nil
}

func setInstallReason(ctx context.Context,
	cmdBuilder exe.ICmdBuilder, mode parser.TargetMode,
	cmdArgs *parser.Arguments, deps, exps []string,
) error {
	if len(deps)+len(exps) == 0 {
		return nil
	}

	if errDeps := asdeps(ctx, cmdBuilder, mode, cmdArgs, deps); errDeps != nil {
		return errDeps
	}

	return asexp(ctx, cmdBuilder, mode, cmdArgs, exps)
}
