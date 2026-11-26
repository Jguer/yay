//go:build !integration

package workdir

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/settings/exe"
	"github.com/Jguer/yay/v13/pkg/text"
)

func TestGitSource(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		source   string
		wantName string
		wantURL  string
		wantOK   bool
	}{
		{"git+https://github.com/openclaw/wacli.git", "wacli", "https://github.com/openclaw/wacli.git", true},
		{"git+https://github.com/openclaw/wacli.git#tag=v0.21.0", "wacli", "https://github.com/openclaw/wacli.git", true},
		{"git+https://example.org/repo/?signed", "repo", "https://example.org/repo/", true},
		{"git://example.org/repo.git", "repo", "git://example.org/repo.git", true},
		{"src::git+https://example.org/repo.git#branch=dev", "src", "https://example.org/repo.git", true},
		{"https://example.org/repo-1.0.tar.gz", "", "", false},
		{"hg+https://example.org/repo", "", "", false},
		{"local.patch", "", "", false},
	}

	for _, tc := range testCases {
		name, url, ok := gitSource(tc.source)
		assert.Equal(t, tc.wantOK, ok, tc.source)
		assert.Equal(t, tc.wantName, name, tc.source)
		assert.Equal(t, tc.wantURL, url, tc.source)
	}
}

func TestRemoveMovedGitClones(t *testing.T) {
	t.Parallel()

	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available")
	}

	pkgBuildDir := t.TempDir()
	srcinfo := "pkgbase = demo\n" +
		"\tpkgver = 1\n" +
		"\tpkgrel = 1\n" +
		"\tarch = any\n" +
		"\tsource = git+https://example.org/new/moved.git\n" +
		"\tsource = git+https://example.org/same/kept.git#tag=v1\n" +
		"\npkgname = demo\n"
	require.NoError(t, os.WriteFile(filepath.Join(pkgBuildDir, ".SRCINFO"), []byte(srcinfo), 0o600))

	makeClone := func(name, url string) {
		dir := filepath.Join(pkgBuildDir, name)
		require.NoError(t, exec.CommandContext(t.Context(), gitBin, "init", "-q", "--bare", dir).Run())
		require.NoError(t, exec.CommandContext(t.Context(), gitBin, "-C", dir, "remote", "add", "origin", url).Run())
	}
	makeClone("moved", "https://example.org/old/moved.git")
	makeClone("kept", "https://example.org/same/kept")

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")
	cmdBuilder := &exe.CmdBuilder{GitBin: gitBin, Runner: &exe.OSRunner{Log: logger}}

	removeMovedGitClones(t.Context(), cmdBuilder, logger, pkgBuildDir)

	assert.NoDirExists(t, filepath.Join(pkgBuildDir, "moved"))
	assert.DirExists(t, filepath.Join(pkgBuildDir, "kept"))
}
