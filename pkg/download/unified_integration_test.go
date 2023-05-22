//go:build integration
// +build integration

package download

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Jguer/aur"

	mockaur "github.com/Jguer/yay/v12/pkg/dep/mock"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
)

func TestIntegrationPKGBUILDReposDefinedDBClone(t *testing.T) {
	dir := t.TempDir()

	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{}}, nil // fakes a package found for all
		},
	}
	targets := []string{"core/linux", "yay-bin", "yay-git"}
	cmdRunner := &exe.OSRunner{Log: text.GlobalLogger}
	cmdBuilder := &exe.CmdBuilder{
		Runner:   cmdRunner,
		GitBin:   "git",
		GitFlags: []string{},
	}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"linux": "core"},
	}
	cloned, err := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder,
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
	cmdRunner := &exe.OSRunner{Log: text.GlobalLogger}
	cmdBuilder := &exe.CmdBuilder{
		Runner:   cmdRunner,
		GitBin:   "git",
		GitFlags: []string{},
	}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"yay": "core"},
	}
	cloned, err := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder,
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

	targets := []string{"core/linux", "aur/yay-bin", "yay-git"}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"linux": "core"},
	}

	fetched, err := PKGBUILDs(searcher, mockClient, &http.Client{},
		targets, "https://aur.archlinux.org", parser.ModeAny)

	assert.NoError(t, err)

	for _, target := range targets {
		assert.Contains(t, fetched, target)
		assert.NotEmpty(t, fetched[target])
	}
}
