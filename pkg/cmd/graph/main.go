package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"

	"github.com/leonelquinteros/gotext"
	"github.com/pkg/errors"
)

func handleCmd(logger *text.Logger) error {
	cfg, err := settings.NewConfig(logger, settings.GetConfigPath(), "")
	if err != nil {
		return err
	}

	cmdArgs := parser.MakeArguments()
	if errP := cfg.ParseCommandLine(cmdArgs); errP != nil {
		return errP
	}

	run, err := runtime.NewRuntime(cfg, cmdArgs, "1.0.0")
	if err != nil {
		return err
	}

	return graphPackage(context.Background(), run.Grapher, cmdArgs.Targets)
}

func main() {
	fallbackLog := text.NewLogger(os.Stdout, os.Stderr, os.Stdin, false, "fallback")
	if err := handleCmd(fallbackLog); err != nil {
		fallbackLog.Errorln(err)
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

	graph, err := grapher.GraphFromTargets(ctx, nil, []string{targets[0]})
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, graph.String())
	fmt.Fprintln(os.Stdout, "\nlayers map\n", graph.TopoSortedLayerMap(nil))

	return nil
}
