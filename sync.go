package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/multierror"
	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/sync"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/upgrade"
)

func syncInstall(ctx context.Context,
	run *runtime.Runtime,
	cmdArgs *parser.Arguments,
	dbExecutor db.Executor,
) error {
	aurCache := run.AURClient
	refreshArg := cmdArgs.ExistsArg("y", "refresh")
	noDeps := cmdArgs.ExistsArg("d", "nodeps")
	noCheck := strings.Contains(run.Cfg.MFlags, "--nocheck")
	if noDeps {
		run.CmdBuilder.AddMakepkgFlag("-d")
	}

	if refreshArg && run.Cfg.Mode.AtLeastRepo() {
		if errR := earlyRefresh(ctx, run.Cfg, run.CmdBuilder, cmdArgs); errR != nil {
			return fmt.Errorf("%s - %w", gotext.Get("error refreshing databases"), errR)
		}

		// we may have done -Sy, our handle now has an old
		// database.
		if errRefresh := dbExecutor.RefreshHandle(); errRefresh != nil {
			return errRefresh
		}
	}

	grapher := dep.NewGrapher(dbExecutor, aurCache, false, settings.NoConfirm,
		noDeps, noCheck, cmdArgs.ExistsArg("needed"), run.Logger.Child("grapher"))

	normalTargets, externalInstallTargets := splitExternalTargets(run.Cfg, dbExecutor, cmdArgs.Targets)
	cmdArgs.Targets = normalTargets

	graph, err := grapher.GraphFromTargets(ctx, nil, cmdArgs.Targets)
	if err != nil {
		return err
	}

	excluded := []string{}
	var upService *upgrade.UpgradeService
	if cmdArgs.ExistsArg("u", "sysupgrade") {
		var errSysUp error

		upService = upgrade.NewUpgradeService(
			grapher, aurCache, dbExecutor, run.VCSStore,
			run.Cfg, settings.NoConfirm, run.Logger.Child("upgrade"))

		graph, errSysUp = upService.GraphUpgrades(ctx,
			graph, cmdArgs.ExistsDouble("u", "sysupgrade"),
			func(*upgrade.Upgrade) bool { return true })
		if errSysUp != nil {
			return errSysUp
		}

		upService.AURWarnings.Print()

		excluded, errSysUp = upService.UserExcludeUpgrades(graph)
		if errSysUp != nil {
			return errSysUp
		}
	}

	opService := sync.NewOperationService(ctx, dbExecutor, run)
	multiErr := &multierror.MultiError{}
	targets := graph.TopoSortedLayers(func(s string, ii *dep.InstallInfo) error {
		if ii.Source == dep.Missing {
			multiErr.Add(fmt.Errorf("%w: %s %s", ErrPackagesNotFound, s, ii.Version))
		}
		return nil
	})

	if err := multiErr.Return(); err != nil {
		return err
	}

	shouldRunMainOps := len(targets) > 0 || (len(externalInstallTargets) == 0 && (upService == nil || !upService.HasSelectedExternalUpgrades()))
	if shouldRunMainOps {
		if err := opService.Run(ctx, run, cmdArgs, targets, excluded); err != nil {
			return err
		}
	}

	if err := runExternalInstallTargets(ctx, run.Cfg, externalInstallTargets); err != nil {
		return err
	}

	if upService != nil {
		if err := upService.RunExternalUpgrades(ctx); err != nil {
			return err
		}
	}

	return nil
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

func splitExternalTargets(cfg *settings.Configuration, dbExecutor db.Executor, targets []string) ([]string, map[string][]settings.ExternalInstallTarget) {
	repoSet := make(map[string]struct{}, len(dbExecutor.Repos()))
	for _, repo := range dbExecutor.Repos() {
		repoSet[repo] = struct{}{}
	}

	normalTargets := make([]string, 0, len(targets))
	externalTargets := make(map[string][]settings.ExternalInstallTarget)
	for _, target := range targets {
		repository, name := text.SplitDBFromName(target)
		if repository == "" || repository == "aur" {
			normalTargets = append(normalTargets, target)
			continue
		}
		if _, ok := repoSet[repository]; ok {
			normalTargets = append(normalTargets, target)
			continue
		}
		if cfg == nil || !cfg.HasExternalProvider(repository) {
			normalTargets = append(normalTargets, target)
			continue
		}

		externalTargets[repository] = append(externalTargets[repository], settings.ExternalInstallTarget{
			Name:       name,
			Repository: repository,
		})
	}

	return normalTargets, externalTargets
}

func runExternalInstallTargets(ctx context.Context, cfg *settings.Configuration, externalTargets map[string][]settings.ExternalInstallTarget) error {
	for repository, targets := range externalTargets {
		handled, err := cfg.InstallExternalPackages(ctx, repository, targets)
		if err != nil {
			return err
		}
		if !handled {
			return fmt.Errorf("no external install provider configured for %s", repository)
		}
	}

	return nil
}
