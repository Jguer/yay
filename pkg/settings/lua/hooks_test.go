package lua

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderHandledReturnsLine(t *testing.T) {
	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.on("render_search", function(results)
			local out = {}
			for _, pkg in ipairs(results) do
				out[#out + 1] = pkg.source .. "/" .. pkg.name .. " " .. pkg.version
			end
			return table.concat(out, "\n")
		end)
	`))

	assert.True(t, e.HasHooks())

	out, handled, err := e.RenderSearch([]map[string]any{
		{"source": "aur", "name": "yay", "version": "12.0.0"},
		{"source": "extra", "name": "bash", "version": "5.2"},
	})
	require.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, "aur/yay 12.0.0\nextra/bash 5.2", out)
}

func TestRenderNoHookReturnsUnhandled(t *testing.T) {
	e := New()
	defer e.Close()

	assert.False(t, e.HasHooks())

	out, handled, err := e.RenderSearch([]map[string]any{{"name": "yay"}})
	require.NoError(t, err)
	assert.False(t, handled)
	assert.Empty(t, out)
}

func TestRenderNonStringReturnIsUnhandled(t *testing.T) {
	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.on("render_search", function(results)
			return 42
		end)
	`))

	out, handled, err := e.RenderSearch([]map[string]any{{"name": "bash"}})
	require.NoError(t, err)
	assert.False(t, handled)
	assert.Empty(t, out)
}

func TestRenderNilReturnIsUnhandled(t *testing.T) {
	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.on("render_search", function(results)
			return nil
		end)
	`))

	_, handled, err := e.RenderSearch([]map[string]any{{"name": "bash"}})
	require.NoError(t, err)
	assert.False(t, handled)
}

func TestRenderRuntimeErrorWraps(t *testing.T) {
	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.on("render_search", function(results)
			error("boom")
		end)
	`))

	_, handled, err := e.RenderSearch([]map[string]any{{"name": "yay"}})
	require.Error(t, err)
	assert.False(t, handled)
	assert.Contains(t, err.Error(), "init.lua render_search hook")
	assert.Contains(t, err.Error(), "boom")
}

func TestOnUnknownEventErrors(t *testing.T) {
	e := New()
	defer e.Close()

	err := e.L.DoString(`yay.on("bogus", function(pkg) return "x" end)`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown event")
	assert.False(t, e.HasHooks())
}

func TestOnLastWinsSingleSlot(t *testing.T) {
	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.on("render_search", function(results) return "first" end)
		yay.on("render_search", function(results) return "second" end)
	`))

	out, handled, err := e.RenderSearch([]map[string]any{{"name": "yay"}})
	require.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, "second", out)
}

func TestToTableConvertsAllKinds(t *testing.T) {
	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.on("render_search", function(results)
			local pkg = results[1]
			assert(pkg.name == "yay", "name")
			assert(pkg.votes == 10, "votes")
			assert(math.abs(pkg.popularity - 1.5) < 0.001, "popularity")
			assert(pkg.size == 4096, "size")
			assert(pkg.installed == true, "installed")
			assert(pkg.installed_version == nil, "installed_version absent")
			assert(#pkg.provides == 2, "provides len")
			assert(pkg.provides[1] == "a" and pkg.provides[2] == "b", "provides items")
			return "ok"
		end)
	`))

	out, handled, err := e.RenderSearch([]map[string]any{{
		"name":       "yay",
		"votes":      10,
		"popularity": 1.5,
		"size":       int64(4096),
		"installed":  true,
		"provides":   []string{"a", "b"},
	}})
	require.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, "ok", out)
}
