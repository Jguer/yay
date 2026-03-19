//go:build integration
// +build integration

package download

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Jguer/aur"

	mockaur "github.com/Jguer/yay/v12/pkg/dep/mock"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
)

func integrationCmdBuilder(t *testing.T) (*text.Logger, *exe.CmdBuilder) {
	t.Helper()
	testLogger := text.NewLogger(os.Stdout, os.Stderr, strings.NewReader(""), true, "test")
	cmdRunner := &exe.OSRunner{Log: testLogger}
	cmdBuilder := &exe.CmdBuilder{
		Runner:   cmdRunner,
		GitBin:   "git",
		GitFlags: []string{},
		Log:      testLogger,
	}
	return testLogger, cmdBuilder
}

func TestIntegrationPKGBUILDReposDefinedDBClone(t *testing.T) {
	dir := t.TempDir()

	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{}}, nil // fakes a package found for all
		},
	}
	targets := []string{"core/linux", "yay-bin", "yay-git"}

	testLogger := text.NewLogger(os.Stdout, os.Stderr, strings.NewReader(""), true, "test")
	cmdRunner := &exe.OSRunner{Log: testLogger}
	cmdBuilder := &exe.CmdBuilder{
		Runner:   cmdRunner,
		GitBin:   "git",
		GitFlags: []string{},
		Log:      testLogger,
	}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"linux": "core"},
	}
	cloned, err := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder, testLogger.Child("test"),
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, false)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string]bool{"core/linux": true, "yay-bin": true, "yay-git": true}, cloned)
}

func TestIntegrationPKGBUILDReposNotExist(t *testing.T) {
	dir := t.TempDir()

	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{}}, nil // fakes a package found for all
		},
	}
	targets := []string{"core/yay", "yay-bin", "yay-git"}
	testLogger := text.NewLogger(os.Stdout, os.Stderr, strings.NewReader(""), true, "test")
	cmdRunner := &exe.OSRunner{Log: testLogger}
	cmdBuilder := &exe.CmdBuilder{
		Runner:   cmdRunner,
		GitBin:   "git",
		GitFlags: []string{},
		Log:      testLogger,
	}

	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"yay": "core"},
	}
	cloned, err := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder, testLogger.Child("test"),
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, false)

	assert.Error(t, err)
	assert.EqualValues(t, map[string]bool{"yay-bin": true, "yay-git": true}, cloned)
}

// GIVEN 2 aur packages and 1 in repo
// WHEN defining as specified targets
// THEN all aur be found and cloned
func TestIntegrationPKGBUILDFull(t *testing.T) {
	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{}}, nil
		},
	}

	testLogger := text.NewLogger(os.Stdout, os.Stderr, strings.NewReader(""), true, "test")
	targets := []string{"core/linux", "aur/yay-bin", "yay-git"}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"linux": "core"},
	}

	fetched, err := PKGBUILDs(searcher, mockClient, &http.Client{}, testLogger.Child("test"),
		targets, "https://aur.archlinux.org", parser.ModeAny)

	assert.NoError(t, err)

	for _, target := range targets {
		assert.Contains(t, fetched, target)
		assert.NotEmpty(t, fetched[target])
	}
}

// WHEN checkouts already exist and force is false
// THEN git pull runs and newClone is false for each target
func TestIntegrationPKGBUILDReposPullWhenExists(t *testing.T) {
	dir := t.TempDir()
	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{}}, nil
		},
	}
	targets := []string{"core/linux", "yay-bin"}
	testLogger, cmdBuilder := integrationCmdBuilder(t)
	searcher := &testDBSearcher{absPackagesDB: map[string]string{"linux": "core"}}

	cloned1, err1 := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder, testLogger.Child("test"),
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, false)
	assert.NoError(t, err1)
	assert.EqualValues(t, map[string]bool{"core/linux": true, "yay-bin": true}, cloned1)

	cloned2, err2 := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder, testLogger.Child("test2"),
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, false)
	assert.NoError(t, err2)
	assert.EqualValues(t, map[string]bool{"core/linux": false, "yay-bin": false}, cloned2)
}

// WHEN checkouts already exist and force is true
// THEN directories are removed and fresh clones return newClone true
func TestIntegrationPKGBUILDReposForceRecloneAfterClone(t *testing.T) {
	dir := t.TempDir()
	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{}}, nil
		},
	}
	targets := []string{"core/linux", "yay-bin"}
	testLogger, cmdBuilder := integrationCmdBuilder(t)
	searcher := &testDBSearcher{absPackagesDB: map[string]string{"linux": "core"}}

	cloned1, err1 := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder, testLogger.Child("test"),
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, false)
	assert.NoError(t, err1)
	assert.EqualValues(t, map[string]bool{"core/linux": true, "yay-bin": true}, cloned1)

	cloned2, err2 := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder, testLogger.Child("test2"),
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, true)
	assert.NoError(t, err2)
	assert.EqualValues(t, map[string]bool{"core/linux": true, "yay-bin": true}, cloned2)
}

// WHEN mode is repo-only
// THEN only sync db targets are cloned; bare AUR names are skipped
func TestIntegrationPKGBUILDReposModeRepoOnlyRepo(t *testing.T) {
	dir := t.TempDir()
	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{}, nil
		},
	}
	targets := []string{"linux", "yay-bin", "yay-git"}
	testLogger, cmdBuilder := integrationCmdBuilder(t)
	searcher := &testDBSearcher{absPackagesDB: map[string]string{"linux": "core"}}

	cloned, err := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder, testLogger.Child("test"),
		targets, parser.ModeRepo, "https://aur.archlinux.org", dir, false)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string]bool{"linux": true}, cloned)
}

// WHEN a repo/db-qualified target is wrong for the package
// THEN that target is skipped and AUR targets still clone
func TestIntegrationPKGBUILDReposWrongDBSkipsTarget(t *testing.T) {
	dir := t.TempDir()
	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{}}, nil
		},
	}
	targets := []string{"extra/yay", "yay-bin"}
	testLogger, cmdBuilder := integrationCmdBuilder(t)
	searcher := &testDBSearcher{absPackagesDB: map[string]string{"yay": "core"}}

	cloned, err := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder, testLogger.Child("test"),
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, false)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string]bool{"yay-bin": true}, cloned)
}

// WHEN mode is repo-only
// THEN PKGBUILD bytes are only fetched for repo targets
func TestIntegrationPKGBUILDsModeRepoOnlyRepo(t *testing.T) {
	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{}, nil
		},
	}
	targets := []string{"core/linux", "yay-bin"}
	testLogger, _ := integrationCmdBuilder(t)
	searcher := &testDBSearcher{absPackagesDB: map[string]string{"linux": "core"}}

	fetched, err := PKGBUILDs(searcher, mockClient, &http.Client{}, testLogger.Child("test"),
		targets, "https://aur.archlinux.org", parser.ModeRepo)

	assert.NoError(t, err)
	assert.Len(t, fetched, 1)
	assert.Contains(t, fetched, "core/linux")
	assert.NotEmpty(t, fetched["core/linux"])
}

// WHEN AUR RPC fails for an AUR target
// THEN repo PKGBUILD still downloads and error aggregates only failed fetches
func TestIntegrationPKGBUILDsAURRPCErrorStillFetchesRepo(t *testing.T) {
	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return nil, errors.New("aur rpc unavailable")
		},
	}
	targets := []string{"core/linux", "yay-git"}
	testLogger, _ := integrationCmdBuilder(t)
	searcher := &testDBSearcher{absPackagesDB: map[string]string{"linux": "core"}}

	fetched, err := PKGBUILDs(searcher, mockClient, &http.Client{}, testLogger.Child("test"),
		targets, "https://aur.archlinux.org", parser.ModeAny)

	assert.NoError(t, err)
	assert.Contains(t, fetched, "core/linux")
	assert.NotEmpty(t, fetched["core/linux"])
	assert.NotContains(t, fetched, "yay-git")
}

// WHEN AUR returns no packages for an AUR needle
// THEN that target is skipped and repo fetch still succeeds
func TestIntegrationPKGBUILDsAUREmptySkipsAUR(t *testing.T) {
	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{}, nil
		},
	}
	targets := []string{"core/linux", "yay-git"}
	testLogger, _ := integrationCmdBuilder(t)
	searcher := &testDBSearcher{absPackagesDB: map[string]string{"linux": "core"}}

	fetched, err := PKGBUILDs(searcher, mockClient, &http.Client{}, testLogger.Child("test"),
		targets, "https://aur.archlinux.org", parser.ModeAny)

	assert.NoError(t, err)
	assert.Contains(t, fetched, "core/linux")
	assert.NotEmpty(t, fetched["core/linux"])
	assert.NotContains(t, fetched, "yay-git")
}
