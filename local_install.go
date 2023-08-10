// Experimental code for install local with dependency refactoring
// Not at feature parity with install.go
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/multierror"
	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/sync"

	"github.com/leonelquinteros/gotext"
	"github.com/pkg/errors"
)

func installLocalPKGBUILD(
	ctx context.Context,
	run *runtime.Runtime,
	cmdArgs *parser.Arguments,
	dbExecutor db.Executor,
) error {
	noCheck := strings.Contains(run.Cfg.MFlags, "--nocheck")
	grapher := dep.NewGrapher(dbExecutor, run.AURClient, run.CmdBuilder, false, settings.NoConfirm,
		cmdArgs.ExistsDouble("d", "nodeps"), noCheck, cmdArgs.ExistsArg("needed"),
		run.Logger.Child("grapher"))

	if len(cmdArgs.Targets) < 1 {
		return errors.New(gotext.Get("no target directories specified"))
	}

	graph, err := grapher.GraphFromSrcInfoDirs(ctx, nil, cmdArgs.Targets)
	if err != nil {
		return err
	}

	opService := sync.NewOperationService(ctx, dbExecutor, run)
	multiErr := &multierror.MultiError{}
	targets := graph.TopoSortedLayerMap(func(name string, ii *dep.InstallInfo) error {
		if ii.Source == dep.Missing {
			multiErr.Add(fmt.Errorf("%w: %s %s", ErrPackagesNotFound, name, ii.Version))
		}
		return nil
	})

	if err := multiErr.Return(); err != nil {
		return err
	}
	return opService.Run(ctx, run, cmdArgs, targets, []string{})
}
