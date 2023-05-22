//go:build !integration
// +build !integration

package text

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/leonelquinteros/gotext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLessRunes(t *testing.T) {
	t.Parallel()

	type args struct {
		iRunes []rune
		jRunes []rune
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "nilslices", args: args{iRunes: nil, jRunes: nil}, want: false},
		{name: "emptyslices", args: args{iRunes: []rune{}, jRunes: []rune{}}, want: false},
		{name: "simpleslice a,b", args: args{iRunes: []rune{'a'}, jRunes: []rune{'b'}}, want: true},
		{name: "simpleslice b,a", args: args{iRunes: []rune{'b'}, jRunes: []rune{'a'}}, want: false},
		{name: "equalslice", args: args{iRunes: []rune{'a', 'a', 'a'}, jRunes: []rune{'a', 'a', 'a'}}, want: false},
		{name: "uppercase", args: args{iRunes: []rune{'a'}, jRunes: []rune{'A'}}, want: false},
		{name: "longerFirstArg", args: args{iRunes: []rune{'a', 'b'}, jRunes: []rune{'a'}}, want: false},
		{name: "longerSecondArg", args: args{iRunes: []rune{'a'}, jRunes: []rune{'a', 'b'}}, want: true},
		{name: "utf8 less", args: args{iRunes: []rune{'世', '2', '0'}, jRunes: []rune{'世', '界', '3'}}, want: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := LessRunes(tt.args.iRunes, tt.args.jRunes)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContinueTask(t *testing.T) {
	t.Parallel()
	type args struct {
		s         string
		preset    bool
		noConfirm bool
		input     string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "noconfirm-true", args: args{s: "", preset: true, noConfirm: true}, want: true},
		{name: "noconfirm-false", args: args{s: "", preset: false, noConfirm: true}, want: false},
		{name: "noinput-false", args: args{s: "", preset: false, noConfirm: false}, want: false},
		{name: "noinput-true", args: args{s: "", preset: true, noConfirm: false}, want: true},
		{name: "input-false", args: args{s: "", input: "n", preset: true, noConfirm: false}, want: false},
		{name: "input-true", args: args{s: "", input: "y", preset: false, noConfirm: false}, want: true},
		{name: "input-false-complete", args: args{s: "", input: "no", preset: true, noConfirm: false}, want: false},
		{name: "input-true-complete", args: args{s: "", input: "yes", preset: false, noConfirm: false}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// create io.Reader with value of input
			in := strings.NewReader(tt.args.input)
			got := ContinueTask(in, tt.args.s, tt.args.preset, tt.args.noConfirm)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestContinueTaskRU(t *testing.T) {
	strCustom := `
msgid "yes"
msgstr "да"
	`

	// Create Locales directory and files on temp location
	tmpDir := t.TempDir()
	dirname := path.Join(tmpDir, "en_US")
	err := os.MkdirAll(dirname, os.ModePerm)
	require.NoError(t, err)

	fDefault, err := os.Create(path.Join(dirname, "yay.po"))
	require.NoError(t, err)

	defer fDefault.Close()

	_, err = fDefault.WriteString(strCustom)
	require.NoError(t, err)

	gotext.Configure(tmpDir, "en_US", "yay")
	require.Equal(t, "да", gotext.Get("yes"))

	type args struct {
		s         string
		preset    bool
		noConfirm bool
		input     string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "default input false", args: args{s: "", input: "n", preset: true, noConfirm: false}, want: false},
		{name: "default input true", args: args{s: "", input: "y", preset: false, noConfirm: false}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.NewReader(tt.args.input)
			got := ContinueTask(in, tt.args.s, tt.args.preset, tt.args.noConfirm)
			require.Equal(t, tt.want, got)
		})
	}
	gotext.SetLanguage("")
}

func TestContinueTaskDE(t *testing.T) {
	strCustom := `
msgid "yes"
msgstr "ja"
	`

	// Create Locales directory and files on temp location
	tmpDir := t.TempDir()
	dirname := path.Join(tmpDir, "en_US")
	err := os.MkdirAll(dirname, os.ModePerm)
	require.NoError(t, err)

	fDefault, err := os.Create(path.Join(dirname, "yay.po"))
	require.NoError(t, err)

	defer fDefault.Close()

	_, err = fDefault.WriteString(strCustom)
	require.NoError(t, err)

	gotext.Configure(tmpDir, "en_US", "yay")
	require.Equal(t, "ja", gotext.Get("yes"))

	type args struct {
		s         string
		preset    bool
		noConfirm bool
		input     string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "default input false", args: args{s: "", input: "n", preset: true, noConfirm: false}, want: false},
		{name: "default input true", args: args{s: "", input: "y", preset: false, noConfirm: false}, want: true},
		{name: "custom input true", args: args{s: "", input: "j", preset: false, noConfirm: false}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.NewReader(tt.args.input)
			got := ContinueTask(in, tt.args.s, tt.args.preset, tt.args.noConfirm)
			require.Equal(t, tt.want, got)
		})
	}
	gotext.SetLanguage("")
}
