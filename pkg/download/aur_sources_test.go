//go:build !integration

package download

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/Jguer/aur"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/settings/parser"
)

type mapHTTPClient map[string]string // url -> body; missing urls return 404

func (m mapHTTPClient) Get(url string) (*http.Response, error) {
	body, ok := m[url]
	status := http.StatusOK

	if !ok {
		status = http.StatusNotFound
	}

	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
}

// fakeCloneBuilder records git clone URLs and fails clones from failURLs.
type fakeCloneBuilder struct {
	mu       sync.Mutex
	failURLs map[string]bool
	clones   []string
}

func (f *fakeCloneBuilder) BuildGitCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "git", extraArgs...)
}

func (f *fakeCloneBuilder) Capture(cmd *exec.Cmd) (stdout, stderr string, err error) {
	args := cmd.Args[1:]
	if len(args) >= 4 && args[0] == "clone" {
		url := args[len(args)-2]

		f.mu.Lock()
		f.clones = append(f.clones, url)
		f.mu.Unlock()

		if f.failURLs[url] {
			return "", "fatal: repository not found", errors.New("exit status 128")
		}
	}

	return "", "", nil
}

func (f *fakeCloneBuilder) Show(cmd *exec.Cmd) error { return nil }

type fakeResolver map[string][]string

func (f fakeResolver) URLsFor(name string) []string { return f[name] }

func TestAURPKGBUILDFromAnyFallsBack(t *testing.T) {
	t.Parallel()

	client := mapHTTPClient{
		"https://second.example/cgit/aur.git/plain/PKGBUILD?h=pkg": "pkgbuild-from-second",
	}

	got, err := aurPKGBUILDFromAny(client, "pkg", []string{"https://first.example", "https://second.example"})
	require.NoError(t, err)
	assert.Equal(t, "pkgbuild-from-second", string(got))

	_, err = aurPKGBUILDFromAny(client, "missing", []string{"https://first.example", "https://second.example"})
	require.Error(t, err)
}

func TestAURPKGBUILDReposUsesResolverOrderAndFallsBack(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	builder := &fakeCloneBuilder{failURLs: map[string]bool{"https://first.example/b.git": true}}
	resolver := fakeResolver{
		"a": {"https://second.example", "https://first.example"},
		"b": {"https://first.example", "https://second.example"},
	}

	cloned, err := AURPKGBUILDRepos(context.Background(), builder, newTestLogger(),
		[]string{"a", "b"}, resolver, dir, false)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"a": true, "b": true}, cloned)
	assert.ElementsMatch(t, []string{
		"https://second.example/a.git", // resolved source tried first
		"https://first.example/b.git",  // fails...
		"https://second.example/b.git", // ...falls back to the next source
	}, builder.clones)
}

func TestAURPKGBUILDReposAllSourcesFail(t *testing.T) {
	t.Parallel()

	builder := &fakeCloneBuilder{failURLs: map[string]bool{
		"https://first.example/c.git":  true,
		"https://second.example/c.git": true,
	}}

	cloned, err := AURPKGBUILDRepos(context.Background(), builder, newTestLogger(),
		[]string{"c"}, StaticAURURLs{"https://first.example", "https://second.example"}, t.TempDir(), false)
	require.Error(t, err)
	assert.Empty(t, cloned)
	assert.Len(t, builder.clones, 2)
}

// pinningAUR accepts "mirror" as an AUR source prefix and records pins.
type pinningAUR struct {
	pinned map[string]string
}

func (p *pinningAUR) Get(context.Context, *aur.Query) ([]aur.Pkg, error) {
	return []aur.Pkg{{Name: "foo", PackageBase: "foo"}}, nil
}

func (p *pinningAUR) PinAURSource(pkgName, source string) bool {
	if source != "mirror" {
		return false
	}

	p.pinned[pkgName] = source

	return true
}

func (p *pinningAUR) UnpinAURSource(pkgName string) {
	p.pinned[pkgName] = "<unpinned>"
}

func TestGetPackageUsableNameAcceptsAURSourcePrefix(t *testing.T) {
	t.Parallel()

	client := &pinningAUR{pinned: map[string]string{}}
	searcher := &testDBSearcher{absPackagesDB: map[string]string{}}

	dbName, name, isAUR, skip := getPackageUsableName(searcher, client, newTestLogger(), "mirror/foo", parser.ModeAny)
	assert.Equal(t, "aur", dbName)
	assert.Equal(t, "foo", name)
	assert.True(t, isAUR)
	assert.False(t, skip)
	assert.Equal(t, map[string]string{"foo": "mirror"}, client.pinned)

	dbName, _, isAUR, skip = getPackageUsableName(searcher, client, newTestLogger(), "aur/bar", parser.ModeAny)
	assert.Equal(t, "aur", dbName)
	assert.True(t, isAUR)
	assert.False(t, skip)
	assert.Equal(t, "<unpinned>", client.pinned["bar"], "aur/<pkg> drops a recorded source")

	// Unknown prefixes are still treated as pacman repositories.
	dbName, name, isAUR, skip = getPackageUsableName(searcher, client, newTestLogger(), "nosuchrepo/foo", parser.ModeAny)
	assert.Equal(t, "nosuchrepo", dbName)
	assert.Equal(t, "foo", name)
	assert.True(t, isAUR)
	assert.True(t, skip)
}
