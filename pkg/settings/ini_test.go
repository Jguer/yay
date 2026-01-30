package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigurationLoadINI(t *testing.T) {
	t.Parallel()

	t.Run("load nonexistent file returns nil", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig("test")

		err := cfg.loadINI("/nonexistent/path/yay.conf")
		assert.NoError(t, err)
	})

	t.Run("load valid INI file", func(t *testing.T) {
		t.Parallel()

		content := `# System-wide yay configuration
[options]
AurUrl = https://custom.aur.org
BuildDir = /var/cache/yay
Editor = vim
Devel = true
SudoLoop = yes
RequestSplitN = 200
BottomUp = false

; This is also a comment
CleanAfter = 1
`
		tmpDir := t.TempDir()
		iniPath := filepath.Join(tmpDir, "yay.conf")
		require.NoError(t, os.WriteFile(iniPath, []byte(content), 0o644))

		cfg := DefaultConfig("test")
		err := cfg.loadINI(iniPath)
		require.NoError(t, err)

		assert.Equal(t, "https://custom.aur.org", cfg.AURURL)
		assert.Equal(t, "/var/cache/yay", cfg.BuildDir)
		assert.Equal(t, "vim", cfg.Editor)
		assert.True(t, cfg.Devel)
		assert.True(t, cfg.SudoLoop)
		assert.Equal(t, 200, cfg.RequestSplitN)
		assert.False(t, cfg.BottomUp)
		assert.True(t, cfg.CleanAfter)
	})

	t.Run("load INI file with all option types", func(t *testing.T) {
		t.Parallel()

		content := `
# String options
AurUrl = https://aur.example.com
AurRpcUrl = https://aur.example.com/rpc
BuildDir = /tmp/build
Editor = nvim
EditorFlags = -p
MakepkgBin = /usr/bin/makepkg
MakepkgConf = /etc/makepkg.conf
PacmanBin = /usr/bin/pacman
PacmanConf = /etc/pacman.conf
GitBin = /usr/bin/git
GpgBin = /usr/bin/gpg
GpgFlags = --keyserver-options
MFlags = -s
SortBy = votes
SearchBy = name
GitFlags = --depth=1
RemoveMake = yes
SudoBin = doas
SudoFlags = -n
ReDownload = all
AnswerClean = All
AnswerDiff = None
AnswerEdit = None
AnswerUpgrade = None
ReBuild = all

# Integer options
RequestSplitN = 100
CompletionInterval = 3
MaxConcurrentDownloads = 4

# Boolean options
BottomUp = true
SudoLoop = false
TimeUpdate = yes
Devel = no
CleanAfter = true
KeepSrc = false
Provides = true
PgpFetch = false
CleanMenu = yes
DiffMenu = no
EditMenu = true
CombinedUpgrade = false
UseAsk = true
BatchInstall = false
SingleLineResults = true
SeparateSources = false
Debug = no
Rpc = yes
DoubleConfirm = false
`
		tmpDir := t.TempDir()
		iniPath := filepath.Join(tmpDir, "yay.conf")
		require.NoError(t, os.WriteFile(iniPath, []byte(content), 0o644))

		cfg := DefaultConfig("test")
		err := cfg.loadINI(iniPath)
		require.NoError(t, err)

		// String options
		assert.Equal(t, "https://aur.example.com", cfg.AURURL)
		assert.Equal(t, "https://aur.example.com/rpc", cfg.AURRPCURL)
		assert.Equal(t, "/tmp/build", cfg.BuildDir)
		assert.Equal(t, "nvim", cfg.Editor)
		assert.Equal(t, "-p", cfg.EditorFlags)
		assert.Equal(t, "/usr/bin/makepkg", cfg.MakepkgBin)
		assert.Equal(t, "/etc/makepkg.conf", cfg.MakepkgConf)
		assert.Equal(t, "/usr/bin/pacman", cfg.PacmanBin)
		assert.Equal(t, "/etc/pacman.conf", cfg.PacmanConf)
		assert.Equal(t, "/usr/bin/git", cfg.GitBin)
		assert.Equal(t, "/usr/bin/gpg", cfg.GpgBin)
		assert.Equal(t, "--keyserver-options", cfg.GpgFlags)
		assert.Equal(t, "-s", cfg.MFlags)
		assert.Equal(t, "votes", cfg.SortBy)
		assert.Equal(t, "name", cfg.SearchBy)
		assert.Equal(t, "--depth=1", cfg.GitFlags)
		assert.Equal(t, "yes", cfg.RemoveMake)
		assert.Equal(t, "doas", cfg.SudoBin)
		assert.Equal(t, "-n", cfg.SudoFlags)
		assert.Equal(t, "all", cfg.ReDownload)
		assert.Equal(t, "All", cfg.AnswerClean)
		assert.Equal(t, "None", cfg.AnswerDiff)
		assert.Equal(t, "None", cfg.AnswerEdit)
		assert.Equal(t, "None", cfg.AnswerUpgrade)
		assert.Equal(t, "all", string(cfg.ReBuild))

		// Integer options
		assert.Equal(t, 100, cfg.RequestSplitN)
		assert.Equal(t, 3, cfg.CompletionInterval)
		assert.Equal(t, 4, cfg.MaxConcurrentDownloads)

		// Boolean options
		assert.True(t, cfg.BottomUp)
		assert.False(t, cfg.SudoLoop)
		assert.True(t, cfg.TimeUpdate)
		assert.False(t, cfg.Devel)
		assert.True(t, cfg.CleanAfter)
		assert.False(t, cfg.KeepSrc)
		assert.True(t, cfg.Provides)
		assert.False(t, cfg.PGPFetch)
		assert.True(t, cfg.CleanMenu)
		assert.False(t, cfg.DiffMenu)
		assert.True(t, cfg.EditMenu)
		assert.False(t, cfg.CombinedUpgrade)
		assert.True(t, cfg.UseAsk)
		assert.False(t, cfg.BatchInstall)
		assert.True(t, cfg.SingleLineResults)
		assert.False(t, cfg.SeparateSources)
		assert.False(t, cfg.Debug)
		assert.True(t, cfg.UseRPC)
		assert.False(t, cfg.DoubleConfirm)
	})

	t.Run("load INI without section header", func(t *testing.T) {
		t.Parallel()

		content := `# Config without section header
AurUrl = https://custom.aur.org
Devel = true
RequestSplitN = 250
`
		tmpDir := t.TempDir()
		iniPath := filepath.Join(tmpDir, "yay.conf")
		require.NoError(t, os.WriteFile(iniPath, []byte(content), 0o644))

		cfg := DefaultConfig("test")
		err := cfg.loadINI(iniPath)
		require.NoError(t, err)

		assert.Equal(t, "https://custom.aur.org", cfg.AURURL)
		assert.True(t, cfg.Devel)
		assert.Equal(t, 250, cfg.RequestSplitN)
	})

	t.Run("load INI with boolean keys (no value)", func(t *testing.T) {
		t.Parallel()

		content := `# Boolean keys without values are treated as true
Devel
SudoLoop
CleanAfter
`
		tmpDir := t.TempDir()
		iniPath := filepath.Join(tmpDir, "yay.conf")
		require.NoError(t, os.WriteFile(iniPath, []byte(content), 0o644))

		cfg := DefaultConfig("test")
		cfg.Devel = false
		cfg.SudoLoop = false
		cfg.CleanAfter = false

		err := cfg.loadINI(iniPath)
		require.NoError(t, err)

		assert.True(t, cfg.Devel)
		assert.True(t, cfg.SudoLoop)
		assert.True(t, cfg.CleanAfter)
	})

	t.Run("unknown options are ignored", func(t *testing.T) {
		t.Parallel()

		content := `AurUrl = https://custom.aur.org
unknownoption = somevalue
anotherunknown = 123
`
		tmpDir := t.TempDir()
		iniPath := filepath.Join(tmpDir, "yay.conf")
		require.NoError(t, os.WriteFile(iniPath, []byte(content), 0o644))

		cfg := DefaultConfig("test")
		err := cfg.loadINI(iniPath)
		require.NoError(t, err)

		assert.Equal(t, "https://custom.aur.org", cfg.AURURL)
	})
}

func TestSystemConfigPath(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/etc/yay.conf", SystemConfigPath)
}
