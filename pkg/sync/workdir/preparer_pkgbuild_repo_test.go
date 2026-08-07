//go:build !integration

package workdir

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	glua "github.com/yuin/gopher-lua"

	"github.com/Jguer/yay/v13/pkg/db/mock"
	"github.com/Jguer/yay/v13/pkg/dep"
	"github.com/Jguer/yay/v13/pkg/runtime"
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/settings/exe"
	settingslua "github.com/Jguer/yay/v13/pkg/settings/lua"
)

// GIVEN a PKGBUILD-repo target whose PKGBUILD is already local
// WHEN the workspace is prepared
// THEN its build dir is the repo directory and no git clone is performed.
func TestPrepareWorkspacePkgbuildRepoUsesLocalDir(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	cfg := &settings.Configuration{BuildDir: t.TempDir(), ReDownload: "no"}

	runner := &exe.MockRunner{}
	builder := &exe.MockBuilder{Runner: runner}

	preper := NewPreparerWithoutHooks(&mock.DBExecutor{}, builder, cfg, newTestLogger(), false)

	targets := []map[string]*dep.InstallInfo{
		{"foo": {Source: dep.PkgbuildRepo, AURBase: "foo", SrcinfoPath: repoDir, Version: "1-1"}},
	}

	dirs, err := preper.PrepareWorkspace(t.Context(), nil, targets)
	require.NoError(t, err)

	assert.Equal(t, repoDir, dirs["foo"])
	assert.Empty(t, runner.CaptureCalls)
}

// GIVEN a PKGBUILD-repo target and source downloading enabled
// WHEN the workspace is prepared
// THEN the repo directory is never git reset/merged (it is not a per-package
// clone), unlike AUR/SrcInfo checkouts.
func TestPrepareWorkspacePkgbuildRepoNotMerged(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	cfg := &settings.Configuration{BuildDir: t.TempDir(), ReDownload: "no", MaxConcurrentDownloads: 1}

	runner := &exe.MockRunner{}
	builder := &exe.MockBuilder{Runner: runner}

	dbExe := &mock.DBExecutor{InstalledRemotePackageNamesFn: func() []string { return nil }}
	preper := NewPreparerWithoutHooks(dbExe, builder, cfg, newTestLogger(), true)

	run := &runtime.Runtime{Cfg: cfg, Logger: newTestLogger(), CmdBuilder: builder}

	targets := []map[string]*dep.InstallInfo{
		{"foo": {Source: dep.PkgbuildRepo, AURBase: "foo", SrcinfoPath: repoDir, Version: "1-1"}},
	}

	dirs, err := preper.PrepareWorkspace(t.Context(), run, targets)
	require.NoError(t, err)
	assert.Equal(t, repoDir, dirs["foo"])

	for _, call := range runner.CaptureCalls {
		cmd := call.Args[0].(*exec.Cmd)
		assert.NotContains(t, cmd.Args, "reset", "repo dir must not be git reset")
		assert.NotContains(t, cmd.Args, "merge", "repo dir must not be git merged")
	}
}

// GIVEN a PKGBUILD-repo target and Lua autocmds registered for AURPreInstall
// and AURPostDownload
// WHEN the workspace is prepared
// THEN both hooks fire for the pkgbuild-repo base, proving Lua hook
// invocation is not gated to dep.AUR/dep.SrcInfo sources.
func TestPrepareWorkspacePkgbuildRepoFiresLuaHooks(t *testing.T) {
	t.Parallel()

	base := "foo"
	repoDir := writeAURPreInstallPackage(t, base)
	cfg := &settings.Configuration{BuildDir: t.TempDir(), ReDownload: "no", MaxConcurrentDownloads: 1}

	runner := &exe.MockRunner{}
	builder := &exe.MockBuilder{Runner: runner}

	dbExe := &mock.DBExecutor{InstalledRemotePackageNamesFn: func() []string { return nil }}
	preper := NewPreparerWithoutHooks(dbExe, builder, cfg, newTestLogger(), true)

	engine := settingslua.New()
	t.Cleanup(engine.Close)

	fired := []string{}
	engine.L.SetGlobal("record", engine.L.NewFunction(func(L *glua.LState) int {
		fired = append(fired, L.CheckString(1))
		return 0
	}))
	require.NoError(t, engine.L.DoString(`
		yay.create_autocmd("AURPreInstall", {
			callback = function(event) record("pre:" .. event.match) end,
		})
		yay.create_autocmd("AURPostDownload", {
			callback = function(event) record("post:" .. event.match) end,
		})
	`))

	run := &runtime.Runtime{Cfg: cfg, Logger: newTestLogger(), CmdBuilder: builder, Lua: engine}

	targets := []map[string]*dep.InstallInfo{
		{"demo": {Source: dep.PkgbuildRepo, AURBase: base, SrcinfoPath: repoDir, Version: "1:1.2.3-4"}},
	}

	dirs, err := preper.PrepareWorkspace(t.Context(), run, targets)
	require.NoError(t, err)
	assert.Equal(t, repoDir, dirs[base])
	assert.Equal(t, []string{"pre:" + base, "post:" + base}, fired)
}

func TestShouldCleanAURDirsExcludesPkgbuildRepos(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	preper := NewPreparerWithoutHooks(&mock.DBExecutor{}, &exe.MockBuilder{},
		&settings.Configuration{CleanAfter: true}, newTestLogger(), false)

	hook := preper.ShouldCleanAURDirs(nil, map[string]string{"foo": repoDir}, []map[string]*dep.InstallInfo{
		{"foo": {Source: dep.PkgbuildRepo, AURBase: "foo", SrcinfoPath: repoDir}},
	})

	assert.Nil(t, hook)
}
