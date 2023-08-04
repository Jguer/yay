package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Jguer/yay/v12/pkg/completion"
	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/multierror"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/srcinfo"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/upgrade"

	"github.com/leonelquinteros/gotext"
)

func syncInstall(ctx context.Context,
	run *settings.Runtime,
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

	opService := NewOperationService(ctx, dbExecutor, run)
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

type OperationService struct {
	ctx        context.Context
	cfg        *settings.Configuration
	dbExecutor db.Executor
	logger     *text.Logger
}

func NewOperationService(ctx context.Context,
	dbExecutor db.Executor,
	run *settings.Runtime,
) *OperationService {
	return &OperationService{
		ctx:        ctx,
		cfg:        run.Cfg,
		dbExecutor: dbExecutor,
		logger:     run.Logger.Child("operation"),
	}
}

func (o *OperationService) Run(ctx context.Context, run *settings.Runtime,
	cmdArgs *parser.Arguments,
	targets []map[string]*dep.InstallInfo, excluded []string,
) error {
	if len(targets) == 0 {
		fmt.Fprintln(os.Stdout, "", gotext.Get("there is nothing to do"))
		return nil
	}
	preparer := NewPreparer(o.dbExecutor, run.CmdBuilder, o.cfg)
	installer := NewInstaller(o.dbExecutor, run.CmdBuilder,
		run.VCSStore, o.cfg.Mode, o.cfg.ReBuild,
		cmdArgs.ExistsArg("w", "downloadonly"), run.Logger.Child("installer"))

	pkgBuildDirs, errInstall := preparer.Run(ctx, run, os.Stdout, targets)
	if errInstall != nil {
		return errInstall
	}

	if cleanFunc := preparer.ShouldCleanMakeDeps(run, cmdArgs); cleanFunc != nil {
		installer.AddPostInstallHook(cleanFunc)
	}

	if cleanAURDirsFunc := preparer.ShouldCleanAURDirs(run, pkgBuildDirs); cleanAURDirsFunc != nil {
		installer.AddPostInstallHook(cleanAURDirsFunc)
	}

	go func() {
		errComp := completion.Update(ctx, run.HTTPClient, o.dbExecutor,
			o.cfg.AURURL, o.cfg.CompletionPath, o.cfg.CompletionInterval, false)
		if errComp != nil {
			text.Warnln(errComp)
		}
	}()

	srcInfo, errInstall := srcinfo.NewService(o.dbExecutor, o.cfg, run.CmdBuilder, run.VCSStore, pkgBuildDirs)
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

	if errInstall := installer.Install(ctx, cmdArgs, targets, pkgBuildDirs,
		excluded, o.manualConfirmRequired(cmdArgs)); errInstall != nil {
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

func (o *OperationService) manualConfirmRequired(cmdArgs *parser.Arguments) bool {
	return (!cmdArgs.ExistsArg("u", "sysupgrade") && cmdArgs.Op != "Y") || o.cfg.DoubleConfirm
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
