//go:build !integration

package query

import (
	"io"
	"strings"
	"testing"

	settingslua "github.com/Jguer/yay/v13/pkg/settings/lua"
	"github.com/Jguer/yay/v13/pkg/settings/parser"
	"github.com/Jguer/yay/v13/pkg/text"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSourceQueryBuilderRenderAURHook verifies that a RenderAUR hook replaces
// the default AUR row rendering while sync rows keep their default formatting.
func TestSourceQueryBuilderRenderAURHook(t *testing.T) {
	t.Parallel()

	mockDB, mockAUR := newYayQueryBuilderMocks()

	w := &strings.Builder{}
	logger := text.NewLogger(w, io.Discard, strings.NewReader(""), false, "test")

	e := settingslua.New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("RenderAUR", {
			callback = function(event)
				return "AURLINE:" .. event.data.name
			end,
		})
	`))

	qb := NewSourceQueryBuilder(mockAUR, logger, "", parser.ModeAny, "", false, false, false)
	qb.SetLua(e)
	qb.Execute(t.Context(), mockDB, []string{"yay"})

	err := qb.Results(mockDB, Detailed)
	require.NoError(t, err)

	out := w.String()

	// Both AUR packages should use the hook's output.
	assert.Contains(t, out, "AURLINE:yay", "hook output for yay must appear")
	assert.Contains(t, out, "AURLINE:yay-git", "hook output for yay-git must appear")

	// Default AUR rendering includes the votes marker; hook replaces it entirely.
	assert.NotContains(t, out, "(+2461", "default AUR rendering must not appear when hook is active")

	// Sync package should keep its default rendering (no RenderSync hook registered).
	assert.Contains(t, out, "ruby-yard", "sync package must still appear with default rendering")
}

// TestSourceQueryBuilderRenderSyncHook verifies that a RenderSync hook replaces
// the default sync row rendering.
func TestSourceQueryBuilderRenderSyncHook(t *testing.T) {
	t.Parallel()

	mockDB, mockAUR := newYayQueryBuilderMocks()

	w := &strings.Builder{}
	logger := text.NewLogger(w, io.Discard, strings.NewReader(""), false, "test")

	e := settingslua.New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("RenderSync", {
			callback = function(event)
				return "SYNCLINE:" .. event.data.name .. "/" .. event.data.repository
			end,
		})
	`))

	qb := NewSourceQueryBuilder(mockAUR, logger, "", parser.ModeAny, "", false, false, false)
	qb.SetLua(e)
	qb.Execute(t.Context(), mockDB, []string{"yay"})

	err := qb.Results(mockDB, Detailed)
	require.NoError(t, err)

	out := w.String()
	assert.Contains(t, out, "SYNCLINE:ruby-yard/extra", "hook output for ruby-yard must appear")
}

// TestSourceQueryBuilderRenderAURNilFallsBack verifies that a RenderAUR hook
// that returns nil causes the default formatter to be used for that row.
func TestSourceQueryBuilderRenderAURNilFallsBack(t *testing.T) {
	t.Parallel()

	mockDB, mockAUR := newYayQueryBuilderMocks()

	w := &strings.Builder{}
	logger := text.NewLogger(w, io.Discard, strings.NewReader(""), false, "test")

	e := settingslua.New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("RenderAUR", {
			callback = function(event)
				return nil
			end,
		})
	`))

	qb := NewSourceQueryBuilder(mockAUR, logger, "", parser.ModeAny, "", false, false, false)
	qb.SetLua(e)
	qb.Execute(t.Context(), mockDB, []string{"yay"})

	err := qb.Results(mockDB, Detailed)
	require.NoError(t, err)

	out := w.String()
	// Default AUR rendering must appear when hook returns nil.
	assert.Contains(t, out, "(+2461", "default AUR rendering must appear when hook returns nil")
}
