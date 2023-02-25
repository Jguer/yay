// Experimental code for install local with dependency refactoring
// Not at feature parity with install.go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/multierror"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/topo"

	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"
	"github.com/pkg/errors"
)

var (
	ErrInstallRepoPkgs = errors.New(gotext.Get("error installing repo packages"))
	ErrNoBuildFiles    = errors.New(gotext.Get("cannot find PKGBUILD and .SRCINFO in directory"))
)

func srcinfoExists(ctx context.Context,
	cmdBuilder exe.ICmdBuilder, targetDir string,
) error {
	if _, err := os.Stat(filepath.Join(targetDir, ".SRCINFO")); err == nil {
		if _, err := os.Stat(filepath.Join(targetDir, "PKGBUILD")); err == nil {
			return nil
		}
	}

	if _, err := os.Stat(filepath.Join(targetDir, "PKGBUILD")); err == nil {
		// run makepkg to generate .SRCINFO
		srcinfo, stderr, err := cmdBuilder.Capture(cmdBuilder.BuildMakepkgCmd(ctx, targetDir, "--printsrcinfo"))
		if err != nil {
			return fmt.Errorf("unable to generate SRCINFO: %w - %s", err, stderr)
		}

		if err := os.WriteFile(filepath.Join(targetDir, ".SRCINFO"), []byte(srcinfo), 0o600); err != nil {
			return fmt.Errorf("unable to write SRCINFO: %w", err)
		}

		return nil
	}

	return fmt.Errorf("%w: %s", ErrNoBuildFiles, targetDir)
}

func installLocalPKGBUILD(
	ctx context.Context,
	config *settings.Configuration,
	cmdArgs *parser.Arguments,
	dbExecutor db.Executor,
) error {
	aurCache := config.Runtime.AURCache
	noCheck := strings.Contains(config.MFlags, "--nocheck")

	if len(cmdArgs.Targets) < 1 {
		return errors.New(gotext.Get("no target directories specified"))
	}

	grapher := dep.NewGrapher(dbExecutor, aurCache, false, settings.NoConfirm,
		cmdArgs.ExistsDouble("d", "nodeps"), noCheck, cmdArgs.ExistsArg("needed"),
		config.Runtime.Logger.Child("grapher"))
	graph := topo.New[string, *dep.InstallInfo]()
	for _, target := range cmdArgs.Targets {
		if err := srcinfoExists(ctx, config.Runtime.CmdBuilder, target); err != nil {
			return err
		}

		pkgbuild, err := gosrc.ParseFile(filepath.Join(target, ".SRCINFO"))
		if err != nil {
			return errors.Wrap(err, gotext.Get("failed to parse .SRCINFO"))
		}

		var errG error
		graph, errG = grapher.GraphFromSrcInfo(ctx, graph, target, pkgbuild)
		if errG != nil {
			return errG
		}
	}

	opService := NewOperationService(ctx, config, dbExecutor)
	multiErr := &multierror.MultiError{}
	targets := graph.TopoSortedLayerMap(func(name string, ii *dep.InstallInfo) error {
		if ii.Source == dep.Missing {
			multiErr.Add(errors.New(gotext.Get("could not find %s%s", name, ii.Version)))
		}
		return nil
	})

	if err := multiErr.Return(); err != nil {
		return err
	}
	return opService.Run(ctx, cmdArgs, targets)
}
