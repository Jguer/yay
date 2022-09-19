package main

import (
	"context"
	"fmt"
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
)

func syncInstall(ctx context.Context,
	config *settings.Configuration,
	cmdArgs *parser.Arguments, dbExecutor db.Executor,
) error {
	aurCache, err := metadata.NewAURCache(filepath.Join(config.BuildDir, "aur.json"))
	if err != nil {
		return errors.Wrap(err, gotext.Get("failed to retrieve aur Cache"))
	}

	grapher := dep.NewGrapher(dbExecutor, aurCache, false, settings.NoConfirm, os.Stdout)

	graph, err := grapher.GraphFromAURCache(cmdArgs.Targets)
	if err != nil {
		return err
	}

	topoSorted := graph.TopoSortedLayerMap()
	fmt.Println(topoSorted, len(topoSorted))

	preparer := &Preparer{
		dbExecutor: dbExecutor,
		cmdBuilder: config.Runtime.CmdBuilder,
		config:     config,
	}
	installer := &Installer{dbExecutor: dbExecutor}

	err = preparer.Present(os.Stdout, topoSorted)
	if err != nil {
		return err
	}

	cleanFunc := preparer.ShouldCleanMakeDeps(ctx)
	if cleanFunc != nil {
		installer.AddPostInstallHook(cleanFunc)
	}

	pkgBuildDirs, err := preparer.PrepareWorkspace(ctx, topoSorted)
	if err != nil {
		return err
	}

	err = installer.Install(ctx, cmdArgs, topoSorted, pkgBuildDirs)
	if err != nil {
		if errHook := installer.RunPostInstallHooks(ctx); errHook != nil {
			text.Errorln(errHook)
		}

		return err
	}

	return installer.RunPostInstallHooks(ctx)
}
