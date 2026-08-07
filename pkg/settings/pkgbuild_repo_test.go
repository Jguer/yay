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

// GIVEN an init.lua that sets "name" inside an entry table
// WHEN it is loaded
// THEN it is rejected: the name comes from the table key, and honoring the
// inner key would let two entries collapse onto one name.
func TestPkgbuildReposRejectsNameOverride(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	luaPath := filepath.Join(dir, "init.lua")
	require.NoError(t, os.WriteFile(luaPath, []byte(`
		yay.opt.pkgbuild_repos = {
			["a"] = { name = "dup", url = "https://example.invalid/a" },
			["b"] = { name = "dup", url = "https://example.invalid/b" },
		}
	`), 0o600))

	cfg := DefaultConfig("test")
	assert.Error(t, lua.LoadInto(nil, luaPath, cfg))
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

	for _, name := range []string{"../outside", "."} {
		cfg := DefaultConfig("test")
		cfg.PkgbuildRepos = []PkgbuildRepo{{Name: name, URL: "https://example.invalid/a"}}

		assert.Error(t, cfg.NormalizePkgbuildRepos(), name)
	}
}

// GIVEN URLs that Resolve would otherwise reinterpret as a local path
// WHEN they are normalized
// THEN they are rejected up front with a clear error instead of failing later
// as a confusing "no such file or directory".
func TestNormalizePkgbuildReposRejectsUnsafeURL(t *testing.T) {
	t.Parallel()

	for _, url := range []string{
		"",                                  // no url at all
		"http://example.invalid/repo.git",   // tamperable transport
		"git://example.invalid/repo.git",    // tamperable transport
		"git+http://example.invalid/r.git",  // tamperable transport behind git+
		"ftp://example.invalid/repo",        // unsupported scheme
		"pkgbuilds",                         // relative to yay's cwd
		"--upload-pack=/bin/false/repo.git", // would reach git as an option
	} {
		cfg := DefaultConfig("test")
		cfg.PkgbuildRepos = []PkgbuildRepo{{Name: "r", URL: url}}

		assert.Error(t, cfg.NormalizePkgbuildRepos(), "url %q must be rejected", url)
	}
}

func TestNormalizePkgbuildReposAcceptsSupportedURLs(t *testing.T) {
	t.Parallel()

	for _, url := range []string{
		"https://github.com/Jguer/yay-PKGBUILD",
		"ssh://git@example.invalid/repo.git",
		"git+file:///srv/pkgbuilds",
		"file:///srv/pkgbuilds",
		"/srv/pkgbuilds",
		"git@example.invalid:user/repo.git",
	} {
		cfg := DefaultConfig("test")
		cfg.PkgbuildRepos = []PkgbuildRepo{{Name: "r", URL: url}}

		assert.NoError(t, cfg.NormalizePkgbuildRepos(), "url %q must be accepted", url)
	}
}

// GIVEN two repos that resolve to the same name
// WHEN normalized
// THEN the duplicate is rejected, because both would share one cache checkout.
func TestNormalizePkgbuildReposRejectsDuplicateName(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("test")
	cfg.PkgbuildRepos = []PkgbuildRepo{
		{Name: "dup", URL: "https://example.invalid/a"},
		{Name: "dup", URL: "https://example.invalid/b"},
	}

	assert.Error(t, cfg.NormalizePkgbuildRepos())
}
