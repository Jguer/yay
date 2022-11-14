// Experimental code for install local with dependency refactoring
// Not at feature parity with install.go
package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/metadata"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/text"
	"github.com/leonelquinteros/gotext"
	"github.com/pkg/errors"

	gosrc "github.com/Morganamilo/go-srcinfo"
)

var ErrInstallRepoPkgs = errors.New(gotext.Get("error installing repo packages"))

func installLocalPKGBUILD(
	ctx context.Context,
	cmdArgs *parser.Arguments,
	dbExecutor db.Executor,
) error {
	aurCache, err := metadata.NewAURCache(filepath.Join(config.BuildDir, "aur.json"))
	if err != nil {
		return errors.Wrap(err, gotext.Get("failed to retrieve aur Cache"))
	}

	wd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, gotext.Get("failed to retrieve working directory"))
	}

	if len(cmdArgs.Targets) > 1 {
		return errors.New(gotext.Get("only one target is allowed"))
	}

	if len(cmdArgs.Targets) == 1 {
		wd = cmdArgs.Targets[0]
	}

	pkgbuild, err := gosrc.ParseFile(filepath.Join(wd, ".SRCINFO"))
	if err != nil {
		return errors.Wrap(err, gotext.Get("failed to parse .SRCINFO"))
	}

	grapher := dep.NewGrapher(dbExecutor, aurCache, false, settings.NoConfirm, os.Stdout)

	graph, err := grapher.GraphFromSrcInfo(ctx, nil, wd, pkgbuild)
	if err != nil {
		return err
	}

	topoSorted := graph.TopoSortedLayerMap()

	preparer := &Preparer{
		dbExecutor: dbExecutor,
		cmdBuilder: config.Runtime.CmdBuilder,
		config:     config,
	}
	installer := &Installer{dbExecutor: dbExecutor}

	if err = preparer.Present(os.Stdout, topoSorted); err != nil {
		return err
	}

	if cleanFunc := preparer.ShouldCleanMakeDeps(ctx); cleanFunc != nil {
		installer.AddPostInstallHook(cleanFunc)
	}

	pkgBuildDirs, err := preparer.PrepareWorkspace(ctx, topoSorted)
	if err != nil {
		return err
	}

	if cleanAURDirsFunc := preparer.ShouldCleanAURDirs(ctx, pkgBuildDirs); cleanAURDirsFunc != nil {
		installer.AddPostInstallHook(cleanAURDirsFunc)
	}

	if err = installer.Install(ctx, cmdArgs, topoSorted, pkgBuildDirs); err != nil {
		if errHook := installer.RunPostInstallHooks(ctx); errHook != nil {
			text.Errorln(errHook)
		}

		return err
	}

	return installer.RunPostInstallHooks(ctx)
}
