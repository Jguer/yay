//go:build !integration
// +build !integration

package query

import (
	"context"
	"io"
	"strings"
	"testing"

	settingslua "github.com/Jguer/yay/v12/pkg/settings/lua"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSourceQueryBuilderSearchFilterHook verifies that a SearchFilter hook
// registered on the Lua engine is applied after ranking, filtering s.results
// to only the entries the callback returns.
func TestSourceQueryBuilderSearchFilterHook(t *testing.T) {
	t.Parallel()

	mockDB, mockAUR := newYayQueryBuilderMocks()

	w := &strings.Builder{}
	logger := text.NewLogger(w, io.Discard, strings.NewReader(""), false, "test")

	// Baseline: build without a Lua hook and record count.
	baselineQB := NewSourceQueryBuilder(mockAUR, logger, "", parser.ModeAny, "", false, false, false)
	baselineQB.Execute(context.Background(), mockDB, []string{"yay"})
	baselineCount := len(baselineQB.results)
	require.Greater(t, baselineCount, 0, "baseline query must return at least one result")

	// Count how many AUR results exist in the baseline.
	aurCount := 0
	for _, r := range baselineQB.results {
		if r.source == "aur" {
			aurCount++
		}
	}
	require.Greater(t, aurCount, 0, "baseline must have at least one AUR result")

	// The mock also returns a non-AUR (sync) result for "yay".
	// Confirm there is at least one so the filter actually does something.
	nonAURCount := baselineCount - aurCount
	require.Greater(t, nonAURCount, 0, "baseline must have at least one non-AUR result for the filter to exercise")

	// Build with a SearchFilter that keeps only AUR results.
	e := settingslua.New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("SearchFilter", {
			callback = function(event)
				local out = {}
				for _, r in ipairs(event.data.results) do
					if r.source == "aur" then
						out[#out + 1] = { source = r.source, name = r.name }
					end
				end
				return out
			end,
		})
	`))

	filteredQB := NewSourceQueryBuilder(mockAUR, logger, "", parser.ModeAny, "", false, false, false)
	filteredQB.SetLua(e)
	filteredQB.Execute(context.Background(), mockDB, []string{"yay"})

	assert.Equal(t, aurCount, len(filteredQB.results),
		"filtered results should equal the number of AUR packages in the baseline")

	for _, r := range filteredQB.results {
		assert.Equal(t, "aur", r.source,
			"every result after the SearchFilter hook must be from aur")
	}
}
