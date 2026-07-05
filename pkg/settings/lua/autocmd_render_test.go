package lua

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunRenderAUREventShapeAndStringReturn(t *testing.T) {
	t.Parallel()

	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("RenderAUR", {
			callback = function(event)
				if event.event ~= "RenderAUR" then error("bad event name: " .. tostring(event.event)) end
				local d = event.data
				if d.name ~= "pkgA" then error("bad name") end
				if d.version ~= "1.0-1" then error("bad version") end
				if d.description ~= "A great package" then error("bad description") end
				if d.base ~= "pkgA-base" then error("bad base") end
				if d.votes ~= 42 then error("bad votes") end
				if math.abs(d.popularity - 3.14) > 0.001 then error("bad popularity") end
				if d.maintainer ~= "alice" then error("bad maintainer") end
				if d.out_of_date ~= 0 then error("bad out_of_date") end
				if d.first_submitted ~= 1000 then error("bad first_submitted") end
				if d.last_modified ~= 2000 then error("bad last_modified") end
				if d.local_version ~= "" then error("bad local_version") end
				return "X:" .. d.name
			end,
		})
	`))

	rendered, ok, err := e.RunRenderAUR(&RenderAUREvent{
		Name:           "pkgA",
		Version:        "1.0-1",
		Description:    "A great package",
		Base:           "pkgA-base",
		Votes:          42,
		Popularity:     3.14,
		Maintainer:     "alice",
		OutOfDate:      0,
		FirstSubmitted: 1000,
		LastModified:   2000,
		LocalVersion:   "",
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "X:pkgA", rendered)
}

func TestRunRenderAURNilReturnFallsBack(t *testing.T) {
	t.Parallel()

	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("RenderAUR", {
			callback = function(event)
				return nil
			end,
		})
	`))

	rendered, ok, err := e.RunRenderAUR(&RenderAUREvent{Name: "pkgA"})
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, "", rendered)
}

func TestRunRenderAURNonStringReturnIsError(t *testing.T) {
	t.Parallel()

	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("RenderAUR", {
			callback = function(event)
				return 42
			end,
		})
	`))

	_, ok, err := e.RunRenderAUR(&RenderAUREvent{Name: "pkgA"})
	require.Error(t, err)
	require.False(t, ok)
	require.Contains(t, err.Error(), "callback must return a string or nil")
}

func TestRunRenderAURLastHookWins(t *testing.T) {
	t.Parallel()

	// Sub-case 1: second hook returns "B" — last non-nil wins.
	t.Run("second_returns_B", func(t *testing.T) {
		t.Parallel()

		e := New()
		defer e.Close()

		require.NoError(t, e.L.DoString(`
			yay.create_autocmd("RenderAUR", {
				callback = function(event) return "A" end,
			})
			yay.create_autocmd("RenderAUR", {
				callback = function(event) return "B" end,
			})
		`))

		rendered, ok, err := e.RunRenderAUR(&RenderAUREvent{Name: "pkgA"})
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "B", rendered)
	})

	// Sub-case 2: second hook returns nil — first hook's "A" is retained.
	t.Run("second_returns_nil_keeps_first", func(t *testing.T) {
		t.Parallel()

		e := New()
		defer e.Close()

		require.NoError(t, e.L.DoString(`
			yay.create_autocmd("RenderAUR", {
				callback = function(event) return "A" end,
			})
			yay.create_autocmd("RenderAUR", {
				callback = function(event) return nil end,
			})
		`))

		rendered, ok, err := e.RunRenderAUR(&RenderAUREvent{Name: "pkgA"})
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "A", rendered)
	})
}

func TestRunRenderAURAbort(t *testing.T) {
	t.Parallel()

	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("RenderAUR", {
			callback = function(event)
				yay.abort("blocked")
			end,
		})
	`))

	_, _, err := e.RunRenderAUR(&RenderAUREvent{Name: "pkgA"})
	require.EqualError(t, err, "RenderAUR: blocked")
}

func TestRunRenderSyncEventShapeAndStringReturn(t *testing.T) {
	t.Parallel()

	e := New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("RenderSync", {
			callback = function(event)
				if event.event ~= "RenderSync" then error("bad event name") end
				local d = event.data
				if d.repository ~= "extra" then error("bad repository") end
				if d.name ~= "mypkg" then error("bad name") end
				if d.version ~= "2.0-1" then error("bad version") end
				if d.groups[1] ~= "base" then error("bad groups[1]") end
				if d.local_version ~= "1.9-2" then error("bad local_version") end
				return "SYNC:" .. d.name .. "/" .. d.repository
			end,
		})
	`))

	rendered, ok, err := e.RunRenderSync(&RenderSyncEvent{
		Repository:    "extra",
		Name:          "mypkg",
		Description:   "My sync package",
		Version:       "2.0-1",
		Groups:        []string{"base"},
		LocalVersion:  "1.9-2",
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "SYNC:mypkg/extra", rendered)
}

func TestRunRenderNoHookReturnsFalse(t *testing.T) {
	t.Parallel()

	t.Run("aur", func(t *testing.T) {
		t.Parallel()

		e := New()
		defer e.Close()

		rendered, ok, err := e.RunRenderAUR(&RenderAUREvent{Name: "pkgA"})
		require.NoError(t, err)
		require.False(t, ok)
		require.Equal(t, "", rendered)
	})

	t.Run("sync", func(t *testing.T) {
		t.Parallel()

		e := New()
		defer e.Close()

		rendered, ok, err := e.RunRenderSync(&RenderSyncEvent{Name: "mypkg"})
		require.NoError(t, err)
		require.False(t, ok)
		_ = strings.Contains(rendered, "") // use rendered
	})
}
