package settings

import (
	"os"
	"path/filepath"

	"github.com/leonelquinteros/gotext"
	"github.com/pkg/errors"
)

// configFileName holds the name of the config file.
const configFileName string = "config.json"

// vcsFileName holds the name of the vcs file.
const vcsFileName string = "vcs.json"

const completionFileName string = "completion.cache"

func getConfigPath() (string, error) {
	var configHome string

	if configHome = os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		configHome = filepath.Join(configHome, "yay")
	} else if configHome = os.Getenv("HOME"); configHome != "" {
		configHome = filepath.Join(configHome, ".config", "yay")
	} else {
		return "", errors.New(gotext.Get("%s and %s unset", "XDG_CACHE_HOME", "HOME"))
	}

	if err := initDir(configHome); err != nil {
		return "", err
	}

	return filepath.Join(configHome, configFileName), nil
}

func getCacheHome() string {
	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		if err := initDir(cacheHome); err == nil {
			return filepath.Join(cacheHome, "yay")
		}
	}

	if cacheHome := os.Getenv("HOME"); cacheHome != "" {
		if err := initDir(cacheHome); err == nil {
			return filepath.Join(cacheHome, ".cache", "yay")
		}
	}

	return "/tmp"
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
