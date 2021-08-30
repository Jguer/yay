package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// GIVEN default config
// WHEN setPrivilegeElevator gets called
// THEN sudobin should stay as "sudo" (given sudo exists)
func TestConfiguration_setPrivilegeElevator(t *testing.T) {
	oldPath := os.Getenv("PATH")

	path, err := os.MkdirTemp("", "yay-test")
	assert.NoError(t, err)

	doas := filepath.Join(path, "sudo")
	_, err = os.Create(doas)
	os.Chmod(doas, 0o755)
	assert.NoError(t, err)

	defer os.RemoveAll(path)

	config := DefaultConfig()
	config.SudoLoop = true
	config.SudoFlags = "-v"

	os.Setenv("PATH", path)
	err = config.setPrivilegeElevator()
	os.Setenv("PATH", oldPath)
	assert.NoError(t, err)

	assert.Equal(t, "sudo", config.SudoBin)
	assert.Equal(t, "-v", config.SudoFlags)
	assert.True(t, config.SudoLoop)
}

// GIVEN default config and sudo loop enabled
// GIVEN only su in path
// WHEN setPrivilegeElevator gets called
// THEN sudobin should be changed to "su"
func TestConfiguration_setPrivilegeElevator_su(t *testing.T) {
	oldPath := os.Getenv("PATH")

	path, err := os.MkdirTemp("", "yay-test")
	assert.NoError(t, err)

	doas := filepath.Join(path, "su")
	_, err = os.Create(doas)
	os.Chmod(doas, 0o755)
	assert.NoError(t, err)

	defer os.RemoveAll(path)

	config := DefaultConfig()
	config.SudoLoop = true
	config.SudoFlags = "-v"

	os.Setenv("PATH", path)
	err = config.setPrivilegeElevator()
	os.Setenv("PATH", oldPath)

	assert.NoError(t, err)
	assert.Equal(t, "su", config.SudoBin)
	assert.Equal(t, "", config.SudoFlags)
	assert.False(t, config.SudoLoop)
}

// GIVEN default config and sudo loop enabled
// GIVEN no sudo in path
// WHEN setPrivilegeElevator gets called
// THEN sudobin should be changed to "su"
func TestConfiguration_setPrivilegeElevator_no_path(t *testing.T) {
	oldPath := os.Getenv("PATH")

	os.Setenv("PATH", "")
	config := DefaultConfig()
	config.SudoLoop = true
	config.SudoFlags = "-v"

	err := config.setPrivilegeElevator()
	os.Setenv("PATH", oldPath)

	assert.Error(t, err)
	assert.Equal(t, "sudo", config.SudoBin)
	assert.Equal(t, "", config.SudoFlags)
	assert.False(t, config.SudoLoop)
}

// GIVEN default config and sudo loop enabled
// GIVEN doas in path
// WHEN setPrivilegeElevator gets called
// THEN sudobin should be changed to "doas"
func TestConfiguration_setPrivilegeElevator_doas(t *testing.T) {
	oldPath := os.Getenv("PATH")

	path, err := os.MkdirTemp("", "yay-test")
	assert.NoError(t, err)

	doas := filepath.Join(path, "doas")
	_, err = os.Create(doas)
	os.Chmod(doas, 0o755)
	assert.NoError(t, err)

	defer os.RemoveAll(path)

	config := DefaultConfig()
	config.SudoLoop = true
	config.SudoFlags = "-v"

	os.Setenv("PATH", path)
	err = config.setPrivilegeElevator()
	os.Setenv("PATH", oldPath)
	assert.NoError(t, err)
	assert.Equal(t, "doas", config.SudoBin)
	assert.Equal(t, "", config.SudoFlags)
	assert.False(t, config.SudoLoop)
}

// GIVEN config with wrapper and sudo loop enabled
// GIVEN wrapper is in path
// WHEN setPrivilegeElevator gets called
// THEN sudobin should be kept as the wrapper
func TestConfiguration_setPrivilegeElevator_custom_script(t *testing.T) {
	oldPath := os.Getenv("PATH")

	path, err := os.MkdirTemp("", "yay-test")
	assert.NoError(t, err)

	wrapper := filepath.Join(path, "custom-wrapper")
	_, err = os.Create(wrapper)
	os.Chmod(wrapper, 0o755)
	assert.NoError(t, err)

	defer os.RemoveAll(path)

	config := DefaultConfig()
	config.SudoLoop = true
	config.SudoBin = wrapper
	config.SudoFlags = "-v"

	os.Setenv("PATH", path)
	err = config.setPrivilegeElevator()
	os.Setenv("PATH", oldPath)

	assert.NoError(t, err)
	assert.Equal(t, wrapper, config.SudoBin)
	assert.Equal(t, "-v", config.SudoFlags)
	assert.True(t, config.SudoLoop)
}
