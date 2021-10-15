package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// GIVEN no user directories and sudo user
// WHEN cache home is selected
// THEN the selected cache home should be in the tmp dir
func Test_getCacheHome(t *testing.T) {
	dir, err := os.MkdirTemp(os.TempDir(), "yay-cache-home")
	assert.NoError(t, err)
	os.Unsetenv("XDG_CACHE_HOME")
	os.Unsetenv("HOME")
	os.Setenv("SUDO_USER", "test")
	os.Setenv("TMPDIR", dir)
	got, err := getCacheHome()
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "yay"), got)
}
