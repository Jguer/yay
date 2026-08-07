//go:build !integration

package pkgbuildrepo

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/settings/exe"
)

// GIVEN a local file:// repo containing one package
// WHEN loaded
// THEN the package is scanned and indexed to its directory without any git use.
func TestLoadIndexesLocalRepo(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	pkgDir := filepath.Join(repoDir, "foo")
	writePkgbuild(t, pkgDir)
	writeSrcinfo(t, pkgDir, "pkgbase = foo\n\tpkgver = 1\n\tpkgrel = 1\n\tarch = x86_64\n\npkgname = foo\n")

	runner := &exe.MockRunner{}
	builder := &exe.MockBuilder{Runner: runner}

	idx, err := Load(context.Background(), builder,
		[]RepoConfig{{Name: "myrepo", URL: "file://" + repoDir, Depth: 3}},
		t.TempDir(), false)
	require.NoError(t, err)

	e, ok := idx.Get("foo")
	require.True(t, ok)
	assert.Equal(t, pkgDir, e.Dir)
	assert.Equal(t, "myrepo", e.RepoName)

	assert.Empty(t, runner.CaptureCalls)
}

// GIVEN a repo package that ships a PKGBUILD but no .SRCINFO
// WHEN loaded
// THEN it is rejected without executing the PKGBUILD.
func TestLoadRejectsMissingSrcinfoWithoutExecutingPKGBUILD(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	pkgDir := filepath.Join(repoDir, "foo")
	writePkgbuild(t, pkgDir)

	runner := &exe.MockRunner{
		CaptureFn: func(*exec.Cmd) (string, string, error) {
			t.Fatal("loading must not execute PKGBUILD content")
			return "", "", nil
		},
	}
	builder := &exe.MockBuilder{Runner: runner}

	_, err := Load(context.Background(), builder,
		[]RepoConfig{{Name: "r", URL: "file://" + repoDir, Depth: 3}},
		t.TempDir(), false)
	require.Error(t, err)
	assert.Empty(t, runner.CaptureCalls)
}
