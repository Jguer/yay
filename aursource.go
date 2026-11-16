package main

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v13/pkg/aursource"
	"github.com/Jguer/yay/v13/pkg/db"
	"github.com/Jguer/yay/v13/pkg/runtime"
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/settings/parser"
	"github.com/Jguer/yay/v13/pkg/text"
)

// handleAURSource implements `yay -Y --aursource <action> [args]`, which lists
// and edits the aururls list stored in config.json.
func handleAURSource(logger *text.Logger, cfg *settings.Configuration,
	cmdArgs *parser.Arguments, configPath string, installed func(string) bool,
) error {
	action, _, _ := cmdArgs.GetArg("aursource")
	args := cmdArgs.Targets

	var (
		mutate    func(*settings.Configuration) error
		afterSave func() error
	)

	switch action {
	case "list", "ls":
		printAURSources(logger, cfg)
		return nil
	case "pins":
		if len(args) != 0 {
			return errors.New(gotext.Get("usage: yay -Y --aursource pins"))
		}

		return printAURPins(logger, cfg, installed)
	case "unpin":
		if len(args) == 0 {
			return errors.New(gotext.Get("usage: yay -Y --aursource unpin <package>..."))
		}

		return unpinAURSources(logger, cfg, args)
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

		if newName, ok := changes["name"]; ok && newName != args[0] {
			afterSave = func() error {
				return updateAURPins(cfg.AURPinsPath, func(f *aursource.PinFile) {
					for _, pkg := range f.RenameSource(args[0], newName) {
						logger.Println(gotext.Get("Updated the source of %s to '%s'", pkg, newName))
					}
				})
			}
		}
	case "delete", "remove", "rm":
		if len(args) != 1 {
			return errors.New(gotext.Get("usage: yay -Y --aursource delete <name>"))
		}

		mutate = func(c *settings.Configuration) error { return c.DeleteAURSource(args[0]) }
		afterSave = func() error {
			return updateAURPins(cfg.AURPinsPath, func(f *aursource.PinFile) {
				if dropped := f.DropSource(args[0]); len(dropped) > 0 {
					logger.Println(text.Bold("::"), gotext.Get(
						"%s no longer pinned (installed from deleted source '%s'); they will be upgraded from the normal source order",
						strings.Join(dropped, " "), args[0]))
				}
			})
		}
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
		return errors.New(gotext.Get("unknown --aursource action '%s' (expected list, add, edit, delete, move, pins or unpin)", action))
	}

	saved, err := settings.UpdateConfigFile(configPath, cfg.Version, mutate)
	if err != nil {
		return err
	}

	logger.Println(gotext.Get("Saved AUR sources to %s", configPath))
	printAURSources(logger, saved)

	if afterSave != nil {
		return afterSave()
	}

	return nil
}

// updateAURPins applies fn to the pin file at path and saves it if needed.
func updateAURPins(path string, fn func(*aursource.PinFile)) error {
	if path == "" {
		return nil
	}

	file, err := aursource.LoadPinFile(path)
	if err != nil {
		return err
	}

	before := maps.Clone(file.Packages)
	fn(file)

	if maps.Equal(before, file.Packages) {
		return nil
	}

	return file.Save(path)
}

func printAURPins(logger *text.Logger, cfg *settings.Configuration, installed func(string) bool) error {
	file, err := aursource.LoadPinFile(cfg.AURPinsPath)
	if err != nil {
		return err
	}

	if len(file.Packages) == 0 {
		logger.Println(gotext.Get("No packages are pinned to an AUR source"))
		return nil
	}

	configured := make(map[string]bool)
	for _, src := range cfg.AURSources() {
		configured[src.Name] = true
	}

	for _, name := range file.Names() {
		source := file.Packages[name]

		var notes []string
		if !configured[source] {
			notes = append(notes, gotext.Get("source not configured"))
		}

		if installed != nil && !installed(name) {
			notes = append(notes, gotext.Get("not installed"))
		}

		line := name + " " + text.Bold(source)
		if len(notes) > 0 {
			line += " (" + strings.Join(notes, ", ") + ")"
		}

		logger.Println(line)
	}

	return nil
}

func unpinAURSources(logger *text.Logger, cfg *settings.Configuration, pkgs []string) error {
	var missing []string

	err := updateAURPins(cfg.AURPinsPath, func(f *aursource.PinFile) {
		for _, pkg := range pkgs {
			if _, ok := f.Packages[pkg]; !ok {
				missing = append(missing, pkg)
				continue
			}

			delete(f.Packages, pkg)
			logger.Println(gotext.Get("%s is no longer pinned", pkg))
		}
	})
	if err != nil {
		return err
	}

	if len(missing) > 0 {
		return errors.New(gotext.Get("not pinned: %s", strings.Join(missing, " ")))
	}

	return nil
}

// loadAURPins makes installed packages resolve from the AUR source they were
// installed from. Packages whose source is gone are warned about on
// operations that look for upgrades.
func loadAURPins(run *runtime.Runtime, cmdArgs *parser.Arguments, dbExecutor db.Executor) {
	if run.AURSources == nil || run.Cfg.AURPinsPath == "" ||
		cmdArgs.ExistsArg("aursource") {
		return
	}

	file, err := aursource.LoadPinFile(run.Cfg.AURPinsPath)
	if err != nil {
		run.Logger.Warnln(err)
		return
	}

	missing := run.AURSources.LoadPins(file, func(name string) bool {
		return dbExecutor.LocalPackage(name) != nil
	})

	upgrading := cmdArgs.Op == "S" || cmdArgs.Op == "sync" || cmdArgs.Op == "Y" || cmdArgs.Op == "yay" ||
		((cmdArgs.Op == "Q" || cmdArgs.Op == "query") && cmdArgs.ExistsArg("u", "upgrades"))
	if !upgrading {
		return
	}

	names := make([]string, 0, len(missing))
	for name := range missing {
		names = append(names, name)
	}

	slices.Sort(names)

	for _, name := range names {
		run.Logger.Warnln(gotext.Get(
			"%s was installed from AUR source '%s', which is no longer configured; using the normal source order (see yay -Y --aursource unpin)",
			name, missing[name]))
	}
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
