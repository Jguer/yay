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
		yay.on("search_aur", function(pkg)
			return pkg.source .. "/" .. pkg.name .. " " .. pkg.version
		end)
	`))

	assert.True(t, e.HasHooks())

	line, handled, err := e.Render("search_aur", map[string]any{
		"source": "aur", "name": "yay", "version": "12.0.0",
	})
	require.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, "aur/yay 12.0.0", line)
}

func TestRenderNoHookReturnsUnhandled(t *testing.T) {
	e := New()
	defer e.Close()

	assert.False(t, e.HasHooks())

	line, handled, err := e.Render("search_aur", map[string]any{"name": "yay"})
	require.NoError(t, err)
	assert.False(t, handled)
	assert.Empty(t, line)
}

func TestRenderNonStringReturnIsUnhandled(t *testing.T) {
	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.on("search_repo", function(pkg)
			return 42
		end)
	`))

	line, handled, err := e.Render("search_repo", map[string]any{"name": "bash"})
	require.NoError(t, err)
	assert.False(t, handled)
	assert.Empty(t, line)
}

func TestRenderNilReturnIsUnhandled(t *testing.T) {
	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.on("search_repo", function(pkg)
			return nil
		end)
	`))

	_, handled, err := e.Render("search_repo", map[string]any{"name": "bash"})
	require.NoError(t, err)
	assert.False(t, handled)
}

func TestRenderRuntimeErrorWraps(t *testing.T) {
	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.on("search_aur", function(pkg)
			error("boom")
		end)
	`))

	_, handled, err := e.Render("search_aur", map[string]any{"name": "yay"})
	require.Error(t, err)
	assert.False(t, handled)
	assert.Contains(t, err.Error(), "init.lua search_aur hook")
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
		yay.on("search_aur", function(pkg) return "first" end)
		yay.on("search_aur", function(pkg) return "second" end)
	`))

	line, handled, err := e.Render("search_aur", map[string]any{"name": "yay"})
	require.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, "second", line)
}

func TestToTableConvertsAllKinds(t *testing.T) {
	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.on("search_aur", function(pkg)
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

	line, handled, err := e.Render("search_aur", map[string]any{
		"name":       "yay",
		"votes":      10,
		"popularity": 1.5,
		"size":       int64(4096),
		"installed":  true,
		"provides":   []string{"a", "b"},
	})
	require.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, "ok", line)
}
