package lua

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadIntoRegistersBuiltins(t *testing.T) {
	configHome := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", homeDir)
	t.Setenv("YAY_LUA_EDITOR", "nvim")

	configDir := filepath.Join(configHome, "yay")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "init.lua"), []byte(`
		yay.api.info("lua init starting")
		yay.api.warn("lua init warning")
		yay.api.error("lua init error")
		
		yay.opt.editor = yay.api.getenv("YAY_LUA_EDITOR")
		yay.opt.build_dir = yay.api.expand("~/lua-build")
	`), 0o644))

	cfg := settings.DefaultConfig("v1.0.0")
	logger := text.NewLogger(os.Stdout, os.Stderr, os.Stdin, false, "lua-test")

	require.NoError(t, LoadInto(logger, cfg))
	defer cfg.CloseLua()

	assert.Equal(t, "nvim", cfg.Editor)
	assert.Equal(t, filepath.Join(homeDir, "lua-build"), cfg.BuildDir)
	assert.NotNil(t, cfg)
}

func TestLoadIntoExternalProvider(t *testing.T) {
	configHome := t.TempDir()
	homeDir := t.TempDir()
	runLog := filepath.Join(t.TempDir(), "provider-run.log")

	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", homeDir)
	t.Setenv("YAY_LUA_RUN_LOG", runLog)

	configDir := filepath.Join(configHome, "yay")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "init.lua"), []byte(`
		yay.provider.homebrew = {
			list = function()
				local stdout, stderr, code = yay.api.capture("sh", "-c", "printf '%s' '[{\"name\":\"wget\",\"installed_versions\":[\"1.0\"],\"current_version\":\"2.0\"}]'")
				if code ~= 0 then
					error(stderr)
				end

				local decoded = yay.api.json_decode(stdout)
				local upgrades = {}
				for _, item in ipairs(decoded) do
					table.insert(upgrades, {
						name = item.name,
						repository = "homebrew",
						local_version = item.installed_versions[1],
						remote_version = item.current_version,
						extra = "formula",
					})
				end

				return upgrades
			end,
			upgrade = function(items)
				local names = {}
				for _, item in ipairs(items) do
					table.insert(names, item.name)
				end

				local script = string.format("printf '%s' '%s' > \"%s\"", "%s", table.concat(names, ","), yay.api.expand("$YAY_LUA_RUN_LOG"))
				local code = yay.api.run("sh", "-c", script)
				if code ~= 0 then
					error("provider execution failed")
				end
			end,
		}
	`), 0o644))

	cfg := settings.DefaultConfig("v1.0.0")
	logger := text.NewLogger(os.Stdout, os.Stderr, os.Stdin, false, "lua-test")

	require.NoError(t, LoadInto(logger, cfg))
	defer cfg.CloseLua()

	upgrades, err := cfg.ExternalUpgrades(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []settings.ExternalUpgrade{{
		Name:          "wget",
		Repository:    "homebrew",
		LocalVersion:  "1.0",
		RemoteVersion: "2.0",
		Extra:         "formula",
	}}, upgrades)

	require.NoError(t, cfg.RunExternalUpgradeProvider(context.Background(), "homebrew", upgrades))

	runOutput, err := os.ReadFile(runLog)
	require.NoError(t, err)
	assert.Equal(t, "wget", string(runOutput))
}

func TestLoadIntoPrefersWorkingDirectoryInitLua(t *testing.T) {
	configHome := t.TempDir()
	homeDir := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", homeDir)
	t.Setenv("YAY_LUA_EDITOR", "vim")

	require.NoError(t, os.MkdirAll(filepath.Join(configHome, "yay"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configHome, "yay", "init.lua"), []byte(`
		yay.opt.editor = "config-dir"
	`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "init.lua"), []byte(`
		yay.opt.editor = yay.api.getenv("YAY_LUA_EDITOR")
	`), 0o644))

	oldWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(cwd))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(oldWD))
	})

	cfg := settings.DefaultConfig("v1.0.0")
	logger := text.NewLogger(os.Stdout, os.Stderr, os.Stdin, false, "lua-test")

	require.NoError(t, LoadInto(logger, cfg))
	defer cfg.CloseLua()

	assert.Equal(t, "vim", cfg.Editor)
}
