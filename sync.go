package main

import (
	"context"
	"fmt"
	"strings"

	aur "github.com/Jguer/aur"
	alpm "github.com/Jguer/go-alpm/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/multierror"
	"github.com/Jguer/yay/v12/pkg/query"
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
	noDeps := cmdArgs.ExistsArg("d", "nodeps")
	noCheck := strings.Contains(run.Cfg.MFlags, "--nocheck")
	if noDeps {
		run.CmdBuilder.AddMakepkgFlag("-d")
	}

	if err := earlyRefreshIfNeeded(ctx, run, cmdArgs, dbExecutor); err != nil {
		return err
	}

	grapher := dep.NewGrapher(dbExecutor, aurCache, false, settings.NoConfirm,
		noDeps, noCheck, cmdArgs.ExistsArg("needed"), run.Logger.Child("grapher"))

	graph, err := grapher.GraphFromTargets(ctx, nil, cmdArgs.Targets)
	if err != nil {
		return err
	}

	excluded := []string{}
	if cmdArgs.ExistsArg("u", "sysupgrade") {
		var errSysUp error

		upService := upgrade.NewUpgradeService(
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
	targets := graph.TopoSortedLayerMap(func(s string, ii *dep.InstallInfo) error {
		if ii.Source == dep.Missing {
			multiErr.Add(fmt.Errorf("%w: %s %s", ErrPackagesNotFound, s, ii.Version))
		}
		return nil
	})

	if err := multiErr.Return(); err != nil {
		return err
	}

	return opService.Run(ctx, run, cmdArgs, targets, excluded)
}

func syncPrint(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments,
	dbExecutor db.Executor,
) error {
	var (
		remoteAurPkgs []aur.Pkg
		localAurPkgs  []alpm.IPackage
		err           error
	)

	if err := earlyRefreshIfNeeded(ctx, run, cmdArgs, dbExecutor); err != nil {
		return err
	}

	pkgS := query.RemoveInvalidTargets(run.Logger, cmdArgs.Targets, run.Cfg.Mode)
	pkgS = ExpandPackages(pkgS, dbExecutor)
	aurS, repoS := PackageSlices(pkgS, run.Cfg, dbExecutor)

	if len(aurS) != 0 {
		// Use the AUR client to search for AUR packages not currently installed

		noDB := make([]string, 0, len(aurS))

		for _, pkg := range aurS {
			_, name := text.SplitDBFromName(pkg)

			localPkg := dbExecutor.LocalPackage(name)
			if localPkg != nil {
				localAurPkgs = append(localAurPkgs, localPkg)
			} else {
				noDB = append(noDB, name)
			}
		}

		remoteAurPkgs, err = run.AURClient.Get(ctx, &aur.Query{
			Needles: noDB,
			By:      aur.Name,
		})
		if err != nil {
			run.Logger.Errorln(err)
		}

		// Check for any missing packages, print errors for any not found
		found := make(map[string]struct{}, len(remoteAurPkgs))
		for i := range remoteAurPkgs {
			found[remoteAurPkgs[i].Name] = struct{}{}
		}

		missing := false
		for _, name := range noDB {
			if _, ok := found[name]; !ok {
				missing = true
				run.Logger.Errorln(gotext.Get("No AUR package found for"), " ", name)
			}
		}

		// Mimic pacman's behavior by exiting if any packages are missing.
		if missing {
			return nil
		}
	}

	if len(repoS) > 0 {
		// Use pacman to print repo packages

		arguments := cmdArgs.Copy()
		// If this argument is present, we already refreshed the databases. Remove so pacman doesn't
		// do it again.
		arguments.DelArg("y", "refresh")
		arguments.ClearTargets()
		arguments.AddTarget(repoS...)

		if err := run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx, arguments,
			run.Cfg.Mode, settings.NoConfirm)); err != nil {
			return err
		}
	}

	printLocalPackages(run.Cfg, localAurPkgs)
	printAurPackages(run.Cfg, remoteAurPkgs)

	return nil
}

func earlyRefreshIfNeeded(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments,
	dbExecutor db.Executor,
) error {
	refreshArg := cmdArgs.ExistsArg("y", "refresh")
	if !refreshArg || !run.Cfg.Mode.AtLeastRepo() {
		return nil
	}

	if errR := earlyRefresh(ctx, run.Cfg, run.CmdBuilder, cmdArgs); errR != nil {
		return fmt.Errorf("%s - %w", gotext.Get("error refreshing databases"), errR)
	}

	// we may have done -Sy, our handle now has an old
	// database.
	return dbExecutor.RefreshHandle()
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
