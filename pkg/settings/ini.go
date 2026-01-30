package settings

import (
	"fmt"
	"os"

	"gopkg.in/ini.v1"
)

// SystemConfigPath is the path to the system-wide INI configuration file.
const SystemConfigPath = "/etc/yay.conf"

// loadINI parses an INI configuration file and applies values to the Configuration.
// It silently returns nil if the file doesn't exist.
// Uses struct tags for mapping (e.g., `ini:"aururl"`).
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
