package lua

import (
	"testing"

	"github.com/stretchr/testify/require"
	glua "github.com/yuin/gopher-lua"
)

func TestCreateAutocmdRegistersAndRunsInOrder(t *testing.T) {
	e := New()
	defer e.Close()

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

func TestCreateAutocmdRejectsInvalidEvent(t *testing.T) {
	e := New()
	defer e.Close()

	err := e.L.DoString(`
		yay.create_autocmd("User", {
			callback = function() end,
		})
	`)
	require.Error(t, err)
	require.Contains(t, err.Error(), `unsupported event "User"`)
}

func TestCreateAutocmdRejectsMissingCallback(t *testing.T) {
	e := New()
	defer e.Close()

	err := e.L.DoString(`
		yay.create_autocmd("AURPreInstall", {
			desc = "missing callback",
		})
	`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "callback must be a function")
}

func TestRunAURPreInstallReturnsCallbackErrorWithEventAndBase(t *testing.T) {
	e := New()
	defer e.Close()

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
	e := New()
	defer e.Close()

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
