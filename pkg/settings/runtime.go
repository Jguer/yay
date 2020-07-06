package settings

import (
	"os"
	"path/filepath"

	"github.com/Jguer/go-alpm"
	"github.com/Morganamilo/go-pacmanconf"
	"github.com/leonelquinteros/gotext"
	"github.com/pkg/errors"
)

type TargetMode int

// configFileName holds the name of the config file.
const configFileName string = "config.json"

// vcsFileName holds the name of the vcs file.
const vcsFileName string = "vcs.json"

const completionFileName string = "completion.cache"

const (
	ModeAny TargetMode = iota
	ModeAUR
	ModeRepo
)

type Runtime struct {
	Mode           TargetMode
	SaveConfig     bool
	CompletionPath string
	ConfigPath     string
	VCSPath        string
	PacmanConf     *pacmanconf.Config
	AlpmHandle     *alpm.Handle
}

func MakeRuntime() (*Runtime, error) {
	cacheHome := ""
	configHome := ""

	runtime := &Runtime{
		Mode:           ModeAny,
		SaveConfig:     false,
		CompletionPath: "",
	}

	if configHome = os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		configHome = filepath.Join(configHome, "yay")
	} else if configHome = os.Getenv("HOME"); configHome != "" {
		configHome = filepath.Join(configHome, ".config", "yay")
	} else {
		return nil, errors.New(gotext.Get("%s and %s unset", "XDG_CONFIG_HOME", "HOME"))
	}

	if err := initDir(configHome); err != nil {
		return nil, err
	}

	if cacheHome = os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		cacheHome = filepath.Join(cacheHome, "yay")
	} else if cacheHome = os.Getenv("HOME"); cacheHome != "" {
		cacheHome = filepath.Join(cacheHome, ".cache", "yay")
	} else {
		return nil, errors.New(gotext.Get("%s and %s unset", "XDG_CACHE_HOME", "HOME"))
	}

	if err := initDir(cacheHome); err != nil {
		return runtime, err
	}

	runtime.ConfigPath = filepath.Join(configHome, configFileName)
	runtime.VCSPath = filepath.Join(cacheHome, vcsFileName)
	runtime.CompletionPath = filepath.Join(cacheHome, completionFileName)

	return runtime, nil
}

func initDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0o755); err != nil {
			return errors.New(gotext.Get("failed to create config directory '%s': %s", dir, err))
		}
	} else if err != nil {
		return err
	}

	return nil
}
