//go:build !integration
// +build !integration

package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GIVEN no user directories and sudo user
// WHEN cache home is selected
// THEN the selected cache home should be in the tmp dir
func Test_getCacheHome(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Unsetenv("XDG_CACHE_HOME"))
	require.NoError(t, os.Unsetenv("HOME"))
	t.Setenv("SUDO_USER", "test")
	t.Setenv("TMPDIR", dir)

	got, err := getCacheHome()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "yay"), got)

	require.NoError(t, os.Unsetenv("TMPDIR"))
	require.NoError(t, os.Unsetenv("SUDO_USER"))
}

func TestGetLuaConfigPath(t *testing.T) {
	t.Run("uses XDG_CONFIG_HOME when set", func(t *testing.T) {
		configHome := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configHome)
		t.Setenv("HOME", "/unused-home")

		assert.Equal(t, filepath.Join(configHome, "yay", "init.lua"), GetLuaConfigPath())
	})

	t.Run("falls back to HOME when XDG_CONFIG_HOME is unset", func(t *testing.T) {
		require.NoError(t, os.Unsetenv("XDG_CONFIG_HOME"))
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		assert.Equal(t, filepath.Join(homeDir, ".config", "yay", "init.lua"), GetLuaConfigPath())
	})
}

func TestResolveLuaConfigPath(t *testing.T) {
	t.Run("prefers init.lua in current working directory", func(t *testing.T) {
		cwd := t.TempDir()
		configHome := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configHome)

		require.NoError(t, os.MkdirAll(filepath.Join(configHome, "yay"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(configHome, "yay", "init.lua"), []byte("return true\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(cwd, "init.lua"), []byte("return true\n"), 0o644))

		oldWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(cwd))
		t.Cleanup(func() {
			require.NoError(t, os.Chdir(oldWD))
		})

		resolved, err := ResolveLuaConfigPath()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(cwd, "init.lua"), resolved)
	})

	t.Run("falls back to config directory", func(t *testing.T) {
		cwd := t.TempDir()
		configHome := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configHome)

		require.NoError(t, os.MkdirAll(filepath.Join(configHome, "yay"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(configHome, "yay", "init.lua"), []byte("return true\n"), 0o644))

		oldWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(cwd))
		t.Cleanup(func() {
			require.NoError(t, os.Chdir(oldWD))
		})

		resolved, err := ResolveLuaConfigPath()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(configHome, "yay", "init.lua"), resolved)
	})
}
