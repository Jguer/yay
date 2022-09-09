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

	topoSorted := graph.TopoSortedLayerMap()
	fmt.Println(topoSorted, len(topoSorted))

	installer := &Installer{dbExecutor: dbExecutor}

	installer.Install(ctx, topoSorted)

	return nil
}

type Installer struct {
	dbExecutor db.Executor
}

func (installer *Installer) Install(ctx context.Context, targets []map[string]*dep.InstallInfo) error {
	// Reorganize targets into layers of dependencies
	for i := len(targets) - 1; i >= 0; i-- {
		err := installer.handleLayer(ctx, targets[i])
		if err != nil {
			// rollback
			return err
		}
	}

	return nil
}

type MapBySourceAndType map[dep.Source]map[dep.Reason][]string

func (m *MapBySourceAndType) String() string {
	var s string
	for source, reasons := range *m {
		s += fmt.Sprintf("%s: [", source)
		for reason, names := range reasons {
			s += fmt.Sprintf(" %d: [%v] ", reason, names)
		}

		s += "], "
	}

	return s
}

func (installer *Installer) handleLayer(ctx context.Context, layer map[string]*dep.InstallInfo) error {
	// Install layer
	depByTypeAndReason := make(MapBySourceAndType)
	for name, info := range layer {
		if _, ok := depByTypeAndReason[info.Source]; !ok {
			depByTypeAndReason[info.Source] = make(map[dep.Reason][]string)
		}

		depByTypeAndReason[info.Source][info.Reason] = append(depByTypeAndReason[info.Source][info.Reason], name)
	}

	fmt.Printf("%v\n", depByTypeAndReason)

	return nil
}
