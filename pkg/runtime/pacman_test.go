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
		RootDir: "/", DBPath: "/var/lib/pacman/",
		CacheDir: []string{"/var/cache/pacman/pkg/"},
		HookDir:  []string{"/etc/pacman.d/hooks/"},
		GPGDir:   "/etc/pacman.d/gnupg/", LogFile: "/var/log/pacman.log",
		HoldPkg: []string{"pacman", "glibc"}, IgnorePkg: []string{"xorm"},
		IgnoreGroup: []string{"yorm"}, Architecture: []string{"x86_64"},
		XferCommand: "/usr/bin/wget --passive-ftp -c -O %o %u",
		NoUpgrade:   []string(nil), NoExtract: []string(nil), CleanMethod: []string{"KeepInstalled"},
		SigLevel:           []string{"PackageRequired", "PackageTrustedOnly", "DatabaseOptional", "DatabaseTrustedOnly"},
		LocalFileSigLevel:  []string{"PackageOptional", "PackageTrustedOnly"},
		RemoteFileSigLevel: []string{"PackageRequired", "PackageTrustedOnly"}, UseSyslog: true,
		Color: true, UseDelta: 0, TotalDownload: false, CheckSpace: true,
		VerbosePkgLists: true, DisableDownloadTimeout: false,
		Repos: []pacmanconf.Repository{
			{
				Name: "core", Servers: []string{"Core"},
				SigLevel: []string(nil), Usage: []string{"All"},
			},
			{
				Name: "extra", Servers: []string{"Extra"}, SigLevel: []string(nil),
				Usage: []string{"All"},
			},
			{
				Name: "multilib", Servers: []string{"repo3", "multilib"},
				SigLevel: []string(nil), Usage: []string{"All"},
			},
		},
	}

	pacmanConf, color, err := retrievePacmanConfig(parser.MakeArguments(), "../../testdata/pacman.conf")
	assert.Nil(t, err)
	assert.NotNil(t, pacmanConf)
	assert.Equal(t, color, false)
	assert.EqualValues(t, expectedPacmanConf, pacmanConf)
}
