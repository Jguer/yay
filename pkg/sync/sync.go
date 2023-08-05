package sync

import (
	"context"
	"fmt"
	"os"

	"github.com/Jguer/yay/v12/pkg/completion"
	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/multierror"
	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/sync/build"
	"github.com/Jguer/yay/v12/pkg/sync/srcinfo"
	"github.com/Jguer/yay/v12/pkg/sync/workdir"
	"github.com/Jguer/yay/v12/pkg/text"

	"github.com/leonelquinteros/gotext"
)

type OperationService struct {
	ctx        context.Context
	cfg        *settings.Configuration
	dbExecutor db.Executor
	logger     *text.Logger
}

func NewOperationService(ctx context.Context,
	dbExecutor db.Executor,
	run *runtime.Runtime,
) *OperationService {
	return &OperationService{
		ctx:        ctx,
		cfg:        run.Cfg,
		dbExecutor: dbExecutor,
		logger:     run.Logger.Child("operation"),
	}
}

func (o *OperationService) Run(ctx context.Context, run *runtime.Runtime,
	cmdArgs *parser.Arguments,
	targets []map[string]*dep.InstallInfo, excluded []string,
) error {
	if len(targets) == 0 {
		o.logger.Println("", gotext.Get("there is nothing to do"))
		return nil
	}
	preparer := workdir.NewPreparer(o.dbExecutor, run.CmdBuilder, o.cfg)
	installer := build.NewInstaller(o.dbExecutor, run.CmdBuilder,
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

	srcInfo, errInstall := srcinfo.NewService(o.dbExecutor, o.cfg,
		o.logger.Child("srcinfo"), run.CmdBuilder, run.VCSStore, pkgBuildDirs)
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

	failedAndIgnored, err := installer.CompileFailedAndIgnored()
	if err != nil {
		multiErr.Add(err)
	}

	if !cmdArgs.ExistsArg("w", "downloadonly") {
		if err := srcInfo.UpdateVCSStore(ctx, targets, failedAndIgnored); err != nil {
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
