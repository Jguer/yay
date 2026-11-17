package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentHashChangesWithContent(t *testing.T) {
	dir := writeFixture(t, "PKGBUILD", benignPKGBUILD)
	first, err := ContentHash(dir)
	require.NoError(t, err)
	second, err := ContentHash(dir)
	require.NoError(t, err)
	assert.Equal(t, first, second, "hash must be deterministic")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(maliciousPKGBUILD), 0o644))
	third, err := ContentHash(dir)
	require.NoError(t, err)
	assert.NotEqual(t, first, third)

	// VCS metadata must not invalidate the cache on every git operation.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref"), 0o644))
	fourth, err := ContentHash(dir)
	require.NoError(t, err)
	assert.Equal(t, third, fourth)
	// Build artifacts must not invalidate the cache after a successful build.
	for dirName := range map[string]bool{"src": true, "pkg": true} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, dirName), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, dirName, "built.sh"), []byte("output"), 0o644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "evil-1.0-1.pkg.tar.zst"), []byte("pkg"), 0o644))
	fifth, err := ContentHash(dir)
	require.NoError(t, err)
	assert.Equal(t, third, fifth)
}

func TestLoadCachedRoundtrip(t *testing.T) {
	dir := writeFixture(t, "PKGBUILD", maliciousPKGBUILD)
	reportDir := t.TempDir()
	report, err := New(Options{Package: "evil", Dir: dir, ReportDir: reportDir}).Run(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, report.ContentHash)

	cached := LoadCached(reportDir, "evil", report.ContentHash)
	require.NotNil(t, cached, "identical content must hit the cache")
	assert.Equal(t, VerdictMalicious, cached.Verdict)

	assert.Nil(t, LoadCached(reportDir, "evil", "different-hash"))
	assert.Nil(t, LoadCached(reportDir, "missing", report.ContentHash))
}
