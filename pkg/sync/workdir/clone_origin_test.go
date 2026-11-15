//go:build !integration

package workdir

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/dep"
	"github.com/Jguer/yay/v13/pkg/runtime"
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/settings/exe"
)

// A cached clone from another AUR source is refused even when its .SRCINFO
// is new enough for the "PKGBUILD up to date" shortcut.
func TestPrepareWorkspaceRefusesCloneFromOtherSource(t *testing.T) {
	t.Parallel()

	buildDir := t.TempDir()
	dir := filepath.Join(buildDir, "foo")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".SRCINFO"),
		[]byte("pkgbase = foo\n\tpkgver = 5.0\n\tpkgrel = 1\n\tarch = any\n\npkgname = foo\n"), 0o644))

	var gitCalls [][]string

	runner := &exe.MockRunner{CaptureFn: func(cmd *exec.Cmd) (string, string, error) {
		gitCalls = append(gitCalls, cmd.Args[1:])
		if len(cmd.Args) == 4 && cmd.Args[1] == "remote" {
			return "http://local/foo.git\n", "", nil
		}

		return "", "", nil
	}}

	cfg := &settings.Configuration{
		BuildDir: buildDir,
		AURURLs: []settings.AURSource{
			{Name: "mirror", URL: "http://mirror"},
			{Name: "local", URL: "http://local"},
		},
	}
	preper := NewPreparer(nil, &exe.MockBuilder{Runner: runner}, cfg, newTestLogger())

	_, err := preper.PrepareWorkspace(context.Background(), &runtime.Runtime{Cfg: cfg},
		[]map[string]*dep.InstallInfo{{"foo": {Source: dep.AUR, AURBase: "foo", Version: "3.0-1"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(),
		dir+" was cloned from http://local/foo.git but foo is being fetched from http://mirror/foo.git; remove "+dir+" and try again")
	assert.Equal(t, [][]string{{"remote", "get-url", "origin"}}, gitCalls, "nothing is pulled or built")
}
