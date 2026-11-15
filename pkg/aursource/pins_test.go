//go:build !integration

package aursource

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Jguer/aur"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/text"
)

var threeSources = []settings.AURSource{
	{Name: "aur", URL: "https://aur.archlinux.org"},
	{Name: "mirror", URL: "https://aur.example.org"},
	{Name: "other", URL: "https://other.example.org"},
}

func TestPinFileOps(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "sub", PinFileName)

	f, err := LoadPinFile(path)
	require.NoError(t, err)
	assert.Empty(t, f.Packages)

	require.NoError(t, f.Save(path))
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err), "an empty pin file is not created")

	assert.True(t, f.Record("foo", "mirror"))
	assert.False(t, f.Record("foo", "mirror"))
	assert.True(t, f.Record("bar", "other"))
	assert.True(t, f.Record("baz", "mirror"))
	assert.True(t, f.Record("baz", ""), "the default source removes the pin")
	assert.False(t, f.Record("baz", ""))
	require.NoError(t, f.Save(path))

	f, err = LoadPinFile(path)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"foo": "mirror", "bar": "other"}, f.Packages)
	assert.Equal(t, []string{"bar", "foo"}, f.Names())

	assert.Equal(t, []string{"foo"}, f.RenameSource("mirror", "mirror2"))
	assert.Equal(t, []string{"bar"}, f.DropSource("other"))
	assert.Equal(t, map[string]string{"foo": "mirror2"}, f.Packages)
	assert.Equal(t, []string{"foo"}, f.Prune(func(string) bool { return false }))
	assert.Empty(t, f.Packages)

	require.NoError(t, f.Save(path))
	f, err = LoadPinFile(path)
	require.NoError(t, err)
	assert.Empty(t, f.Packages, "an existing file is emptied")

	require.NoError(t, os.WriteFile(path, []byte("{nope"), 0o644))
	_, err = LoadPinFile(path)
	assert.ErrorContains(t, err, path)
}

func pinTestClient(logOut io.Writer) *Client {
	return New(threeSources, []aur.QueryClient{
		returning([]aur.Pkg{{Name: "shared", Version: "1"}, {Name: "only-aur", Version: "1"}}, nil),
		returning([]aur.Pkg{{Name: "shared", Version: "2"}}, nil),
		returning([]aur.Pkg{{Name: "shared", Version: "3"}, {Name: "only-aur", Version: "3"}}, nil),
	}, text.NewLogger(logOut, logOut, strings.NewReader(""), false, "test"))
}

func versions(t *testing.T, c *Client) map[string]string {
	t.Helper()

	pkgs, err := c.Get(context.Background(), &aur.Query{Needles: []string{"shared", "only-aur"}, By: aur.Name})
	require.NoError(t, err)

	got := map[string]string{}
	for _, p := range pkgs {
		got[p.Name] = p.Version
	}

	return got
}

func TestLoadPinsPrefersRecordedSource(t *testing.T) {
	t.Parallel()

	var log bytes.Buffer
	c := pinTestClient(&log)

	missing := c.LoadPins(&PinFile{Packages: map[string]string{
		"shared":      "mirror",
		"only-aur":    "mirror", // mirror lacks it
		"removed":     "mirror",
		"stale":       "gone",
		"notinstalle": "gone",
	}}, func(name string) bool { return name != "removed" && name != "notinstalle" })
	assert.Equal(t, map[string]string{"stale": "gone"}, missing)

	got := versions(t, c)
	assert.Equal(t, "2", got["shared"], "the recorded source wins over priority")
	assert.Equal(t, "1", got["only-aur"], "falls back to priority order")
	assert.Equal(t, "mirror", c.SourceFor("shared").Name)
	assert.Equal(t, "aur", c.SourceFor("only-aur").Name, "the fallback source is used for downloads")
	assert.Contains(t, log.String(),
		"only-aur was installed from AUR source 'mirror', which no longer has it; using 'aur' instead")

	log.Reset()
	versions(t, c)
	assert.Empty(t, log.String(), "the fallback is warned about once")
}

func TestExplicitTargetsOverrideRecordedSource(t *testing.T) {
	t.Parallel()

	var log bytes.Buffer
	c := pinTestClient(&log)
	c.LoadPins(&PinFile{Packages: map[string]string{"shared": "mirror", "only-aur": "other"}},
		func(string) bool { return true })

	require.True(t, c.PinAURSource("shared", "other"))
	c.UnpinAURSource("only-aur")

	got := versions(t, c)
	assert.Equal(t, "3", got["shared"], "<source>/<pkg> beats the recorded source")
	assert.Equal(t, "1", got["only-aur"], "aur/<pkg> resolves in priority order")

	c.PinAURSource("only-aur", "mirror")
	versions(t, c)
	assert.Contains(t, log.String(), "package 'only-aur' was not found in AUR source 'mirror', using 'aur' instead")
}

func TestRecordInstalled(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), PinFileName)
	require.NoError(t, os.WriteFile(path,
		[]byte(`{"packages":{"shared":"other","only-aur":"mirror","uninstalled":"mirror"}}`), 0o644))

	c := pinTestClient(io.Discard)
	require.True(t, c.PinAURSource("shared", "mirror"))
	versions(t, c) // only-aur resolves from the first source

	installed := func(name string) bool { return name == "kept" }
	require.NoError(t, c.RecordInstalled(path, []string{"shared", "only-aur"}, installed))

	f, err := LoadPinFile(path)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"shared": "mirror"}, f.Packages,
		"pins are updated, the default source clears them and uninstalled packages are pruned")

	single := New(testSources[:1], []aur.QueryClient{returning(nil, nil)}, nil)
	other := filepath.Join(t.TempDir(), PinFileName)
	require.NoError(t, single.RecordInstalled(other, []string{"x"}, installed))
	_, err = os.Stat(other)
	assert.True(t, os.IsNotExist(err), "nothing is recorded with a single source")
}
