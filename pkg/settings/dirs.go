package settings

import (
	"os"
	"path/filepath"
)

const (
	configFileName     string = "config.json" // configFileName holds the name of the config file.
	iniConfigFileName  string = "yay.conf"    // iniConfigFileName holds the name of the INI config file.
	luaConfigFileName  string = "init.lua"    // luaConfigFileName holds the name of the experimental Lua config file.
	vcsFileName        string = "vcs.json"    // vcsFileName holds the name of the vcs file.
	completionFileName string = "completion.cache"
	systemdCache       string = "/var/cache/yay" // systemd should handle cache creation
)

func GetConfigPath() string {
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

// GetINIConfigPath returns the path to the user's INI config file (yay.conf).
// This is used for both loading (with priority over JSON) and saving.
func GetINIConfigPath() string {
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		configDir := filepath.Join(configHome, "yay")
		if err := initDir(configDir); err == nil {
			return filepath.Join(configDir, iniConfigFileName)
		}
	}

	if configHome := os.Getenv("HOME"); configHome != "" {
		configDir := filepath.Join(configHome, ".config", "yay")
		if err := initDir(configDir); err == nil {
			return filepath.Join(configDir, iniConfigFileName)
		}
	}

	return ""
}

// GetLuaConfigPath returns the path to the user's experimental Lua config
// file (init.lua). The directory is NOT created on lookup so that callers can
// cheaply check for existence; use os.Stat to test before opening.
func GetLuaConfigPath() string {
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		return filepath.Join(configHome, "yay", luaConfigFileName)
	}

	if configHome := os.Getenv("HOME"); configHome != "" {
		return filepath.Join(configHome, ".config", "yay", luaConfigFileName)
	}

	return ""
}

// ResolveLuaConfigPath returns the first existing Lua config path.
//
// Resolution order:
//  1. ./init.lua in the current working directory
//  2. $XDG_CONFIG_HOME/yay/init.lua
//  3. $HOME/.config/yay/init.lua
func ResolveLuaConfigPath() (string, error) {
	if wd, err := os.Getwd(); err == nil {
		cwdPath := filepath.Join(wd, luaConfigFileName)
		if _, err := os.Stat(cwdPath); err == nil {
			return cwdPath, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
	} else {
		return "", err
	}

	configPath := GetLuaConfigPath()
	if configPath == "" {
		return "", nil
	}

	if _, err := os.Stat(configPath); err == nil {
		return configPath, nil
	} else if os.IsNotExist(err) {
		return "", nil
	} else {
		return "", err
	}
}

func getCacheHome() (string, error) {
	uid := os.Geteuid()

	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" && uid != 0 {
		cacheDir := filepath.Join(cacheHome, "yay")
		if err := initDir(cacheDir); err == nil {
			return cacheDir, nil
		}
	}

	if cacheHome := os.Getenv("HOME"); cacheHome != "" && uid != 0 {
		cacheDir := filepath.Join(cacheHome, ".cache", "yay")
		if err := initDir(cacheDir); err == nil {
			return cacheDir, nil
		}
	}

	if uid == 0 && os.Getenv("SUDO_USER") == "" && os.Getenv("DOAS_USER") == "" {
		return systemdCache, nil // Don't create directory if systemd-run takes care of it
	}

	tmpDir := filepath.Join(os.TempDir(), "yay")

	return tmpDir, initDir(tmpDir)
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
