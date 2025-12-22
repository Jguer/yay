//go:build !integration
// +build !integration

package runtime

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Morganamilo/go-pacmanconf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/settings/parser"
)

// normalizePath removes trailing slashes from paths (except for root "/").
func normalizePath(p string) string {
	if p == "/" {
		return p
	}
	return strings.TrimSuffix(p, "/")
}

// normalizePaths normalizes a slice of paths.
func normalizePaths(paths []string) []string {
	result := make([]string, len(paths))
	for i, p := range paths {
		result[i] = normalizePath(p)
	}
	return result
}

// normalizePacmanConf normalizes directory paths in a pacmanconf.Config
// to handle differences between pacman versions (with/without trailing slashes).
func normalizePacmanConf(conf *pacmanconf.Config) {
	conf.RootDir = normalizePath(conf.RootDir)
	conf.DBPath = normalizePath(conf.DBPath)
	conf.GPGDir = normalizePath(conf.GPGDir)
	conf.CacheDir = normalizePaths(conf.CacheDir)
	conf.HookDir = normalizePaths(conf.HookDir)
}

func TestPacmanConf(t *testing.T) {
	t.Parallel()
	path := "../../testdata/pacman.conf"

	absPath, err := filepath.Abs(path)
	require.NoError(t, err)

	// detect the architecture of the system
	expectedArch := []string{"x86_64"}
	if runtime.GOARCH == "arm64" {
		expectedArch = []string{"aarch64"}
	}

	expectedPacmanConf := &pacmanconf.Config{
		RootDir: "/", DBPath: "/var/lib/pacman",
		CacheDir: []string{"/var/cache/pacman/pkg"},
		HookDir:  []string{"/etc/pacman.d/hooks"},
		GPGDir:   "/etc/pacman.d/gnupg", LogFile: "/var/log/pacman.log",
		HoldPkg: []string{"pacman", "glibc"}, IgnorePkg: []string{"xorm"},
		IgnoreGroup: []string{"yorm"}, Architecture: expectedArch,
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

	pacmanConf, color, err := retrievePacmanConfig(parser.MakeArguments(), absPath)
	assert.Nil(t, err)
	assert.NotNil(t, pacmanConf)
	assert.Equal(t, color, false)

	// Normalize paths to handle differences between pacman versions
	// (some versions include trailing slashes, some don't)
	normalizePacmanConf(pacmanConf)
	assert.EqualValues(t, expectedPacmanConf, pacmanConf)
}
