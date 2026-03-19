//go:build integration
// +build integration

package download

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/text"
)

// WHEN some AUR bases exist and others are fresh
// THEN AURPKGBUILDRepos returns mixed newClone flags
func TestIntegrationAURPKGBUILDReposMixedCloneAndPull(t *testing.T) {
	dir := t.TempDir()
	testLogger := text.NewLogger(os.Stdout, os.Stderr, strings.NewReader(""), true, "test")
	cmdRunner := &exe.OSRunner{Log: testLogger}
	cmdBuilder := &exe.CmdBuilder{
		Runner:   cmdRunner,
		GitBin:   "git",
		GitFlags: []string{},
		Log:      testLogger,
	}
	targets := []string{"yay-bin", "yay-git"}

	cloned1, err1 := AURPKGBUILDRepos(context.Background(), cmdBuilder, testLogger.Child("dl"),
		targets, "https://aur.archlinux.org", dir, false)
	require.NoError(t, err1)
	assert.EqualValues(t, map[string]bool{"yay-bin": true, "yay-git": true}, cloned1)

	cloned2, err2 := AURPKGBUILDRepos(context.Background(), cmdBuilder, testLogger.Child("dl2"),
		targets, "https://aur.archlinux.org", dir, false)
	require.NoError(t, err2)
	assert.EqualValues(t, map[string]bool{"yay-bin": false, "yay-git": false}, cloned2)
}

func TestIntegrationGetPackageScannerGzip(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, err := gw.Write([]byte("line1\nline2\n"))
	require.NoError(t, err)
	require.NoError(t, gw.Close())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/packages.gz" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf.Bytes())
	}))
	t.Cleanup(srv.Close)

	testLogger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	sc, err := GetPackageScanner(context.Background(), srv.Client(), srv.URL, testLogger)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, sc.Close()) })

	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	require.NoError(t, sc.Err())
	assert.Equal(t, []string{"line1", "line2"}, lines)
}

func TestIntegrationGetPackageScannerRawFallback(t *testing.T) {
	body := []byte("alpha\nbeta\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/packages.gz" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	testLogger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	sc, err := GetPackageScanner(context.Background(), srv.Client(), srv.URL, testLogger)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, sc.Close()) })

	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	require.NoError(t, sc.Err())
	assert.Equal(t, []string{"alpha", "beta"}, lines)
}

func TestIntegrationGetPackageScannerHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	testLogger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	sc, err := GetPackageScanner(context.Background(), srv.Client(), srv.URL, testLogger)
	assert.Error(t, err)
	assert.Nil(t, sc)
}
