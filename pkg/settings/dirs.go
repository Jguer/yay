package settings

import (
	"os"
	"path/filepath"
)

// configFileName holds the name of the config file.
const configFileName string = "config.json"

// vcsFileName holds the name of the vcs file.
const vcsFileName string = "vcs.json"

const completionFileName string = "completion.cache"

func getConfigPath() string {
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		configDir := filepath.Join(configHome, "yay")
		if err := initDir(configDir); err == nil {
			return filepath.Join(configDir, configFileName)
		}
	}

	if configHome := os.Getenv("HOME"); configHome != "" {
		configDir := filepath.Join(configHome, ".config", "yay")
		if err := initDir(configDir); err == nil {
			return filepath.Join(configDir, configFileName)
		}
	}

	return ""
}

func getCacheHome() string {
	uid := os.Geteuid()

	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" && uid != 0 {
		cacheDir := filepath.Join(cacheHome, "yay")
		if err := initDir(cacheDir); err == nil {
			return cacheDir
		}
	}

	if cacheHome := os.Getenv("HOME"); cacheHome != "" && uid != 0 {
		cacheDir := filepath.Join(cacheHome, ".cache", "yay")
		if err := initDir(cacheDir); err == nil {
			return cacheDir
		}
	}

	if uid == 0 && os.Getenv("SUDO_USER") == "" && os.Getenv("DOAS_USER") == "" {
		return "/var/cache/yay" // Don't create directory if systemd-run takes care of it
	}

	return os.TempDir()
}

func initDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0o755); err != nil {
			return &ErrRuntimeDir{inner: err, dir: dir}
		}
	} else if err != nil {
		return err
	}

	return nil
}
