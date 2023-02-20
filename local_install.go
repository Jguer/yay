// Experimental code for install local with dependency refactoring
// Not at feature parity with install.go
package main

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/multierror"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
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
	noCheck := strings.Contains(config.MFlags, "--nocheck")

	if len(cmdArgs.Targets) < 1 {
		return errors.New(gotext.Get("no target directories specified"))
	}

	grapher := dep.NewGrapher(dbExecutor, aurCache, false, settings.NoConfirm,
		cmdArgs.ExistsDouble("d", "nodeps"), noCheck,
		config.Runtime.Logger.Child("grapher"))
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

	opService := NewOperationService(ctx, config, dbExecutor)
	multiErr := &multierror.MultiError{}
	targets := graph.TopoSortedLayerMap(func(name string, ii *dep.InstallInfo) error {
		if ii.Source == dep.Missing {
			multiErr.Add(errors.New(gotext.Get("could not find %s%s", name, ii.Version)))
		}
		return nil
	})

	if err := multiErr.Return(); err != nil {
		return err
	}
	return opService.Run(ctx, cmdArgs, targets)
}
