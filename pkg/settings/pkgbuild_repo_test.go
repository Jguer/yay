//go:build !integration

package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/settings/lua"
)

// GIVEN an init.lua declaring pkgbuild_repos
// WHEN it is loaded onto a Configuration
// THEN the repos are applied, keyed by name and sorted deterministically
func TestPkgbuildReposFromLua(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	luaPath := filepath.Join(dir, "init.lua")
	require.NoError(t, os.WriteFile(luaPath, []byte(`
		yay.opt.pkgbuild_repos = {
			["yay-pkgbuild"] = { url = "https://github.com/Jguer/yay-PKGBUILD", depth = 2 },
			["local-repo"] = { url = "file:///srv/pkgbuilds" },
		}
	`), 0o600))

	cfg := DefaultConfig("test")
	require.NoError(t, lua.LoadInto(nil, luaPath, cfg))

	require.Len(t, cfg.PkgbuildRepos, 2)

	assert.Equal(t, "local-repo", cfg.PkgbuildRepos[0].Name)
	assert.Equal(t, "file:///srv/pkgbuilds", cfg.PkgbuildRepos[0].URL)

	assert.Equal(t, "yay-pkgbuild", cfg.PkgbuildRepos[1].Name)
	assert.Equal(t, "https://github.com/Jguer/yay-PKGBUILD", cfg.PkgbuildRepos[1].URL)
	assert.Equal(t, 2, cfg.PkgbuildRepos[1].Depth)
}

// GIVEN repos where some omit depth
// WHEN NormalizePkgbuildRepos runs
// THEN missing depths fall back to the default of 3 and explicit depths are kept
func TestNormalizePkgbuildReposDefaultsDepth(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("test")
	cfg.PkgbuildRepos = []PkgbuildRepo{
		{Name: "a", URL: "https://example.invalid/a"},
		{Name: "b", URL: "https://example.invalid/b", Depth: 1},
	}

	require.NoError(t, cfg.NormalizePkgbuildRepos())

	assert.Equal(t, DefaultPkgbuildRepoDepth, cfg.PkgbuildRepos[0].Depth)
	assert.Equal(t, 1, cfg.PkgbuildRepos[1].Depth)
}

func TestNormalizePkgbuildReposRejectsUnsafeName(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("test")
	cfg.PkgbuildRepos = []PkgbuildRepo{{Name: "../outside", URL: "https://example.invalid/a"}}

	assert.Error(t, cfg.NormalizePkgbuildRepos())
}
