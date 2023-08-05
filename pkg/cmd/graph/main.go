package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Jguer/yay/v12/pkg/db/ialpm"
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"

	"github.com/Jguer/aur/metadata"
	"github.com/leonelquinteros/gotext"
	"github.com/pkg/errors"
)

func handleCmd() error {
	cfg, err := settings.NewConfig(settings.GetConfigPath(), "")
	if err != nil {
		return err
	}

	cmdArgs := parser.MakeArguments()
	if errP := cfg.ParseCommandLine(cmdArgs); errP != nil {
		return errP
	}

	run, err := runtime.BuildRuntime(cfg, cmdArgs, "1.0.0")
	if err != nil {
		return err
	}

	// Reload CmdBuilder
	run.CmdBuilder = run.Cfg.CmdBuilder(nil)

	pacmanConf, _, err := settings.RetrievePacmanConfig(cmdArgs, cfg.PacmanConf)
	if err != nil {
		return err
	}

	dbExecutor, err := ialpm.NewExecutor(pacmanConf, text.GlobalLogger)
	if err != nil {
		return err
	}

	aurCache, err := metadata.New(
		metadata.WithCacheFilePath(
			filepath.Join(cfg.BuildDir, "aur.json")))
	if err != nil {
		return errors.Wrap(err, gotext.Get("failed to retrieve aur Cache"))
	}

	grapher := dep.NewGrapher(dbExecutor, aurCache, true, settings.NoConfirm,
		cmdArgs.ExistsDouble("d", "nodeps"), false, false,
		run.Logger.Child("grapher"))

	return graphPackage(context.Background(), grapher, cmdArgs.Targets)
}

func main() {
	if err := handleCmd(); err != nil {
		text.Errorln(err)
		os.Exit(1)
	}
}

func graphPackage(
	ctx context.Context,
	grapher *dep.Grapher,
	targets []string,
) error {
	if len(targets) != 1 {
		return errors.New(gotext.Get("only one target is allowed"))
	}

	graph, err := grapher.GraphFromAUR(ctx, nil, []string{targets[0]})
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, graph.String())
	fmt.Fprintln(os.Stdout, "\nlayers map\n", graph.TopoSortedLayerMap(nil))

	return nil
}
