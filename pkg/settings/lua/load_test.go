package lua

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	lua "github.com/yuin/gopher-lua"
)

func writeLuaFile(t *testing.T, body string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "init.lua")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	return path
}

func TestLoadIntoStrictFailsOnUnknownKey(t *testing.T) {
	t.Parallel()
	path := writeLuaFile(t, `
		yay.opt.unknown_key = true
	`)

	cfg := &testConfig{}
	err := LoadInto(nil, path, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown yay.opt key: unknown_key")
}

func TestLoadIntoStrictFailsOnTypeMismatch(t *testing.T) {
	t.Parallel()
	path := writeLuaFile(t, `
		yay.opt.devel = "true"
	`)

	cfg := &testConfig{}
	err := LoadInto(nil, path, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "yay.opt.devel: expected boolean")
}

func TestLoadIntoAppliesValidValues(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	path := writeLuaFile(t, `
		yay.create_autocmd("AURPreInstall", {
			callback = function() end,
		})
		yay.opt.build_dir = "/tmp/yay"
	`)

	cfg := &testConfig{}
	engine, err := Load(nil, path, cfg)
	require.NoError(t, err)
	t.Cleanup(engine.Close)

	require.Equal(t, "/tmp/yay", cfg.BuildDir)
	require.True(t, engine.HasAutocmd(EventAURPreInstall))
}

func TestLoadIntoResolvesRequireRelativeToConfigDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "hooks"), 0o755))

	hookBody := []byte(`
		yay.create_autocmd("AURPreInstall", {
			desc = "from required module",
			callback = function() end,
		})
	`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hooks", "maintainer_change.lua"), hookBody, 0o600))

	initBody := []byte(`
		require("hooks.maintainer_change")
		yay.opt.build_dir = "/tmp/yay"
	`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "init.lua"), initBody, 0o600))

	cfg := &testConfig{}

	// Run from a different working directory to prove require resolves
	// relative to the config dir, not the process CWD.
	wd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(wd) })
	require.NoError(t, os.Chdir(t.TempDir()))

	err = LoadInto(nil, filepath.Join(dir, "init.lua"), cfg)
	require.NoError(t, err)
	require.Equal(t, "/tmp/yay", cfg.BuildDir)
}

func TestLoadDoesNotResolveRequireFromCWD(t *testing.T) {
	t.Parallel()

	// A module that only exists in the process CWD must NOT be resolvable,
	// since the CWD is not the location of init.lua.
	cwd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "stray.lua"), []byte(`return {}`), 0o600))

	wd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(wd) })
	require.NoError(t, os.Chdir(cwd))

	path := writeLuaFile(t, `require("stray")`)

	cfg := &testConfig{}
	err = LoadInto(nil, path, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stray")
}

func TestSetSearchDirBuildsPackagePath(t *testing.T) {
	t.Parallel()

	engine := New()
	t.Cleanup(engine.Close)

	pkg := engine.L.GetGlobal("package")
	original := string(engine.L.GetField(pkg, "path").(lua.LString))

	const dir = "/etc/yay"
	engine.SetSearchDir(dir)

	got := string(engine.L.GetField(pkg, "path").(lua.LString))

	// The produced path is the init.lua directory patterns followed by the
	// absolute system entries — never the default "./?.lua" CWD entry.
	expected := filepath.Join(dir, "?.lua") + ";" +
		filepath.Join(dir, "?", "init.lua") + ";" +
		absolutePatterns(original)
	require.Equal(t, expected, got)
}
