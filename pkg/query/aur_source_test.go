//go:build !integration

package query

import (
	"io"
	"regexp"
	"strings"
	"testing"

	aurc "github.com/Jguer/aur"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/settings"
	settingslua "github.com/Jguer/yay/v13/pkg/settings/lua"
	"github.com/Jguer/yay/v13/pkg/settings/parser"
	"github.com/Jguer/yay/v13/pkg/text"
)

// labeledAUR adds per-package source information to a query client, like
// aursource.Client does.
type labeledAUR struct {
	aurc.QueryClient
	sources map[string]settings.AURSource
}

func (l labeledAUR) SourceFor(name string) settings.AURSource {
	if src, ok := l.sources[name]; ok {
		return src
	}

	return settings.AURSource{Name: "aur", URL: "https://aur.archlinux.org"}
}

// ansiRe matches SGR colour codes and OSC 8 hyperlink sequences.
var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m|\x1b\\]8;;[^\x1b]*\x1b\\\\")

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

func TestAURRepoLink(t *testing.T) {
	// Not parallel: toggles the global text.UseColor.
	old := text.UseColor
	t.Cleanup(func() { text.UseColor = old })

	text.UseColor = true
	assert.Contains(t, aurRepoLink("mirror", "https://aur.example.org", "foo"), "https://aur.example.org/packages/foo")
	assert.Contains(t, aurRepoLink("aur", "https://ignored.example", "foo"), "https://aur.archlinux.org/packages/foo")
	assert.Equal(t, "mirror/foo", stripANSI(aurRepoLink("mirror", "https://aur.example.org", "foo")))

	text.UseColor = false
	assert.Equal(t, "mirror/foo", aurRepoLink("mirror", "https://aur.example.org", "foo"))
	assert.Equal(t, "aur/foo", aurRepoLink("aur", "", "foo"))
}

func newLabeledMocks() (labeledAUR, func() *SourceQueryBuilder, *strings.Builder) {
	mockDB, mockAUR := newYayQueryBuilderMocks()
	_ = mockDB
	client := labeledAUR{QueryClient: mockAUR, sources: map[string]settings.AURSource{
		"yay-git": {Name: "mirror", URL: "https://aur.example.org"},
	}}
	w := &strings.Builder{}
	logger := text.NewLogger(w, io.Discard, strings.NewReader(""), false, "test")

	return client, func() *SourceQueryBuilder {
		return NewSourceQueryBuilder(client, logger, "", parser.ModeAny, "", false, false, false)
	}, w
}

func TestAURSourceOf(t *testing.T) {
	t.Parallel()

	client, _, _ := newLabeledMocks()

	label, url := AURSourceOf(client, "yay-git")
	assert.Equal(t, "mirror", label)
	assert.Equal(t, "https://aur.example.org", url)

	assert.Equal(t, "aur", AURSourceLabel(client, "yay"))
	assert.Equal(t, "aur", AURSourceLabel(struct{}{}, "anything"), "clients without sources use the default label")
	assert.Equal(t, "aur", AURSourceLabel(labeledAUR{sources: map[string]settings.AURSource{"x": {}}}, "x"),
		"unnamed sources fall back to the default label")
}

func TestSearchResultsLabelNonDefaultSources(t *testing.T) {
	t.Parallel()

	mockDB, _ := newYayQueryBuilderMocks()
	_, newQB, w := newLabeledMocks()

	qb := newQB()
	qb.Execute(t.Context(), mockDB, []string{"yay"})
	require.NoError(t, qb.Results(mockDB, Detailed))

	out := stripANSI(w.String())
	assert.Contains(t, out, "aur/yay ", "default source keeps the aur/ label")
	assert.Contains(t, out, "mirror/yay-git ", "other sources are labelled by name")
	assert.NotContains(t, out, "aur/yay-git")

	// Internally both stay AUR results, so -a/-N filtering and the number
	// menu treat them the same.
	for _, r := range qb.results {
		if r.name == "yay" || r.name == "yay-git" {
			assert.Equal(t, "aur", r.source)
		}
	}
}

func TestSearchResultsNonDefaultSourceFilteredByRepoMode(t *testing.T) {
	t.Parallel()

	mockDB, mockAUR := newYayQueryBuilderMocks()
	client := labeledAUR{QueryClient: mockAUR, sources: map[string]settings.AURSource{
		"yay-git": {Name: "mirror", URL: "https://aur.example.org"},
	}}
	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")

	qb := NewSourceQueryBuilder(client, logger, "", parser.ModeRepo, "", false, false, false)
	qb.Execute(t.Context(), mockDB, []string{"yay"})

	for _, r := range qb.results {
		assert.NotEqual(t, "aur", r.source, "-N must hide packages from every AUR source")
	}
}

func TestLuaHooksReceiveAURSource(t *testing.T) {
	t.Parallel()

	mockDB, _ := newYayQueryBuilderMocks()
	_, newQB, w := newLabeledMocks()

	e := settingslua.New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("SearchFilter", {
			callback = function(event)
				local out = {}
				for _, r in ipairs(event.data.results) do
					-- source stays "aur" for every AUR source; aur_source names it
					if r.source == "aur" and r.aur_source == "mirror" then
						out[#out + 1] = { source = r.source, name = r.name }
					end
				end
				return out
			end,
		})
		yay.create_autocmd("RenderAUR", {
			callback = function(event)
				return "ROW:" .. event.data.aur_source .. ":" .. event.data.name
			end,
		})
	`))

	qb := newQB()
	qb.SetLua(e)
	qb.Execute(t.Context(), mockDB, []string{"yay"})
	require.NoError(t, qb.Results(mockDB, Detailed))

	out := w.String()
	assert.Contains(t, out, "ROW:mirror:yay-git")
	assert.NotContains(t, out, "ROW:aur:yay\n")
	require.Len(t, qb.results, 1)
}

func TestRenderAURHookDefaultSourceLabel(t *testing.T) {
	t.Parallel()

	mockDB, mockAUR := newYayQueryBuilderMocks()
	w := &strings.Builder{}
	logger := text.NewLogger(w, io.Discard, strings.NewReader(""), false, "test")

	e := settingslua.New()
	defer e.Close()

	require.NoError(t, e.L.DoString(`
		yay.create_autocmd("RenderAUR", {
			callback = function(event) return "ROW:" .. event.data.aur_source .. ":" .. event.data.name end,
		})
	`))

	qb := NewSourceQueryBuilder(mockAUR, logger, "", parser.ModeAny, "", false, false, false)
	qb.SetLua(e)
	qb.Execute(t.Context(), mockDB, []string{"yay"})
	require.NoError(t, qb.Results(mockDB, Detailed))

	assert.Contains(t, w.String(), "ROW:aur:yay-git", "plain clients report the default label")
}
