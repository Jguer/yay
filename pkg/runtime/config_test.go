package runtime

import (
	"reflect"
	"testing"

	"github.com/Jguer/yay/v10/pkg/types"
)

func TestConfig(t *testing.T) {
	config := &Configuration{}
	cmdArgs := types.MakeArguments()

	pacmanConf, err := InitPacmanConf(cmdArgs, "../../testdata/pacman.conf")
	if err != nil {
		t.Fatal(err)
	}

	h, err := InitAlpmHandle(config, pacmanConf, nil)
	if err != nil {
		t.Fatal(err)
	}

	root, err := h.Root()
	expect(t, "RootDir", "/", root, err)

	cache, err := h.CacheDirs()
	expect(t, "CacheDir", []string{"/cachedir/", "/another/"}, cache.Slice(), err)

	log, err := h.LogFile()
	expect(t, "LogFile", "/logfile", log, err)

	gpg, err := h.GPGDir()
	expect(t, "GPGDir", "/gpgdir/", gpg, err)

	// Test doesn't work if alpm lib is installed in a non-standard location. Check only for /hookdir/
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

func expect(t *testing.T, field string, a interface{}, b interface{}, err error) {
	if err != nil {
		t.Error(err)
	} else if !reflect.DeepEqual(a, b) {
		t.Errorf("%s expected: %s got %s", field, a, b)
	}
}
