//go:build !integration

package pkgbuildrepo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/settings/exe"
)

// GIVEN a git-URL repo
// WHEN refreshed into an empty cache
// THEN it is cloned and the scan directory points at the clone.
func TestRefreshClonesGitRepo(t *testing.T) {
	t.Parallel()

	// A not-yet-existing cache dir must be created before cloning.
	cache := filepath.Join(t.TempDir(), "repos")
	runner := &exe.MockRunner{}
	builder := &exe.MockBuilder{Runner: runner}

	loc, err := Refresh(context.Background(), builder, "yay-pkgbuild",
		"https://github.com/Jguer/yay-PKGBUILD", cache, false)
	require.NoError(t, err)

	_, statErr := os.Stat(cache)
	require.NoError(t, statErr)

	assert.True(t, loc.IsGit)
	assert.Equal(t, filepath.Join(cache, "yay-pkgbuild"), loc.Dir)

	require.Len(t, runner.CaptureCalls, 1)
	cmd := runner.CaptureCalls[0].Args[0].(*exec.Cmd)
	assert.Contains(t, cmd.Args, "clone")
	assert.Contains(t, cmd.Args, "https://github.com/Jguer/yay-PKGBUILD")
	assert.Contains(t, cmd.Args, "yay-pkgbuild")
}

// GIVEN a file:// directory repo
// WHEN refreshed
// THEN no git command runs and the scan directory is the local path.
func TestRefreshLocalDirNoGit(t *testing.T) {
	t.Parallel()

	runner := &exe.MockRunner{}
	builder := &exe.MockBuilder{Runner: runner}

	loc, err := Refresh(context.Background(), builder, "local", "file:///srv/pkgbuilds", t.TempDir(), false)
	require.NoError(t, err)

	assert.False(t, loc.IsGit)
	assert.Equal(t, "/srv/pkgbuilds", loc.Dir)
	assert.Empty(t, runner.CaptureCalls)
}
