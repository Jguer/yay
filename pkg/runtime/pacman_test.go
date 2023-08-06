//go:build !integration
// +build !integration

package runtime

import (
	"testing"

	"github.com/Morganamilo/go-pacmanconf"
	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v12/pkg/settings/parser"
)

func TestPacmanConf(t *testing.T) {
	t.Parallel()

	expectedPacmanConf := &pacmanconf.Config{
		RootDir:                "/",
		DBPath:                 "//var/lib/pacman/",
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
		TotalDownload:          false,
		CheckSpace:             true,
		VerbosePkgLists:        true,
		DisableDownloadTimeout: false,
		Repos: []pacmanconf.Repository{
			{Name: "repo1", Servers: []string{"repo1"}, SigLevel: []string(nil), Usage: []string{"All"}},
			{Name: "repo2", Servers: []string{"repo2"}, SigLevel: []string(nil), Usage: []string{"All"}},
		},
	}

	pacmanConf, color, err := retrievePacmanConfig(parser.MakeArguments(), "../../testdata/pacman.conf")
	assert.Nil(t, err)
	assert.NotNil(t, pacmanConf)
	assert.Equal(t, color, false)
	assert.EqualValues(t, expectedPacmanConf, pacmanConf)
}
