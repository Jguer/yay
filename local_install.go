// Experimental code for install local with dependency refactoring
// Not at feature parity with install.go
package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/text"
	"github.com/Jguer/yay/v11/pkg/topo"

	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"
	"github.com/pkg/errors"
)

var ErrInstallRepoPkgs = errors.New(gotext.Get("error installing repo packages"))

func installLocalPKGBUILD(
	ctx context.Context,
	config *settings.Configuration,
	cmdArgs *parser.Arguments,
	dbExecutor db.Executor,
) error {
	aurCache := config.Runtime.AURCache

	if len(cmdArgs.Targets) < 1 {
		return errors.New(gotext.Get("no target directories specified"))
	}

	grapher := dep.NewGrapher(dbExecutor, aurCache, false, settings.NoConfirm, os.Stdout)
	graph := topo.New[string, *dep.InstallInfo]()
	for _, target := range cmdArgs.Targets {
		var errG error

		pkgbuild, err := gosrc.ParseFile(filepath.Join(target, ".SRCINFO"))
		if err != nil {
			return errors.Wrap(err, gotext.Get("failed to parse .SRCINFO"))
		}

		graph, errG = grapher.GraphFromSrcInfo(ctx, graph, target, pkgbuild)
		if errG != nil {
			return err
		}
	}

	topoSorted := graph.TopoSortedLayerMap()

	preparer := &Preparer{
		dbExecutor: dbExecutor,
		cmdBuilder: config.Runtime.CmdBuilder,
		config:     config,
	}
	installer := &Installer{dbExecutor: dbExecutor}

	pkgBuildDirs, err := preparer.Run(ctx, os.Stdout, topoSorted)
	if err != nil {
		return err
	}

	if cleanFunc := preparer.ShouldCleanMakeDeps(); cleanFunc != nil {
		installer.AddPostInstallHook(cleanFunc)
	}

	if cleanAURDirsFunc := preparer.ShouldCleanAURDirs(pkgBuildDirs); cleanAURDirsFunc != nil {
		installer.AddPostInstallHook(cleanAURDirsFunc)
	}

	srcinfoOp := srcinfoOperator{dbExecutor: dbExecutor}

	srcinfos, err := srcinfoOp.Run(pkgBuildDirs)
	if err != nil {
		return err
	}

	if err = installer.Install(ctx, cmdArgs, topoSorted, pkgBuildDirs, srcinfos); err != nil {
		if errHook := installer.RunPostInstallHooks(ctx); errHook != nil {
			text.Errorln(errHook)
		}

		return err
	}

	return installer.RunPostInstallHooks(ctx)
}
