//go:build !integration
// +build !integration

package dep

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/Jguer/aur"
	alpm "github.com/Jguer/dyalpm"
	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/db/mock"
	mockaur "github.com/Jguer/yay/v12/pkg/dep/mock"
	"github.com/Jguer/yay/v12/pkg/dep/topo"
	"github.com/Jguer/yay/v12/pkg/text"
)

func TestDepSplitDep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantName   string
		wantMod    string
		wantDepVer string
	}{
		{name: "plain", input: "base", wantName: "base"},
		{name: "greater", input: "base>=1.0", wantName: "base", wantMod: ">=", wantDepVer: "1.0"},
		{name: "less", input: "base<2.0", wantName: "base", wantMod: "<", wantDepVer: "2.0"},
		{name: "equal", input: "base=1", wantName: "base", wantMod: "=", wantDepVer: "1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			name, mod, depVer := splitDep(tc.input)
			require.Equal(t, tc.wantName, name)
			require.Equal(t, tc.wantMod, mod)
			require.Equal(t, tc.wantDepVer, depVer)
		})
	}
}

func TestDepSatisfies(t *testing.T) {
	t.Parallel()

	require.True(t, pkgSatisfies("linux", "1.0", "linux"))
	require.False(t, pkgSatisfies("linux", "1.0", "linux>=1.5"))
	require.True(t, pkgSatisfies("linux", "1.7", "linux>=1.5"))
	require.True(t, verSatisfies("1.5", "<=", "1.5"))
	require.False(t, verSatisfies("1.4", ">=", "1.5"))
}

func TestProvideSatisfies(t *testing.T) {
	t.Parallel()

	require.True(t, provideSatisfies("dep=2.0", "dep>=1.0", "1.0"))
	require.True(t, provideSatisfies("dep", "dep>=1.0", "2.0"))
	require.False(t, provideSatisfies("dep", "dep>=3.0", "2.0"))
	require.False(t, provideSatisfies("other=2.0", "dep>=1.0", "2.0"))
}

func TestSatisfiesAurProvides(t *testing.T) {
	t.Parallel()

	pkg := &aur.Pkg{
		Name:    "pkgbase",
		Version: "10",
		Provides: []string{
			"pkgbase",
			"oldpkg=1.2",
		},
	}

	require.True(t, satisfiesAur("pkgbase>=10", pkg))
	require.True(t, satisfiesAur("oldpkg=1.2", pkg))
	require.False(t, satisfiesAur("oldpkg>=2", pkg))
}

func TestTargetParsing(t *testing.T) {
	t.Parallel()

	target := ToTarget("core/libpng>=1.6")
	require.Equal(t, "core", target.DB)
	require.Equal(t, "libpng", target.Name)
	require.Equal(t, ">=", target.Mod)
	require.Equal(t, "1.6", target.Version)

	target = ToTarget("yay")
	require.Equal(t, "", target.DB)
	require.Equal(t, "yay", target.Name)
	require.Equal(t, "", target.Mod)
	require.Equal(t, "", target.Version)
}

func TestGrapher_GraphSyncPkgAndUpgrade(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")

	mockDb := &mock.DBExecutor{
		LocalPackageFn: func(s string) mock.IPackage {
			return &mock.Package{
				PName:    "yay",
				PVersion: "1.0.0-1",
				PReason:  alpm.PkgReasonDepend,
				PDB:      mock.NewDB("core"),
			}
		},
	}

	grapher := NewGrapher(mockDb, &mockaur.MockAUR{}, false, false, false, false, false, logger)
	graph := grapher.GraphSyncPkg(context.TODO(), nil, &mock.Package{
		PName:    "yay",
		PVersion: "1.2.0",
		PDB:      mock.NewDB("core"),
	}, nil)

	info := graph.GetNodeInfo("yay").Value
	require.NotNil(t, info)
	require.Equal(t, Dep, info.Reason)
	require.False(t, info.Upgrade)

	upgraded := grapher.GraphSyncPkg(context.TODO(), nil, &mock.Package{
		PName:    "yaysrc",
		PVersion: "2.0.0",
		PDB:      mock.NewDB("core"),
	}, &db.SyncUpgrade{
		Package: &mock.Package{
			PName: "yaysrc",
			PDB:   mock.NewDB("core"),
		},
		LocalVersion: "1.0.0",
		Reason:       alpm.PkgReasonExplicit,
	})

	upgradeInfo := upgraded.GetNodeInfo("yaysrc").Value
	require.NotNil(t, upgradeInfo)
	require.True(t, upgradeInfo.Upgrade)
	require.Equal(t, "1.0.0", upgradeInfo.LocalVersion)
}

func TestGrapher_GraphSyncGroupAndValidateNodeInfo(t *testing.T) {
	t.Parallel()

	grapher := NewGrapher(&mock.DBExecutor{}, &mockaur.MockAUR{}, false, false, false, false, false, text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test"))

	graph := grapher.GraphSyncGroup(context.TODO(), nil, "editors", "community")
	groupInfo := graph.GetNodeInfo("editors").Value
	require.NotNil(t, groupInfo)
	require.True(t, groupInfo.IsGroup)
	require.Equal(t, "community", *groupInfo.SyncDBName)

	target := "grouped"
	graph.SetNodeInfo(target, &topo.NodeInfo[*InstallInfo]{Value: &InstallInfo{Reason: Explicit}})
	grapher.ValidateAndSetNodeInfo(graph, target, &topo.NodeInfo[*InstallInfo]{Value: &InstallInfo{Reason: MakeDep}})
	require.Equal(t, Explicit, graph.GetNodeInfo(target).Value.Reason)

	graph.SetNodeInfo(target, &topo.NodeInfo[*InstallInfo]{Value: &InstallInfo{Reason: Explicit, Upgrade: true}})
	grapher.ValidateAndSetNodeInfo(graph, target, &topo.NodeInfo[*InstallInfo]{Value: &InstallInfo{Reason: Explicit, Upgrade: false}})
	require.True(t, graph.GetNodeInfo(target).Value.Upgrade)
}

func TestProvideMenuAndMakeAURPKGFromSrcinfo(t *testing.T) {
	t.Parallel()

	grapher := NewGrapher(&mock.DBExecutor{}, &mockaur.MockAUR{}, false, true, false, false, false, text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test"))
	opts := []aur.Pkg{
		{Name: "aur-pkg-one", Version: "1"},
		{Name: "aur-pkg-two", Version: "2"},
	}

	require.Equal(t, "aur-pkg-one", grapher.provideMenu("dep", opts).Name)

	grapherNoConfirm := NewGrapher(&mock.DBExecutor{}, &mockaur.MockAUR{}, false, false, false, false, false,
		text.NewLogger(io.Discard, io.Discard, strings.NewReader("2\n"), false, "test"))
	require.Equal(t, "aur-pkg-two", grapherNoConfirm.provideMenu("dep", opts).Name)
}

func TestMakeAURPKGFromSrcinfo(t *testing.T) {
	t.Parallel()

	assertErr := errors.New("arch error")

	dbExecutor := &mock.DBExecutor{
		AlpmArchitecturesFn: func() ([]string, error) {
			return []string{"x86_64"}, nil
		},
	}

	srcinfo := &gosrc.Srcinfo{
		PackageBase: gosrc.PackageBase{
			Pkgbase: "yay",
		},
		Package: gosrc.Package{
			Depends: []gosrc.ArchString{
				{Arch: "x86_64", Value: "xdep"},
				{Arch: "arm", Value: "ignored"},
			},
		},
		Packages: []gosrc.Package{
			{
				Pkgname: "yay",
				Depends: []gosrc.ArchString{
					{Value: "pkgdep"},
				},
			},
		},
	}

	pkgs, err := makeAURPKGFromSrcinfo(dbExecutor, srcinfo)
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
	require.Equal(t, []string{"pkgdep", "xdep"}, pkgs[0].Depends)

	dbFail := &mock.DBExecutor{
		AlpmArchitecturesFn: func() ([]string, error) {
			return nil, assertErr
		},
	}

	_, err = makeAURPKGFromSrcinfo(dbFail, srcinfo)
	require.Error(t, err)
}
