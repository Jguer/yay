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

func TestFindEligibleCachePackage(t *testing.T) {
	t.Parallel()

	const day = 24 * time.Hour
	now := time.Unix(1_700_000_000, 0)
	text.NowFunc = func() time.Time { return now }

	oldBuilt := now.Add(-30 * day).Unix()
	recentBuilt := now.Add(-2 * day).Unix()

	cacheDir := t.TempDir()
	oldPkg := filepath.Join(cacheDir, "foo-1.0.0-1-x86_64.pkg.tar.zst")
	newerOldPkg := filepath.Join(cacheDir, "foo-1.1.0-1-x86_64.pkg.tar.zst")
	recentPkg := filepath.Join(cacheDir, "foo-2.0.0-1-x86_64.pkg.tar.zst")

	require.NoError(t, os.WriteFile(oldPkg, []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(newerOldPkg, []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(recentPkg, []byte("x"), 0o600))

	query := map[string]struct {
		version string
		built   int64
	}{
		oldPkg:      {"1.0.0-1", oldBuilt},
		newerOldPkg: {"1.1.0-1", oldBuilt},
		recentPkg:   {"2.0.0-1", recentBuilt},
	}

	runner := &exe.MockRunner{
		CaptureFn: func(cmd *exec.Cmd) (string, string, error) {
			if !strings.Contains(strings.Join(cmd.Args, " "), "-Qp") {
				return "", "", fmt.Errorf("unexpected command: %v", cmd.Args)
			}

			info, ok := query[cmd.Args[len(cmd.Args)-1]]
			if !ok {
				return "", "", fmt.Errorf("unexpected package path: %s", cmd.Args[len(cmd.Args)-1])
			}

			return fmt.Sprintf("foo|%s|%d", info.version, info.built), "", nil
		},
	}

	path, version, built, ok := FindEligibleCachePackage(
		context.Background(),
		&exe.MockBuilder{Runner: runner},
		"pacman",
		[]string{cacheDir},
		"foo",
		7,
	)
	require.True(t, ok)
	require.Equal(t, newerOldPkg, path)
	require.Equal(t, "1.1.0-1", version)
	require.Equal(t, oldBuilt, built)
}

func TestFindEligibleCachePackage_noEligible(t *testing.T) {
	t.Parallel()

	const day = 24 * time.Hour
	now := time.Unix(1_700_000_000, 0)
	text.NowFunc = func() time.Time { return now }

	recentBuilt := now.Add(-2 * day).Unix()
	cacheDir := t.TempDir()
	recentPkg := filepath.Join(cacheDir, "foo-2.0.0-1-x86_64.pkg.tar.zst")
	require.NoError(t, os.WriteFile(recentPkg, []byte("x"), 0o600))

	runner := &exe.MockRunner{
		CaptureFn: func(cmd *exec.Cmd) (string, string, error) {
			return fmt.Sprintf("foo|2.0.0-1|%d", recentBuilt), "", nil
		},
	}

	path, version, built, ok := FindEligibleCachePackage(
		context.Background(),
		&exe.MockBuilder{Runner: runner},
		"pacman",
		[]string{cacheDir},
		"foo",
		7,
	)
	require.False(t, ok)
	require.Empty(t, path)
	require.Empty(t, version)
	require.Zero(t, built)
}

func TestFindEligibleCachePackage_disabled(t *testing.T) {
	t.Parallel()

	path, version, built, ok := FindEligibleCachePackage(
		context.Background(), nil, "pacman", []string{"/var/cache/pacman/pkg"}, "foo", 0)
	require.False(t, ok)
	require.Empty(t, path)
	require.Empty(t, version)
	require.Zero(t, built)
}
