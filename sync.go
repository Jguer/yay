package main

import (
	"context"
	"fmt"
	"strings"

	aur "github.com/Jguer/aur"
	alpm "github.com/Jguer/go-alpm/v2"
	mapset "github.com/deckarep/golang-set/v2"
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
	targets := graph.TopoSortedLayers(func(s string, ii *dep.InstallInfo) error {
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

	aurNames := mapset.NewThreadUnsafeSet[string]()
	if len(aurS) != 0 {
		// Use the AUR client to search for AUR packages not currently installed

		noDB := make([]string, 0, len(aurS))

		for _, pkg := range aurS {
			_, name := text.SplitDBFromName(pkg)

			if aurNames.Contains(name) {
				// This package has already been specified
				continue
			}

			aurNames.Add(name)
			if localPkg := dbExecutor.LocalPackage(name); localPkg != nil {
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

		// Check for any missing packages and print errors for any not found
		found := mapset.NewThreadUnsafeSet[string]()
		for i := range remoteAurPkgs {
			found.Add(remoteAurPkgs[i].Name)
		}

		missing := false
		for _, name := range noDB {
			if !found.Contains(name) {
				missing = true
				run.Logger.Errorln(gotext.Get("No AUR package found for"), " ", name)
			}
		}

		// Mimic pacman's behavior by exiting if any packages are missing.
		if missing {
			return nil
		}
	}

	// Include any pending AUR upgrades if requested
	if cmdArgs.ExistsArg("u", "sysupgrade") && run.Cfg.Mode.AtLeastAUR() {
		grapher := dep.NewGrapher(dbExecutor, run.AURClient, false, settings.NoConfirm,
			true, true, cmdArgs.ExistsArg("needed"), run.Logger.Child("grapher"))
		upService := upgrade.NewUpgradeService(
			grapher, run.AURClient, dbExecutor, run.VCSStore,
			run.Cfg, settings.NoConfirm, run.Logger.Child("upgrade"))

		aurData, aurUp, develUp, err := upService.GetAURUpgrades(ctx, false)

		if err == nil {
			processUpgrade := func(slice upgrade.UpSlice) {
				for i := range slice.Up {
					up := &slice.Up[i]
					// Ensure we don't add duplicates
					if aurNames.Contains(up.Name) {
						continue
					}

					aurNames.Add(up.Name)
					// Since these are upgrades, all packages should be local. Check just in case
					if localPkg := dbExecutor.LocalPackage(up.Name); localPkg != nil {
						localAurPkgs = append(localAurPkgs, localPkg)
					} else {
						aurPkg := aurData[up.Name]
						remoteAurPkgs = append(remoteAurPkgs, *aurPkg)
					}
				}
			}

			processUpgrade(develUp)
			processUpgrade(aurUp)
		} else {
			run.Logger.Errorln(err)
		}
	}

	if len(repoS) > 0 || (cmdArgs.ExistsArg("u", "sysupgrade") && run.Cfg.Mode.AtLeastRepo()) {
		// Use pacman to print repo packages and upgrades

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
