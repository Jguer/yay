//go:build !integration
// +build !integration

package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GIVEN no user directories and sudo user
// WHEN cache home is selected
// THEN the selected cache home should be in the tmp dir
func Test_getCacheHome(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Unsetenv("XDG_CACHE_HOME"))
	require.NoError(t, os.Unsetenv("HOME"))
	t.Setenv("SUDO_USER", "test")
	t.Setenv("TMPDIR", dir)

	got, err := getCacheHome()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "yay"), got)

	require.NoError(t, os.Unsetenv("TMPDIR"))
	require.NoError(t, os.Unsetenv("SUDO_USER"))
}
