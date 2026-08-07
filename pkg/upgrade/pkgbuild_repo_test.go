//go:build !integration

package upgrade

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aur "github.com/Jguer/aur"
	alpm "github.com/Jguer/dyalpm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/db"
	"github.com/Jguer/yay/v13/pkg/db/mock"
	"github.com/Jguer/yay/v13/pkg/dep"
	mockaur "github.com/Jguer/yay/v13/pkg/dep/mock"
	"github.com/Jguer/yay/v13/pkg/pkgbuildrepo"
	"github.com/Jguer/yay/v13/pkg/query"
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/settings/parser"
	"github.com/Jguer/yay/v13/pkg/text"
	"github.com/Jguer/yay/v13/pkg/vcs"
)

// GIVEN an installed package owned by a PKGBUILD repo with a newer version
// WHEN upgrades are graphed
// THEN it is queued as a PkgbuildRepo upgrade and the AUR is never asked about it.
func TestUpgradeService_pkgbuildRepoUpgradeMasksAUR(t *testing.T) {
	t.Parallel()

	pkgDir := filepath.Join(t.TempDir(), "foo")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".SRCINFO"),
		[]byte("pkgbase = foo\n\tpkgver = 2\n\tpkgrel = 1\n\tarch = x86_64\n\npkgname = foo\n"), 0o600))

	idx := pkgbuildrepo.NewIndex()
	require.NoError(t, idx.AddRepo("myrepo", []string{pkgDir}))

	dbExe := &mock.DBExecutor{
		AlpmArchitecturesFn:           func() ([]string, error) { return []string{"x86_64"}, nil },
		InstalledRemotePackageNamesFn: func() []string { return []string{"foo"} },
		InstalledRemotePackagesFn: func() map[string]mock.IPackage {
			return map[string]mock.IPackage{
				"foo": &mock.Package{PName: "foo", PBase: "foo", PVersion: "1-1", PReason: alpm.PkgReasonExplicit},
			}
		},
		LocalSatisfierExistsFn: func(string) bool { return false },
		SyncUpgradesFn:         func(bool) (map[string]db.SyncUpgrade, error) { return map[string]db.SyncUpgrade{}, nil },
		ReposFn:                func() []string { return nil },
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(_ context.Context, q *aur.Query) ([]aur.Pkg, error) {
		for _, needle := range q.Needles {
			if needle == "foo" {
				t.Errorf("AUR must not be queried for repo-owned package foo, got %+v", q)
			}
		}
		return []aur.Pkg{}, nil
	}}

	logger := text.NewLogger(io.Discard, os.Stderr, strings.NewReader(""), false, "test")
	grapher := dep.NewGrapher(dbExe, mockAUR, false, true, false, false, false, logger)
	grapher.SetPkgbuildRepos(idx)

	u := &UpgradeService{
		log:         logger,
		grapher:     grapher,
		aurCache:    mockAUR,
		dbExecutor:  dbExe,
		vcsStore:    &vcs.Mock{},
		cfg:         &settings.Configuration{Mode: parser.ModeAny},
		AURWarnings: query.NewWarnings(logger),
	}

	graph, err := u.GraphUpgrades(t.Context(), nil, false, func(*Upgrade) bool { return true })
	require.NoError(t, err)

	info := graph.GetNodeInfo("foo")
	require.NotNil(t, info)
	require.NotNil(t, info.Value)
	assert.Equal(t, dep.PkgbuildRepo, info.Value.Source)
	assert.True(t, info.Value.Upgrade)
	assert.Equal(t, "1-1", info.Value.LocalVersion)
	assert.Equal(t, "2-1", info.Value.Version)

	// Repo-only mode must not build packages from PKGBUILD repositories.
	u.cfg.Mode = parser.ModeRepo
	graph, err = u.GraphUpgrades(t.Context(), nil, false, func(*Upgrade) bool { return true })
	require.NoError(t, err)
	assert.Nil(t, graph.GetNodeInfo("foo"))
}

func TestUpgradeService_pkgbuildRepoDowngrade(t *testing.T) {
	t.Parallel()

	pkgDir := filepath.Join(t.TempDir(), "foo")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".SRCINFO"),
		[]byte("pkgbase = foo\n\tpkgver = 2\n\tpkgrel = 1\n\tarch = x86_64\n\npkgname = foo\n"), 0o600))
	idx := pkgbuildrepo.NewIndex()
	require.NoError(t, idx.AddRepo("myrepo", []string{pkgDir}))

	logger := text.NewLogger(io.Discard, os.Stderr, strings.NewReader(""), false, "test")
	grapher := dep.NewGrapher(&mock.DBExecutor{
		AlpmArchitecturesFn:    func() ([]string, error) { return []string{"x86_64"}, nil },
		LocalSatisfierExistsFn: func(string) bool { return false },
	}, &mockaur.MockAUR{}, false, true, false, false, false, logger)
	grapher.SetPkgbuildRepos(idx)
	u := &UpgradeService{grapher: grapher}

	remote := map[string]db.IPackage{
		"foo": &mock.Package{PName: "foo", PBase: "foo", PVersion: "3-1", PReason: alpm.PkgReasonExplicit},
	}

	var errs []error
	graph := dep.NewGraph()
	u.graphPkgbuildRepoUpgrades(t.Context(), graph, remote, false, nil, &errs)
	assert.Empty(t, errs)
	assert.Nil(t, graph.GetNodeInfo("foo"))

	graph = dep.NewGraph()
	u.graphPkgbuildRepoUpgrades(t.Context(), graph, remote, true, nil, &errs)
	info := graph.GetNodeInfo("foo")
	require.NotNil(t, info)
	require.NotNil(t, info.Value)
	assert.True(t, info.Value.Upgrade)
	assert.Equal(t, "3-1", info.Value.LocalVersion)
	assert.Equal(t, "2-1", info.Value.Version)

	equalRemote := map[string]db.IPackage{
		"foo": &mock.Package{PName: "foo", PBase: "foo", PVersion: "2-1", PReason: alpm.PkgReasonExplicit},
	}
	graph = dep.NewGraph()
	u.graphPkgbuildRepoUpgrades(t.Context(), graph, equalRemote, true, nil, &errs)
	assert.Nil(t, graph.GetNodeInfo("foo"))
}
