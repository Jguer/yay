package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Jguer/yay/v11/pkg/db/ialpm"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/metadata"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/text"
	"github.com/leonelquinteros/gotext"
	"github.com/pkg/errors"
)

func splitDep(dep string) (pkg, mod, ver string) {
	split := strings.FieldsFunc(dep, func(c rune) bool {
		match := c == '>' || c == '<' || c == '='

		if match {
			mod += string(c)
		}

		return match
	})

	if len(split) == 0 {
		return "", "", ""
	}

	if len(split) == 1 {
		return split[0], "", ""
	}

	return split[0], mod, split[1]
}

func handleCmd() error {
	config, err := settings.NewConfig("")
	if err != nil {
		return err
	}

	cmdArgs := parser.MakeArguments()
	if err := config.ParseCommandLine(cmdArgs); err != nil {
		return err
	}

	pacmanConf, _, err := settings.RetrievePacmanConfig(cmdArgs, config.PacmanConf)
	if err != nil {
		return err
	}

	dbExecutor, err := ialpm.NewExecutor(pacmanConf)
	if err != nil {
		return err
	}

	aurCache, err := metadata.NewAURCache(filepath.Join(config.BuildDir, "aur.json"))
	if err != nil {
		return errors.Wrap(err, gotext.Get("failed to retrieve aur Cache"))
	}

	grapher := dep.NewGrapher(dbExecutor, aurCache, true, settings.NoConfirm, os.Stdout)

	return graphPackage(grapher, cmdArgs.Targets)
}

func main() {
	if err := handleCmd(); err != nil {
		text.Errorln(err)
		os.Exit(1)
	}
}

func graphPackage(
	grapher *dep.Grapher,
	targets []string,
) error {
	if len(targets) != 1 {
		return errors.New(gotext.Get("only one target is allowed"))
	}

	graph, err := grapher.GraphFromAURCache(nil, []string{targets[0]})
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, graph.String())
	fmt.Fprintln(os.Stdout, graph.TopoSortedLayers())
	fmt.Fprintln(os.Stdout, graph.TopoSorted())

	return nil
}
