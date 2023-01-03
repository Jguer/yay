package download

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"

	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
)

// GIVEN 2 aur packages and 1 in repo
// GIVEN package in repo is already present
// WHEN defining package db as a target
// THEN all should be found and cloned, except the repo one
func TestPKGBUILDReposDefinedDBPull(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "yay", ".git"), 0o777)

	targets := []string{"core/yay", "yay-bin", "yay-git"}
	cmdRunner := &testRunner{}
	cmdBuilder := &testGitBuilder{
		index: 0,
		test:  t,
		parentBuilder: &exe.CmdBuilder{
			Runner:   cmdRunner,
			GitBin:   "/usr/local/bin/git",
			GitFlags: []string{},
		},
	}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"yay": "core"},
	}
	cloned, err := PKGBUILDRepos(context.Background(), searcher,
		cmdBuilder,
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, false)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string]bool{"core/yay": false, "yay-bin": true, "yay-git": true}, cloned)
}

// GIVEN 2 aur packages and 1 in repo
// WHEN defining package db as a target
// THEN all should be found and cloned
func TestPKGBUILDReposDefinedDBClone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	targets := []string{"core/yay", "yay-bin", "yay-git"}
	cmdRunner := &testRunner{}
	cmdBuilder := &testGitBuilder{
		index: 0,
		test:  t,
		parentBuilder: &exe.CmdBuilder{
			Runner:   cmdRunner,
			GitBin:   "/usr/local/bin/git",
			GitFlags: []string{},
		},
	}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"yay": "core"},
	}
	cloned, err := PKGBUILDRepos(context.Background(), searcher,
		cmdBuilder,
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, false)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string]bool{"core/yay": true, "yay-bin": true, "yay-git": true}, cloned)
}

// GIVEN 2 aur packages and 1 in repo
// WHEN defining as non specified targets
// THEN all should be found and cloned
func TestPKGBUILDReposClone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	targets := []string{"yay", "yay-bin", "yay-git"}
	cmdRunner := &testRunner{}
	cmdBuilder := &testGitBuilder{
		index: 0,
		test:  t,
		parentBuilder: &exe.CmdBuilder{
			Runner:   cmdRunner,
			GitBin:   "/usr/local/bin/git",
			GitFlags: []string{},
		},
	}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"yay": "core"},
	}
	cloned, err := PKGBUILDRepos(context.Background(), searcher,
		cmdBuilder,
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, false)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string]bool{"yay": true, "yay-bin": true, "yay-git": true}, cloned)
}

// GIVEN 2 aur packages and 1 in repo but wrong db
// WHEN defining as non specified targets
// THEN all aur be found and cloned
func TestPKGBUILDReposNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	targets := []string{"extra/yay", "yay-bin", "yay-git"}
	cmdRunner := &testRunner{}
	cmdBuilder := &testGitBuilder{
		index: 0,
		test:  t,
		parentBuilder: &exe.CmdBuilder{
			Runner:   cmdRunner,
			GitBin:   "/usr/local/bin/git",
			GitFlags: []string{},
		},
	}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"yay": "core"},
	}
	cloned, err := PKGBUILDRepos(context.Background(), searcher,
		cmdBuilder,
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, false)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string]bool{"yay-bin": true, "yay-git": true}, cloned)
}

// GIVEN 2 aur packages and 1 in repo
// WHEN defining as non specified targets in repo mode
// THEN only repo should be cloned
func TestPKGBUILDReposRepoMode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	targets := []string{"yay", "yay-bin", "yay-git"}
	cmdRunner := &testRunner{}
	cmdBuilder := &testGitBuilder{
		index: 0,
		test:  t,
		parentBuilder: &exe.CmdBuilder{
			Runner:   cmdRunner,
			GitBin:   "/usr/local/bin/git",
			GitFlags: []string{},
		},
	}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"yay": "core"},
	}
	cloned, err := PKGBUILDRepos(context.Background(), searcher,
		cmdBuilder,
		targets, parser.ModeRepo, "https://aur.archlinux.org", dir, false)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string]bool{"yay": true}, cloned)
}

// GIVEN 2 aur packages and 1 in repo
// WHEN defining as specified targets
// THEN all aur be found and cloned
func TestPKGBUILDFull(t *testing.T) {
	t.Parallel()

	gock.New("https://aur.archlinux.org").
		Get("/cgit/aur.git/plain/PKGBUILD").MatchParam("h", "yay-git").
		Reply(200).
		BodyString("example_yay-git")
	gock.New("https://aur.archlinux.org").
		Get("/cgit/aur.git/plain/PKGBUILD").MatchParam("h", "yay-bin").
		Reply(200).
		BodyString("example_yay-bin")

	gock.New("https://github.com/").
		Get("/archlinux/svntogit-packages/raw/packages/yay/trunk/PKGBUILD").
		Reply(200).
		BodyString("example_yay")

	defer gock.Off()
	targets := []string{"core/yay", "aur/yay-bin", "yay-git"}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"yay": "core"},
	}

	fetched, err := PKGBUILDs(searcher, &http.Client{},
		targets, "https://aur.archlinux.org", parser.ModeAny)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string][]byte{
		"core/yay":    []byte("example_yay"),
		"aur/yay-bin": []byte("example_yay-bin"),
		"yay-git":     []byte("example_yay-git"),
	}, fetched)
}
