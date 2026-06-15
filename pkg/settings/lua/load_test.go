package lua

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeLuaFile(t *testing.T, body string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "init.lua")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	return path
}

func TestLoadIntoStrictFailsOnUnknownKey(t *testing.T) {
	path := writeLuaFile(t, `
		yay.opt.unknown_key = true
	`)

	cfg := &testConfig{}
	err := LoadInto(nil, path, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown yay.opt key: unknown_key")
}

func TestLoadIntoStrictFailsOnTypeMismatch(t *testing.T) {
	path := writeLuaFile(t, `
		yay.opt.devel = "true"
	`)

	cfg := &testConfig{}
	err := LoadInto(nil, path, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "yay.opt.devel: expected boolean")
}

func TestLoadIntoAppliesValidValues(t *testing.T) {
	path := writeLuaFile(t, `
		yay.opt.build_dir = "/tmp/yay"
		yay.opt.request_split_n = 123
		yay.opt.devel = true
	`)

	cfg := &testConfig{}
	err := LoadInto(nil, path, cfg)
	require.NoError(t, err)
	require.Equal(t, "/tmp/yay", cfg.BuildDir)
	require.Equal(t, 123, cfg.RequestSplitN)
	require.True(t, cfg.Devel)
}

func TestLoadReturnsLiveEngine(t *testing.T) {
	path := writeLuaFile(t, `
		yay.create_autocmd("AURPreInstall", {
			callback = function() end,
		})
		yay.opt.build_dir = "/tmp/yay"
	`)

	cfg := &testConfig{}
	engine, err := Load(nil, path, cfg)
	require.NoError(t, err)
	defer engine.Close()

	require.Equal(t, "/tmp/yay", cfg.BuildDir)
	require.True(t, engine.HasAutocmd(EventAURPreInstall))
}
