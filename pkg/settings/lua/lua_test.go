package lua

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testConfig struct {
	BuildDir      string `json:"buildDir" lua:"build_dir"`
	RequestSplitN int    `json:"requestsplitn" lua:"request_split_n"`
	Devel         bool   `json:"devel" lua:"devel"`
	AnswerClean   string `json:"answerclean"`
	Ignored       string `json:"-" lua:"-"`
}

func TestApply(t *testing.T) {
	t.Parallel()
	e := New()
	t.Cleanup(e.Close)

	require.NoError(t, e.L.DoString(`
		yay.opt.build_dir = "/tmp/yay"
		yay.opt.request_split_n = 200
		yay.opt.devel = true
	`))

	cfg := &testConfig{}
	unknown, errs := e.Apply(cfg)

	assert.Empty(t, unknown)
	assert.Empty(t, errs)
	assert.Equal(t, "/tmp/yay", cfg.BuildDir)
	assert.Equal(t, 200, cfg.RequestSplitN)
	assert.True(t, cfg.Devel)
}

func TestApplyUnknownAndTypeMismatch(t *testing.T) {
	t.Parallel()
	e := New()
	t.Cleanup(e.Close)

	require.NoError(t, e.L.DoString(`
		yay.opt.does_not_exist = "x"
		yay.opt.buildDir = "/tmp/nope"
		yay.opt.requestsplitn = 200
		yay.opt.Devel = true
		yay.opt.devel = "not a bool"
		yay.opt.build_dir = "/tmp/ok"
	`))

	cfg := &testConfig{}
	unknown, errs := e.Apply(cfg)

	assert.ElementsMatch(t, []string{"does_not_exist", "buildDir", "requestsplitn", "Devel"}, unknown)
	assert.Len(t, errs, 1)
	assert.Zero(t, cfg.RequestSplitN)
	assert.False(t, cfg.Devel)
	assert.Equal(t, "/tmp/ok", cfg.BuildDir)
}

func TestApplyIgnoresAnswerOptionsWithoutLuaTags(t *testing.T) {
	t.Parallel()
	e := New()
	t.Cleanup(e.Close)

	require.NoError(t, e.L.DoString(`
		yay.opt.answer_clean = "All"
	`))

	cfg := &testConfig{}
	unknown, errs := e.Apply(cfg)

	assert.Equal(t, []string{"answer_clean"}, unknown)
	assert.Empty(t, errs)
	assert.Empty(t, cfg.AnswerClean)
}

func TestApplyRejectsNonPointer(t *testing.T) {
	t.Parallel()
	e := New()
	t.Cleanup(e.Close)

	_, errs := e.Apply(testConfig{})
	assert.Len(t, errs, 1)
}
