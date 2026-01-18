//go:build !integration
// +build !integration

package dep

import (
	"context"
	"io"
	"os"
	"testing"

	aurc "github.com/Jguer/aur"
	alpm "github.com/Jguer/dyalpm"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/db/mock"
	mockaur "github.com/Jguer/yay/v12/pkg/dep/mock"
	aur "github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/text"
)

// benchCase represents a single benchmark scenario with expected results for validation.
type benchCase struct {
	name           string
	targets        []string
	expectedLayers []map[string]*InstallInfo
	noDeps         bool
	noCheckDeps    bool
}

// newBenchMockDB creates a mock DB executor for benchmarking gstreamer-git scenarios.
func newBenchMockDB() *mock.DBExecutor {
	return &mock.DBExecutor{
		SyncPackageFn:       func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return []mock.IPackage{} },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "gstreamer-git", "gst-plugins-base-libs-git", "gst-plugins-good-git",
				"gstreamer-git=1.24.0.r37-1", "gst-plugins-base-libs-git=1.24.0.r37-1":
				return nil
			case "libxml2":
				return &mock.Package{
					PName:    "libxml2",
					PVersion: "2.12.0-1",
					PDB:      mock.NewDB("core"),
				}
			case "glib2":
				return &mock.Package{
					PName:    "glib2",
					PVersion: "2.78.0-1",
					PDB:      mock.NewDB("core"),
				}
			case "orc":
				return &mock.Package{
					PName:    "orc",
					PVersion: "0.4.34-1",
					PDB:      mock.NewDB("extra"),
				}
			case "libxv":
				return &mock.Package{
					PName:    "libxv",
					PVersion: "1.0.12-1",
					PDB:      mock.NewDB("extra"),
				}
			case "iso-codes":
				return &mock.Package{
					PName:    "iso-codes",
					PVersion: "4.15.0-1",
					PDB:      mock.NewDB("extra"),
				}
			case "libpulse":
				return &mock.Package{
					PName:    "libpulse",
					PVersion: "16.1-1",
					PDB:      mock.NewDB("extra"),
				}
			case "wavpack":
				return &mock.Package{
					PName:    "wavpack",
					PVersion: "5.6.0-1",
					PDB:      mock.NewDB("extra"),
				}
			}
			return nil
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "gstreamer-git", "gstreamer-git=1.24.0.r37-1",
				"gst-plugins-base-libs-git", "gst-plugins-base-libs-git=1.24.0.r37-1",
				"gst-plugins-good-git":
				return false
			case "libxml2", "glib2", "orc", "libxv", "iso-codes", "libpulse", "wavpack",
				"git", "meson", "ninja", "llvm", "clang":
				return true
			}
			return true
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}
}

// newBenchMockAUR creates a mock AUR client for benchmarking gstreamer-git scenarios.
func newBenchMockAUR(t testing.TB) *mockaur.MockAUR {
	return &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		if len(query.Needles) > 0 {
			for _, needle := range query.Needles {
				if needle == "gstreamer-git" || needle == "gst-plugins-base-libs-git" || needle == "gst-plugins-good-git" {
					gstFn := getFromFile(t, "testdata/gstreamer-git.json")
					return gstFn(ctx, query)
				}
			}
		}
		return []aur.Pkg{}, nil
	}}
}

// newJellyfinMockDB creates a mock DB for jellyfin scenarios.
func newJellyfinMockDB() *mock.DBExecutor {
	return &mock.DBExecutor{
		SyncPackageFn: func(string) mock.IPackage { return nil },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "jellyfin":
				return nil
			case "dotnet-runtime-6.0":
				return &mock.Package{
					PName:    "dotnet-runtime-6.0",
					PBase:    "dotnet-runtime-6.0",
					PVersion: "6.0.100-1",
					PDB:      mock.NewDB("community"),
				}
			case "dotnet-sdk-6.0":
				return &mock.Package{
					PName:    "dotnet-sdk-6.0",
					PBase:    "dotnet-sdk-6.0",
					PVersion: "6.0.100-1",
					PDB:      mock.NewDB("community"),
				}
			}
			return nil
		},
		PackagesFromGroupFn: func(string) []mock.IPackage { return nil },
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "dotnet-sdk-6.0", "dotnet-runtime-6.0", "jellyfin-server=10.8.8", "jellyfin-web=10.8.8":
				return false
			}
			return true
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}
}

// newJellyfinMockAUR creates a mock AUR for jellyfin scenarios.
func newJellyfinMockAUR(t testing.TB) *mockaur.MockAUR {
	return &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		if len(query.Needles) == 0 {
			return []aur.Pkg{}, nil
		}
		switch query.Needles[0] {
		case "jellyfin":
			return getFromFile(t, "testdata/jellyfin.json")(ctx, query)
		case "jellyfin-web":
			return getFromFile(t, "testdata/jellyfin-web.json")(ctx, query)
		case "jellyfin-server":
			return getFromFile(t, "testdata/jellyfin-server.json")(ctx, query)
		}
		return []aur.Pkg{}, nil
	}}
}

// newCephMockDB creates a mock DB for ceph scenarios with providers.
func newCephMockDB() *mock.DBExecutor {
	return &mock.DBExecutor{
		SyncPackageFn:       func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return []mock.IPackage{} },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "ceph-bin", "ceph-libs-bin", "ceph", "ceph-libs", "ceph-libs=17.2.6-2":
				return nil
			}
			return nil
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "ceph-libs", "ceph-libs=17.2.6-2":
				return false
			case "dep1", "dep2", "dep3", "makedep1", "makedep2", "checkdep1":
				return true
			}
			return true
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}
}

// newCephMockAUR creates a mock AUR for ceph scenarios.
func newCephMockAUR() *mockaur.MockAUR {
	mockPkgs := map[string]aur.Pkg{
		"ceph-bin": {
			Name:        "ceph-bin",
			PackageBase: "ceph-bin",
			Version:     "17.2.6-2",
			Depends:     []string{"ceph-libs=17.2.6-2", "dep1"},
			Provides:    []string{"ceph=17.2.6-2"},
		},
		"ceph-libs-bin": {
			Name:        "ceph-libs-bin",
			PackageBase: "ceph-bin",
			Version:     "17.2.6-2",
			Depends:     []string{"dep1", "dep2"},
			Provides:    []string{"ceph-libs=17.2.6-2"},
		},
		"ceph": {
			Name:         "ceph",
			PackageBase:  "ceph",
			Version:      "17.2.6-2",
			Depends:      []string{"ceph-libs=17.2.6-2", "dep1"},
			MakeDepends:  []string{"makedep1"},
			CheckDepends: []string{"checkdep1"},
			Provides:     []string{"ceph=17.2.6-2"},
		},
		"ceph-libs": {
			Name:         "ceph-libs",
			PackageBase:  "ceph",
			Version:      "17.2.6-2",
			Depends:      []string{"dep1", "dep2", "dep3"},
			MakeDepends:  []string{"makedep1", "makedep2"},
			CheckDepends: []string{"checkdep1"},
			Provides:     []string{"ceph-libs=17.2.6-2"},
		},
	}

	return &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		pkgs := []aur.Pkg{}
		for _, needle := range query.Needles {
			if pkg, ok := mockPkgs[needle]; ok {
				pkgs = append(pkgs, pkg)
			}
		}
		return pkgs, nil
	}}
}

// newAndroidSDKMockDB creates a mock DB for android-sdk scenarios.
func newAndroidSDKMockDB() *mock.DBExecutor {
	return &mock.DBExecutor{
		SyncPackageFn: func(string) mock.IPackage { return nil },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "android-sdk":
				return nil
			case "jdk11-openjdk":
				return &mock.Package{
					PName:    "jdk11-openjdk",
					PVersion: "11.0.12.u7-1",
					PDB:      mock.NewDB("community"),
					PProvides: mock.DependList{
						Depends: []alpm.Depend{
							{Name: "java-environment", Version: "11", Mod: alpm.DepModEQ},
							{Name: "java-environment-openjdk", Version: "11", Mod: alpm.DepModEQ},
							{Name: "jdk11-openjdk", Version: "11.0.19.u7-1", Mod: alpm.DepModEQ},
						},
					},
				}
			}
			return nil
		},
		PackagesFromGroupFn: func(string) []mock.IPackage { return nil },
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "java-environment":
				return false
			case "libxtst", "fontconfig", "freetype2", "lib32-gcc-libs", "lib32-glibc",
				"libx11", "libxext", "libxrender", "zlib", "gcc-libs":
				return true
			}
			return true
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}
}

// newAndroidSDKMockAUR creates a mock AUR for android-sdk scenarios.
func newAndroidSDKMockAUR(t testing.TB) *mockaur.MockAUR {
	return &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		if len(query.Needles) > 0 && query.Needles[0] == "android-sdk" {
			return getFromFile(t, "testdata/android-sdk.json")(ctx, query)
		}
		return []aur.Pkg{}, nil
	}}
}

// verifyLayers checks that the actual layers match expected layers.
func verifyLayers(t testing.TB, expected, actual []map[string]*InstallInfo) {
	t.Helper()
	require.Equal(t, len(expected), len(actual), "layer count mismatch")
	for i := range expected {
		require.Equal(t, len(expected[i]), len(actual[i]), "layer %d package count mismatch", i)
		for name, expectedInfo := range expected[i] {
			actualInfo, ok := actual[i][name]
			require.True(t, ok, "missing package %s in layer %d", name, i)
			require.Equal(t, expectedInfo.Source, actualInfo.Source, "source mismatch for %s", name)
			require.Equal(t, expectedInfo.Reason, actualInfo.Reason, "reason mismatch for %s", name)
			require.Equal(t, expectedInfo.Version, actualInfo.Version, "version mismatch for %s", name)
			if expectedInfo.AURBase != nil {
				require.NotNil(t, actualInfo.AURBase, "AURBase should not be nil for %s", name)
				require.Equal(t, *expectedInfo.AURBase, *actualInfo.AURBase, "AURBase mismatch for %s", name)
			}
			if expectedInfo.SyncDBName != nil {
				require.NotNil(t, actualInfo.SyncDBName, "SyncDBName should not be nil for %s", name)
				require.Equal(t, *expectedInfo.SyncDBName, *actualInfo.SyncDBName, "SyncDBName mismatch for %s", name)
			}
		}
	}
}

// BenchmarkGraphFromTargets_GstreamerGit benchmarks dependency graph construction
// for the gstreamer-git split package scenario.
func BenchmarkGraphFromTargets_GstreamerGit(b *testing.B) {
	cases := []benchCase{
		{
			name:    "SingleTarget",
			targets: []string{"gst-plugins-good-git"},
			expectedLayers: []map[string]*InstallInfo{
				{"gst-plugins-good-git": {Source: AUR, Reason: Explicit, Version: "1.24.0.r37-1", AURBase: ptrString("gstreamer-git")}},
				{"gst-plugins-base-libs-git": {Source: AUR, Reason: Dep, Version: "1.24.0.r37-1", AURBase: ptrString("gstreamer-git")}},
				{"gstreamer-git": {Source: AUR, Reason: Dep, Version: "1.24.0.r37-1", AURBase: ptrString("gstreamer-git")}},
			},
		},
		{
			name:    "TwoTargets",
			targets: []string{"gstreamer-git", "gst-plugins-good-git"},
			expectedLayers: []map[string]*InstallInfo{
				{"gst-plugins-good-git": {Source: AUR, Reason: Explicit, Version: "1.24.0.r37-1", AURBase: ptrString("gstreamer-git")}},
				{"gst-plugins-base-libs-git": {Source: AUR, Reason: Dep, Version: "1.24.0.r37-1", AURBase: ptrString("gstreamer-git")}},
				{"gstreamer-git": {Source: AUR, Reason: Explicit, Version: "1.24.0.r37-1", AURBase: ptrString("gstreamer-git")}},
			},
		},
		{
			name:    "AllThreeExplicit",
			targets: []string{"gstreamer-git", "gst-plugins-base-libs-git", "gst-plugins-good-git"},
			expectedLayers: []map[string]*InstallInfo{
				{"gst-plugins-good-git": {Source: AUR, Reason: Explicit, Version: "1.24.0.r37-1", AURBase: ptrString("gstreamer-git")}},
				{"gst-plugins-base-libs-git": {Source: AUR, Reason: Explicit, Version: "1.24.0.r37-1", AURBase: ptrString("gstreamer-git")}},
				{"gstreamer-git": {Source: AUR, Reason: Explicit, Version: "1.24.0.r37-1", AURBase: ptrString("gstreamer-git")}},
			},
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			mockDB := newBenchMockDB()
			mockAUR := newBenchMockAUR(b)
			logger := text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test")
			g := NewGrapher(mockDB, mockAUR, false, true, tc.noDeps, tc.noCheckDeps, false, logger)

			// Verify correctness once before benchmarking
			graph, err := g.GraphFromTargets(context.Background(), nil, tc.targets)
			require.NoError(b, err)
			layers := graph.TopoSortedLayers(nil)
			verifyLayers(b, tc.expectedLayers, layers)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = g.GraphFromTargets(context.Background(), nil, tc.targets)
			}
		})
	}
}

// BenchmarkGraphFromTargets_Jellyfin benchmarks dependency graph construction
// for the jellyfin package with mixed AUR/sync dependencies.
func BenchmarkGraphFromTargets_Jellyfin(b *testing.B) {
	cases := []benchCase{
		{
			name:        "NoDeps",
			targets:     []string{"jellyfin"},
			noDeps:      true,
			noCheckDeps: false,
			expectedLayers: []map[string]*InstallInfo{
				{"jellyfin": {Source: AUR, Reason: Explicit, Version: "10.8.8-1", AURBase: ptrString("jellyfin")}},
				{"dotnet-sdk-6.0": {Source: Sync, Reason: MakeDep, Version: "6.0.100-1", SyncDBName: ptrString("community")}},
			},
		},
		{
			name:        "WithDeps",
			targets:     []string{"jellyfin"},
			noDeps:      false,
			noCheckDeps: false,
			expectedLayers: []map[string]*InstallInfo{
				{"jellyfin": {Source: AUR, Reason: Explicit, Version: "10.8.8-1", AURBase: ptrString("jellyfin")}},
				{
					"jellyfin-web":    {Source: AUR, Reason: Dep, Version: "10.8.8-1", AURBase: ptrString("jellyfin")},
					"jellyfin-server": {Source: AUR, Reason: Dep, Version: "10.8.8-1", AURBase: ptrString("jellyfin")},
				},
				{
					"dotnet-sdk-6.0":     {Source: Sync, Reason: MakeDep, Version: "6.0.100-1", SyncDBName: ptrString("community")},
					"dotnet-runtime-6.0": {Source: Sync, Reason: Dep, Version: "6.0.100-1", SyncDBName: ptrString("community")},
				},
			},
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			mockDB := newJellyfinMockDB()
			mockAUR := newJellyfinMockAUR(b)
			logger := text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test")
			g := NewGrapher(mockDB, mockAUR, false, true, tc.noDeps, tc.noCheckDeps, false, logger)

			// Verify correctness once before benchmarking
			graph, err := g.GraphFromTargets(context.Background(), nil, tc.targets)
			require.NoError(b, err)
			layers := graph.TopoSortedLayers(nil)
			verifyLayers(b, tc.expectedLayers, layers)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = g.GraphFromTargets(context.Background(), nil, tc.targets)
			}
		})
	}
}

// BenchmarkGraphFromTargets_CephProvides benchmarks dependency graph construction
// for packages with virtual provides (ceph-bin provides ceph-libs).
func BenchmarkGraphFromTargets_CephProvides(b *testing.B) {
	cases := []benchCase{
		{
			name:    "CephBinWithLibsBin",
			targets: []string{"ceph-bin", "ceph-libs-bin"},
			expectedLayers: []map[string]*InstallInfo{
				{"ceph-bin": {Source: AUR, Reason: Explicit, Version: "17.2.6-2", AURBase: ptrString("ceph-bin")}},
				{"ceph-libs-bin": {Source: AUR, Reason: Explicit, Version: "17.2.6-2", AURBase: ptrString("ceph-bin")}},
			},
		},
		{
			name:    "CephOnly",
			targets: []string{"ceph"},
			expectedLayers: []map[string]*InstallInfo{
				{"ceph": {Source: AUR, Reason: Explicit, Version: "17.2.6-2", AURBase: ptrString("ceph")}},
				{"ceph-libs": {Source: AUR, Reason: Dep, Version: "17.2.6-2", AURBase: ptrString("ceph")}},
			},
		},
		{
			name:    "CephBinOnly",
			targets: []string{"ceph-bin"},
			expectedLayers: []map[string]*InstallInfo{
				{"ceph-bin": {Source: AUR, Reason: Explicit, Version: "17.2.6-2", AURBase: ptrString("ceph-bin")}},
				{"ceph-libs": {Source: AUR, Reason: Dep, Version: "17.2.6-2", AURBase: ptrString("ceph")}},
			},
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			mockDB := newCephMockDB()
			mockAUR := newCephMockAUR()
			logger := text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test")
			g := NewGrapher(mockDB, mockAUR, false, true, tc.noDeps, tc.noCheckDeps, false, logger)

			// Verify correctness once before benchmarking
			graph, err := g.GraphFromTargets(context.Background(), nil, tc.targets)
			require.NoError(b, err)
			layers := graph.TopoSortedLayers(nil)
			verifyLayers(b, tc.expectedLayers, layers)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = g.GraphFromTargets(context.Background(), nil, tc.targets)
			}
		})
	}
}

// BenchmarkGraphFromTargets_AndroidSDK benchmarks dependency graph construction
// for packages with explicit sync dependencies providing virtual packages.
func BenchmarkGraphFromTargets_AndroidSDK(b *testing.B) {
	tc := benchCase{
		name:    "WithJDK",
		targets: []string{"android-sdk", "jdk11-openjdk"},
		expectedLayers: []map[string]*InstallInfo{
			{"android-sdk": {Source: AUR, Reason: Explicit, Version: "26.1.1-2", AURBase: ptrString("android-sdk")}},
			{"jdk11-openjdk": {Source: Sync, Reason: Explicit, Version: "11.0.12.u7-1", SyncDBName: ptrString("community")}},
		},
	}

	mockDB := newAndroidSDKMockDB()
	mockAUR := newAndroidSDKMockAUR(b)
	logger := text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test")
	g := NewGrapher(mockDB, mockAUR, false, true, tc.noDeps, tc.noCheckDeps, false, logger)

	// Verify correctness once before benchmarking
	graph, err := g.GraphFromTargets(context.Background(), nil, tc.targets)
	require.NoError(b, err)
	layers := graph.TopoSortedLayers(nil)
	verifyLayers(b, tc.expectedLayers, layers)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = g.GraphFromTargets(context.Background(), nil, tc.targets)
	}
}

// BenchmarkTopoSortedLayers benchmarks the topological sort operation on pre-built graphs.
func BenchmarkTopoSortedLayers(b *testing.B) {
	b.Run("GstreamerGit", func(b *testing.B) {
		mockDB := newBenchMockDB()
		mockAUR := newBenchMockAUR(b)
		logger := text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test")
		g := NewGrapher(mockDB, mockAUR, false, true, false, false, false, logger)

		graph, err := g.GraphFromTargets(context.Background(), nil, []string{"gst-plugins-good-git"})
		require.NoError(b, err)

		// Verify correctness
		layers := graph.TopoSortedLayers(nil)
		require.Len(b, layers, 3)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = graph.TopoSortedLayers(nil)
		}
	})

	b.Run("Jellyfin", func(b *testing.B) {
		mockDB := newJellyfinMockDB()
		mockAUR := newJellyfinMockAUR(b)
		logger := text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test")
		g := NewGrapher(mockDB, mockAUR, false, true, false, false, false, logger)

		graph, err := g.GraphFromTargets(context.Background(), nil, []string{"jellyfin"})
		require.NoError(b, err)

		// Verify correctness
		layers := graph.TopoSortedLayers(nil)
		require.Len(b, layers, 3)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = graph.TopoSortedLayers(nil)
		}
	})
}

// BenchmarkNewGraph benchmarks graph creation overhead.
func BenchmarkNewGraph(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewGraph()
	}
}

// BenchmarkGraphDependOn benchmarks adding dependency edges.
func BenchmarkGraphDependOn(b *testing.B) {
	b.Run("SmallGraph", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			graph := NewGraph()
			_ = graph.DependOn("pkg1", "pkg2")
			_ = graph.DependOn("pkg2", "pkg3")
			_ = graph.DependOn("pkg3", "pkg4")
		}
	})

	b.Run("MediumGraph", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			graph := NewGraph()
			// Create a chain of 20 dependencies
			for j := 0; j < 20; j++ {
				_ = graph.DependOn(
					"pkg"+string(rune('A'+j)),
					"pkg"+string(rune('A'+j+1)),
				)
			}
		}
	})
}
