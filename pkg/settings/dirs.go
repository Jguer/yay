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

func getConfigPath() string {
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		if err := initDir(configHome); err == nil {
			return filepath.Join(configHome, "yay", configFileName)
		}
	}

	if configHome := os.Getenv("HOME"); configHome != "" {
		if err := initDir(configHome); err == nil {
			return filepath.Join(configHome, ".config", "yay", configFileName)
		}
	}

	return ""
}

func getCacheHome() string {
	uid := os.Geteuid()

	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" && uid != 0 {
		if err := initDir(cacheHome); err == nil {
			return filepath.Join(cacheHome, "yay")
		}
	}

	if cacheHome := os.Getenv("HOME"); cacheHome != "" && uid != 0 {
		if err := initDir(cacheHome); err == nil {
			return filepath.Join(cacheHome, ".cache", "yay")
		}
	}

	return os.TempDir()
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
