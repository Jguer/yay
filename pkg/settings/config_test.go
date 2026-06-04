//go:build !integration
// +build !integration

package settings

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLuaHookRunner struct {
	callPrompt          func(name, defaultAns string) (string, bool, error)
	callAURUpdate       func(candidate AURUpdateContext) (bool, bool, error)
	callExternalUpdates func(ctx context.Context) ([]ExternalUpgrade, error)
	callRunExternal     func(ctx context.Context, repository string, upgrades []ExternalUpgrade) (bool, error)
	callSearchExternal  func(ctx context.Context, terms []string) ([]ExternalSearchResult, error)
	callInstallExternal func(ctx context.Context, repository string, targets []ExternalInstallTarget) (bool, error)
	hasProvider         func(name string) bool
	closed              bool
}

func (m *mockLuaHookRunner) CallOnPrompt(name, defaultAns string) (string, bool, error) {
	if m.callPrompt == nil {
		return "", false, nil
	}

	return m.callPrompt(name, defaultAns)
}

func (m *mockLuaHookRunner) CallShouldIncludeAURUpdate(candidate AURUpdateContext) (bool, bool, error) {
	if m.callAURUpdate == nil {
		return false, false, nil
	}

	return m.callAURUpdate(candidate)
}

func (m *mockLuaHookRunner) CallListExternalUpgrades(ctx context.Context) ([]ExternalUpgrade, error) {
	if m.callExternalUpdates == nil {
		return nil, nil
	}

	return m.callExternalUpdates(ctx)
}

func (m *mockLuaHookRunner) CallRunExternalUpgrades(ctx context.Context, repository string, upgrades []ExternalUpgrade) (bool, error) {
	if m.callRunExternal == nil {
		return false, nil
	}

	return m.callRunExternal(ctx, repository, upgrades)
}

func (m *mockLuaHookRunner) CallSearchExternalPackages(ctx context.Context, terms []string) ([]ExternalSearchResult, error) {
	if m.callSearchExternal == nil {
		return nil, nil
	}

	return m.callSearchExternal(ctx, terms)
}

func (m *mockLuaHookRunner) CallInstallExternalPackages(ctx context.Context, repository string, targets []ExternalInstallTarget) (bool, error) {
	if m.callInstallExternal == nil {
		return false, nil
	}

	return m.callInstallExternal(ctx, repository, targets)
}

func (m *mockLuaHookRunner) HasProvider(name string) bool {
	if m.hasProvider == nil {
		return false
	}

	return m.hasProvider(name)
}

func (m *mockLuaHookRunner) Close() {
	m.closed = true
}

// GIVEN a non existing build dir in the config
// WHEN the config is loaded
// THEN the directory should be created
func TestNewConfig(t *testing.T) {
	configDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(configDir, "yay"), 0o755)
	assert.NoError(t, err)

	t.Setenv("XDG_CONFIG_HOME", configDir)

	cacheDir := t.TempDir()

	config := map[string]string{"BuildDir": filepath.Join(cacheDir, "test-build-dir")}

	f, err := os.Create(filepath.Join(configDir, "yay", "config.json"))
	assert.NoError(t, err)

	defer f.Close()

	configJSON, _ := json.Marshal(config)
	_, err = f.WriteString(string(configJSON))
	assert.NoError(t, err)

	newConfig, err := NewConfig(nil, GetConfigPath(), "v1.0.0")
	assert.NoError(t, err)

	assert.Equal(t, filepath.Join(cacheDir, "test-build-dir"), newConfig.BuildDir)

	_, err = os.Stat(filepath.Join(cacheDir, "test-build-dir"))
	assert.NoError(t, err)
}

func TestNewConfigSkipsLegacyConfigWhenLuaConfigExists(t *testing.T) {
	configDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, "yay"), 0o755))
	t.Setenv("XDG_CONFIG_HOME", configDir)

	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	legacyBuildDir := filepath.Join(t.TempDir(), "legacy-build-dir")
	config := map[string]string{"BuildDir": legacyBuildDir}
	configJSON, err := json.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "yay", "config.json"), configJSON, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "yay", "init.lua"), []byte("return true\n"), 0o644))

	newConfig, err := NewConfig(nil, GetConfigPath(), "v1.0.0")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(cacheHome, "yay"), newConfig.BuildDir)
	assert.NotEqual(t, legacyBuildDir, newConfig.BuildDir)
}

func TestNewConfigSkipsLegacyConfigWhenLuaConfigExistsInWorkingDir(t *testing.T) {
	configDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, "yay"), 0o755))
	t.Setenv("XDG_CONFIG_HOME", configDir)

		cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	legacyBuildDir := filepath.Join(t.TempDir(), "legacy-build-dir")
	config := map[string]string{"BuildDir": legacyBuildDir}
	configJSON, err := json.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "yay", "config.json"), configJSON, 0o644))

	cwd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "init.lua"), []byte("return true\n"), 0o644))

	oldWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(cwd))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(oldWD))
	})

	newConfig, err := NewConfig(nil, GetConfigPath(), "v1.0.0")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(cacheHome, "yay"), newConfig.BuildDir)
	assert.NotEqual(t, legacyBuildDir, newConfig.BuildDir)
}

// GIVEN a non existing build dir in the config and AURDEST set to a non-existing folder
// WHEN the config is loaded
// THEN the directory of AURDEST should be created and selected
func TestNewConfigAURDEST(t *testing.T) {
	configDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(configDir, "yay"), 0o755)
	assert.NoError(t, err)

	t.Setenv("XDG_CONFIG_HOME", configDir)

	cacheDir := t.TempDir()

	config := map[string]string{"BuildDir": filepath.Join(cacheDir, "test-other-dir")}
	t.Setenv("AURDEST", filepath.Join(cacheDir, "test-build-dir"))

	f, err := os.Create(filepath.Join(configDir, "yay", "config.json"))
	assert.NoError(t, err)

	defer f.Close()

	configJSON, _ := json.Marshal(config)
	_, err = f.WriteString(string(configJSON))
	assert.NoError(t, err)

	newConfig, err := NewConfig(nil, GetConfigPath(), "v1.0.0")
	assert.NoError(t, err)

	assert.Equal(t, filepath.Join(cacheDir, "test-build-dir"), newConfig.BuildDir)

	_, err = os.Stat(filepath.Join(cacheDir, "test-build-dir"))
	assert.NoError(t, err)
}

// Test tilde expansion in AURDEST
func TestNewConfigAURDESTTildeExpansion(t *testing.T) {
	configDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(configDir, "yay"), 0o755)
	assert.NoError(t, err)

	t.Setenv("XDG_CONFIG_HOME", configDir)

	homeDir := t.TempDir()
	cacheDir := t.TempDir()

	config := map[string]string{"BuildDir": filepath.Join(cacheDir, "test-other-dir")}
	t.Setenv("AURDEST", "~/test-build-dir")
	t.Setenv("HOME", homeDir)

	f, err := os.Create(filepath.Join(configDir, "yay", "config.json"))
	assert.NoError(t, err)

	defer f.Close()

	configJSON, _ := json.Marshal(config)
	_, err = f.WriteString(string(configJSON))
	assert.NoError(t, err)

	newConfig, err := NewConfig(nil, GetConfigPath(), "v1.0.0")
	assert.NoError(t, err)

	assert.Equal(t, filepath.Join(homeDir, "test-build-dir"), newConfig.BuildDir)

	_, err = os.Stat(filepath.Join(homeDir, "test-build-dir"))
	assert.NoError(t, err)
}

// GIVEN default config
// WHEN setPrivilegeElevator gets called
// THEN sudobin should stay as "sudo" (given sudo exists)
func TestConfiguration_setPrivilegeElevator(t *testing.T) {
	path := t.TempDir()

	doas := filepath.Join(path, "sudo")
	_, err := os.Create(doas)
	os.Chmod(doas, 0o755)
	assert.NoError(t, err)

	config := DefaultConfig("test")
	config.SudoLoop = true
	config.SudoFlags = "-v"

	t.Setenv("PATH", path)
	err = config.setPrivilegeElevator()
	assert.NoError(t, err)

	assert.Equal(t, "sudo", config.SudoBin)
	assert.Equal(t, "-v", config.SudoFlags)
	assert.True(t, config.SudoLoop)
}

func TestConfigurationOnPrompt(t *testing.T) {
	t.Run("returns default when no engine is attached", func(t *testing.T) {
		cfg := &Configuration{}
		assert.Equal(t, "default", cfg.OnPrompt("clean", "default"))
	})

	t.Run("returns override when hook provides one", func(t *testing.T) {
		cfg := &Configuration{}
		cfg.SetLuaEngine(&mockLuaHookRunner{
			callPrompt: func(name, defaultAns string) (string, bool, error) {
				assert.Equal(t, "clean", name)
				assert.Equal(t, "default", defaultAns)
				return "1 2 3", true, nil
			},
		})

		assert.Equal(t, "1 2 3", cfg.OnPrompt("clean", "default"))
	})

	t.Run("falls back to default on hook error", func(t *testing.T) {
		cfg := &Configuration{}
		cfg.SetLuaEngine(&mockLuaHookRunner{
			callPrompt: func(name, defaultAns string) (string, bool, error) {
				return "", false, errors.New("boom")
			},
		})

		assert.Equal(t, "default", cfg.OnPrompt("clean", "default"))
	})
}

func TestConfigurationCloseLua(t *testing.T) {
	runner := &mockLuaHookRunner{}
	cfg := &Configuration{}
	cfg.SetLuaEngine(runner)

	cfg.CloseLua()
	assert.True(t, runner.closed)
	assert.Equal(t, "default", cfg.OnPrompt("clean", "default"))
}

func TestConfigurationShouldIncludeAURUpdate(t *testing.T) {
	candidate := AURUpdateContext{DefaultInclude: false, RemoteLastModified: 200, LocalBuildDate: 100}

	t.Run("returns default when no engine is attached", func(t *testing.T) {
		cfg := &Configuration{}
		assert.False(t, cfg.ShouldIncludeAURUpdate(candidate))
	})

	t.Run("returns override when hook provides one", func(t *testing.T) {
		cfg := &Configuration{}
		cfg.SetLuaEngine(&mockLuaHookRunner{
			callAURUpdate: func(got AURUpdateContext) (bool, bool, error) {
				assert.Equal(t, candidate.RemoteLastModified, got.RemoteLastModified)
				return true, true, nil
			},
		})

		assert.True(t, cfg.ShouldIncludeAURUpdate(candidate))
	})

	t.Run("falls back to default on hook error", func(t *testing.T) {
		cfg := &Configuration{}
		cfg.SetLuaEngine(&mockLuaHookRunner{
			callAURUpdate: func(AURUpdateContext) (bool, bool, error) {
				return false, false, errors.New("boom")
			},
		})

		assert.False(t, cfg.ShouldIncludeAURUpdate(candidate))
	})
}

func TestConfigurationExternalUpgrades(t *testing.T) {
	t.Run("returns nil when no engine is attached", func(t *testing.T) {
		cfg := &Configuration{}
		upgrades, err := cfg.ExternalUpgrades(context.Background())
		assert.NoError(t, err)
		assert.Nil(t, upgrades)
	})

	t.Run("returns upgrades from lua engine", func(t *testing.T) {
		cfg := &Configuration{}
		cfg.SetLuaEngine(&mockLuaHookRunner{
			callExternalUpdates: func(context.Context) ([]ExternalUpgrade, error) {
				return []ExternalUpgrade{{Name: "wget", Repository: "homebrew", LocalVersion: "1.0", RemoteVersion: "2.0"}}, nil
			},
		})

		upgrades, err := cfg.ExternalUpgrades(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, []ExternalUpgrade{{Name: "wget", Repository: "homebrew", LocalVersion: "1.0", RemoteVersion: "2.0"}}, upgrades)
	})
}

func TestConfigurationRunExternalUpgradeProvider(t *testing.T) {
	t.Run("noops when no engine is attached", func(t *testing.T) {
		cfg := &Configuration{}
		assert.NoError(t, cfg.RunExternalUpgradeProvider(context.Background(), "homebrew", []ExternalUpgrade{{Name: "wget"}}))
	})

	t.Run("forwards execution to lua engine", func(t *testing.T) {
		cfg := &Configuration{}
		cfg.SetLuaEngine(&mockLuaHookRunner{
			callRunExternal: func(_ context.Context, repository string, upgrades []ExternalUpgrade) (bool, error) {
				assert.Equal(t, "homebrew", repository)
				assert.Equal(t, []ExternalUpgrade{{Name: "wget", Repository: "homebrew"}}, upgrades)
				return true, nil
			},
		})

		assert.NoError(t, cfg.RunExternalUpgradeProvider(context.Background(), "homebrew", []ExternalUpgrade{{Name: "wget", Repository: "homebrew"}}))
	})
}

func TestConfigurationSearchExternalPackages(t *testing.T) {
	t.Run("returns nil when no engine is attached", func(t *testing.T) {
		cfg := &Configuration{}
		results, err := cfg.SearchExternalPackages(context.Background(), []string{"ripgrep"})
		assert.NoError(t, err)
		assert.Nil(t, results)
	})

	t.Run("returns results from lua engine", func(t *testing.T) {
		cfg := &Configuration{}
		cfg.SetLuaEngine(&mockLuaHookRunner{
			callSearchExternal: func(_ context.Context, terms []string) ([]ExternalSearchResult, error) {
				assert.Equal(t, []string{"ripgrep"}, terms)
				return []ExternalSearchResult{{Name: "ripgrep", Repository: "homebrew", Description: "search tool"}}, nil
			},
		})

		results, err := cfg.SearchExternalPackages(context.Background(), []string{"ripgrep"})
		assert.NoError(t, err)
		assert.Equal(t, []ExternalSearchResult{{Name: "ripgrep", Repository: "homebrew", Description: "search tool"}}, results)
	})
}

func TestConfigurationInstallExternalPackages(t *testing.T) {
	t.Run("noops when no engine is attached", func(t *testing.T) {
		cfg := &Configuration{}
		handled, err := cfg.InstallExternalPackages(context.Background(), "homebrew", []ExternalInstallTarget{{Name: "ripgrep", Repository: "homebrew"}})
		assert.NoError(t, err)
		assert.False(t, handled)
	})

	t.Run("forwards install targets to lua engine", func(t *testing.T) {
		cfg := &Configuration{}
		cfg.SetLuaEngine(&mockLuaHookRunner{
			callInstallExternal: func(_ context.Context, repository string, targets []ExternalInstallTarget) (bool, error) {
				assert.Equal(t, "homebrew", repository)
				assert.Equal(t, []ExternalInstallTarget{{Name: "ripgrep", Repository: "homebrew"}}, targets)
				return true, nil
			},
		})

		handled, err := cfg.InstallExternalPackages(context.Background(), "homebrew", []ExternalInstallTarget{{Name: "ripgrep", Repository: "homebrew"}})
		assert.NoError(t, err)
		assert.True(t, handled)
	})
}

func TestConfigurationHasExternalProvider(t *testing.T) {
	cfg := &Configuration{}
	assert.False(t, cfg.HasExternalProvider("homebrew"))

	cfg.SetLuaEngine(&mockLuaHookRunner{hasProvider: func(name string) bool {
		return name == "homebrew"
	}})
	assert.True(t, cfg.HasExternalProvider("homebrew"))
	assert.False(t, cfg.HasExternalProvider("flatpak"))
}

// GIVEN default config and sudo loop enabled
// GIVEN only su in path
// WHEN setPrivilegeElevator gets called
// THEN sudobin should be changed to "su"
func TestConfiguration_setPrivilegeElevator_su(t *testing.T) {
	path := t.TempDir()

	doas := filepath.Join(path, "su")
	_, err := os.Create(doas)
	os.Chmod(doas, 0o755)
	assert.NoError(t, err)

	config := DefaultConfig("test")
	config.SudoLoop = true
	config.SudoFlags = "-v"

	t.Setenv("PATH", path)
	err = config.setPrivilegeElevator()

	assert.NoError(t, err)
	assert.Equal(t, "su", config.SudoBin)
	assert.Equal(t, "", config.SudoFlags)
	assert.False(t, config.SudoLoop)
}

// GIVEN default config and sudo loop enabled
// GIVEN no sudo in path
// WHEN setPrivilegeElevator gets called
// THEN sudobin should be changed to "su"
func TestConfiguration_setPrivilegeElevator_no_path(t *testing.T) {
	t.Setenv("PATH", "")
	config := DefaultConfig("test")
	config.SudoLoop = true
	config.SudoFlags = "-v"

	err := config.setPrivilegeElevator()

	assert.Error(t, err)
	assert.Equal(t, "sudo", config.SudoBin)
	assert.Equal(t, "", config.SudoFlags)
	assert.False(t, config.SudoLoop)
}

// GIVEN default config and sudo loop enabled
// GIVEN doas in path
// WHEN setPrivilegeElevator gets called
// THEN sudobin should be changed to "doas"
func TestConfiguration_setPrivilegeElevator_doas(t *testing.T) {
	path := t.TempDir()

	doas := filepath.Join(path, "doas")
	_, err := os.Create(doas)
	os.Chmod(doas, 0o755)
	assert.NoError(t, err)

	config := DefaultConfig("test")
	config.SudoLoop = true
	config.SudoFlags = "-v"

	t.Setenv("PATH", path)
	err = config.setPrivilegeElevator()
	assert.NoError(t, err)
	assert.Equal(t, "doas", config.SudoBin)
	assert.Equal(t, "", config.SudoFlags)
	assert.False(t, config.SudoLoop)
}

// GIVEN default config and sudo loop enabled
// GIVEN run0 in path
// WHEN setPrivilegeElevator gets called
// THEN sudobin should be changed to "run0"
func TestConfiguration_setPrivilegeElevator_run0(t *testing.T) {
	path := t.TempDir()

	doas := filepath.Join(path, "run0")
	_, err := os.Create(doas)
	os.Chmod(doas, 0o755)
	assert.NoError(t, err)

	config := DefaultConfig("test")
	config.SudoLoop = true
	config.SudoFlags = "-v"

	t.Setenv("PATH", path)
	err = config.setPrivilegeElevator()
	assert.NoError(t, err)
	assert.Equal(t, "run0", config.SudoBin)
	assert.Equal(t, "", config.SudoFlags)
	assert.False(t, config.SudoLoop)
}

// GIVEN config with wrapper and sudo loop enabled
// GIVEN wrapper is in path
// WHEN setPrivilegeElevator gets called
// THEN sudobin should be kept as the wrapper
func TestConfiguration_setPrivilegeElevator_custom_script(t *testing.T) {
	path := t.TempDir()

	wrapper := filepath.Join(path, "custom-wrapper")
	_, err := os.Create(wrapper)
	os.Chmod(wrapper, 0o755)
	assert.NoError(t, err)

	config := DefaultConfig("test")
	config.SudoLoop = true
	config.SudoBin = wrapper
	config.SudoFlags = "-v"

	t.Setenv("PATH", path)
	err = config.setPrivilegeElevator()

	assert.NoError(t, err)
	assert.Equal(t, wrapper, config.SudoBin)
	assert.Equal(t, "-v", config.SudoFlags)
	assert.True(t, config.SudoLoop)
}

// GIVEN default config and sudo loop enabled
// GIVEN doas as PACMAN_AUTH env variable
// WHEN setPrivilegeElevator gets called
// THEN sudobin should be changed to "doas"
func TestConfiguration_setPrivilegeElevator_pacman_auth_doas(t *testing.T) {
	path := t.TempDir()

	doas := filepath.Join(path, "doas")
	_, err := os.Create(doas)
	os.Chmod(doas, 0o755)
	require.NoError(t, err)

	sudo := filepath.Join(path, "sudo")
	_, err = os.Create(sudo)
	os.Chmod(sudo, 0o755)
	require.NoError(t, err)

	config := DefaultConfig("test")
	config.SudoBin = "sudo"
	config.SudoLoop = true
	config.SudoFlags = "-v"

	t.Setenv("PACMAN_AUTH", "doas")
	t.Setenv("PATH", path)
	err = config.setPrivilegeElevator()
	assert.NoError(t, err)
	assert.Equal(t, "doas", config.SudoBin)
	assert.Equal(t, "", config.SudoFlags)
	assert.False(t, config.SudoLoop)
}

// GIVEN config with doas configed and sudo loop enabled
// GIVEN sudo as PACMAN_AUTH env variable
// WHEN setPrivilegeElevator gets called
// THEN sudobin should be changed to "sudo"
func TestConfiguration_setPrivilegeElevator_pacman_auth_sudo(t *testing.T) {
	path := t.TempDir()

	doas := filepath.Join(path, "doas")
	_, err := os.Create(doas)
	os.Chmod(doas, 0o755)
	require.NoError(t, err)

	sudo := filepath.Join(path, "sudo")
	_, err = os.Create(sudo)
	os.Chmod(sudo, 0o755)
	require.NoError(t, err)

	config := DefaultConfig("test")
	config.SudoBin = "doas"
	config.SudoLoop = true
	config.SudoFlags = "-v"

	t.Setenv("PACMAN_AUTH", "sudo")
	t.Setenv("PATH", path)
	err = config.setPrivilegeElevator()
	assert.NoError(t, err)
	assert.Equal(t, "sudo", config.SudoBin)
	assert.Equal(t, "-v", config.SudoFlags)
	assert.True(t, config.SudoLoop)
}
