//go:build !integration

package pkgbuildrepo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writePkgbuild(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte("# pkgbuild\n"), 0o600))
}

// GIVEN a repo tree with PKGBUILDs at several depths
// WHEN scanned with a depth bound
// THEN only PKGBUILD directories within the bound are returned, sorted, and
// version-control/hidden directories are skipped.
func TestScanRespectsDepthAndSkipsHidden(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writePkgbuild(t, root)                                    // level 0
	writePkgbuild(t, filepath.Join(root, "foo"))              // level 1
	writePkgbuild(t, filepath.Join(root, "bar", "baz"))       // level 2
	writePkgbuild(t, filepath.Join(root, "a", "b", "c", "d")) // level 4 - excluded

	// A .git dir containing a stray PKGBUILD must not be indexed.
	writePkgbuild(t, filepath.Join(root, ".git", "hooksdir"))

	dirs, err := Scan(root, 3)
	require.NoError(t, err)

	assert.Equal(t, []string{
		root,
		filepath.Join(root, "bar", "baz"),
		filepath.Join(root, "foo"),
	}, dirs)
}

// GIVEN a repo with a single top-level PKGBUILD
// WHEN scanned with depth 0
// THEN only the root is returned.
func TestScanDepthZeroOnlyRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writePkgbuild(t, root)
	writePkgbuild(t, filepath.Join(root, "foo"))

	dirs, err := Scan(root, 0)
	require.NoError(t, err)

	assert.Equal(t, []string{root}, dirs)
}

func TestScanIgnoresSymlinkedPKGBUILD(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "target")
	writePkgbuild(t, target)

	linkDir := filepath.Join(root, "link")
	require.NoError(t, os.MkdirAll(linkDir, 0o755))
	require.NoError(t, os.Symlink(filepath.Join(target, "PKGBUILD"), filepath.Join(linkDir, "PKGBUILD")))

	dirs, err := Scan(root, 1)
	require.NoError(t, err)
	assert.Equal(t, []string{target}, dirs)
}
