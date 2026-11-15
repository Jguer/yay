//go:build !integration

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Jguer/aur"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/settings/parser"
	"github.com/Jguer/yay/v13/pkg/text"
)

func runAURSource(t *testing.T, configPath string, action string, targets ...string) (string, error) {
	t.Helper()

	cfg := settings.DefaultConfig("test")
	_ = loadJSON(configPath, cfg)

	cmdArgs := parser.MakeArguments()
	cmdArgs.Op = "Y"
	cmdArgs.CreateOrAppendOption("aursource", action)
	cmdArgs.AddTarget(targets...)

	var out bytes.Buffer
	logger := text.NewLogger(&out, io.Discard, strings.NewReader(""), false, "test")

	err := handleAURSource(logger, cfg, cmdArgs, configPath)

	return out.String(), err
}

func loadJSON(path string, cfg *settings.Configuration) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, cfg)
}

func TestHandleAURSourceLifecycle(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.json")

	out, err := runAURSource(t, configPath, "list")
	require.NoError(t, err)
	assert.Contains(t, out, "https://aur.archlinux.org")

	out, err = runAURSource(t, configPath, "add", "mirror", "https://aur.example.org")
	require.NoError(t, err)
	assert.Contains(t, out, "mirror")

	_, err = runAURSource(t, configPath, "add", "other", "https://other.example", "https://other.example/api/rpc")
	require.NoError(t, err)

	_, err = runAURSource(t, configPath, "move", "other", "1")
	require.NoError(t, err)

	_, err = runAURSource(t, configPath, "edit", "mirror", "url=https://aur2.example.org", "name=mirror2")
	require.NoError(t, err)

	_, err = runAURSource(t, configPath, "delete", "aur")
	require.NoError(t, err)

	saved := settings.DefaultConfig("test")
	require.NoError(t, loadJSON(configPath, saved))

	assert.Equal(t, []settings.AURSource{
		{Name: "other", URL: "https://other.example", RPCURL: "https://other.example/api/rpc"},
		{Name: "mirror2", URL: "https://aur2.example.org"},
	}, saved.AURURLs)

	out, err = runAURSource(t, configPath, "list")
	require.NoError(t, err)
	assert.Contains(t, out, "1 ")
	assert.Contains(t, out, "(rpc: https://other.example/api/rpc)")
}

func TestHandleAURSourceErrors(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.json")

	for _, tc := range []struct {
		action  string
		targets []string
	}{
		{"bogus", nil},
		{"add", []string{"only-name"}},
		{"edit", []string{"aur"}},
		{"edit", []string{"aur", "no-equals"}},
		{"delete", nil},
		{"delete", []string{"missing"}},
		{"move", []string{"aur", "x"}},
	} {
		_, err := runAURSource(t, configPath, tc.action, tc.targets...)
		assert.Error(t, err, "%s %v", tc.action, tc.targets)
	}

	_, statErr := os.Stat(configPath)
	assert.True(t, os.IsNotExist(statErr), "failed commands must not write the config")
}

func TestPrintInfoUsesSourceLabel(t *testing.T) {
	t.Parallel()

	pkg := &aur.Pkg{Name: "foo", URLPath: "/cgit/aur.git/snapshot/foo.tar.gz"}

	for _, tc := range []struct {
		source   settings.AURSource
		repo     string
		pkgURL   string
		snapshot string
	}{
		{settings.AURSource{Name: "aur", URL: "https://aur.archlinux.org"}, "aur",
			"https://aur.archlinux.org/packages/foo", "https://aur.archlinux.org/cgit/aur.git/snapshot/foo.tar.gz"},
		{settings.AURSource{Name: "mirror", URL: "https://aur.example.org"}, "mirror",
			"https://aur.example.org/packages/foo", "https://aur.example.org/cgit/aur.git/snapshot/foo.tar.gz"},
		{settings.AURSource{URL: "https://aur.example.org"}, "aur", "https://aur.example.org/packages/foo", ""},
	} {
		var out bytes.Buffer
		logger := text.NewLogger(&out, io.Discard, strings.NewReader(""), false, "test")
		printInfo(logger, tc.source, pkg, true)

		got := out.String()
		assert.Regexp(t, `Repository\s+: (\x1b\[0m)?`+tc.repo+`\n`, got)
		assert.Contains(t, got, tc.pkgURL)

		if tc.snapshot != "" {
			assert.Contains(t, got, tc.snapshot)
		}
	}
}
