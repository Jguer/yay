//go:build !integration

package workdir

import (
	"os"
	"path/filepath"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/require"
	glua "github.com/yuin/gopher-lua"

	"github.com/Jguer/yay/v13/pkg/dep"
	"github.com/Jguer/yay/v13/pkg/runtime"
	settingslua "github.com/Jguer/yay/v13/pkg/settings/lua"
)

func TestAURPostDownloadEventsUseAURPreInstallPayload(t *testing.T) {
	t.Parallel()
	base := "demo-base"
	dir := writeAURPostDownloadPackage(t, base)

	events, err := aurPostDownloadEvents(map[string]string{base: dir},
		mapset.NewThreadUnsafeSet[string](),
		[]map[string]*dep.InstallInfo{
			{
				"demo": {Source: dep.AUR, AURBase: base, Version: "1.0-1"},
			},
		})
	require.NoError(t, err)
	require.Len(t, events, 1)

	event := events[0]
	require.Equal(t, base, event.Base)
	require.Equal(t, dir, event.Dir)
	require.Equal(t, filepath.Join(dir, "PKGBUILD"), event.PKGBUILDPath)
	require.Equal(t, filepath.Join(dir, ".SRCINFO"), event.SRCINFOPath)
	require.Contains(t, event.PKGBUILD, "pkgbase=demo-base")
	require.Equal(t, "1.0-1", event.Version)
	require.Equal(t, base, event.SRCINFO.Pkgbase)
	require.Equal(t, "1.0", event.SRCINFO.Pkgver)
	require.Equal(t, "1", event.SRCINFO.Pkgrel)
	require.Equal(t, "1.0-1", event.SRCINFO.Version)
	require.Equal(t, []string{"any"}, event.SRCINFO.Arch)
	require.Equal(t, []settingslua.AURPreInstallPackage{{Name: "demo", Version: "1.0-1", Reason: "explicit"}}, event.Packages)
}

func TestRunAURPostDownloadLuaHooksRunsBasesInSortedOrder(t *testing.T) {
	t.Parallel()
	firstDir := writeAURPostDownloadPackage(t, "a-base")
	secondDir := writeAURPostDownloadPackage(t, "z-base")

	engine := settingslua.New()
	t.Cleanup(engine.Close)

	order := []string{}
	engine.L.SetGlobal("record", engine.L.NewFunction(func(L *glua.LState) int {
		order = append(order, L.CheckString(1))
		return 0
	}))
	require.NoError(t, engine.L.DoString(`
		yay.create_autocmd("AURPostDownload", {
			callback = function(event)
				record(event.match .. ":" .. event.data.pkgbuild_path)
			end,
		})
	`))

	err := runAURPostDownloadLuaHooks(&runtime.Runtime{Lua: engine},
		map[string]string{"z-base": secondDir, "a-base": firstDir},
		mapset.NewThreadUnsafeSet[string](),
		[]map[string]*dep.InstallInfo{
			{
				"a": {Source: dep.AUR, AURBase: "a-base"},
				"z": {Source: dep.AUR, AURBase: "z-base"},
			},
		})
	require.NoError(t, err)
	require.Equal(t, []string{
		"a-base:" + filepath.Join(firstDir, "PKGBUILD"),
		"z-base:" + filepath.Join(secondDir, "PKGBUILD"),
	}, order)
}

func writeAURPostDownloadPackage(t *testing.T, base string) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(`pkgbase=`+base+`
pkgname=(demo)
pkgver=1.0
pkgrel=1
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(`pkgbase = `+base+`
	pkgver = 1.0
	pkgrel = 1
	arch = any

pkgname = demo
`), 0o600))

	return dir
}
