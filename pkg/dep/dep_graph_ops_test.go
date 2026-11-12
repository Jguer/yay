//go:build !integration

package dep

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/Jguer/aur"
	"github.com/Jguer/dyalpm"
	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/db/mock"
	mockaur "github.com/Jguer/yay/v13/pkg/dep/mock"
	"github.com/Jguer/yay/v13/pkg/query"
	"github.com/Jguer/yay/v13/pkg/settings/parser"
	"github.com/Jguer/yay/v13/pkg/text"
)

func TestGrapher_RebuildTreeIncludesInstalledAURDependencies(t *testing.T) {
	t.Parallel()

	for _, mode := range []parser.RebuildMode{parser.RebuildModeNo, parser.RebuildModeAll, parser.RebuildModeTree} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			installed := map[string]mock.IPackage{
				"lib":    &mock.Package{PName: "lib", PVersion: "1", PReason: dyalpm.PkgReasonExplicit},
				"nested": &mock.Package{PName: "nested", PVersion: "1", PReason: dyalpm.PkgReasonDepend},
				"provider-git": &mock.Package{
					PName: "provider-git", PVersion: "1", PReason: dyalpm.PkgReasonDepend,
					PProvides: mock.DependList{Depends: []dyalpm.Depend{{Name: "virtual", Version: "1", Mod: dyalpm.DepModEQ}}},
				},
				"local-only": &mock.Package{PName: "local-only", PVersion: "1"},
			}
			packages := map[string]aur.Pkg{
				"app":          {Name: "app", PackageBase: "app", Version: "1", Depends: []string{"lib>=1", "virtual>=1", "system", "local-only"}},
				"lib":          {Name: "lib", PackageBase: "lib", Version: "1", Depends: []string{"nested"}},
				"nested":       {Name: "nested", PackageBase: "nested", Version: "1"},
				"provider-git": {Name: "provider-git", PackageBase: "provider-git", Version: "1", Provides: []string{"virtual=1"}},
			}
			dbExecutor := &mock.DBExecutor{
				InstalledRemotePackagesFn: func() map[string]mock.IPackage { return installed },
				LocalPackageFn:            func(name string) mock.IPackage { return installed[name] },
				LocalSatisfierExistsFn:    func(string) bool { return true },
				SyncSatisfierFn: func(name string) mock.IPackage {
					if name == "virtual>=1" || name == "provider-git" {
						return &mock.Package{PName: "repo-provider", PVersion: "1", PDB: mock.NewDB("extra")}
					}
					return nil
				},
			}
			queries := 0
			client := &mockaur.MockAUR{GetFn: func(_ context.Context, q *aur.Query) ([]aur.Pkg, error) {
				queries++
				require.Equal(t, aur.Name, q.By)
				pkgs := make([]aur.Pkg, 0, len(q.Needles))
				for _, name := range q.Needles {
					if pkg, ok := packages[name]; ok {
						pkgs = append(pkgs, pkg)
					}
				}
				return pkgs, nil
			}}
			logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")
			grapher := NewGrapher(dbExecutor, client, false, true, false, false, false, logger)
			require.NoError(t, grapher.SetRebuildMode(t.Context(), mode))
			graph, err := grapher.GraphFromAUR(t.Context(), nil, []string{"app"})
			require.NoError(t, err)

			require.False(t, graph.Exists("system"))
			require.False(t, graph.Exists("local-only"))
			require.False(t, graph.Exists("repo-provider"))
			if mode != parser.RebuildModeTree {
				require.Equal(t, 1, graph.Len())
				require.Equal(t, 1, queries)
				return
			}

			require.Equal(t, 4, graph.Len())
			require.Equal(t, 2, queries)
			for _, name := range []string{"lib", "nested", "provider-git"} {
				require.True(t, graph.Exists(name), name)
				require.Equal(t, AUR, graph.GetNodeInfo(name).Value.Source)
				require.Equal(t, "1", graph.GetNodeInfo(name).Value.LocalVersion)
			}
			require.Equal(t, Explicit, graph.GetNodeInfo("lib").Value.Reason)
			require.Equal(t, Dep, graph.GetNodeInfo("nested").Value.Reason)
		})
	}
}

func TestGrapher_RebuildTreeReturnsLookupError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("AUR lookup failed")
	grapher := NewGrapher(&mock.DBExecutor{
		InstalledRemotePackagesFn: func() map[string]mock.IPackage {
			return map[string]mock.IPackage{"lib": &mock.Package{PName: "lib", PVersion: "1"}}
		},
	}, &mockaur.MockAUR{GetFn: func(context.Context, *aur.Query) ([]aur.Pkg, error) {
		return nil, wantErr
	}}, false, true, false, false, false, text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test"))

	require.ErrorIs(t, grapher.SetRebuildMode(t.Context(), parser.RebuildModeTree), wantErr)
}

func TestGrapher_RebuildTreePreservesDependencyConstraints(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name           string
		dep            string
		installed      *mock.Package
		remote         aur.Pkg
		localSatisfied bool
		repo           mock.IPackage
		wantSource     Source
		wantVersion    string
	}{
		{
			name: "package version", dep: "lib>=2",
			installed:      &mock.Package{PName: "lib", PVersion: "2"},
			remote:         aur.Pkg{Name: "lib", PackageBase: "lib", Version: "1"},
			localSatisfied: true, wantSource: Missing, wantVersion: ">=2",
		},
		{
			name: "provided version", dep: "virtual>=2",
			installed: &mock.Package{
				PName: "provider-git", PVersion: "10",
				PProvides: mock.DependList{Depends: []dyalpm.Depend{{Name: "virtual", Version: "2", Mod: dyalpm.DepModEQ}}},
			},
			remote:         aur.Pkg{Name: "provider-git", PackageBase: "provider-git", Version: "11", Provides: []string{"virtual=1"}},
			localSatisfied: true, wantSource: Missing, wantVersion: ">=2",
		},
		{
			name: "unversioned installed provider does not override repository", dep: "virtual>=2",
			installed: &mock.Package{
				PName: "provider-git", PVersion: "10",
				PProvides: mock.DependList{Depends: []dyalpm.Depend{{Name: "virtual", Mod: dyalpm.DepModAny}}},
			},
			remote:     aur.Pkg{Name: "provider-git", PackageBase: "provider-git", Version: "10", Provides: []string{"virtual"}},
			repo:       &mock.Package{PName: "virtual", PVersion: "2", PDB: mock.NewDB("extra")},
			wantSource: Sync, wantVersion: "2",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			installed := map[string]mock.IPackage{tc.installed.Name(): tc.installed}
			queries := 0
			client := &mockaur.MockAUR{GetFn: func(_ context.Context, q *aur.Query) ([]aur.Pkg, error) {
				queries++
				require.Equal(t, aur.Name, q.By)
				if q.Needles[0] == "app" {
					return []aur.Pkg{{Name: "app", PackageBase: "app", Version: "1", Depends: []string{tc.dep}}}, nil
				}
				return []aur.Pkg{tc.remote}, nil
			}}
			grapher := NewGrapher(&mock.DBExecutor{
				InstalledRemotePackagesFn: func() map[string]mock.IPackage { return installed },
				LocalPackageFn:            func(name string) mock.IPackage { return installed[name] },
				LocalSatisfierExistsFn:    func(string) bool { return tc.localSatisfied },
				SyncSatisfierFn:           func(string) mock.IPackage { return tc.repo },
			}, client, false, true, false, false, false, text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test"))
			require.NoError(t, grapher.SetRebuildMode(t.Context(), parser.RebuildModeTree))
			graph, err := grapher.GraphFromAUR(t.Context(), nil, []string{"app"})
			require.NoError(t, err)

			depName, _, _ := splitDep(tc.dep)
			require.True(t, graph.Exists(depName))
			require.Equal(t, tc.wantSource, graph.GetNodeInfo(depName).Value.Source)
			require.Equal(t, tc.wantVersion, graph.GetNodeInfo(depName).Value.Version)
			require.Equal(t, 2, queries)
		})
	}
}

func TestGrapher_RebuildTreeStopsAtProviderCycle(t *testing.T) {
	t.Parallel()

	installed := map[string]mock.IPackage{
		"a-git": &mock.Package{PName: "a-git", PVersion: "1", PProvides: mock.DependList{
			Depends: []dyalpm.Depend{{Name: "virtual-a", Mod: dyalpm.DepModAny}},
		}},
		"b-git": &mock.Package{PName: "b-git", PVersion: "1", PProvides: mock.DependList{
			Depends: []dyalpm.Depend{{Name: "virtual-b", Mod: dyalpm.DepModAny}},
		}},
	}
	packages := map[string]aur.Pkg{
		"app":   {Name: "app", PackageBase: "app", Version: "1", Depends: []string{"virtual-a"}},
		"a-git": {Name: "a-git", PackageBase: "a-git", Version: "1", Provides: []string{"virtual-a"}, Depends: []string{"virtual-b"}},
		"b-git": {Name: "b-git", PackageBase: "b-git", Version: "1", Provides: []string{"virtual-b"}, Depends: []string{"virtual-a"}},
	}
	localChecks := 0
	grapher := NewGrapher(&mock.DBExecutor{
		InstalledRemotePackagesFn: func() map[string]mock.IPackage { return installed },
		LocalPackageFn:            func(name string) mock.IPackage { return installed[name] },
		LocalSatisfierExistsFn: func(string) bool {
			localChecks++
			require.LessOrEqual(t, localChecks, 3, "provider cycle was expanded repeatedly")
			return true
		},
	}, &mockaur.MockAUR{GetFn: func(_ context.Context, q *aur.Query) ([]aur.Pkg, error) {
		pkgs := make([]aur.Pkg, 0, len(q.Needles))
		for _, name := range q.Needles {
			if pkg, ok := packages[name]; ok {
				pkgs = append(pkgs, pkg)
			}
		}
		return pkgs, nil
	}}, false, true, false, false, false, text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test"))
	require.NoError(t, grapher.SetRebuildMode(t.Context(), parser.RebuildModeTree))
	graph, err := grapher.GraphFromAUR(t.Context(), nil, []string{"app"})
	require.NoError(t, err)

	require.Equal(t, 3, graph.Len())
	require.Equal(t, 2, localChecks)
	require.Len(t, graph.TopoSortedLayers(nil), 3)
}

func TestGrapher_GraphFromTargetsBranches(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	dbExecutor := &mock.DBExecutor{
		LocalPackageFn: func(string) mock.IPackage { return nil },
		SatisfierFromDBFn: func(_, db string) (mock.IPackage, error) {
			return &mock.Package{
				PName:    "repo-pkg",
				PVersion: "1",
				PDB:      mock.NewDB(db),
			}, nil
		},
		PackagesFromGroupAndDBFn: func(name, dbName string) ([]mock.IPackage, error) {
			return nil, nil
		},
	}

	grapher := NewGrapher(dbExecutor, &mockaur.MockAUR{}, false, false, false, false, false, logger)
	graph, err := grapher.GraphFromTargets(context.Background(), nil, []string{"core/repo-pkg"})
	require.NoError(t, err)
	require.True(t, graph.Exists("repo-pkg"))
}

func TestGrapher_GraphFromTargetsFallbackToGroup(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	dbExecutor := &mock.DBExecutor{
		SyncSatisfierFn: func(string) mock.IPackage {
			return nil
		},
		PackagesFromGroupFn: func(name string) []mock.IPackage {
			return []mock.IPackage{
				&mock.Package{
					PName: "grouped",
					PDB:   mock.NewDB("extra"),
				},
			}
		},
	}

	grapher := NewGrapher(dbExecutor, &mockaur.MockAUR{}, false, false, false, false, false, logger)
	graph, err := grapher.GraphFromTargets(context.Background(), nil, []string{"grouped"})
	require.NoError(t, err)
	require.True(t, graph.Exists("grouped"))
	require.True(t, graph.GetNodeInfo("grouped").Value.IsGroup)
}

func TestGrapher_GraphFromTargetsFallbackToAur(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	dbExecutor := &mock.DBExecutor{
		SyncSatisfierFn:     func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return nil },
		LocalPackageFn:      func(string) mock.IPackage { return nil },
	}

	grapher := NewGrapher(dbExecutor, &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{Name: "aur-target", Version: "1"}}, nil
		},
	}, false, false, false, false, false, logger)

	graph, err := grapher.GraphFromTargets(context.Background(), nil, []string{"aur-target"})
	require.NoError(t, err)
	require.Equal(t, AUR, graph.GetNodeInfo("aur-target").Value.Source)
}

func TestGrapher_GraphFromAURNeededSkipsUpToDate(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	dbExecutor := &mock.DBExecutor{
		LocalPackageFn: func(name string) mock.IPackage {
			return &mock.Package{
				PName:    name,
				PVersion: "2",
				PReason:  dyalpm.PkgReasonExplicit,
			}
		},
	}

	grapher := NewGrapher(dbExecutor, &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{
				Name:    "needless",
				Version: "1",
			}}, nil
		},
	}, false, false, false, false, true, logger)

	graph, err := grapher.GraphFromAUR(context.Background(), nil, []string{"needless"})
	require.NoError(t, err)
	require.False(t, graph.Exists("needless"))
}

func TestGrapher_GraphFromAURNotFoundReturnsErr(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	grapher := NewGrapher(&mock.DBExecutor{}, &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return nil, nil
		},
	}, false, false, false, false, false, logger)

	graph, err := grapher.GraphFromAUR(context.Background(), nil, []string{"ghost"})
	var targetNotFound *query.ErrTargetNotFound
	require.ErrorAs(t, err, &targetNotFound)
	require.NotNil(t, graph)
	require.False(t, graph.Exists("ghost"))
}

func TestGrapher_GraphFromSrcInfosSinglePkg(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	dbExecutor := &mock.DBExecutor{
		LocalPackageFn: func(string) mock.IPackage { return nil },
		AlpmArchitecturesFn: func() ([]string, error) {
			return []string{"x86_64"}, nil
		},
	}

	grapher := NewGrapher(dbExecutor, &mockaur.MockAUR{}, false, false, false, false, false, logger)
	graph, err := grapher.GraphFromSrcInfos(context.Background(), nil, map[string]*gosrc.Srcinfo{
		"foo": {
			PackageBase: gosrc.PackageBase{
				Pkgbase: "foo",
				Pkgver:  "1",
			},
			Packages: []gosrc.Package{
				{
					Pkgname: "foo",
				},
			},
		},
	})

	require.NoError(t, err)
	info := graph.GetNodeInfo("foo")
	require.NotNil(t, info)
	require.Equal(t, SrcInfo, info.Value.Source)
}

func TestGrapher_FindDepsFromAURSatisfiesMissingAndErrorPaths(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	dbExecutor := &mock.DBExecutor{
		LocalSatisfierExistsFn: func(string) bool { return false },
		SyncSatisfierFn:        func(string) mock.IPackage { return nil },
	}

	grapher := NewGrapher(dbExecutor, &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{
				{
					Name:    "dep-provider",
					Version: "2.0",
					Provides: []string{
						"dep-lib",
					},
				},
			}, nil
		},
	}, false, false, false, false, false, logger)

	graph := NewGraph()
	graph.AddNode("root")
	graph.AddProvides("dep-lib", &dyalpm.Depend{Name: "dep-lib", Version: "2.0"}, "dep-provider")
	found, _ := grapher.findDepsFromAUR(context.Background(), graph, "root", []string{"dep-lib"})
	require.Len(t, found, 1)
}
