//go:build !integration

package upgrade

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/settings/exe"
	"github.com/Jguer/yay/v13/pkg/text"
)

const testAURSrcinfo = `pkgbase = foobase
pkgver = 1.2.3
pkgrel = 2
arch = x86_64

pkgname = foobase
`

func TestFindAURVersionBefore(t *testing.T) {
	t.Parallel()

	const day = 24 * time.Hour
	now := time.Unix(1_700_000_000, 0)
	text.NowFunc = func() time.Time { return now }

	buildDir := t.TempDir()
	pkgDir := filepath.Join(buildDir, "foobase")
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".git"), 0o755))

	oldTS := now.Add(-30 * day).Unix()
	commit := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	runner := &exe.MockRunner{
		CaptureFn: func(cmd *exec.Cmd) (string, string, error) {
			joined := strings.Join(cmd.Args, " ")
			switch {
			case strings.Contains(joined, " pull "):
				return "", "", nil
			case strings.Contains(joined, " log "):
				return fmt.Sprintf("%d %s", oldTS, commit), "", nil
			case strings.Contains(joined, " show "):
				return testAURSrcinfo, "", nil
			default:
				return "", "", fmt.Errorf("unexpected command: %s", joined)
			}
		},
	}

	gitRef, version, built, ok := findAURVersionBefore(
		context.Background(),
		&exe.MockBuilder{Runner: runner},
		"https://aur.archlinux.org",
		buildDir,
		"foobase",
		7,
	)
	require.True(t, ok)
	require.Equal(t, commit, gitRef)
	require.Equal(t, "1.2.3-2", version)
	require.Equal(t, oldTS, built)
}

func TestFindAURVersionBefore_noHistory(t *testing.T) {
	t.Parallel()

	buildDir := t.TempDir()
	pkgDir := filepath.Join(buildDir, "foobase")
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".git"), 0o755))

	runner := &exe.MockRunner{
		CaptureFn: func(cmd *exec.Cmd) (string, string, error) {
			joined := strings.Join(cmd.Args, " ")
			switch {
			case strings.Contains(joined, " pull "):
				return "", "", nil
			case strings.Contains(joined, " log "):
				return "", "", nil
			default:
				return "", "", fmt.Errorf("unexpected command: %s", joined)
			}
		},
	}

	gitRef, version, built, ok := findAURVersionBefore(
		context.Background(),
		&exe.MockBuilder{Runner: runner},
		"https://aur.archlinux.org",
		buildDir,
		"foobase",
		7,
	)
	require.False(t, ok)
	require.Empty(t, gitRef)
	require.Empty(t, version)
	require.Zero(t, built)
}

func TestCheckoutAURGitRef(t *testing.T) {
	t.Parallel()

	t.Run("empty ref", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, CheckoutAURGitRef(context.Background(), &exe.MockBuilder{}, t.TempDir(), ""))
	})

	t.Run("missing git dir", func(t *testing.T) {
		t.Parallel()
		err := CheckoutAURGitRef(context.Background(), &exe.MockBuilder{}, t.TempDir(), "abc")
		require.Error(t, err)
	})

	t.Run("checkout", func(t *testing.T) {
		t.Parallel()

		pkgDir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".git"), 0o755))

		checkedOut := ""
		runner := &exe.MockRunner{
			CaptureFn: func(cmd *exec.Cmd) (string, string, error) {
				require.Contains(t, strings.Join(cmd.Args, " "), " checkout abc")
				checkedOut = "abc"

				return "", "", nil
			},
		}

		require.NoError(t, CheckoutAURGitRef(context.Background(), &exe.MockBuilder{Runner: runner}, pkgDir, "abc"))
		require.Equal(t, "abc", checkedOut)
	})
}
