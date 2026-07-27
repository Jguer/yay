//go:build !integration

package workdir

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/dep"
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/settings/exe"
	"github.com/Jguer/yay/v13/pkg/text"
)

const preparerTestSrcinfo = `pkgbase = foobase
pkgver = 1.2.3
pkgrel = 2
arch = x86_64

pkgname = foobase
`

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

func TestPreparer_PrepareWorkspace_checkoutFailure(t *testing.T) {
	t.Parallel()

	buildDir := t.TempDir()
	pkgDir := filepath.Join(buildDir, "foobase")
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".SRCINFO"), []byte(preparerTestSrcinfo), 0o600))

	runner := &exe.MockRunner{
		CaptureFn: func(cmd *exec.Cmd) (string, string, error) {
			if strings.Contains(strings.Join(cmd.Args, " "), " checkout ") {
				return "", "", errors.New("checkout failed")
			}

			return "", "", nil
		},
	}

	preper := NewPreparerWithoutHooks(nil, &exe.MockBuilder{Runner: runner}, &settings.Configuration{
		BuildDir: buildDir,
		AURURL:   "https://aur.archlinux.org",
	}, newTestLogger(), false)

	targets := []map[string]*dep.InstallInfo{{
		"foobase": {
			Source:    dep.AUR,
			AURBase:   "foobase",
			Version:   "1.2.3-2",
			AURGitRef: "deadbeef",
		},
	}}

	_, err := preper.PrepareWorkspace(context.Background(), nil, targets)
	require.Error(t, err)
	require.Contains(t, err.Error(), "checkout failed")
}
