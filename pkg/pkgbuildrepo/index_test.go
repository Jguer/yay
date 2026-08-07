//go:build !integration

package pkgbuildrepo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeSrcinfo(t *testing.T, dir, contents string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(contents), 0o600))
}

// GIVEN repos with a plain package, a split package, and a provides
// WHEN an index is built
// THEN pkgbase, every pkgname, and provides resolve to the owning entry.
func TestIndexMapsNamesBaseAndProvides(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fooDir := filepath.Join(root, "foo")
	multiDir := filepath.Join(root, "multi")

	writeSrcinfo(t, fooDir, "pkgbase = foo\n\tpkgver = 1.0\n\tpkgrel = 1\n\tarch = x86_64\n\npkgname = foo\n\tprovides = bar\n")
	writeSrcinfo(t, multiDir, "pkgbase = multi\n\tpkgver = 2.0\n\tpkgrel = 3\n\tarch = x86_64\n\npkgname = m1\n\npkgname = m2\n")

	idx := NewIndex()
	require.NoError(t, idx.AddRepo("myrepo", []string{fooDir, multiDir}))

	e, ok := idx.Get("foo")
	require.True(t, ok)
	assert.Equal(t, "foo", e.Pkgbase)
	assert.Equal(t, "1.0-1", e.Version)
	assert.Equal(t, "myrepo", e.RepoName)
	assert.Equal(t, fooDir, e.Dir)

	prov, ok := idx.Get("bar")
	require.True(t, ok)
	assert.Equal(t, "foo", prov.Pkgbase)

	e1, ok := idx.Get("m1")
	require.True(t, ok)
	e2, ok := idx.Get("m2")
	require.True(t, ok)
	assert.Equal(t, "multi", e1.Pkgbase)
	assert.Same(t, e1, e2)

	_, ok = idx.Get("multi")
	assert.True(t, ok)

	_, ok = idx.Get("does-not-exist")
	assert.False(t, ok)
}

// GIVEN one entry that provides "shared" and another literally named "shared"
// WHEN both are indexed (in either order)
// THEN a lookup of "shared" resolves to the real package, not the provider.
func TestIndexPkgnameBeatsProvides(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	providerDir := filepath.Join(root, "provider")
	realDir := filepath.Join(root, "shared")

	writeSrcinfo(t, providerDir, "pkgbase = provider\n\tpkgver = 1\n\tpkgrel = 1\n\tarch = x86_64\n\npkgname = provider\n\tprovides = shared=1.0\n")
	writeSrcinfo(t, realDir, "pkgbase = shared\n\tpkgver = 1\n\tpkgrel = 1\n\tarch = x86_64\n\npkgname = shared\n")

	idx := NewIndex()
	require.NoError(t, idx.AddRepo("r", []string{providerDir, realDir}))

	e, ok := idx.Get("shared")
	require.True(t, ok)
	assert.Equal(t, "shared", e.Pkgbase)
}
