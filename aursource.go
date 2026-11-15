package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/settings/parser"
	"github.com/Jguer/yay/v13/pkg/text"
)

// handleAURSource implements `yay -Y --aursource <action> [args]`, which lists
// and edits the aururls list stored in config.json.
func handleAURSource(logger *text.Logger, cfg *settings.Configuration,
	cmdArgs *parser.Arguments, configPath string,
) error {
	action, _, _ := cmdArgs.GetArg("aursource")
	args := cmdArgs.Targets

	var mutate func(*settings.Configuration) error

	switch action {
	case "list", "ls":
		printAURSources(logger, cfg)
		return nil
	case "add":
		if len(args) < 2 || len(args) > 3 {
			return errors.New(gotext.Get("usage: yay -Y --aursource add <name> <url> [rpcurl]"))
		}

		rpcURL := ""
		if len(args) == 3 {
			rpcURL = args[2]
		}

		mutate = func(c *settings.Configuration) error { return c.AddAURSource(args[0], args[1], rpcURL) }
	case "edit":
		if len(args) < 2 {
			return errors.New(gotext.Get("usage: yay -Y --aursource edit <name> [name=<new>] [url=<url>] [rpcurl=<url>]"))
		}

		changes := make(map[string]string, len(args)-1)

		for _, kv := range args[1:] {
			key, value, ok := strings.Cut(kv, "=")
			if !ok {
				return errors.New(gotext.Get("expected key=value, got '%s'", kv))
			}

			changes[key] = value
		}

		mutate = func(c *settings.Configuration) error { return c.EditAURSource(args[0], changes) }
	case "delete", "remove", "rm":
		if len(args) != 1 {
			return errors.New(gotext.Get("usage: yay -Y --aursource delete <name>"))
		}

		mutate = func(c *settings.Configuration) error { return c.DeleteAURSource(args[0]) }
	case "move":
		if len(args) != 2 {
			return errors.New(gotext.Get("usage: yay -Y --aursource move <name> <position>"))
		}

		position, err := strconv.Atoi(args[1])
		if err != nil {
			return errors.New(gotext.Get("invalid position '%s'", args[1]))
		}

		mutate = func(c *settings.Configuration) error { return c.MoveAURSource(args[0], position) }
	default:
		return errors.New(gotext.Get("unknown --aursource action '%s' (expected list, add, edit, delete or move)", action))
	}

	saved, err := settings.UpdateConfigFile(configPath, cfg.Version, mutate)
	if err != nil {
		return err
	}

	logger.Println(gotext.Get("Saved AUR sources to %s", configPath))
	printAURSources(logger, saved)

	return nil
}

func printAURSources(logger *text.Logger, cfg *settings.Configuration) {
	if len(cfg.AURURLs) == 0 {
		logger.Println(gotext.Get("No aururls configured; using aururl:"))
	}

	for i, src := range cfg.AURSources() {
		line := fmt.Sprintf("%d %s %s", i+1, text.Bold(src.Name), src.URL)
		if src.RPCURL != src.URL+"/rpc?" {
			line += " " + gotext.Get("(rpc: %s)", strings.TrimSuffix(src.RPCURL, "?"))
		}

		logger.Println(line)
	}
}
