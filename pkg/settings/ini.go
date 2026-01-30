package settings

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/ini.v1"
)

// SystemConfigPath is the path to the system-wide INI configuration file.
const SystemConfigPath = "/etc/yay.conf"

// loadINI parses an INI configuration file and applies values to the Configuration.
// It silently returns nil if the file doesn't exist.
// Uses struct tags for mapping (e.g., `ini:"AurUrl"`).
func (c *Configuration) loadINI(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	cfg, err := ini.LoadSources(ini.LoadOptions{
		AllowBooleanKeys:    true,
		Insensitive:         true,
		InsensitiveSections: true,
		IgnoreInlineComment: true,
	}, path)
	if err != nil {
		return fmt.Errorf("failed to load INI config file '%s': %w", path, err)
	}

	// Map the default section to the config struct
	if err := cfg.Section("").MapTo(c); err != nil {
		return fmt.Errorf("failed to map INI config '%s': %w", path, err)
	}

	// Also map [options] section if present (for compatibility)
	if cfg.HasSection("options") {
		if err := cfg.Section("options").MapTo(c); err != nil {
			return fmt.Errorf("failed to map INI [options] section '%s': %w", path, err)
		}
	}

	return nil
}

// SaveINI writes the configuration to an INI file at the specified path.
func (c *Configuration) SaveINI(path string) error {
	cfg := ini.Empty(ini.LoadOptions{
		AllowBooleanKeys: true,
	})

	// Use [options] section for compatibility with system config
	section, err := cfg.NewSection("options")
	if err != nil {
		return fmt.Errorf("failed to create INI section: %w", err)
	}

	if err := section.ReflectFrom(c); err != nil {
		return fmt.Errorf("failed to reflect config to INI: %w", err)
	}

	// Ensure parent directory exists
	if dir := filepath.Dir(path); dir != "" {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
				return fmt.Errorf("failed to create config directory: %w", mkErr)
			}
		}
	}

	return cfg.SaveTo(path)
}
