package lua

import (
	"testing"

	"github.com/stretchr/testify/require"
	glua "github.com/yuin/gopher-lua"
)

func TestCreateAutocmdRegistersAndRunsInOrder(t *testing.T) {
	t.Parallel()
	e := New()
	t.Cleanup(e.Close)

	order := []string{}
	e.L.SetGlobal("record", e.L.NewFunction(func(L *glua.LState) int {
		order = append(order, L.CheckString(1))
		return 0
	}))

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("AURPreInstall", {
			desc = "first",
			callback = function(event)
				record("first:" .. event.match .. ":" .. event.data.packages[1].name .. ":" .. event.data.srcinfo.pkgbase)
			end,
		})
		yay.create_autocmd("AURPreInstall", {
			callback = function()
				record("second")
			end,
		})
	`))

	autocmds := e.autocmds[EventAURPreInstall]
	require.Len(t, autocmds, 2)
	require.Equal(t, "first", autocmds[0].Desc)

	err := e.RunAURPreInstall(&AURPreInstallEvent{
		Base: "demo-base",
		Packages: []AURPreInstallPackage{{
			Name: "demo",
		}},
		SRCINFO: AURPreInstallSRCINFO{
			Pkgbase: "demo-base",
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"first:demo-base:demo:demo-base", "second"}, order)
}

func TestCreateAutocmdRegistersUpgradeSelect(t *testing.T) {
	t.Parallel()
	e := New()
	t.Cleanup(e.Close)

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("UpgradeSelect", {
			desc = "filter upgrades",
			callback = function() end,
		})
	`))

	autocmds := e.autocmds[EventUpgradeSelect]
	require.Len(t, autocmds, 1)
	require.Equal(t, "filter upgrades", autocmds[0].Desc)
	require.True(t, e.HasAutocmd(EventUpgradeSelect))
}

func TestCreateAutocmdRegistersAURPostDownload(t *testing.T) {
	t.Parallel()
	e := New()
	t.Cleanup(e.Close)

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("AURPostDownload", {
			desc = "inspect downloaded sources",
			callback = function() end,
		})
	`))

	autocmds := e.autocmds[EventAURPostDownload]
	require.Len(t, autocmds, 1)
	require.Equal(t, "inspect downloaded sources", autocmds[0].Desc)
	require.True(t, e.HasAutocmd(EventAURPostDownload))
}

func TestCreateAutocmdRejectsInvalidEvent(t *testing.T) {
	t.Parallel()
	e := New()
	t.Cleanup(e.Close)

	err := e.L.DoString(`
		yay.create_autocmd("User", {
			callback = function() end,
		})
	`)
	require.Error(t, err)
	require.Contains(t, err.Error(), `unsupported event "User"`)
}

func TestCreateAutocmdRejectsMissingCallback(t *testing.T) {
	t.Parallel()
	e := New()
	t.Cleanup(e.Close)

	err := e.L.DoString(`
		yay.create_autocmd("AURPreInstall", {
			desc = "missing callback",
		})
	`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "callback must be a function")
}

func TestRunAURPreInstallReturnsCallbackErrorWithEventAndBase(t *testing.T) {
	t.Parallel()
	e := New()
	t.Cleanup(e.Close)

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("AURPreInstall", {
			callback = function()
				error("blocked by policy")
			end,
		})
	`))

	err := e.RunAURPreInstall(&AURPreInstallEvent{Base: "demo-base"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "AURPreInstall demo-base")
	require.Contains(t, err.Error(), "blocked by policy")
}

func TestRunAURPreInstallReturnsAbortWithoutTraceback(t *testing.T) {
	t.Parallel()
	e := New()
	t.Cleanup(e.Close)

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("AURPreInstall", {
			callback = function()
				yay.abort("blocked by policy")
			end,
		})
	`))

	err := e.RunAURPreInstall(&AURPreInstallEvent{Base: "demo-base"})
	require.EqualError(t, err, "AURPreInstall demo-base: blocked by policy")
}

func TestRunAURPostDownloadEventTableShape(t *testing.T) {
	t.Parallel()
	e := New()
	t.Cleanup(e.Close)

	seen := []string{}
	e.L.SetGlobal("record", e.L.NewFunction(func(L *glua.LState) int {
		seen = append(seen, L.CheckString(1))
		return 0
	}))

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("AURPostDownload", {
			callback = function(event)
				if event.event ~= "AURPostDownload" then error("bad event") end
				if event.match ~= "demo-base" then error("bad match") end
				if event.data.base ~= "demo-base" then error("bad base") end
				if event.data.dir ~= "/build/demo-base" then error("bad dir") end
				if event.data.pkgbuild_path ~= "/build/demo-base/PKGBUILD" then error("bad pkgbuild path") end
				if event.data.srcinfo_path ~= "/build/demo-base/.SRCINFO" then error("bad srcinfo path") end
				if event.data.pkgbuild ~= "pkgbase=demo-base" then error("bad pkgbuild") end
				if event.data.version ~= "1.0-1" then error("bad version") end
				if event.data.last_modified ~= 123 then error("bad last modified") end
				if event.data.installed ~= true then error("bad installed") end
				if event.data.packages[1].name ~= "demo" then error("bad package") end
				if event.data.srcinfo.pkgbase ~= "demo-base" then error("bad srcinfo") end
				if event.data.install_paths ~= nil then error("unexpected install paths") end
				if event.data.source_paths ~= nil then error("unexpected source paths") end
				if event.data.sources ~= nil then error("unexpected sources") end

				record(event.match .. ":" .. event.data.pkgbuild_path)
			end,
		})
	`))

	err := e.RunAURPostDownload(&AURPreInstallEvent{
		Base:         "demo-base",
		Dir:          "/build/demo-base",
		PKGBUILDPath: "/build/demo-base/PKGBUILD",
		SRCINFOPath:  "/build/demo-base/.SRCINFO",
		PKGBUILD:     "pkgbase=demo-base",
		Version:      "1.0-1",
		LastModified: 123,
		Installed:    true,
		Packages: []AURPreInstallPackage{{
			Name: "demo",
		}},
		SRCINFO: AURPreInstallSRCINFO{
			Pkgbase: "demo-base",
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"demo-base:/build/demo-base/PKGBUILD"}, seen)
}

func TestRunAURPostDownloadReturnsAbortWithoutTraceback(t *testing.T) {
	t.Parallel()
	e := New()
	t.Cleanup(e.Close)

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("AURPostDownload", {
			callback = function()
				yay.abort("blocked by policy")
			end,
		})
	`))

	err := e.RunAURPostDownload(&AURPreInstallEvent{Base: "demo-base"})
	require.EqualError(t, err, "AURPostDownload demo-base: blocked by policy")
}

func TestRunUpgradeSelectEventTableShapeAndReturn(t *testing.T) {
	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("UpgradeSelect", {
			callback = function(event)
				if event.event ~= "UpgradeSelect" then error("bad event") end
				if event.data.upgrades[1].id ~= 2 then error("bad upgrade id") end
				if event.data.upgrades[1].name ~= "linux" then error("bad upgrade name") end
				if event.data.upgrades[1].base ~= "linux" then error("bad upgrade base") end
				if event.data.upgrades[1].repository ~= "core" then error("bad upgrade repo") end
				if event.data.upgrades[1].local_version ~= "1.0" then error("bad local version") end
				if event.data.upgrades[1].remote_version ~= "2.0" then error("bad remote version") end
				if event.data.upgrades[1].reason ~= "explicit" then error("bad reason") end
				if event.data.upgrades[1].last_modified ~= 123 then error("bad last modified") end
				if event.data.upgrades[2].id ~= 1 then error("bad second upgrade id") end
				if event.data.pulled_dependencies[1].id ~= 0 then error("bad dependency id") end
				if event.data.pulled_dependencies[1].name ~= "new-dep" then error("bad dependency name") end

				return { exclude = { "linux" }, skip_menu = true }
			end,
		})
	`))

	result, err := e.RunUpgradeSelect(&UpgradeSelectEvent{
		Upgrades: []UpgradeSelectPackage{
			{
				ID:            2,
				Name:          "linux",
				Base:          "linux",
				Repository:    "core",
				LocalVersion:  "1.0",
				RemoteVersion: "2.0",
				Reason:        "explicit",
				LastModified:  123,
			},
			{ID: 1, Name: "yay", Base: "yay", Repository: "aur"},
		},
		PulledDependencies: []UpgradeSelectPackage{
			{ID: 0, Name: "new-dep", Repository: "core", Reason: "dependency"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, UpgradeSelectResult{Exclude: []string{"linux"}, SkipMenu: true}, result)
}

func TestRunUpgradeSelectNilReturnMeansNoExclusionsAndNoSkip(t *testing.T) {
	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("UpgradeSelect", {
			callback = function() end,
		})
	`))

	result, err := e.RunUpgradeSelect(&UpgradeSelectEvent{
		Upgrades: []UpgradeSelectPackage{{Name: "linux"}},
	})
	require.NoError(t, err)
	require.Empty(t, result.Exclude)
	require.False(t, result.SkipMenu)
}

func TestRunUpgradeSelectMultipleHooksUnionExclusionsAndSkip(t *testing.T) {
	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("UpgradeSelect", {
			callback = function()
				return { exclude = { "pkg-a" } }
			end,
		})
		yay.create_autocmd("UpgradeSelect", {
			callback = function()
				return { exclude = { "pkg-b", "pkg-a" }, skip_menu = true }
			end,
		})
	`))

	result, err := e.RunUpgradeSelect(&UpgradeSelectEvent{
		Upgrades: []UpgradeSelectPackage{{Name: "pkg-a"}, {Name: "pkg-b"}},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"pkg-a", "pkg-b"}, result.Exclude)
	require.True(t, result.SkipMenu)
}

func TestRunUpgradeSelectRejectsUnknownExcludedPackage(t *testing.T) {
	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("UpgradeSelect", {
			callback = function()
				return { exclude = { "typo" } }
			end,
		})
	`))

	_, err := e.RunUpgradeSelect(&UpgradeSelectEvent{
		Upgrades: []UpgradeSelectPackage{{Name: "pkg-a"}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "UpgradeSelect")
	require.Contains(t, err.Error(), `unknown upgrade exclusion "typo"`)
}

func TestRunUpgradeSelectReturnsAbortWithoutTraceback(t *testing.T) {
	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("UpgradeSelect", {
			callback = function()
				yay.abort("blocked by policy")
			end,
		})
	`))

	_, err := e.RunUpgradeSelect(&UpgradeSelectEvent{
		Upgrades: []UpgradeSelectPackage{{Name: "pkg-a"}},
	})
	require.EqualError(t, err, "UpgradeSelect: blocked by policy")
}
