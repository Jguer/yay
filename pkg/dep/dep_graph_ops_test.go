//go:build !integration

package dep

import (
	"context"
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

func TestGrapher_GraphFromTargetsSourceMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mode       parser.TargetMode
		target     string
		wantSource Source
		wantSkip   bool
		wantError  bool
	}{
		{name: "default prefers repository", target: "shared", wantSource: Sync},
		{name: "default falls back to AUR", target: "aur-only", wantSource: AUR},
		{name: "AUR mode prefers AUR", mode: parser.ModeAUR, target: "shared", wantSource: AUR},
		{name: "AUR mode accepts AUR prefix", mode: parser.ModeAUR, target: "aur/shared", wantSource: AUR},
		{name: "AUR mode skips repository prefix", mode: parser.ModeAUR, target: "extra/shared", wantSkip: true},
		{name: "repo mode selects repository", mode: parser.ModeRepo, target: "shared", wantSource: Sync},
		{name: "repo mode accepts repository prefix", mode: parser.ModeRepo, target: "extra/shared", wantSource: Sync},
		{name: "repo mode skips AUR prefix", mode: parser.ModeRepo, target: "aur/shared", wantSkip: true},
		{name: "repo mode never falls back to AUR", mode: parser.ModeRepo, target: "aur-only", wantError: true},
		{name: "default reports missing target", target: "missing", wantError: true},
		{name: "AUR mode reports missing target", mode: parser.ModeAUR, target: "missing", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repoPackage := func(name string) mock.IPackage {
				if name != "shared" && name != "repo-dep" {
					return nil
				}
				return &mock.Package{PName: name, PVersion: "1", PDB: mock.NewDB("extra")}
			}
			dbExecutor := &mock.DBExecutor{
				SyncSatisfierFn: repoPackage,
				SatisfierFromDBFn: func(name, dbName string) (mock.IPackage, error) {
					require.Equal(t, "extra", dbName)
					return repoPackage(name), nil
				},
				PackagesFromGroupFn:    func(string) []mock.IPackage { return nil },
				LocalPackageFn:         func(string) mock.IPackage { return nil },
				LocalSatisfierExistsFn: func(string) bool { return false },
			}
			aurCalls := 0
			aurClient := &mockaur.MockAUR{GetFn: func(_ context.Context, q *aur.Query) ([]aur.Pkg, error) {
				aurCalls++
				require.NotEqual(t, parser.ModeRepo, tt.mode, "repository-only targets must not query AUR")
				if q.By != aur.Name || q.Needles[0] == "missing" {
					return nil, nil
				}
				name := q.Needles[0]
				return []aur.Pkg{{Name: name, PackageBase: name, Version: "1", Depends: []string{"repo-dep"}}}, nil
			}}
			logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")
			grapher := NewGrapher(dbExecutor, aurClient, false, true, false, false, false, logger)
			grapher.SetTargetMode(tt.mode)

			graph, err := grapher.GraphFromTargets(t.Context(), nil, []string{tt.target})
			if tt.wantError {
				require.ErrorAs(t, err, new(*query.ErrTargetNotFound))
				return
			}
			require.NoError(t, err)
			if tt.wantSkip {
				require.Zero(t, graph.Len())
				require.Zero(t, aurCalls)
				return
			}

			name := ToTarget(tt.target).Name
			require.Equal(t, tt.wantSource, graph.GetNodeInfo(name).Value.Source)
			if tt.wantSource == AUR {
				require.Equal(t, Sync, graph.GetNodeInfo("repo-dep").Value.Source,
					"AUR-only targets may still need repository dependencies")
			}
		})
	}
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
