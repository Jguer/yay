package lua

import (
	"testing"

	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimal struct mirroring the relevant Configuration fields/tags, kept local
// to avoid an import cycle with pkg/settings.
type fakeCfg struct {
	Editor   string `ini:"Editor"`
	BottomUp bool   `ini:"BottomUp"`
	SplitN   int    `ini:"RequestSplitN"`
	Skipped  string `ini:"-"`
	Untagged string
}

func TestEngine_ApplyAndOnPrompt(t *testing.T) {
	t.Parallel()
	e := New()
	defer e.Close()

	src := `
		yay.opt.editor = "nvim"
		yay.opt.bottom_up = true
		yay.opt.RequestSplitN = 42
		yay.opt.does_not_exist = "ignored"
		yay.hook.on_prompt = function(name, default)
			if name == "clean" then return "1 2 3" end
			return default
		end
		yay.hook.should_include_aur_update = function(pkg)
			return pkg.remote_last_modified > pkg.local_build_date
		end
	`
	require.NoError(t, e.RunString(src))

	var cfg fakeCfg
	unknown, errs := e.Apply(&cfg)
	require.Empty(t, errs, "unexpected apply errors: %v", errs)
	assert.ElementsMatch(t, []string{"does_not_exist"}, unknown)
	assert.Equal(t, "nvim", cfg.Editor)
	assert.True(t, cfg.BottomUp)
	assert.Equal(t, 42, cfg.SplitN)

	got, ok, err := e.CallOnPrompt("clean", "")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "1 2 3", got)

	got, ok, err = e.CallOnPrompt("diff", "default-ans")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "default-ans", got)

	include, ok, err := e.CallShouldIncludeAURUpdate(settings.AURUpdateContext{
		Name:               "hello",
		Repository:         "aur",
		LocalVersion:       "2.0.0",
		RemoteVersion:      "2.0.0",
		LocalBuildDate:     100,
		RemoteLastModified: 200,
		DefaultInclude:     false,
	})
	require.NoError(t, err)
	assert.True(t, ok)
	assert.True(t, include)
}

func TestEngine_OnPromptUnsetReturnsFalse(t *testing.T) {
	t.Parallel()
	e := New()
	defer e.Close()

	got, ok, err := e.CallOnPrompt("clean", "x")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "", got)
}

func TestEngine_ShouldIncludeAURUpdateUnsetReturnsFalse(t *testing.T) {
	t.Parallel()
	e := New()
	defer e.Close()

	include, ok, err := e.CallShouldIncludeAURUpdate(settings.AURUpdateContext{})
	require.NoError(t, err)
	assert.False(t, ok)
	assert.False(t, include)
}

func TestEngine_ApplyTypeMismatchReportsError(t *testing.T) {
	t.Parallel()
	e := New()
	defer e.Close()

	require.NoError(t, e.RunString(`yay.opt.editor = 123`))

	var cfg fakeCfg
	_, errs := e.Apply(&cfg)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "yay.opt.editor")
}
