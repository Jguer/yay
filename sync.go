package main

import (
	"context"
	"os"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/text"
)

func syncInstall(ctx context.Context,
	config *settings.Configuration,
	cmdArgs *parser.Arguments,
	dbExecutor db.Executor,
) error {
	aurCache := config.Runtime.AURCache

	if cmdArgs.ExistsArg("u", "sysupgrade") {
		var errSysUp error
		// All of the installs are done as explicit installs, this should be move to a grapher method
		_, errSysUp = addUpgradeTargetsToArgs(ctx, dbExecutor, cmdArgs, []string{}, cmdArgs)
		if errSysUp != nil {
			return errSysUp
		}
	}

	grapher := dep.NewGrapher(dbExecutor, aurCache, false, settings.NoConfirm, os.Stdout)

	graph, err := grapher.GraphFromTargets(cmdArgs.Targets)
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
