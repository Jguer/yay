package workdir

import (
	"context"
	"errors"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/settings/exe"
)

func gitMerge(ctx context.Context, cmdBuilder exe.ICmdBuilder, dir string) error {
	_, stderr, err := cmdBuilder.Capture(
		cmdBuilder.BuildGitCmd(ctx,
			dir, "reset", "--hard", "HEAD"))
	if err != nil {
		return errors.New(gotext.Get("error resetting %s: %s", dir, stderr))
	}

	_, stderr, err = cmdBuilder.Capture(
		cmdBuilder.BuildGitCmd(ctx,
			dir, "merge", "--no-edit", "--ff"))
	if err != nil {
		return errors.New(gotext.Get("error merging %s: %s", dir, stderr))
	}

	return nil
}

func mergePkgbuilds(ctx context.Context, cmdBuilder exe.ICmdBuilder, pkgbuildDirs map[string]string) error {
	for _, dir := range pkgbuildDirs {
		err := gitMerge(ctx, cmdBuilder, dir)
		if err != nil {
			return err
		}
	}

	return nil
}
