//go:build !integration

package workdir

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v13/pkg/dep"
	"github.com/Jguer/yay/v13/pkg/settings"
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

func TestNeedToCloneAURBaseRedownloadModes(t *testing.T) {
	pkgbuildDir := filepath.Join("..", "..", "..", "testdata", "gourou")

	testCases := []struct {
		name       string
		reDownload string
		reason     dep.Reason
		want       bool
	}{
		{
			name:       "redownload target",
			reDownload: "yes",
			reason:     dep.Explicit,
			want:       true,
		},
		{
			name:       "redownload dependency only when stale",
			reDownload: "yes",
			reason:     dep.MakeDep,
			want:       false,
		},
		{
			name:       "redownload all",
			reDownload: "all",
			reason:     dep.MakeDep,
			want:       true,
		},
		{
			name:       "normal download",
			reDownload: "no",
			reason:     dep.MakeDep,
			want:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			preper := NewPreparer(nil, nil, &settings.Configuration{
				ReDownload: tc.reDownload,
			}, newTestLogger())

			got := preper.needToCloneAURBase(&dep.InstallInfo{
				AURBase: "gourou",
				Reason:  tc.reason,
				Version: "0.8.1-4",
			}, pkgbuildDir)

			assert.Equal(t, tc.want, got)
		})
	}
}
