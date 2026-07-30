//go:build !integration

package dep

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aurc "github.com/Jguer/aur"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/db/mock"
	mockaur "github.com/Jguer/yay/v13/pkg/dep/mock"
	"github.com/Jguer/yay/v13/pkg/pkgbuildrepo"
	aur "github.com/Jguer/yay/v13/pkg/query"
	"github.com/Jguer/yay/v13/pkg/text"
)

func newFooRepoIndex(t *testing.T) (idx *pkgbuildrepo.Index, pkgDir string) {
	t.Helper()

	pkgDir = filepath.Join(t.TempDir(), "foo")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".SRCINFO"),
		[]byte("pkgbase = foo\n\tpkgver = 1\n\tpkgrel = 1\n\tarch = x86_64\n\npkgname = foo\n"), 0o600))

	idx = pkgbuildrepo.NewIndex()
	require.NoError(t, idx.AddRepo("myrepo", []string{pkgDir}))

	return idx, pkgDir
}

func writeRepoSrcinfo(t *testing.T, dir, contents string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(contents), 0o600))
}

// GIVEN a target present in a PKGBUILD repo
// WHEN it is graphed
// THEN it resolves to the PkgbuildRepo source pointing at the repo dir, and the
// AUR is never queried for it.
func TestGrapher_GraphFromTargets_pkgbuildRepoMasksAUR(t *testing.T) {
	t.Parallel()

	idx, pkgDir := newFooRepoIndex(t)

	mockDB := &mock.DBExecutor{
		AlpmArchitecturesFn: func() ([]string, error) { return []string{"x86_64"}, nil },
		SyncSatisfierFn:     func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return nil },
		LocalPackageFn:      func(string) mock.IPackage { return nil },
	}
	mockAUR := &mockaur.MockAUR{GetFn: func(_ context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		t.Errorf("AUR must not be queried for a pkgbuild-repo package, got %+v", query)
		return nil, nil
	}}

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")
	g := NewGrapher(mockDB, mockAUR, false, true, true, true, false, logger)
	g.SetPkgbuildRepos(idx)

	graph, err := g.GraphFromTargets(t.Context(), nil, []string{"foo"})
	require.NoError(t, err)

	info := graph.GetNodeInfo("foo")
	require.NotNil(t, info)
	require.NotNil(t, info.Value)
	assert.Equal(t, PkgbuildRepo, info.Value.Source)
	assert.Equal(t, pkgDir, info.Value.SrcinfoPath)
	assert.Equal(t, "foo", info.Value.AURBase)
}

// GIVEN a dependency satisfiable by a PKGBUILD repo
// WHEN a package depending on it is graphed
// THEN the dependency resolves to the PkgbuildRepo source, not the AUR.
func TestGrapher_addNodes_pkgbuildRepoDep(t *testing.T) {
	t.Parallel()

	idx, pkgDir := newFooRepoIndex(t)

	mockDB := &mock.DBExecutor{
		AlpmArchitecturesFn:    func() ([]string, error) { return []string{"x86_64"}, nil },
		SyncSatisfierFn:        func(string) mock.IPackage { return nil },
		LocalSatisfierExistsFn: func(string) bool { return false },
		LocalPackageFn:         func(string) mock.IPackage { return nil },
	}
	mockAUR := &mockaur.MockAUR{GetFn: func(_ context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		t.Errorf("AUR must not be queried for a pkgbuild-repo dependency, got %+v", query)
		return nil, nil
	}}

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")
	g := NewGrapher(mockDB, mockAUR, false, true, false, false, false, logger)
	g.SetPkgbuildRepos(idx)

	graph := NewGraph()
	graph.AddNode("parent")
	g.addNodes(t.Context(), graph, "parent", []string{"foo>=1"}, Dep)

	info := graph.GetNodeInfo("foo")
	require.NotNil(t, info)
	require.NotNil(t, info.Value)
	assert.Equal(t, PkgbuildRepo, info.Value.Source)
	assert.Equal(t, pkgDir, info.Value.SrcinfoPath)
}

func TestGrapher_graphPkgbuildRepoDepSelectsConcreteSatisfier(t *testing.T) {
	t.Parallel()

	pkgDir := filepath.Join(t.TempDir(), "provider")
	writeRepoSrcinfo(t, pkgDir, "pkgbase = provider\n\tpkgver = 1\n\tpkgrel = 1\n\tarch = x86_64\n\npkgname = provider\n\tprovides = virtual=1\n")
	idx := pkgbuildrepo.NewIndex()
	require.NoError(t, idx.AddRepo("myrepo", []string{pkgDir}))

	dbExe := &mock.DBExecutor{
		AlpmArchitecturesFn: func() ([]string, error) { return []string{"x86_64"}, nil },
		LocalPackageFn:      func(string) mock.IPackage { return nil },
	}
	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")
	g := NewGrapher(dbExe, &mockaur.MockAUR{}, false, true, false, false, false, logger)
	entry, ok := idx.Get("virtual")
	require.True(t, ok)

	graph := NewGraph()
	pkgName, err := g.graphPkgbuildRepoDep(t.Context(), graph, entry, "virtual>=1", Dep)
	require.NoError(t, err)
	assert.Equal(t, "provider", pkgName)
	assert.NotNil(t, graph.GetNodeInfo("provider").Value)
	assert.Nil(t, graph.GetNodeInfo("virtual"))

	pkgName, err = g.graphPkgbuildRepoDep(t.Context(), graph, entry, "virtual>=2", Dep)
	require.NoError(t, err)
	assert.Empty(t, pkgName)
}

// GIVEN an installed package a repo has a newer version of
// WHEN graphed as an upgrade
// THEN the node is marked as an upgrade with the local and remote versions.
func TestGrapher_GraphPkgbuildRepoUpgrade(t *testing.T) {
	t.Parallel()

	idx, pkgDir := newFooRepoIndex(t)
	entry, ok := idx.Get("foo")
	require.True(t, ok)

	mockDB := &mock.DBExecutor{
		AlpmArchitecturesFn:    func() ([]string, error) { return []string{"x86_64"}, nil },
		LocalSatisfierExistsFn: func(string) bool { return false },
	}
	mockAUR := &mockaur.MockAUR{GetFn: func(_ context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		t.Errorf("AUR must not be queried, got %+v", query)
		return nil, nil
	}}

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")
	g := NewGrapher(mockDB, mockAUR, false, true, false, false, false, logger)
	g.SetPkgbuildRepos(idx)

	graph, err := g.GraphPkgbuildRepoUpgrade(t.Context(), NewGraph(), entry, "foo", "0.9-1", Dep)
	require.NoError(t, err)

	info := graph.GetNodeInfo("foo")
	require.NotNil(t, info)
	require.NotNil(t, info.Value)
	assert.Equal(t, PkgbuildRepo, info.Value.Source)
	assert.True(t, info.Value.Upgrade)
	assert.Equal(t, "0.9-1", info.Value.LocalVersion)
	assert.Equal(t, "1-1", info.Value.Version)
	assert.Equal(t, pkgDir, info.Value.SrcinfoPath)
	assert.Equal(t, "myrepo", info.Value.RepoName)
}
