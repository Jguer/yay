package lua

import (
	"testing"

	"github.com/stretchr/testify/require"
	glua "github.com/yuin/gopher-lua"
)

func TestRunPostInstallEventTableShape(t *testing.T) {
	t.Parallel()

	e := New()
	defer e.Close()

	ran := false
	e.L.SetGlobal("setRan", e.L.NewFunction(func(_ *glua.LState) int {
		ran = true
		return 0
	}))

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("PostInstall", {
			callback = function(event)
				if event.event ~= "PostInstall" then error("bad event name") end
				if event.data == nil then error("missing data") end
				local pkg = event.data.packages[1]
				if pkg == nil then error("missing package") end
				if pkg.name ~= "mypkg" then error("bad name: " .. tostring(pkg.name)) end
				if pkg.version ~= "1.2.3-1" then error("bad version") end
				if pkg.local_version ~= "1.0.0-1" then error("bad local_version") end
				if pkg.source ~= "aur" then error("bad source") end
				if pkg.reason ~= "explicit" then error("bad reason") end
				if pkg.installed ~= true then error("bad installed") end
				if pkg.upgrade ~= false then error("bad upgrade") end
				if pkg.devel ~= true then error("bad devel") end
				setRan()
			end,
		})
	`))

	err := e.RunPostInstall(&PostInstallEvent{
		Packages: []PostInstallPackage{
			{
				Name:         "mypkg",
				Version:      "1.2.3-1",
				LocalVersion: "1.0.0-1",
				Source:       "aur",
				Reason:       "explicit",
				Installed:    true,
				Upgrade:      false,
				Devel:        true,
			},
		},
	})
	require.NoError(t, err)
	require.True(t, ran)
}

func TestRunPostInstallReturnsAbortWithoutTraceback(t *testing.T) {
	t.Parallel()

	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("PostInstall", {
			callback = function()
				yay.abort("blocked")
			end,
		})
	`))

	err := e.RunPostInstall(&PostInstallEvent{
		Packages: []PostInstallPackage{{Name: "mypkg"}},
	})
	require.EqualError(t, err, "PostInstall: blocked")
}

func TestRunSearchFilterEventTableShapeAndReturn(t *testing.T) {
	t.Parallel()

	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("SearchFilter", {
			callback = function(event)
				if event.event ~= "SearchFilter" then error("bad event name") end
				local r = event.data.results[1]
				if r.source ~= "aur" then error("bad source") end
				if r.name ~= "pkgA" then error("bad name") end
				if r.description ~= "desc A" then error("bad description") end
				if r.base ~= "pkgA" then error("bad base") end
				if r.votes ~= 42 then error("bad votes") end
				if math.abs(r.popularity - 3.14) > 0.001 then error("bad popularity") end
				if r.first_submitted ~= 1000 then error("bad first_submitted") end
				if r.last_modified ~= 2000 then error("bad last_modified") end
				if r.provides[1] ~= "pkgA-compat" then error("bad provides") end

				-- Return reversed order, dropping pkgC
				return {
					{ source = "sync", name = "pkgB" },
					{ source = "aur",  name = "pkgA" },
				}
			end,
		})
	`))

	refs, err := e.RunSearchFilter(&SearchFilterEvent{
		Results: []SearchResultPackage{
			{
				Source:         "aur",
				Name:           "pkgA",
				Description:    "desc A",
				Base:           "pkgA",
				Votes:          42,
				Popularity:     3.14,
				FirstSubmitted: 1000,
				LastModified:   2000,
				Provides:       []string{"pkgA-compat"},
			},
			{Source: "sync", Name: "pkgB"},
			{Source: "aur", Name: "pkgC"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []SearchResultRef{
		{Source: "sync", Name: "pkgB"},
		{Source: "aur", Name: "pkgA"},
	}, refs)
}

func TestRunSearchFilterNilReturnMeansUnchanged(t *testing.T) {
	t.Parallel()

	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("SearchFilter", {
			callback = function(event)
				-- return nothing
			end,
		})
	`))

	refs, err := e.RunSearchFilter(&SearchFilterEvent{
		Results: []SearchResultPackage{
			{Source: "aur", Name: "pkgA"},
			{Source: "sync", Name: "pkgB"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []SearchResultRef{
		{Source: "aur", Name: "pkgA"},
		{Source: "sync", Name: "pkgB"},
	}, refs)
}

func TestRunSearchFilterRejectsUnknownResult(t *testing.T) {
	t.Parallel()

	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("SearchFilter", {
			callback = function()
				return { { source = "x", name = "ghost" } }
			end,
		})
	`))

	_, err := e.RunSearchFilter(&SearchFilterEvent{
		Results: []SearchResultPackage{
			{Source: "aur", Name: "pkgA"},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown search result x/ghost")
}

func TestRunSearchFilterChainsMultipleHooks(t *testing.T) {
	t.Parallel()

	e := New()
	defer e.Close()

	// First hook drops pkgC.
	// Second hook sees only pkgA and pkgB, reorders them.
	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("SearchFilter", {
			callback = function(event)
				-- drop pkgC
				return {
					{ source = "aur",  name = "pkgA" },
					{ source = "sync", name = "pkgB" },
				}
			end,
		})
		yay.create_autocmd("SearchFilter", {
			callback = function(event)
				if #event.data.results ~= 2 then error("expected 2 results, got " .. #event.data.results) end
				-- reorder: B then A
				return {
					{ source = "sync", name = "pkgB" },
					{ source = "aur",  name = "pkgA" },
				}
			end,
		})
	`))

	refs, err := e.RunSearchFilter(&SearchFilterEvent{
		Results: []SearchResultPackage{
			{Source: "aur", Name: "pkgA"},
			{Source: "sync", Name: "pkgB"},
			{Source: "aur", Name: "pkgC"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []SearchResultRef{
		{Source: "sync", Name: "pkgB"},
		{Source: "aur", Name: "pkgA"},
	}, refs)
}
