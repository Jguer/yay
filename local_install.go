package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/metadata"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"
	"github.com/pkg/errors"
)

func installLocalPKGBUILD(
	ctx context.Context,
	cmdArgs *parser.Arguments,
	dbExecutor db.Executor,
	ignoreProviders bool,
) error {
	aurCache, err := metadata.NewAURCache(filepath.Join(config.BuildDir, "aur.json"))
	if err != nil {
		return errors.Wrap(err, gotext.Get("failed to retrieve aur Cache"))
	}

	wd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, gotext.Get("failed to retrieve working directory"))
	}

	if len(cmdArgs.Targets) > 1 {
		return errors.New(gotext.Get("only one target is allowed"))
	}

	if len(cmdArgs.Targets) == 1 {
		wd = cmdArgs.Targets[0]
	}

	pkgbuild, err := gosrc.ParseFile(filepath.Join(wd, ".SRCINFO"))
	if err != nil {
		return errors.Wrap(err, gotext.Get("failed to parse .SRCINFO"))
	}

	grapher := dep.NewGrapher(dbExecutor, aurCache, false, settings.NoConfirm, os.Stdout)

	graph, err := grapher.GraphFromSrcInfo(pkgbuild)
	if err != nil {
		return err
	}

	fmt.Println(graph)
	// aurCache.DebugInfo()

	topoSorted := graph.TopoSortedLayers()
	fmt.Println(topoSorted, len(topoSorted))

	return nil
}
