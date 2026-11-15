//go:build !integration

package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/settings/parser"
)

func TestAURSourcesDefaultsToAURURL(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("test")
	assert.Equal(t, []AURSource{{
		Name: "aur", URL: "https://aur.archlinux.org", RPCURL: "https://aur.archlinux.org/rpc?",
	}}, cfg.AURSources())

	cfg.AURURL = "https://aur.example.org/"
	cfg.AURRPCURL = "https://rpc.example.org"
	assert.Equal(t, []AURSource{{
		Name: "aur", URL: "https://aur.example.org", RPCURL: "https://rpc.example.org/rpc?",
	}}, cfg.AURSources())
}

func TestAURSourcesJSONStringAndObjectEntries(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("test")
	require.NoError(t, json.Unmarshal([]byte(`{
		"aururls": [
			"https://aur.archlinux.org/",
			{"name": "mirror", "url": "https://aur.example.org", "rpcurl": "https://aur.example.org/rpc"},
			"https://aur.archlinux.org",
			{"url": "https://other.example.net"}
		]
	}`), cfg))

	assert.Equal(t, []AURSource{
		{Name: "aur", URL: "https://aur.archlinux.org", RPCURL: "https://aur.archlinux.org/rpc?"},
		{Name: "mirror", URL: "https://aur.example.org", RPCURL: "https://aur.example.org/rpc?"},
		{Name: "other.example.net", URL: "https://other.example.net", RPCURL: "https://other.example.net/rpc?"},
	}, cfg.AURSources(), "duplicates are dropped and names derived")

	assert.Equal(t, []string{
		"https://aur.archlinux.org", "https://aur.example.org", "https://other.example.net",
	}, cfg.AURBaseURLs())
}

func TestAURSourceMarshalJSONRoundTrip(t *testing.T) {
	t.Parallel()

	in := []AURSource{
		{URL: "https://aur.archlinux.org"},
		{Name: "mirror", URL: "https://aur.example.org", RPCURL: "https://aur.example.org/rpc"},
	}

	b, err := json.Marshal(in)
	require.NoError(t, err)
	assert.JSONEq(t, `["https://aur.archlinux.org",{"name":"mirror","url":"https://aur.example.org","rpcurl":"https://aur.example.org/rpc"}]`, string(b))

	var out []AURSource
	require.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, in, out)
}

func TestSaveOmitsUnsetAURURLs(t *testing.T) {
	t.Parallel()

	b, err := json.Marshal(DefaultConfig("test"))
	require.NoError(t, err)
	assert.NotContains(t, string(b), "aururls")
}

func parseArgs(t *testing.T, cfg *Configuration, args ...string) {
	t.Helper()

	a := parser.MakeArguments()
	for i := 0; i < len(args); i++ {
		if i+1 < len(args) {
			// mirrors parser.addParam, which splits values on commas
			a.CreateOrAppendOption(args[i], strings.Split(args[i+1], ",")...)
			i++
		}
	}

	cfg.extractYayOptions(a)
}

func TestCLIAURURLsReplacesList(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("test")
	cfg.AURURLs = []AURSource{{URL: "https://configured.example"}}

	parseArgs(t, cfg, "aururls", "https://a.example,https://b.example", "aururls", "https://c.example")
	assert.Equal(t, []string{"https://a.example", "https://b.example", "https://c.example"}, cfg.AURBaseURLs())
}

func TestCLIAURURLOverridesConfiguredList(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("test")
	cfg.AURURLs = []AURSource{{URL: "https://configured.example"}}

	parseArgs(t, cfg, "aururl", "https://single.example/")
	assert.Equal(t, []string{"https://single.example"}, cfg.AURBaseURLs())
	assert.Equal(t, "https://single.example/rpc?", cfg.AURRPCURL)
}

func TestCLIAURURLsWinsOverAURURL(t *testing.T) {
	t.Parallel()

	for range 20 { // map iteration order must not matter
		cfg := DefaultConfig("test")
		parseArgs(t, cfg, "aururl", "https://single.example", "aururls", "https://a.example,https://b.example")
		assert.Equal(t, []string{"https://a.example", "https://b.example"}, cfg.AURBaseURLs())
	}
}

func TestAddAURSourceSeedsFromAURURL(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("test")
	require.NoError(t, cfg.AddAURSource("mirror", "https://aur.example.org/", ""))
	assert.Equal(t, []AURSource{
		{Name: "aur", URL: "https://aur.archlinux.org"},
		{Name: "mirror", URL: "https://aur.example.org"},
	}, cfg.AURURLs)

	require.Error(t, cfg.AddAURSource("mirror", "https://dup.example", ""), "duplicate name")
	require.Error(t, cfg.AddAURSource("bad name", "https://x.example", ""), "invalid name")
	require.Error(t, cfg.AddAURSource("x", "aur.example.org", ""), "scheme required")
	require.Error(t, cfg.AddAURSource("", "https://x.example", ""), "empty name")
}

func TestEditAURSource(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("test")
	cfg.AURURLs = []AURSource{
		{URL: "https://aur.archlinux.org"}, // unnamed: referenced as "aur"
		{Name: "mirror", URL: "https://aur.example.org"},
	}

	require.NoError(t, cfg.EditAURSource("mirror", map[string]string{
		"url": "https://new.example.org/", "rpcurl": "https://new.example.org/rpc", "name": "new",
	}))
	assert.Equal(t, AURSource{Name: "new", URL: "https://new.example.org", RPCURL: "https://new.example.org/rpc"}, cfg.AURURLs[1])

	require.NoError(t, cfg.EditAURSource("aur", map[string]string{"rpcurl": "https://rpc.example"}))
	assert.Equal(t, AURSource{Name: "aur", URL: "https://aur.archlinux.org", RPCURL: "https://rpc.example"}, cfg.AURURLs[0])

	require.Error(t, cfg.EditAURSource("missing", map[string]string{"url": "https://x.example"}))
	require.Error(t, cfg.EditAURSource("new", map[string]string{"name": "aur"}), "rename onto existing name")
	require.Error(t, cfg.EditAURSource("new", map[string]string{"bogus": "1"}))
	require.Error(t, cfg.EditAURSource("new", map[string]string{"url": "ftp://x"}))
	require.Error(t, cfg.EditAURSource("new", nil))
}

func TestDeleteAndMoveAURSource(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("test")
	cfg.AURURLs = []AURSource{
		{Name: "a", URL: "https://a.example"},
		{Name: "b", URL: "https://b.example"},
		{Name: "c", URL: "https://c.example"},
	}

	require.NoError(t, cfg.MoveAURSource("c", 1))
	assert.Equal(t, []string{"https://c.example", "https://a.example", "https://b.example"}, cfg.AURBaseURLs())

	require.NoError(t, cfg.MoveAURSource("c", 3))
	assert.Equal(t, []string{"https://a.example", "https://b.example", "https://c.example"}, cfg.AURBaseURLs())

	require.Error(t, cfg.MoveAURSource("c", 0))
	require.Error(t, cfg.MoveAURSource("c", 4))
	require.Error(t, cfg.MoveAURSource("zzz", 1))

	require.NoError(t, cfg.DeleteAURSource("b"))
	assert.Equal(t, []string{"https://a.example", "https://c.example"}, cfg.AURBaseURLs())
	require.Error(t, cfg.DeleteAURSource("b"))

	require.NoError(t, cfg.DeleteAURSource("a"))
	require.NoError(t, cfg.DeleteAURSource("c"))
	assert.Empty(t, cfg.AURURLs)
	assert.Equal(t, []string{"https://aur.archlinux.org"}, cfg.AURBaseURLs(), "empty list falls back to aururl")
}

func TestUpdateConfigFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "yay", "config.json")

	// Missing file: starts from defaults.
	_, err := UpdateConfigFile(path, "v1", func(c *Configuration) error {
		return c.AddAURSource("mirror", "https://aur.example.org", "")
	})
	require.NoError(t, err)

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var onDisk map[string]any
	require.NoError(t, json.Unmarshal(raw, &onDisk))
	assert.Equal(t, []any{
		map[string]any{"name": "aur", "url": "https://aur.archlinux.org"},
		map[string]any{"name": "mirror", "url": "https://aur.example.org"},
	}, onDisk["aururls"])

	// Existing settings survive and a failing edit does not rewrite the file.
	require.NoError(t, os.WriteFile(path, []byte(`{"editor":"nvim","aururls":["https://aur.archlinux.org"]}`), 0o600))

	_, err = UpdateConfigFile(path, "v1", func(c *Configuration) error { return c.DeleteAURSource("nope") })
	require.Error(t, err)

	raw, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.JSONEq(t, `{"editor":"nvim","aururls":["https://aur.archlinux.org"]}`, string(raw))

	cfg, err := UpdateConfigFile(path, "v1", func(c *Configuration) error {
		return c.AddAURSource("mirror", "https://aur.example.org", "https://aur.example.org/rpc")
	})
	require.NoError(t, err)
	assert.Equal(t, "nvim", cfg.Editor)
	assert.Equal(t, []string{"https://aur.archlinux.org", "https://aur.example.org"}, cfg.AURBaseURLs())

	// Corrupt file is reported, not overwritten.
	require.NoError(t, os.WriteFile(path, []byte(`{not json`), 0o600))
	_, err = UpdateConfigFile(path, "v1", func(c *Configuration) error { return nil })
	require.Error(t, err)
}

func TestAddAURSourceRejectsReservedNames(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("test")
	for _, name := range []string{"core", "extra", "multilib", "local", "devel", "a/b"} {
		assert.Error(t, cfg.AddAURSource(name, "https://x.example", ""), name)
	}

	require.NoError(t, cfg.AddAURSource("mirror", "https://x.example", ""))
	assert.Error(t, cfg.EditAURSource("mirror", map[string]string{"name": "extra"}))
}
