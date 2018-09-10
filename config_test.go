package main

import (
	"reflect"
	"testing"
)

func expect(t *testing.T, field string, a interface{}, b interface{}, err error) {
	if err != nil {
		t.Error(err)
	} else if !reflect.DeepEqual(a, b) {
		t.Errorf("%s expected: %s got %s", field, a, b)
	}
}

func TestConfig(t *testing.T) {
	config.PacmanConf = "/home/morganamilo/git/yay/testdata/pacman.conf"

	err := initAlpm()
	if err != nil {
		t.Fatal(err)
	}

	h := alpmHandle

	root, err := h.Root()
	expect(t, "RootDir", "/", root, err)

	cache, err := h.CacheDirs()
	expect(t, "CacheDir", []string{"/cachedir/", "/another/"}, cache.Slice(), err)

	log, err := h.LogFile()
	expect(t, "LogFile", "/logfile", log, err)

	gpg, err := h.GPGDir()
	expect(t, "GPGDir", "/gpgdir/", gpg, err)

	hook, err := h.HookDirs()
	expect(t, "HookDir", []string{"/usr/share/libalpm/hooks/", "/hookdir/"}, hook.Slice(), err)

	delta, err := h.DeltaRatio()
	expect(t, "UseDelta", 0.5, delta, err)

	arch, err := h.Arch()
	expect(t, "Architecture", "8086", arch, err)

	ignorePkg, err := h.IgnorePkgs()
	expect(t, "IgnorePkg", []string{"ignore", "this", "package"}, ignorePkg.Slice(), err)

	ignoreGroup, err := h.IgnoreGroups()
	expect(t, "IgnoreGroup", []string{"ignore", "this", "group"}, ignoreGroup.Slice(), err)

	noUp, err := h.NoUpgrades()
	expect(t, "NoUpgrade", []string{"noupgrade"}, noUp.Slice(), err)

	noEx, err := h.NoExtracts()
	expect(t, "NoExtract", []string{"noextract"}, noEx.Slice(), err)

	check, err := h.CheckSpace()
	expect(t, "CheckSpace", true, check, err)
}
