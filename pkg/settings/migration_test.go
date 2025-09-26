package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigMigration(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "yay-migration-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	// Test 1: Migrate config without customRepos field
	t.Run("MigrateConfigWithoutCustomRepos", func(t *testing.T) {
		// Create old config without customRepos
		oldConfig := map[string]interface{}{
			"aururl": "https://aur.archlinux.org",
			"editor": "vim",
			"version": "12.0.0",
		}

		configData, err := json.MarshalIndent(oldConfig, "", "  ")
		require.NoError(t, err)

		err = os.WriteFile(configPath, configData, 0644)
		require.NoError(t, err)

		// Perform migration
		migration := NewConfigMigration(configPath)
		err = migration.MigrateIfNeeded()
		require.NoError(t, err)

		// Verify migration
		migratedData, err := os.ReadFile(configPath)
		require.NoError(t, err)

		var migratedConfig map[string]interface{}
		err = json.Unmarshal(migratedData, &migratedConfig)
		require.NoError(t, err)

		// Check that customRepos field was added
		customRepos, exists := migratedConfig["customRepos"]
		require.True(t, exists)
		assert.Equal(t, []interface{}{}, customRepos)

		// Check that other fields are preserved
		assert.Equal(t, "https://aur.archlinux.org", migratedConfig["aururl"])
		assert.Equal(t, "vim", migratedConfig["editor"])
		assert.Equal(t, "12.0.0", migratedConfig["version"])

		// Check that backup was created
		backupPath := migration.GetBackupPath()
		_, err = os.Stat(backupPath)
		assert.NoError(t, err)
	})

	// Test 2: Config already has customRepos field
	t.Run("ConfigAlreadyMigrated", func(t *testing.T) {
		// Create config with customRepos already present
		configWithCustomRepos := map[string]interface{}{
			"aururl": "https://aur.archlinux.org",
			"editor": "vim",
			"version": "12.0.0",
			"customRepos": []map[string]interface{}{
				{
					"name": "test-repo",
					"type": "local",
					"path": "/tmp/test",
					"searchable": true,
					"priority": 1,
				},
			},
		}

		configData, err := json.MarshalIndent(configWithCustomRepos, "", "  ")
		require.NoError(t, err)

		err = os.WriteFile(configPath, configData, 0644)
		require.NoError(t, err)

		// Perform migration
		migration := NewConfigMigration(configPath)
		err = migration.MigrateIfNeeded()
		require.NoError(t, err)

		// Verify that config was not modified
		migratedData, err := os.ReadFile(configPath)
		require.NoError(t, err)

		var migratedConfig map[string]interface{}
		err = json.Unmarshal(migratedData, &migratedConfig)
		require.NoError(t, err)

		// Check that customRepos field is still present and unchanged
		customRepos, exists := migratedConfig["customRepos"]
		require.True(t, exists)
		customReposList := customRepos.([]interface{})
		require.Len(t, customReposList, 1)
		
		repo := customReposList[0].(map[string]interface{})
		assert.Equal(t, "test-repo", repo["name"])
		assert.Equal(t, "local", repo["type"])
		assert.Equal(t, "/tmp/test", repo["path"])
	})

	// Test 3: Validate configuration
	t.Run("ValidateConfig", func(t *testing.T) {
		// Create valid config
		validConfig := Configuration{
			CustomRepos: []CustomRepo{
				{
					Name:       "test-repo",
					Type:       "local",
					Path:       tmpDir, // Use existing directory
					Searchable: true,
					Priority:   1,
				},
			},
		}

		configData, err := json.MarshalIndent(validConfig, "", "  ")
		require.NoError(t, err)

		err = os.WriteFile(configPath, configData, 0644)
		require.NoError(t, err)

		// Validate config
		migration := NewConfigMigration(configPath)
		err = migration.ValidateConfig()
		assert.NoError(t, err)
	})

	// Test 4: Validate invalid configuration
	t.Run("ValidateInvalidConfig", func(t *testing.T) {
		// Create invalid config
		invalidConfig := Configuration{
			CustomRepos: []CustomRepo{
				{
					Name:       "", // Empty name - should fail
					Type:       "local",
					Path:       "/nonexistent/path", // Non-existent path - should fail
					Searchable: true,
					Priority:   1,
				},
			},
		}

		configData, err := json.MarshalIndent(invalidConfig, "", "  ")
		require.NoError(t, err)

		err = os.WriteFile(configPath, configData, 0644)
		require.NoError(t, err)

		// Validate config - should fail
		migration := NewConfigMigration(configPath)
		err = migration.ValidateConfig()
		assert.Error(t, err)
	})

	// Test 5: Validate authentication
	t.Run("ValidateAuth", func(t *testing.T) {
		// Create config with invalid auth
		invalidAuthConfig := Configuration{
			CustomRepos: []CustomRepo{
				{
					Name:       "test-repo",
					Type:       "git",
					URL:        "https://github.com/test/repo",
					Searchable: true,
					Priority:   1,
					Auth: &RepoAuth{
						Type:    "ssh_key",
						KeyPath: "/nonexistent/key", // Non-existent key - should fail
					},
				},
			},
		}

		configData, err := json.MarshalIndent(invalidAuthConfig, "", "  ")
		require.NoError(t, err)

		err = os.WriteFile(configPath, configData, 0644)
		require.NoError(t, err)

		// Validate config - should fail
		migration := NewConfigMigration(configPath)
		err = migration.ValidateConfig()
		assert.Error(t, err)
	})

	// Test 6: Restore from backup
	t.Run("RestoreFromBackup", func(t *testing.T) {
		// Create original config
		originalConfig := map[string]interface{}{
			"aururl": "https://aur.archlinux.org",
			"editor": "vim",
			"version": "12.0.0",
		}

		originalData, err := json.MarshalIndent(originalConfig, "", "  ")
		require.NoError(t, err)

		err = os.WriteFile(configPath, originalData, 0644)
		require.NoError(t, err)

		// Perform migration (creates backup)
		migration := NewConfigMigration(configPath)
		err = migration.MigrateIfNeeded()
		require.NoError(t, err)

		// Modify config
		modifiedConfig := map[string]interface{}{
			"aururl": "https://modified.aur.archlinux.org",
			"editor": "nano",
			"version": "13.0.0",
			"customRepos": []interface{}{},
		}

		modifiedData, err := json.MarshalIndent(modifiedConfig, "", "  ")
		require.NoError(t, err)

		err = os.WriteFile(configPath, modifiedData, 0644)
		require.NoError(t, err)

		// Restore from backup
		err = migration.RestoreFromBackup()
		require.NoError(t, err)

		// Verify restoration
		restoredData, err := os.ReadFile(configPath)
		require.NoError(t, err)

		assert.Equal(t, originalData, restoredData)
	})
}

func TestGetDefaultConfigPath(t *testing.T) {
	configPath, err := GetDefaultConfigPath()
	require.NoError(t, err)
	
	// Should be in ~/.config/yay/config.json
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)
	
	expectedPath := filepath.Join(homeDir, ".config", "yay", "config.json")
	assert.Equal(t, expectedPath, configPath)
	
	// Directory should be created
	configDir := filepath.Dir(configPath)
	_, err = os.Stat(configDir)
	assert.NoError(t, err)
}
