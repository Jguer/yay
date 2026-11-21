//go:build !integration

package workdir

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/db/mock"
	"github.com/Jguer/yay/v13/pkg/dep"
	"github.com/Jguer/yay/v13/pkg/runtime"
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/settings/exe"
)

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	return strings.TrimSpace(string(out))
}

func commitPkgver(t *testing.T, dir, ver string) string {
	t.Helper()

	srcinfo := "pkgbase = foo\n\tpkgver = " + ver + "\n\tpkgrel = 1\n\tarch = any\n\npkgname = foo\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(srcinfo), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte("pkgver="+ver+"\n"), 0o600))
	gitT(t, dir, "add", ".")
	gitT(t, dir, "commit", "-q", "-m", ver)

	return gitT(t, dir, "rev-parse", "HEAD")
}

// A cached PKGBUILD repo that never had a diff reviewed has no AUR_SEEN.
// The commit it had before the update must become AUR_SEEN, otherwise the
// diff menu compares against an empty tree and shows every line as new.
func TestPrepareWorkspaceSeedsSeenRefBeforeUpdate(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	aur := t.TempDir()
	remote := filepath.Join(aur, "foo.git")
	require.NoError(t, os.Mkdir(remote, 0o755))
	gitT(t, remote, "init", "-q", "-b", "master")
	oldCommit := commitPkgver(t, remote, "1.0")

	buildDir := t.TempDir()
	gitT(t, buildDir, "clone", "-q", remote, "foo")
	newCommit := commitPkgver(t, remote, "2.0")

	logger := newTestLogger()
	cmdBuilder := &exe.CmdBuilder{GitBin: "git", GitFlags: []string{}, Runner: exe.NewOSRunner(logger)}
	cfg := &settings.Configuration{BuildDir: buildDir, AURURL: "file://" + aur, DiffMenu: true}
	preper := NewPreparerWithoutHooks(&mock.DBExecutor{}, cmdBuilder, cfg, logger, false)
	run := &runtime.Runtime{Cfg: cfg, Logger: logger, CmdBuilder: cmdBuilder}
	targets := []map[string]*dep.InstallInfo{{
		"foo": {Source: dep.AUR, AURBase: "foo", Version: "2.0-1"},
	}}

	_, err := preper.PrepareWorkspace(context.Background(), run, targets)
	require.NoError(t, err)

	cache := filepath.Join(buildDir, "foo")
	require.Equal(t, newCommit, gitT(t, cache, "rev-parse", "HEAD"))
	require.Equal(t, oldCommit, gitT(t, cache, "rev-parse", "AUR_SEEN"))
}
