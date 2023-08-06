package workdir

import (
	"context"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
)

func removeMake(ctx context.Context, config *settings.Configuration,
	cmdBuilder exe.ICmdBuilder, makeDeps []string, cmdArgs *parser.Arguments,
) error {
	removeArguments := cmdArgs.CopyGlobal()

	err := removeArguments.AddArg("R", "s", "u")
	if err != nil {
		return err
	}

	for _, pkg := range makeDeps {
		removeArguments.AddTarget(pkg)
	}

	oldValue := settings.NoConfirm
	settings.NoConfirm = true
	err = cmdBuilder.Show(cmdBuilder.BuildPacmanCmd(ctx,
		removeArguments, config.Mode, settings.NoConfirm))
	settings.NoConfirm = oldValue

	return err
}

func cleanAfter(ctx context.Context, run *runtime.Runtime,
	cmdBuilder exe.ICmdBuilder, pkgbuildDirs map[string]string,
) {
	run.Logger.Println(gotext.Get("removing untracked AUR files from cache..."))

	i := 0
	for _, dir := range pkgbuildDirs {
		run.Logger.OperationInfoln(gotext.Get("Cleaning (%d/%d): %s", i+1, len(pkgbuildDirs), text.Cyan(dir)))

		_, stderr, err := cmdBuilder.Capture(
			cmdBuilder.BuildGitCmd(
				ctx, dir, "reset", "--hard", "HEAD"))
		if err != nil {
			run.Logger.Errorln(gotext.Get("error resetting %s: %s", dir, stderr))
		}

		if err := run.CmdBuilder.Show(
			run.CmdBuilder.BuildGitCmd(
				ctx, dir, "clean", "-fx", "--exclude", "*.pkg.*")); err != nil {
			run.Logger.Errorln(err)
		}

		i++
	}
}
