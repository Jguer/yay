package main

import (
	"context"

	"github.com/Jguer/yay/v13/pkg/dep"
	"github.com/Jguer/yay/v13/pkg/dep/topo"
	"github.com/Jguer/yay/v13/pkg/runtime"
	"github.com/Jguer/yay/v13/pkg/settings/parser"
	"github.com/Jguer/yay/v13/pkg/upgrade"
)

func minAgeApplies(cmdArgs *parser.Arguments) bool {
	return cmdArgs.ExistsArg("u", "sysupgrade")
}

func minAgeAdjuster(run *runtime.Runtime, dryRun bool) *upgrade.MinAgeAdjuster {
	return &upgrade.MinAgeAdjuster{
		CacheDirs:  run.PacmanConf.CacheDir,
		BuildDir:   run.Cfg.BuildDir,
		AURURL:     run.Cfg.AURURL,
		PacmanBin:  run.Cfg.PacmanBin,
		CmdBuilder: run.CmdBuilder,
		DryRun:     dryRun,
	}
}

func applyMinAge(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments,
	graph *topo.Graph[string, *dep.InstallInfo], dryRun bool,
) []string {
	if run.Cfg.MinAge <= 0 {
		return nil
	}

	if !dryRun && !minAgeApplies(cmdArgs) {
		return nil
	}

	report, ignore := upgrade.ApplyMinAge(ctx, graph, run.Cfg.MinAge, minAgeAdjuster(run, dryRun))
	upgrade.PrintMinAgeReport(run.Logger, report, run.Cfg.MinAge)

	return ignore
}
