//go:build !integration

package workdir

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/require"
	glua "github.com/yuin/gopher-lua"

	"github.com/Jguer/yay/v13/pkg/dep"
	"github.com/Jguer/yay/v13/pkg/runtime"
	"github.com/Jguer/yay/v13/pkg/settings"
	settingslua "github.com/Jguer/yay/v13/pkg/settings/lua"
)

func TestAURPreInstallEventsFromPackageFiles(t *testing.T) {
	t.Parallel()
	base := "demo-base"
	dir := writeAURPreInstallPackage(t, base)
	targets := []map[string]*dep.InstallInfo{
		{
			"demo-doc": {
				Source:       dep.AUR,
				Reason:       dep.MakeDep,
				Version:      "1:1.2.3-4",
				LocalVersion: "1:1.2.3-3",
				AURBase:      base,
				Upgrade:      true,
				Devel:        true,
				LastModified: 1700000001,
			},
			"demo": {
				Source:       dep.AUR,
				Reason:       dep.Explicit,
				Version:      "1:1.2.3-4",
				AURBase:      base,
				LastModified: 1700000000,
			},
		},
	}

	events, err := aurPreInstallEvents(map[string]string{base: dir},
		mapset.NewThreadUnsafeSet("demo-doc"), targets)
	require.NoError(t, err)
	require.Len(t, events, 1)

	event := events[0]
	require.Equal(t, base, event.Base)
	require.Equal(t, dir, event.Dir)
	require.Equal(t, filepath.Join(dir, "PKGBUILD"), event.PKGBUILDPath)
	require.Equal(t, filepath.Join(dir, ".SRCINFO"), event.SRCINFOPath)
	require.Contains(t, event.PKGBUILD, "pkgbase=demo-base")
	require.Equal(t, "1:1.2.3-4", event.Version)
	require.Equal(t, int64(1700000001), event.LastModified)
	require.True(t, event.Installed)

	require.Len(t, event.Packages, 2)
	require.Equal(t, "demo", event.Packages[0].Name)
	require.Equal(t, "explicit", event.Packages[0].Reason)
	require.False(t, event.Packages[0].Upgrade)
	require.Equal(t, "demo-doc", event.Packages[1].Name)
	require.Equal(t, "make_dependency", event.Packages[1].Reason)
	require.Equal(t, "1:1.2.3-3", event.Packages[1].LocalVersion)
	require.True(t, event.Packages[1].Upgrade)
	require.True(t, event.Packages[1].Devel)

	require.Equal(t, base, event.SRCINFO.Pkgbase)
	require.Equal(t, "1", event.SRCINFO.Epoch)
	require.Equal(t, "1.2.3", event.SRCINFO.Pkgver)
	require.Equal(t, "4", event.SRCINFO.Pkgrel)
	require.Equal(t, "1:1.2.3-4", event.SRCINFO.Version)
	require.Equal(t, "global description", event.SRCINFO.Pkgdesc)
	require.Equal(t, "https://example.test/demo", event.SRCINFO.URL)
	require.Equal(t, []string{"x86_64"}, event.SRCINFO.Arch)
	require.Equal(t, []string{"MIT"}, event.SRCINFO.License)
	require.Equal(t, []string{"glibc", "demo-runtime", "demo-doc-runtime"}, event.SRCINFO.Depends)
	require.Equal(t, []string{"go"}, event.SRCINFO.MakeDepends)
	require.Equal(t, []string{"bats"}, event.SRCINFO.CheckDepends)
	require.Equal(t, []string{"demo-optional: optional support", "demo-doc-viewer"}, event.SRCINFO.OptDepends)
	require.Equal(t, []string{"demo-virtual", "demo-doc-virtual"}, event.SRCINFO.Provides)
	require.Equal(t, []string{"old-demo"}, event.SRCINFO.Conflicts)
	require.Equal(t, []string{"older-demo"}, event.SRCINFO.Replaces)
}

func TestRunAURPreInstallLuaHooksRunsBasesInSortedOrder(t *testing.T) {
	t.Parallel()
	firstDir := writeAURPreInstallPackage(t, "a-base")
	secondDir := writeAURPreInstallPackage(t, "z-base")

	engine := settingslua.New()
	t.Cleanup(engine.Close)

	order := []string{}
	engine.L.SetGlobal("record", engine.L.NewFunction(func(L *glua.LState) int {
		order = append(order, L.CheckString(1))
		return 0
	}))
	require.NoError(t, engine.L.DoString(`
		yay.create_autocmd("AURPreInstall", {
			callback = function(event)
				record(event.match)
			end,
		})
	`))

	err := runAURPreInstallLuaHooks(&runtime.Runtime{Lua: engine},
		map[string]string{"z-base": secondDir, "a-base": firstDir},
		mapset.NewThreadUnsafeSet[string](),
		[]map[string]*dep.InstallInfo{
			{
				"a": {Source: dep.AUR, AURBase: "a-base"},
				"z": {Source: dep.AUR, AURBase: "z-base"},
			},
		})
	require.NoError(t, err)
	require.Equal(t, []string{"a-base", "z-base"}, order)
}

func TestRunPreDownloadSourcesHooksRunsLuaBeforeMenus(t *testing.T) {
	base := "demo-base"
	dir := writeAURPreInstallPackage(t, base)

	engine := settingslua.New()
	defer engine.Close()

	order := []string{}
	engine.L.SetGlobal("record", engine.L.NewFunction(func(L *glua.LState) int {
		order = append(order, L.CheckString(1))
		return 0
	}))
	require.NoError(t, engine.L.DoString(`
		yay.create_autocmd("AURPreInstall", {
			callback = function()
				record("lua")
			end,
		})
	`))

	preper := NewPreparerWithoutHooks(nil, nil, &settings.Configuration{}, newTestLogger(), true)
	preper.hooks = []Hook{
		{Name: "clean", Type: PreDownloadSourcesHook, Hookfn: func(context.Context, *runtime.Runtime, io.Writer, map[string]string, mapset.Set[string]) error {
			order = append(order, "clean")
			return nil
		}},
		{Name: "diff", Type: PreDownloadSourcesHook, Hookfn: func(context.Context, *runtime.Runtime, io.Writer, map[string]string, mapset.Set[string]) error {
			order = append(order, "diff")
			return nil
		}},
		{Name: "edit", Type: PreDownloadSourcesHook, Hookfn: func(context.Context, *runtime.Runtime, io.Writer, map[string]string, mapset.Set[string]) error {
			order = append(order, "edit")
			return nil
		}},
	}

	err := preper.runPreDownloadSourcesHooks(t.Context(), &runtime.Runtime{Lua: engine}, io.Discard,
		map[string]string{base: dir}, mapset.NewThreadUnsafeSet[string](),
		[]map[string]*dep.InstallInfo{
			{
				"demo": {Source: dep.AUR, AURBase: base},
			},
		})
	require.NoError(t, err)
	require.Equal(t, []string{"lua", "clean", "diff", "edit"}, order)
}

func TestRunAURPreInstallLuaHooksReturnsCallbackError(t *testing.T) {
	t.Parallel()
	base := "demo-base"
	dir := writeAURPreInstallPackage(t, base)

	engine := settingslua.New()
	t.Cleanup(engine.Close)

	require.NoError(t, engine.L.DoString(`
		yay.create_autocmd("AURPreInstall", {
			callback = function()
				error("blocked")
			end,
		})
	`))

	err := runAURPreInstallLuaHooks(&runtime.Runtime{Lua: engine},
		map[string]string{base: dir}, mapset.NewThreadUnsafeSet[string](),
		[]map[string]*dep.InstallInfo{
			{
				"demo": {Source: dep.AUR, AURBase: base},
			},
		})
	require.Error(t, err)
	require.Contains(t, err.Error(), "AURPreInstall demo-base")
	require.Contains(t, err.Error(), "blocked")
}

func writeAURPreInstallPackage(t *testing.T, base string) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(`pkgbase=`+base+`
pkgname=(demo demo-doc)
pkgver=1.2.3
pkgrel=4
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(`pkgbase = `+base+`
	pkgdesc = global description
	pkgver = 1.2.3
	pkgrel = 4
	epoch = 1
	url = https://example.test/demo
	arch = x86_64
	license = MIT
	depends = glibc
	makedepends = go
	checkdepends = bats
	optdepends = demo-optional: optional support
	provides = demo-virtual
	conflicts = old-demo
	replaces = older-demo

pkgname = demo
	depends = demo-runtime

pkgname = demo-doc
	depends = demo-doc-runtime
	optdepends = demo-doc-viewer
	provides = demo-doc-virtual
`), 0o600))

	return dir
}
