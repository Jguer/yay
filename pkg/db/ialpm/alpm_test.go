//go:build !integration
// +build !integration

package ialpm

import (
	"io"
	"strings"
	"testing"

	alpm "github.com/Jguer/go-alpm/v2"
	"github.com/Morganamilo/go-pacmanconf"
	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v12/pkg/text"
)

func TestAlpmExecutor(t *testing.T) {
	t.Parallel()
	pacmanConf := &pacmanconf.Config{
		RootDir:                "/",
		DBPath:                 "/var/lib/pacman/",
		CacheDir:               []string{"/cachedir/", "/another/"},
		HookDir:                []string{"/hookdir/"},
		GPGDir:                 "/gpgdir/",
		LogFile:                "/logfile",
		HoldPkg:                []string(nil),
		IgnorePkg:              []string{"ignore", "this", "package"},
		IgnoreGroup:            []string{"ignore", "this", "group"},
		Architecture:           []string{"8086"},
		XferCommand:            "",
		NoUpgrade:              []string{"noupgrade"},
		NoExtract:              []string{"noextract"},
		CleanMethod:            []string{"KeepInstalled"},
		SigLevel:               []string{"PackageOptional", "PackageTrustedOnly", "DatabaseOptional", "DatabaseTrustedOnly"},
		LocalFileSigLevel:      []string(nil),
		RemoteFileSigLevel:     []string(nil),
		UseSyslog:              false,
		Color:                  false,
		UseDelta:               0,
		TotalDownload:          true,
		CheckSpace:             true,
		VerbosePkgLists:        true,
		DisableDownloadTimeout: false,
		Repos: []pacmanconf.Repository{
			{Name: "repo1", Servers: []string{"repo1"}, SigLevel: []string(nil), Usage: []string{"All"}},
			{Name: "repo2", Servers: []string{"repo2"}, SigLevel: []string(nil), Usage: []string{"All"}},
		},
	}

	aExec, err := NewExecutor(pacmanConf, text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test"))
	assert.NoError(t, err)

	assert.NotNil(t, aExec.conf)
	assert.EqualValues(t, pacmanConf, aExec.conf)

	assert.NotNil(t, aExec.localDB)
	assert.NotNil(t, aExec.syncDB)
	assert.NotNil(t, aExec.questionCallback)
	h := aExec.handle
	assert.NotNil(t, h)

	root, err := h.Root()
	assert.Nil(t, err)
	assert.Equal(t, "/", root)

	dbPath, err := h.DBPath()
	assert.Nil(t, err)
	assert.Equal(t, "/var/lib/pacman/", dbPath)

	cache, err := h.CacheDirs()
	assert.Nil(t, err)
	assert.Equal(t, []string{"/cachedir/", "/another/"}, cache.Slice())

	log, err := h.LogFile()
	assert.Nil(t, err)
	assert.Equal(t, "/logfile", log)

	gpg, err := h.GPGDir()
	assert.Nil(t, err)
	assert.Equal(t, "/gpgdir/", gpg)

	hook, err := h.HookDirs()
	assert.Nil(t, err)
	assert.Equal(t, []string{"/usr/share/libalpm/hooks/", "/hookdir/"}, hook.Slice())

	arch, err := alpmTestGetArch(h)
	assert.Nil(t, err)
	assert.Equal(t, []string{"8086"}, arch)

	ignorePkg, err := h.IgnorePkgs()
	assert.Nil(t, err)
	assert.Equal(t, []string{"ignore", "this", "package"}, ignorePkg.Slice())

	ignoreGroup, err := h.IgnoreGroups()
	assert.Nil(t, err)
	assert.Equal(t, []string{"ignore", "this", "group"}, ignoreGroup.Slice())

	noUp, err := h.NoUpgrades()
	assert.Nil(t, err)
	assert.Equal(t, []string{"noupgrade"}, noUp.Slice())

	noEx, err := h.NoExtracts()
	assert.Nil(t, err)
	assert.Equal(t, []string{"noextract"}, noEx.Slice())

	check, err := h.CheckSpace()
	assert.Nil(t, err)
	assert.Equal(t, true, check)
}

func alpmTestGetArch(h *alpm.Handle) ([]string, error) {
	architectures, err := h.GetArchitectures()

	return architectures.Slice(), err
}
