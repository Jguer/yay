//go:build !integration

package workdir

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/settings/parser"
	"github.com/Jguer/yay/v13/pkg/text"
)

func newTestLogger() *text.Logger {
	return text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
}

// Test order of pre-download-sources hooks
func TestPreDownloadSourcesHooks(t *testing.T) {
	testCases := []struct {
		name     string
		cfg      *settings.Configuration
		wantHook []string
	}{
		{
			name: "clean, diff, edit",
			cfg: &settings.Configuration{
				CleanMenu: true,
				DiffMenu:  true,
				EditMenu:  true,
			},
			wantHook: []string{"clean", "diff", "edit"},
		},
		{
			name: "clean, edit",
			cfg: &settings.Configuration{
				CleanMenu: true,
				DiffMenu:  false,
				EditMenu:  true,
			},
			wantHook: []string{"clean", "edit"},
		},
		{
			name: "clean, diff",
			cfg: &settings.Configuration{
				CleanMenu: true,
				DiffMenu:  true,
				EditMenu:  false,
			},
			wantHook: []string{"clean", "diff"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			preper := NewPreparer(nil, nil, tc.cfg, newTestLogger())

			assert.Len(t, preper.hooks, len(tc.wantHook))

			got := make([]string, 0, len(preper.hooks))

			for _, hook := range preper.hooks {
				got = append(got, hook.Name)
			}

			assert.Equal(t, tc.wantHook, got)
		})
	}
}

func TestShouldCleanMakeDepsDownloadOnly(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		flag string
	}{
		{name: "normal install"},
		{name: "short option", flag: "w"},
		{name: "long option", flag: "downloadonly"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			preparer := NewPreparer(nil, nil, &settings.Configuration{RemoveMake: "yes"}, newTestLogger())
			preparer.makeDeps = []string{"library"}
			args := parser.MakeArguments()
			if tc.flag != "" {
				require.NoError(t, args.AddArg(tc.flag))
			}

			hook := preparer.ShouldCleanMakeDeps(nil, args)
			if tc.flag == "" {
				assert.NotNil(t, hook)
			} else {
				assert.Nil(t, hook)
			}
		})
	}
}
