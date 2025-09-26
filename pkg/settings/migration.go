package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ConfigMigration handles migration of configuration files
type ConfigMigration struct {
	configPath string
}

// NewConfigMigration creates a new configuration migration handler
func NewConfigMigration(configPath string) *ConfigMigration {
	return &ConfigMigration{
		configPath: configPath,
	}
}

// MigrateIfNeeded checks if migration is needed and performs it
func (m *ConfigMigration) MigrateIfNeeded() error {
	// Check if config file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		// No config file, nothing to migrate
		return nil
	}

	// Read existing config
	configData, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse as generic JSON to check structure
	var configMap map[string]interface{}
	if err := json.Unmarshal(configData, &configMap); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Check if customRepos field already exists
	if _, exists := configMap["customRepos"]; exists {
		// Already migrated, nothing to do
		return nil
	}

	// Perform migration
	return m.migrateToCustomRepos(configData, configMap)
}

// migrateToCustomRepos migrates old config format to include customRepos
func (m *ConfigMigration) migrateToCustomRepos(originalData []byte, configMap map[string]interface{}) error {
	// Add empty customRepos array to maintain backward compatibility
	configMap["customRepos"] = []CustomRepo{}

	// Create backup of original config
	backupPath := m.configPath + ".backup"
	if err := os.WriteFile(backupPath, originalData, 0644); err != nil {
		return fmt.Errorf("failed to create config backup: %w", err)
	}

	// Write migrated config
	migratedData, err := json.MarshalIndent(configMap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal migrated config: %w", err)
	}

	if err := os.WriteFile(m.configPath, migratedData, 0644); err != nil {
		return fmt.Errorf("failed to write migrated config: %w", err)
	}

	return nil
}

// MigrateFromLegacyFormat migrates from very old config formats if needed
func (m *ConfigMigration) MigrateFromLegacyFormat() error {
	// Check if config file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	// Read existing config
	configData, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Try to parse as current Configuration struct
	var config Configuration
	if err := json.Unmarshal(configData, &config); err != nil {
		// If parsing fails, it might be an old format
		return m.migrateFromOldFormat(configData)
	}

	// If parsing succeeds, check if we need to add customRepos
	if config.CustomRepos == nil {
		config.CustomRepos = []CustomRepo{}
		
		// Write updated config
		updatedData, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal updated config: %w", err)
		}

		if err := os.WriteFile(m.configPath, updatedData, 0644); err != nil {
			return fmt.Errorf("failed to write updated config: %w", err)
		}
	}

	return nil
}

// migrateFromOldFormat handles migration from very old config formats
func (m *ConfigMigration) migrateFromOldFormat(configData []byte) error {
	// This would handle migration from very old formats
	// For now, we'll just create a new config with defaults
	
	// Create backup
	backupPath := m.configPath + ".backup"
	if err := os.WriteFile(backupPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to create config backup: %w", err)
	}

	// Create new config with defaults
	defaultConfig := Configuration{
		CustomRepos: []CustomRepo{},
		// Add other default values as needed
	}

	// Write new config
	newData, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal new config: %w", err)
	}

	if err := os.WriteFile(m.configPath, newData, 0644); err != nil {
		return fmt.Errorf("failed to write new config: %w", err)
	}

	return nil
}

// ValidateConfig validates the configuration file
func (m *ConfigMigration) ValidateConfig() error {
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	configData, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config Configuration
	if err := json.Unmarshal(configData, &config); err != nil {
		return fmt.Errorf("invalid config file format: %w", err)
	}

	// Validate custom repositories
	for i, repo := range config.CustomRepos {
		if err := m.validateCustomRepo(repo, i); err != nil {
			return fmt.Errorf("invalid custom repository at index %d: %w", i, err)
		}
	}

	return nil
}

// validateCustomRepo validates a single custom repository configuration
func (m *ConfigMigration) validateCustomRepo(repo CustomRepo, index int) error {
	if repo.Name == "" {
		return fmt.Errorf("repository name cannot be empty")
	}

	if repo.Type == "" {
		return fmt.Errorf("repository type cannot be empty")
	}

	switch repo.Type {
	case "local":
		if repo.Path == "" {
			return fmt.Errorf("local repository must have a path")
		}
		// Check if path exists
		if _, err := os.Stat(repo.Path); os.IsNotExist(err) {
			return fmt.Errorf("local repository path does not exist: %s", repo.Path)
		}
	case "git":
		if repo.URL == "" {
			return fmt.Errorf("git repository must have a URL")
		}
	case "http":
		if repo.URL == "" {
			return fmt.Errorf("http repository must have a URL")
		}
	default:
		return fmt.Errorf("unsupported repository type: %s", repo.Type)
	}

	// Validate authentication if present
	if repo.Auth != nil {
		if err := m.validateAuth(repo.Auth); err != nil {
			return fmt.Errorf("invalid authentication configuration: %w", err)
		}
	}

	return nil
}

// validateAuth validates authentication configuration
func (m *ConfigMigration) validateAuth(auth *RepoAuth) error {
	if auth.Type == "" {
		return fmt.Errorf("authentication type cannot be empty")
	}

	switch auth.Type {
	case "ssh_key":
		if auth.KeyPath == "" {
			return fmt.Errorf("SSH key authentication requires keyPath")
		}
		// Check if key file exists
		if _, err := os.Stat(auth.KeyPath); os.IsNotExist(err) {
			return fmt.Errorf("SSH key file does not exist: %s", auth.KeyPath)
		}
	case "token":
		if auth.Token == "" {
			return fmt.Errorf("token authentication requires token")
		}
	case "basic":
		if auth.Username == "" || auth.Password == "" {
			return fmt.Errorf("basic authentication requires username and password")
		}
	default:
		return fmt.Errorf("unsupported authentication type: %s", auth.Type)
	}

	return nil
}

// GetConfigPath returns the configuration file path
func (m *ConfigMigration) GetConfigPath() string {
	return m.configPath
}

// GetBackupPath returns the backup file path
func (m *ConfigMigration) GetBackupPath() string {
	return m.configPath + ".backup"
}

// RestoreFromBackup restores the configuration from backup
func (m *ConfigMigration) RestoreFromBackup() error {
	backupPath := m.GetBackupPath()
	
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file does not exist: %s", backupPath)
	}

	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	if err := os.WriteFile(m.configPath, backupData, 0644); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	return nil
}

// GetDefaultConfigPath returns the default configuration file path
func GetDefaultConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "yay")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return filepath.Join(configDir, "config.json"), nil
}
