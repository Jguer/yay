package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v10/pkg/settings"
)

func TestInitAlpm(t *testing.T) {
	alpmHandle, pacmanConf, err := initAlpm(settings.MakeArguments(), "testdata/pacman.conf")
	assert.Nil(t, err)
	assert.NotNil(t, pacmanConf)

	h := alpmHandle

	root, err := h.Root()
	assert.Nil(t, err)
	assert.Equal(t, "/", root)

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

	arch, err := h.Arch()
	assert.Nil(t, err)
	assert.Equal(t, "8086", arch)

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
