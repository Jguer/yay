//go:build !integration

package download

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/settings/parser"
)

// originGitBuilder answers `git remote get-url origin` from origins (keyed by
// directory) and records every other git command.
type originGitBuilder struct {
	mu      sync.Mutex
	origins map[string]string
	ran     [][]string
}

func (b *originGitBuilder) BuildGitCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", extraArgs...)
	cmd.Dir = dir

	return cmd
}

func (b *originGitBuilder) Capture(cmd *exec.Cmd) (stdout, stderr string, err error) {
	args := cmd.Args[1:]
	if len(args) == 3 && args[0] == "remote" && args[1] == "get-url" {
		url, ok := b.origins[cmd.Dir]
		if !ok {
			return "", "error: No such remote 'origin'", errors.New("exit status 2")
		}

		return url + "\n", "", nil
	}

	b.mu.Lock()
	b.ran = append(b.ran, args)
	b.mu.Unlock()

	return "", "", nil
}

func (b *originGitBuilder) Show(*exec.Cmd) error { return nil }

func fakeClone(t *testing.T, dest, name string) string {
	t.Helper()

	dir := filepath.Join(dest, name)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))

	return dir
}

func TestCheckCloneOrigin(t *testing.T) {
	t.Parallel()

	dest := t.TempDir()
	dir := fakeClone(t, dest, "foo")
	noRemote := fakeClone(t, dest, "bar")
	both := []string{"http://mirror", "http://local"}

	b := &originGitBuilder{origins: map[string]string{dir: "http://local/foo.git"}}
	ctx := context.Background()

	err := CheckCloneOrigin(ctx, b, dir, "foo", both)

	var mismatch ErrCloneOriginMismatch
	require.ErrorAs(t, err, &mismatch)
	assert.Equal(t, ErrCloneOriginMismatch{
		Dir: dir, PkgName: "foo", Current: "http://local/foo.git", Expected: "http://mirror/foo.git",
	}, mismatch)
	assert.Contains(t, err.Error(), "remove "+dir+" and try again")

	assert.NoError(t, CheckCloneOrigin(ctx, b, dir, "foo", []string{"http://local", "http://mirror"}))
	assert.NoError(t, CheckCloneOrigin(ctx, b, dir, "foo", []string{"http://local/", "http://mirror"}),
		"trailing slashes and .git suffixes are ignored")
	b.origins[dir] = "http://local/foo"
	assert.NoError(t, CheckCloneOrigin(ctx, b, dir, "foo", []string{"http://local", "http://mirror"}))

	assert.NoError(t, CheckCloneOrigin(ctx, b, dir, "foo", []string{"http://mirror"}),
		"a single AUR source keeps the old behaviour")
	assert.NoError(t, CheckCloneOrigin(ctx, b, noRemote, "bar", both), "unreadable remotes are not checked")
	assert.NoError(t, CheckCloneOrigin(ctx, b, filepath.Join(dest, "missing"), "missing", both))
}

type fixedURLs map[string][]string

func (f fixedURLs) URLsFor(name string) []string { return f[name] }

func TestAURPKGBUILDReposRefusesCloneFromOtherSource(t *testing.T) {
	t.Parallel()

	dest := t.TempDir()
	foo := fakeClone(t, dest, "foo")
	same := fakeClone(t, dest, "same")

	b := &originGitBuilder{origins: map[string]string{
		foo:  "http://local/foo.git",
		same: "http://mirror/same.git",
	}}

	_, err := AURPKGBUILDRepos(context.Background(), b, newTestLogger(), []string{"foo", "same"},
		fixedURLs{"foo": {"http://mirror", "http://local"}, "same": {"http://mirror", "http://local"}},
		dest, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), foo+" was cloned from http://local/foo.git but foo is being fetched from http://mirror/foo.git")
	assert.Equal(t, [][]string{{"pull", "--rebase", "--autostash"}}, b.ran,
		"only the clone from the right source is updated")

	// --force (-Gf) replaces the clone instead.
	b.ran = nil
	_, err = AURPKGBUILDRepos(context.Background(), b, newTestLogger(), []string{"foo"},
		fixedURLs{"foo": {"http://mirror", "http://local"}}, dest, true)
	require.NoError(t, err)
	require.Len(t, b.ran, 1)
	assert.Equal(t, "http://mirror/foo.git", b.ran[0][len(b.ran[0])-2])
}

func TestPKGBUILDReposRefusesCloneFromOtherSource(t *testing.T) {
	t.Parallel()

	dest := t.TempDir()
	foo := fakeClone(t, dest, "foo")
	b := &originGitBuilder{origins: map[string]string{foo: "http://local/foo.git"}}
	client := &pinningAUR{pinned: map[string]string{}}
	searcher := &testDBSearcher{absPackagesDB: map[string]string{}}

	_, err := PKGBUILDRepos(context.Background(), searcher, client, b, newTestLogger(),
		[]string{"mirror/foo"}, parser.ModeAUR,
		fixedURLs{"foo": {"http://mirror", "http://local"}}, dest, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remove "+foo+" and try again")
	assert.Empty(t, b.ran)
}
