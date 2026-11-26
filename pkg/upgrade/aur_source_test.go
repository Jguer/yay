//go:build !integration

package upgrade

import (
	"context"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/Jguer/aur"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/settings"
	settingslua "github.com/Jguer/yay/v13/pkg/settings/lua"
	"github.com/Jguer/yay/v13/pkg/text"
)

// sourcedAUR reports example-git as coming from the "mirror" source.
type sourcedAUR struct{}

func (sourcedAUR) Get(context.Context, *aur.Query) ([]aur.Pkg, error) { return nil, nil }

func (sourcedAUR) SourceFor(name string) settings.AURSource {
	if name == "example-git" {
		return settings.AURSource{Name: "mirror", URL: "https://aur.example.org"}
	}

	return settings.AURSource{Name: "aur", URL: "https://aur.archlinux.org"}
}

var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

func TestDisplayRepository(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "core", DisplayRepository(&Upgrade{Repository: "core"}))
	assert.Equal(t, "aur", DisplayRepository(&Upgrade{Repository: "aur", Source: "aur"}))
	assert.Equal(t, "mirror", DisplayRepository(&Upgrade{Repository: "aur", Source: "mirror"}))
	assert.Equal(t, "devel", DisplayRepository(&Upgrade{Repository: "devel", Source: "mirror"}),
		"devel packages keep the devel label")
	assert.Equal(t, "aur", DisplayRepository(&Upgrade{Repository: "aur"}))
	assert.Equal(t, "mirror/foo",
		ansi.ReplaceAllString(StylizedNameWithRepository(&Upgrade{Name: "foo", Repository: "aur", Source: "mirror"}), ""))
}

func TestUpgradeMenuLabelsAndExcludesBySource(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		input    string
		excluded []string
	}{
		{"mirror\n", []string{"example-git", "new-dep"}},     // only the mirror package
		{"aur\n", []string{"example-git", "new-dep", "yay"}}, // "aur" still matches every AUR source
		{"^mirror\n", []string{"linux", "yay"}},              // include only mirror
		{"^aur\n", []string{"linux"}},                        // include every AUR package
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			graph := newUpgradeSelectTestGraph(t)
			u := newUpgradeSelectTestService(strings.NewReader(tc.input), nil)
			u.aurCache = sourcedAUR{}

			out := &strings.Builder{}
			u.log = text.NewLogger(out, io.Discard, strings.NewReader(tc.input), true, "test")

			excluded, err := u.UserExcludeUpgrades(graph)
			require.NoError(t, err)
			assert.ElementsMatch(t, tc.excluded, excluded)

			menu := ansi.ReplaceAllString(out.String(), "")
			assert.Contains(t, menu, "mirror/example-git")
			assert.Contains(t, menu, "aur/yay")
			assert.Contains(t, menu, "core/linux")
		})
	}
}

func TestUpgradeSelectHookReceivesAURSource(t *testing.T) {
	t.Parallel()

	engine := settingslua.New()
	defer engine.Close()
	require.NoError(t, engine.L.DoString(`
		yay.create_autocmd("UpgradeSelect", {
			callback = function(event)
				local exclude = {}
				for _, up in ipairs(event.data.upgrades) do
					if up.aur_source == "mirror" and up.repository == "aur" then
						exclude[#exclude + 1] = up.name
					end
					if up.repository == "core" and up.aur_source ~= nil then
						error("repo packages must not carry aur_source")
					end
				end
				return { exclude = exclude, skip_menu = true }
			end,
		})
	`))

	graph := newUpgradeSelectTestGraph(t)
	u := newUpgradeSelectTestService(strings.NewReader(""), engine)
	u.aurCache = sourcedAUR{}

	excluded, err := u.UserExcludeUpgrades(graph)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"example-git", "new-dep"}, excluded)
}
