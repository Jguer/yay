package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Jguer/yay/v11/pkg/completion"
	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/multierror"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/srcinfo"
	"github.com/Jguer/yay/v11/pkg/text"
	"github.com/Jguer/yay/v11/pkg/upgrade"

	"github.com/leonelquinteros/gotext"
)

func syncInstall(ctx context.Context,
	config *settings.Configuration,
	cmdArgs *parser.Arguments,
	dbExecutor db.Executor,
) error {
	aurCache := config.Runtime.AURCache
	refreshArg := cmdArgs.ExistsArg("y", "refresh")
	noDeps := cmdArgs.ExistsArg("d", "nodeps")
	noCheck := strings.Contains(config.MFlags, "--nocheck")
	if noDeps {
		config.Runtime.CmdBuilder.AddMakepkgFlag("-d")
	}

	if refreshArg && config.Runtime.Mode.AtLeastRepo() {
		if errR := earlyRefresh(ctx, cmdArgs); errR != nil {
			return fmt.Errorf("%s - %w", gotext.Get("error refreshing databases"), errR)
		}

		// we may have done -Sy, our handle now has an old
		// database.
		if errRefresh := dbExecutor.RefreshHandle(); errRefresh != nil {
			return errRefresh
		}
	}

	grapher := dep.NewGrapher(dbExecutor, aurCache, false, settings.NoConfirm,
		noDeps, noCheck, cmdArgs.ExistsArg("needed"), config.Runtime.Logger.Child("grapher"))

	graph, err := grapher.GraphFromTargets(ctx, nil, cmdArgs.Targets)
	if err != nil {
		return err
	}

	if cmdArgs.ExistsArg("u", "sysupgrade") {
		var errSysUp error

		upService := upgrade.NewUpgradeService(
			grapher, aurCache, dbExecutor, config.Runtime.VCSStore,
			config.Runtime, config, settings.NoConfirm, config.Runtime.Logger.Child("upgrade"))

		graph, errSysUp = upService.GraphUpgrades(ctx, graph, cmdArgs.ExistsDouble("u", "sysupgrade"))
		if errSysUp != nil {
			return errSysUp
		}
	}

	opService := NewOperationService(ctx, config, dbExecutor)
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

	return opService.Run(ctx, cmdArgs, targets)
}

type OperationService struct {
	ctx        context.Context
	cfg        *settings.Configuration
	dbExecutor db.Executor
}

func NewOperationService(ctx context.Context, cfg *settings.Configuration, dbExecutor db.Executor) *OperationService {
	return &OperationService{
		ctx:        ctx,
		cfg:        cfg,
		dbExecutor: dbExecutor,
	}
}

func (o *OperationService) Run(ctx context.Context,
	cmdArgs *parser.Arguments,
	targets []map[string]*dep.InstallInfo,
) error {
	if len(targets) == 0 {
		fmt.Fprintln(os.Stdout, "", gotext.Get("there is nothing to do"))
		return nil
	}
	preparer := NewPreparer(o.dbExecutor, o.cfg.Runtime.CmdBuilder, o.cfg)
	installer := NewInstaller(o.dbExecutor, o.cfg.Runtime.CmdBuilder,
		o.cfg.Runtime.VCSStore, o.cfg.Runtime.Mode,
		cmdArgs.ExistsArg("w", "downloadonly"), o.cfg.Runtime.Logger.Child("installer"))

	pkgBuildDirs, errInstall := preparer.Run(ctx, os.Stdout, targets)
	if errInstall != nil {
		return errInstall
	}

	cleanFunc := preparer.ShouldCleanMakeDeps(cmdArgs)
	if cleanFunc != nil {
		installer.AddPostInstallHook(cleanFunc)
	}

	if cleanAURDirsFunc := preparer.ShouldCleanAURDirs(pkgBuildDirs); cleanAURDirsFunc != nil {
		installer.AddPostInstallHook(cleanAURDirsFunc)
	}

	go func() {
		errComp := completion.Update(ctx, o.cfg.Runtime.HTTPClient, o.dbExecutor,
			o.cfg.AURURL, o.cfg.Runtime.CompletionPath, o.cfg.CompletionInterval, false)
		if errComp != nil {
			text.Warnln(errComp)
		}
	}()

	srcInfo, errInstall := srcinfo.NewService(o.dbExecutor, o.cfg, o.cfg.Runtime.CmdBuilder, o.cfg.Runtime.VCSStore, pkgBuildDirs)
	if errInstall != nil {
		return errInstall
	}

	incompatible, errInstall := srcInfo.IncompatiblePkgs(ctx)
	if errInstall != nil {
		return errInstall
	}

	if errIncompatible := confirmIncompatible(incompatible); errIncompatible != nil {
		return errIncompatible
	}

	if errPGP := srcInfo.CheckPGPKeys(ctx); errPGP != nil {
		return errPGP
	}

	if errInstall := installer.Install(ctx, cmdArgs, targets, pkgBuildDirs); errInstall != nil {
		return errInstall
	}

	var multiErr multierror.MultiError

	if err := installer.CompileFailedAndIgnored(); err != nil {
		multiErr.Add(err)
	}

	if !cmdArgs.ExistsArg("w", "downloadonly") {
		if err := srcInfo.UpdateVCSStore(ctx, targets, installer.failedAndIgnored); err != nil {
			text.Warnln(err)
		}
	}

	if err := installer.RunPostInstallHooks(ctx); err != nil {
		multiErr.Add(err)
	}

	return multiErr.Return()
}

func confirmIncompatible(incompatible []string) error {
	if len(incompatible) > 0 {
		text.Warnln(gotext.Get("The following packages are not compatible with your architecture:"))

		for _, pkg := range incompatible {
			fmt.Print("  " + text.Cyan(pkg))
		}

		fmt.Println()

		if !text.ContinueTask(os.Stdin, gotext.Get("Try to build them anyway?"), true, settings.NoConfirm) {
			return &settings.ErrUserAbort{}
		}
	}

	return nil
}
