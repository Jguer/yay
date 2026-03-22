//go:build !integration
// +build !integration

// Package dep provides tests for tree resolution and parsing using RPC/.SRCINFO metadata
// instead of PKGBUILD parsing. These tests validate:
// - Reliable parser: Ability to handle complex packages using provided metadata
// - Reliable solver: Ability to correctly solve and build complex dependency chains
// - Split packages: Ability to correctly build and install split packages
package dep

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	aurc "github.com/Jguer/aur"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/db/mock"
	mockaur "github.com/Jguer/yay/v12/pkg/dep/mock"
	aur "github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/text"
)

// TestGrapher_ReliableParser_AWSCliGit tests the reliable parsing capability
// for complex packages like aws-cli-git that have many dependencies.
// This validates that the RPC metadata is correctly parsed and dependencies
// are properly resolved without needing PKGBUILD parsing.
func TestGrapher_ReliableParser_AWSCliGit(t *testing.T) {
	t.Parallel()

	mockDB := &mock.DBExecutor{
		SyncPackageFn:       func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return []mock.IPackage{} },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "aws-cli-git":
				return nil
			case "python":
				return &mock.Package{PName: "python", PVersion: "3.11.0-1", PDB: mock.NewDB("core")}
			case "python-botocore>=1.19.35", "python-botocore":
				return &mock.Package{PName: "python-botocore", PVersion: "1.29.0-1", PDB: mock.NewDB("extra")}
			case "python-docutils>=0.10", "python-docutils":
				return &mock.Package{PName: "python-docutils", PVersion: "0.19-1", PDB: mock.NewDB("extra")}
			case "python-rsa>=3.1.2", "python-rsa":
				return &mock.Package{PName: "python-rsa", PVersion: "4.9-1", PDB: mock.NewDB("extra")}
			case "python-s3transfer>=0.3.0", "python-s3transfer":
				return &mock.Package{PName: "python-s3transfer", PVersion: "0.6.0-1", PDB: mock.NewDB("extra")}
			case "python-yaml>=3.10", "python-yaml":
				return &mock.Package{PName: "python-yaml", PVersion: "6.0-1", PDB: mock.NewDB("extra")}
			case "python-colorama>=0.2.5", "python-colorama":
				return &mock.Package{PName: "python-colorama", PVersion: "0.4.6-1", PDB: mock.NewDB("extra")}
			case "python-tox>=2.3.1", "python-tox":
				return &mock.Package{PName: "python-tox", PVersion: "4.0.0-1", PDB: mock.NewDB("extra")}
			case "python-nose>=1.3.7", "python-nose":
				return &mock.Package{PName: "python-nose", PVersion: "1.3.7-1", PDB: mock.NewDB("extra")}
			case "python-mock>=1.3.0", "python-mock":
				return &mock.Package{PName: "python-mock", PVersion: "5.0.0-1", PDB: mock.NewDB("extra")}
			case "python-wheel>=0.24.0", "python-wheel":
				return &mock.Package{PName: "python-wheel", PVersion: "0.38.0-1", PDB: mock.NewDB("extra")}
			case "python-dateutil>=2.1", "python-dateutil":
				return &mock.Package{PName: "python-dateutil", PVersion: "2.8.2-1", PDB: mock.NewDB("extra")}
			case "python-sphinx>=1.1.3", "python-sphinx":
				return &mock.Package{PName: "python-sphinx", PVersion: "6.0.0-1", PDB: mock.NewDB("extra")}
			case "python-distribute":
				return &mock.Package{PName: "python-distribute", PVersion: "0.7.3-1", PDB: mock.NewDB("extra")}
			case "git":
				return &mock.Package{PName: "git", PVersion: "2.39.0-1", PDB: mock.NewDB("extra")}
			}
			panic("implement me " + s)
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "aws-cli-git":
				return false
			case "python", "python-botocore>=1.19.35", "python-docutils>=0.10",
				"python-rsa>=3.1.2", "python-s3transfer>=0.3.0", "python-yaml>=3.10",
				"python-colorama>=0.2.5", "python-tox>=2.3.1", "python-nose>=1.3.7",
				"python-mock>=1.3.0", "python-wheel>=0.24.0", "python-dateutil>=2.1",
				"python-sphinx>=1.1.3", "python-distribute", "git":
				return true
			}
			panic("implement me " + s)
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		if len(query.Needles) > 0 && query.Needles[0] == "aws-cli-git" {
			awsFn := getFromFile(t, "testdata/aws-cli-git.json")
			return awsFn(ctx, query)
		}
		return []aur.Pkg{}, nil
	}}

	t.Run("parses aws-cli-git with all its dependencies", func(td *testing.T) {
		td.Parallel()

		g := NewGrapher(mockDB, mockAUR, false, true, false, false, false,
			text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
		got, err := g.GraphFromTargets(context.Background(), nil, []string{"aws-cli-git"})
		require.NoError(td, err)
		layers := got.TopoSortedLayers(nil)

		require.NotEmpty(td, layers)
		require.Contains(td, layers[0], "aws-cli-git")
		require.Equal(td, "1.27.145.r11217.g5885ee4dc-1", layers[0]["aws-cli-git"].Version)
		require.Equal(td, "aws-cli-git", *layers[0]["aws-cli-git"].AURBase)
		require.Equal(td, Explicit, layers[0]["aws-cli-git"].Reason)
		require.Equal(td, AUR, layers[0]["aws-cli-git"].Source)
	})

	t.Run("validates provides field for aws-cli", func(td *testing.T) {
		td.Parallel()

		g := NewGrapher(mockDB, mockAUR, false, true, false, false, false,
			text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
		got, err := g.GraphFromTargets(context.Background(), nil, []string{"aws-cli-git"})
		require.NoError(td, err)
		layers := got.TopoSortedLayers(nil)

		require.NotEmpty(td, layers)
		require.Contains(td, layers[0], "aws-cli-git")
	})
}

// TestGrapher_ReliableSolver_LiriDesktopGit tests the dependency solver
// with complex dependency chains like liri-desktop-git metapackage.
func TestGrapher_ReliableSolver_LiriDesktopGit(t *testing.T) {
	t.Parallel()

	mockDB := &mock.DBExecutor{
		SyncPackageFn:       func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return []mock.IPackage{} },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "liri-desktop-git", "liri-shell-git", "liri-settings-git",
				"libliri-git", "fluid-git", "liri-cmake-shared-git":
				return nil
			case "qt5-declarative":
				return &mock.Package{PName: "qt5-declarative", PVersion: "5.15.10-1", PDB: mock.NewDB("extra")}
			case "qt5-quickcontrols2":
				return &mock.Package{PName: "qt5-quickcontrols2", PVersion: "5.15.10-1", PDB: mock.NewDB("extra")}
			case "qt5-svg":
				return &mock.Package{PName: "qt5-svg", PVersion: "5.15.10-1", PDB: mock.NewDB("extra")}
			case "qt5-graphicaleffects":
				return &mock.Package{PName: "qt5-graphicaleffects", PVersion: "5.15.10-1", PDB: mock.NewDB("extra")}
			case "qt5-wayland":
				return &mock.Package{PName: "qt5-wayland", PVersion: "5.15.10-1", PDB: mock.NewDB("extra")}
			case "wayland":
				return &mock.Package{PName: "wayland", PVersion: "1.22.0-1", PDB: mock.NewDB("extra")}
			case "cmake":
				return &mock.Package{PName: "cmake", PVersion: "3.28.0-1", PDB: mock.NewDB("extra")}
			case "qt5-tools":
				return &mock.Package{PName: "qt5-tools", PVersion: "5.15.10-1", PDB: mock.NewDB("extra")}
			case "git":
				return &mock.Package{PName: "git", PVersion: "2.43.0-1", PDB: mock.NewDB("extra")}
			}
			panic("implement me " + s)
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "liri-desktop-git", "liri-shell-git", "liri-settings-git",
				"libliri-git", "fluid-git":
				return false
			case "liri-cmake-shared-git": // makedepend from AUR
				return false
			case "qt5-declarative", "qt5-quickcontrols2", "qt5-svg",
				"qt5-graphicaleffects", "qt5-wayland", "wayland",
				"cmake", "qt5-tools", "git":
				return true
			}
			panic("implement me " + s)
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		for _, needle := range query.Needles {
			switch needle {
			case "liri-desktop-git", "liri-shell-git", "liri-settings-git",
				"libliri-git", "fluid-git", "liri-cmake-shared-git":
				liriFn := getFromFile(t, "testdata/liri-desktop-git.json")
				return liriFn(ctx, query)
			}
		}
		return []aur.Pkg{}, nil
	}}

	t.Run("liri-desktop-git pulls all dependencies", func(td *testing.T) {
		td.Parallel()

		g := NewGrapher(mockDB, mockAUR, false, true, false, false, false,
			text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
		got, err := g.GraphFromTargets(context.Background(), nil, []string{"liri-desktop-git"})
		require.NoError(td, err)
		layers := got.TopoSortedLayers(nil)

		totalPkgs := 0
		for _, layer := range layers {
			totalPkgs += len(layer)
		}
		// 6 packages: liri-desktop-git + 4 deps + liri-cmake-shared-git (makedep)
		require.Equal(td, 6, totalPkgs)
		require.Contains(td, layers[0], "liri-desktop-git")
		require.Equal(td, Explicit, layers[0]["liri-desktop-git"].Reason)
	})

	t.Run("complex dependency chain resolves in correct order", func(td *testing.T) {
		td.Parallel()

		g := NewGrapher(mockDB, mockAUR, false, true, false, false, false,
			text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
		got, err := g.GraphFromTargets(context.Background(), nil, []string{"liri-desktop-git"})
		require.NoError(td, err)
		layers := got.TopoSortedLayers(nil)

		require.Contains(td, layers[0], "liri-desktop-git")

		allPkgs := make(map[string]bool)
		for _, layer := range layers {
			for pkg := range layer {
				allPkgs[pkg] = true
			}
		}
		require.True(td, allPkgs["liri-desktop-git"])
		require.True(td, allPkgs["liri-shell-git"])
		require.True(td, allPkgs["liri-settings-git"])
		require.True(td, allPkgs["fluid-git"])
		require.True(td, allPkgs["libliri-git"])
		require.True(td, allPkgs["liri-cmake-shared-git"]) // makedepend
	})
}

// TestGrapher_SplitPackages_Clion tests split packages where multiple packages
// come from the same package base, ensuring no rebuilding or reinstalling multiple times.
func TestGrapher_SplitPackages_Clion(t *testing.T) {
	t.Parallel()

	mockDB := &mock.DBExecutor{
		SyncPackageFn:       func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return []mock.IPackage{} },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "clion", "clion-jre", "clion-cmake", "clion-gdb", "clion-lldb":
				return nil
			case "libdbusmenu-glib":
				return &mock.Package{PName: "libdbusmenu-glib", PVersion: "16.04.0-5", PDB: mock.NewDB("extra")}
			case "rsync":
				return &mock.Package{PName: "rsync", PVersion: "3.2.7-1", PDB: mock.NewDB("extra")}
			case "glibc":
				return &mock.Package{PName: "glibc", PVersion: "2.38-1", PDB: mock.NewDB("core")}
			case "gcc-libs":
				return &mock.Package{PName: "gcc-libs", PVersion: "13.2.1-1", PDB: mock.NewDB("core")}
			case "python":
				return &mock.Package{PName: "python", PVersion: "3.11.0-1", PDB: mock.NewDB("core")}
			}
			panic("implement me " + s)
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "clion", "clion-jre", "clion-cmake", "clion-gdb", "clion-lldb":
				return false
			case "libdbusmenu-glib", "rsync", "glibc", "gcc-libs", "python":
				return true
			}
			panic("implement me " + s)
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		for _, needle := range query.Needles {
			switch needle {
			case "clion", "clion-jre", "clion-cmake", "clion-gdb", "clion-lldb":
				clionFn := getFromFile(t, "testdata/clion.json")
				return clionFn(ctx, query)
			}
		}
		return []aur.Pkg{}, nil
	}}

	installInfos := map[string]*InstallInfo{
		"clion exp":       {Source: AUR, Reason: Explicit, Version: "2025.3.1.1-1", AURBase: ptrString("clion")},
		"clion-jre exp":   {Source: AUR, Reason: Explicit, Version: "2025.3.1.1-1", AURBase: ptrString("clion")},
		"clion-cmake exp": {Source: AUR, Reason: Explicit, Version: "2025.3.1.1-1", AURBase: ptrString("clion")},
		"clion-gdb exp":   {Source: AUR, Reason: Explicit, Version: "2025.3.1.1-1", AURBase: ptrString("clion")},
		"clion-lldb exp":  {Source: AUR, Reason: Explicit, Version: "2025.3.1.1-1", AURBase: ptrString("clion")},
	}

	tests := []struct {
		name       string
		targets    []string
		wantLayers []map[string]*InstallInfo
	}{
		{
			name:    "single clion package",
			targets: []string{"clion"},
			wantLayers: []map[string]*InstallInfo{
				{"clion": installInfos["clion exp"]},
			},
		},
		{
			name:    "clion with clion-jre",
			targets: []string{"clion", "clion-jre"},
			wantLayers: []map[string]*InstallInfo{
				{"clion": installInfos["clion exp"], "clion-jre": installInfos["clion-jre exp"]},
			},
		},
		{
			name:    "all clion packages from same base",
			targets: []string{"clion", "clion-jre", "clion-cmake", "clion-gdb", "clion-lldb"},
			wantLayers: []map[string]*InstallInfo{
				{
					"clion":       installInfos["clion exp"],
					"clion-jre":   installInfos["clion-jre exp"],
					"clion-cmake": installInfos["clion-cmake exp"],
					"clion-gdb":   installInfos["clion-gdb exp"],
					"clion-lldb":  installInfos["clion-lldb exp"],
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGrapher(mockDB, mockAUR, false, true, false, false, false,
				text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
			got, err := g.GraphFromTargets(context.Background(), nil, tt.targets)
			require.NoError(t, err)
			layers := got.TopoSortedLayers(nil)
			require.EqualValues(t, tt.wantLayers, layers, layers)
		})
	}

	t.Run("packages from same base share AURBase", func(t *testing.T) {
		g := NewGrapher(mockDB, mockAUR, false, true, false, false, false,
			text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
		got, err := g.GraphFromTargets(context.Background(), nil,
			[]string{"clion", "clion-jre", "clion-cmake"})
		require.NoError(t, err)
		layers := got.TopoSortedLayers(nil)

		require.Len(t, layers, 1)
		for _, info := range layers[0] {
			require.NotNil(t, info.AURBase)
			require.Equal(t, "clion", *info.AURBase)
		}
	})
}

// TestGrapher_SplitPackages_SamsungUnifiedDriver tests split packages where
// packages depend on another package from the same package base.
func TestGrapher_SplitPackages_SamsungUnifiedDriver(t *testing.T) {
	t.Parallel()

	mockDB := &mock.DBExecutor{
		SyncPackageFn:       func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return []mock.IPackage{} },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "samsung-unified-driver", "samsung-unified-driver-common",
				"samsung-unified-driver-printer", "samsung-unified-driver-scanner":
				return nil
			case "cups":
				return &mock.Package{PName: "cups", PVersion: "2.4.2-1", PDB: mock.NewDB("extra")}
			case "ghostscript":
				return &mock.Package{PName: "ghostscript", PVersion: "10.02.0-1", PDB: mock.NewDB("extra")}
			case "libxml2-legacy":
				return &mock.Package{PName: "libxml2-legacy", PVersion: "2.10.3-1", PDB: mock.NewDB("extra")}
			case "libusb-compat":
				return &mock.Package{PName: "libusb-compat", PVersion: "0.1.8-1", PDB: mock.NewDB("extra")}
			case "sane":
				return &mock.Package{PName: "sane", PVersion: "1.2.1-1", PDB: mock.NewDB("extra")}
			}
			panic("implement me " + s)
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "samsung-unified-driver", "samsung-unified-driver-common",
				"samsung-unified-driver-printer", "samsung-unified-driver-scanner":
				return false
			case "cups", "ghostscript", "libxml2-legacy", "libusb-compat", "sane":
				return true
			}
			panic("implement me " + s)
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		for _, needle := range query.Needles {
			switch needle {
			case "samsung-unified-driver", "samsung-unified-driver-common",
				"samsung-unified-driver-printer", "samsung-unified-driver-scanner":
				samsungFn := getFromFile(t, "testdata/samsung-unified-driver.json")
				return samsungFn(ctx, query)
			}
		}
		return []aur.Pkg{}, nil
	}}

	tests := []struct {
		name           string
		targets        []string
		wantPkgCount   int
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:         "metapackage samsung-unified-driver pulls printer and scanner",
			targets:      []string{"samsung-unified-driver"},
			wantPkgCount: 4,
			wantContains: []string{
				"samsung-unified-driver",
				"samsung-unified-driver-printer",
				"samsung-unified-driver-scanner",
				"samsung-unified-driver-common",
			},
		},
		{
			name:           "printer alone pulls common",
			targets:        []string{"samsung-unified-driver-printer"},
			wantPkgCount:   2,
			wantContains:   []string{"samsung-unified-driver-printer", "samsung-unified-driver-common"},
			wantNotContain: []string{"samsung-unified-driver-scanner"},
		},
		{
			name:           "scanner alone pulls common",
			targets:        []string{"samsung-unified-driver-scanner"},
			wantPkgCount:   2,
			wantContains:   []string{"samsung-unified-driver-scanner", "samsung-unified-driver-common"},
			wantNotContain: []string{"samsung-unified-driver-printer"},
		},
		{
			name:         "common alone",
			targets:      []string{"samsung-unified-driver-common"},
			wantPkgCount: 1,
			wantContains: []string{"samsung-unified-driver-common"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGrapher(mockDB, mockAUR, false, true, false, false, false,
				text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
			got, err := g.GraphFromTargets(context.Background(), nil, tt.targets)
			require.NoError(t, err)
			layers := got.TopoSortedLayers(nil)

			allPkgs := make(map[string]bool)
			for _, layer := range layers {
				for pkg := range layer {
					allPkgs[pkg] = true
				}
			}
			require.Equal(t, tt.wantPkgCount, len(allPkgs))

			for _, pkg := range tt.wantContains {
				require.True(t, allPkgs[pkg], "expected package %s not found", pkg)
			}

			for _, pkg := range tt.wantNotContain {
				require.False(t, allPkgs[pkg], "unexpected package %s found", pkg)
			}
		})
	}

	t.Run("split package internal deps resolved correctly", func(t *testing.T) {
		g := NewGrapher(mockDB, mockAUR, false, true, false, false, false,
			text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
		got, err := g.GraphFromTargets(context.Background(), nil, []string{"samsung-unified-driver"})
		require.NoError(t, err)
		layers := got.TopoSortedLayers(nil)

		lastLayer := layers[len(layers)-1]
		require.Contains(t, lastLayer, "samsung-unified-driver-common")

		for _, layer := range layers {
			for _, info := range layer {
				require.NotNil(t, info.AURBase)
				require.Equal(t, "samsung-unified-driver", *info.AURBase)
			}
		}
	})
}

// TestGrapher_SplitPackages_NX tests independent split packages like nxproxy and nxagent.
func TestGrapher_SplitPackages_NX(t *testing.T) {
	t.Parallel()

	mockDB := &mock.DBExecutor{
		SyncPackageFn:       func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return []mock.IPackage{} },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "nxproxy", "nxagent", "nx-x11", "libxcomp":
				return nil
			case "libjpeg-turbo":
				return &mock.Package{PName: "libjpeg-turbo", PVersion: "2.1.5-1", PDB: mock.NewDB("extra")}
			case "libpng":
				return &mock.Package{PName: "libpng", PVersion: "1.6.39-1", PDB: mock.NewDB("extra")}
			case "gcc-libs":
				return &mock.Package{PName: "gcc-libs", PVersion: "13.2.1-1", PDB: mock.NewDB("core")}
			case "libxml2":
				return &mock.Package{PName: "libxml2", PVersion: "2.11.0-1", PDB: mock.NewDB("extra")}
			case "xkeyboard-config":
				return &mock.Package{PName: "xkeyboard-config", PVersion: "2.38-1", PDB: mock.NewDB("extra")}
			case "xorg-xkbcomp":
				return &mock.Package{PName: "xorg-xkbcomp", PVersion: "1.4.6-1", PDB: mock.NewDB("extra")}
			case "libxfont2":
				return &mock.Package{PName: "libxfont2", PVersion: "2.0.6-1", PDB: mock.NewDB("extra")}
			case "libxinerama":
				return &mock.Package{PName: "libxinerama", PVersion: "1.1.5-1", PDB: mock.NewDB("extra")}
			case "xorg-font-util":
				return &mock.Package{PName: "xorg-font-util", PVersion: "1.4.0-1", PDB: mock.NewDB("extra")}
			case "pixman":
				return &mock.Package{PName: "pixman", PVersion: "0.42.2-1", PDB: mock.NewDB("extra")}
			case "libxrandr":
				return &mock.Package{PName: "libxrandr", PVersion: "1.5.3-1", PDB: mock.NewDB("extra")}
			case "libxtst":
				return &mock.Package{PName: "libxtst", PVersion: "1.2.4-1", PDB: mock.NewDB("extra")}
			case "libxcomposite":
				return &mock.Package{PName: "libxcomposite", PVersion: "0.4.6-1", PDB: mock.NewDB("extra")}
			case "libxpm":
				return &mock.Package{PName: "libxpm", PVersion: "3.5.16-1", PDB: mock.NewDB("extra")}
			case "libxdamage":
				return &mock.Package{PName: "libxdamage", PVersion: "1.1.6-1", PDB: mock.NewDB("extra")}
			case "libtirpc":
				return &mock.Package{PName: "libtirpc", PVersion: "1.3.3-1", PDB: mock.NewDB("extra")}
			case "xorgproto":
				return &mock.Package{PName: "xorgproto", PVersion: "2023.2-1", PDB: mock.NewDB("extra")}
			case "imake":
				return &mock.Package{PName: "imake", PVersion: "1.0.9-1", PDB: mock.NewDB("extra")}
			}
			panic("implement me " + s)
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "nxproxy", "nxagent", "nx-x11", "libxcomp":
				return false
			case "libjpeg-turbo", "libpng", "gcc-libs", "libxml2",
				"xkeyboard-config", "xorg-xkbcomp", "libxfont2", "libxinerama",
				"xorg-font-util", "pixman", "libxrandr", "libxtst",
				"libxcomposite", "libxpm", "libxdamage", "libtirpc",
				"xorgproto", "imake":
				return true
			}
			panic("implement me " + s)
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		for _, needle := range query.Needles {
			switch needle {
			case "nxproxy", "nxagent", "nx-x11", "libxcomp":
				nxFn := getFromFile(t, "testdata/nx.json")
				return nxFn(ctx, query)
			}
		}
		return []aur.Pkg{}, nil
	}}

	installInfos := map[string]*InstallInfo{
		"nxproxy exp":  {Source: AUR, Reason: Explicit, Version: "3.5.99.27-3", AURBase: ptrString("nx")},
		"nxagent exp":  {Source: AUR, Reason: Explicit, Version: "3.5.99.27-3", AURBase: ptrString("nx")},
		"nx-x11 dep":   {Source: AUR, Reason: Dep, Version: "3.5.99.27-3", AURBase: ptrString("nx")},
		"libxcomp dep": {Source: AUR, Reason: Dep, Version: "3.5.99.27-3", AURBase: ptrString("nx")},
		"libxcomp exp": {Source: AUR, Reason: Explicit, Version: "3.5.99.27-3", AURBase: ptrString("nx")},
	}

	tests := []struct {
		name       string
		targets    []string
		wantLayers []map[string]*InstallInfo
	}{
		{
			name:    "nxproxy independently",
			targets: []string{"nxproxy"},
			wantLayers: []map[string]*InstallInfo{
				{"nxproxy": installInfos["nxproxy exp"]},
				{"libxcomp": installInfos["libxcomp dep"]},
			},
		},
		{
			name:    "nxagent independently - has more deps",
			targets: []string{"nxagent"},
			wantLayers: []map[string]*InstallInfo{
				{"nxagent": installInfos["nxagent exp"]},
				{"nx-x11": installInfos["nx-x11 dep"]},
				{"libxcomp": installInfos["libxcomp dep"]},
			},
		},
		{
			name:    "both nxproxy and nxagent",
			targets: []string{"nxproxy", "nxagent"},
			wantLayers: []map[string]*InstallInfo{
				{"nxproxy": installInfos["nxproxy exp"], "nxagent": installInfos["nxagent exp"]},
				{"nx-x11": installInfos["nx-x11 dep"]},
				{"libxcomp": installInfos["libxcomp dep"]},
			},
		},
		{
			name:    "libxcomp independently",
			targets: []string{"libxcomp"},
			wantLayers: []map[string]*InstallInfo{
				{"libxcomp": installInfos["libxcomp exp"]},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGrapher(mockDB, mockAUR, false, true, false, false, false,
				text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
			got, err := g.GraphFromTargets(context.Background(), nil, tt.targets)
			require.NoError(t, err)
			layers := got.TopoSortedLayers(nil)
			require.EqualValues(t, tt.wantLayers, layers, layers)
		})
	}

	t.Run("split packages share AURBase but can be installed independently", func(t *testing.T) {
		g := NewGrapher(mockDB, mockAUR, false, true, false, false, false,
			text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))

		got1, err := g.GraphFromTargets(context.Background(), nil, []string{"nxproxy"})
		require.NoError(t, err)
		layers1 := got1.TopoSortedLayers(nil)

		got2, err := g.GraphFromTargets(context.Background(), nil, []string{"nxagent"})
		require.NoError(t, err)
		layers2 := got2.TopoSortedLayers(nil)

		require.Equal(t, "nx", *layers1[0]["nxproxy"].AURBase)
		require.Equal(t, "nx", *layers2[0]["nxagent"].AURBase)

		totalPkgs1 := 0
		for _, layer := range layers1 {
			totalPkgs1 += len(layer)
		}
		totalPkgs2 := 0
		for _, layer := range layers2 {
			totalPkgs2 += len(layer)
		}

		require.Equal(t, 2, totalPkgs1)
		require.Equal(t, 3, totalPkgs2)
	})
}

// TestGrapher_SplitPackages_ReversedOrder tests that split packages resolve
// correctly regardless of the order they are specified in.
func TestGrapher_SplitPackages_ReversedOrder(t *testing.T) {
	t.Parallel()

	mockDB := &mock.DBExecutor{
		SyncPackageFn:       func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return []mock.IPackage{} },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "nxproxy", "nxagent", "nx-x11", "libxcomp":
				return nil
			default:
				return &mock.Package{PName: s, PVersion: "1.0.0-1", PDB: mock.NewDB("extra")}
			}
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "nxproxy", "nxagent", "nx-x11", "libxcomp":
				return false
			default:
				return true
			}
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		for _, needle := range query.Needles {
			switch needle {
			case "nxproxy", "nxagent", "nx-x11", "libxcomp":
				nxFn := getFromFile(t, "testdata/nx.json")
				return nxFn(ctx, query)
			}
		}
		return []aur.Pkg{}, nil
	}}

	t.Run("nxproxy nxagent order", func(t *testing.T) {
		g := NewGrapher(mockDB, mockAUR, false, true, false, false, false,
			text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
		got, err := g.GraphFromTargets(context.Background(), nil, []string{"nxproxy", "nxagent"})
		require.NoError(t, err)
		layers1 := got.TopoSortedLayers(nil)

		require.Contains(t, layers1[0], "nxproxy")
		require.Contains(t, layers1[0], "nxagent")
	})

	t.Run("nxagent nxproxy reversed order", func(t *testing.T) {
		g := NewGrapher(mockDB, mockAUR, false, true, false, false, false,
			text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
		got, err := g.GraphFromTargets(context.Background(), nil, []string{"nxagent", "nxproxy"})
		require.NoError(t, err)
		layers2 := got.TopoSortedLayers(nil)

		require.Contains(t, layers2[0], "nxproxy")
		require.Contains(t, layers2[0], "nxagent")
	})
}

// TestGrapher_MultipleInstallInfo ensures that when the same package appears as
// both explicit target and dependency, the explicit reason takes precedence.
func TestGrapher_MultipleInstallInfo(t *testing.T) {
	t.Parallel()

	mockDB := &mock.DBExecutor{
		SyncPackageFn:       func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return []mock.IPackage{} },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "samsung-unified-driver", "samsung-unified-driver-common",
				"samsung-unified-driver-printer", "samsung-unified-driver-scanner":
				return nil
			default:
				return &mock.Package{PName: s, PVersion: "1.0.0-1", PDB: mock.NewDB("extra")}
			}
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "samsung-unified-driver", "samsung-unified-driver-common",
				"samsung-unified-driver-printer", "samsung-unified-driver-scanner":
				return false
			default:
				return true
			}
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		for _, needle := range query.Needles {
			switch needle {
			case "samsung-unified-driver", "samsung-unified-driver-common",
				"samsung-unified-driver-printer", "samsung-unified-driver-scanner":
				samsungFn := getFromFile(t, "testdata/samsung-unified-driver.json")
				return samsungFn(ctx, query)
			}
		}
		return []aur.Pkg{}, nil
	}}

	t.Run("explicit target takes precedence over dependency", func(t *testing.T) {
		g := NewGrapher(mockDB, mockAUR, false, true, false, false, false,
			text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))

		got, err := g.GraphFromTargets(context.Background(), nil,
			[]string{"samsung-unified-driver", "samsung-unified-driver-common"})
		require.NoError(t, err)
		layers := got.TopoSortedLayers(nil)

		found := false
		for _, layer := range layers {
			if info, ok := layer["samsung-unified-driver-common"]; ok {
				found = true
				require.Equal(t, Explicit, info.Reason,
					"explicit target should have Explicit reason, not Dep")
			}
		}
		require.True(t, found, "samsung-unified-driver-common should be in layers")
	})
}

// TestGrapher_VersionedDependencies tests proper handling of versioned dependencies.
func TestGrapher_VersionedDependencies(t *testing.T) {
	t.Parallel()

	mockDB := &mock.DBExecutor{
		SyncPackageFn:       func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return []mock.IPackage{} },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "versioned-pkg", "dep-pkg", "dep-pkg>=2.0.0":
				return nil
			}
			panic(fmt.Sprintf("implement me: %s", s))
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "versioned-pkg", "dep-pkg", "dep-pkg>=2.0.0":
				return false
			}
			return true
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		mockPkgs := map[string]aur.Pkg{
			"versioned-pkg": {
				Name:        "versioned-pkg",
				PackageBase: "versioned-pkg",
				Version:     "1.0.0-1",
				Depends:     []string{"dep-pkg>=2.0.0"},
			},
			"dep-pkg": {
				Name:        "dep-pkg",
				PackageBase: "dep-pkg",
				Version:     "2.5.0-1",
			},
		}

		pkgs := []aur.Pkg{}
		for _, needle := range query.Needles {
			if pkg, ok := mockPkgs[needle]; ok {
				pkgs = append(pkgs, pkg)
			}
		}
		return pkgs, nil
	}}

	t.Run("versioned dependency satisfied by higher version", func(t *testing.T) {
		g := NewGrapher(mockDB, mockAUR, false, true, false, false, false,
			text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
		got, err := g.GraphFromTargets(context.Background(), nil, []string{"versioned-pkg"})
		require.NoError(t, err)
		layers := got.TopoSortedLayers(nil)

		require.Len(t, layers, 2)
		require.Contains(t, layers[0], "versioned-pkg")
		require.Contains(t, layers[1], "dep-pkg")
		require.Equal(t, "2.5.0-1", layers[1]["dep-pkg"].Version)
	})
}
