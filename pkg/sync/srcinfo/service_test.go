package srcinfo

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"

	gosrc "github.com/Morganamilo/go-srcinfo"

	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v13/pkg/db/mock"
	"github.com/Jguer/yay/v13/pkg/dep"
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/settings/exe"
	"github.com/Jguer/yay/v13/pkg/text"
	"github.com/Jguer/yay/v13/pkg/vcs"
)

func newTestLogger() *text.Logger {
	return text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
}

func TestNewService(t *testing.T) {
	dbExecutor := &mock.DBExecutor{}
	cfg := &settings.Configuration{}
	cmdBuilder := &exe.MockBuilder{}
	vcsStore := &vcs.Mock{}
	pkgBuildDirs := map[string]string{
		"jellyfin": "../../../testdata/jfin",
		"cephbin":  "../../../testdata/cephbin",
	}

	srv, err := NewService(dbExecutor, cfg, newTestLogger(), cmdBuilder, vcsStore, pkgBuildDirs)
	assert.NoError(t, err)
	assert.NotNil(t, srv)
	assert.Equal(t, dbExecutor, srv.dbExecutor)
	assert.Equal(t, cfg, srv.cfg)
	assert.Equal(t, cmdBuilder, srv.cmdBuilder)
	assert.Equal(t, vcsStore, srv.vcsStore)
	assert.Equal(t, pkgBuildDirs, srv.pkgBuildDirs)
	assert.NotNil(t, srv.srcInfos)
}

func TestService_IncompatiblePkgs(t *testing.T) {
	srv := &Service{
		dbExecutor: &mock.DBExecutor{AlpmArchitecturesFn: func() ([]string, error) {
			return []string{"x86_64"}, nil
		}},
		srcInfos: map[string]*gosrc.Srcinfo{
			"pkg1": {
				Package: gosrc.Package{
					Arch: []string{"x86_64", "any"},
				},
			},
			"pkg2": {
				Package: gosrc.Package{
					Arch: []string{"any"},
				},
			},
			"pkg3": {
				Package: gosrc.Package{
					Arch: []string{"armv7h"},
				},
			},
			"pkg4": {
				Package: gosrc.Package{
					Arch: []string{"i683", "x86_64"},
				},
			},
		},
	}

	incompatible, err := srv.IncompatiblePkgs(context.Background())
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"pkg3"}, incompatible)
}

func TestService_CheckPGPKeys(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		fetch      bool
		keyExists  bool
		importFail bool
		wantCalls  []string
	}{
		{name: "disabled"},
		{name: "enabled with existing key", fetch: true, keyExists: true, wantCalls: []string{"--list-keys"}},
		{name: "enabled with missing key", fetch: true, wantCalls: []string{"--list-keys", "--recv-keys"}},
		{name: "import failure", fetch: true, importFail: true, wantCalls: []string{"--list-keys", "--recv-keys"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var calls []string
			runner := &exe.MockRunner{ShowFn: func(cmd *exec.Cmd) error {
				calls = append(calls, cmd.Args[1])
				if cmd.Args[1] == "--list-keys" && !tc.keyExists || cmd.Args[1] == "--recv-keys" && tc.importFail {
					return errors.New("gpg failed")
				}
				return nil
			}}
			srv := &Service{
				cfg:          &settings.Configuration{PGPFetch: tc.fetch},
				log:          newTestLogger(),
				cmdBuilder:   &exe.MockBuilder{Runner: runner},
				pkgBuildDirs: map[string]string{"pkg1": "/path/to/pkg1"},
				srcInfos: map[string]*gosrc.Srcinfo{
					"pkg1": {PackageBase: gosrc.PackageBase{ValidPGPKeys: []string{"1234567890ABCDEF"}}},
				},
			}

			err := srv.CheckPGPKeys(context.Background())
			if tc.importFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.wantCalls, calls)
		})
	}
}

func TestService_UpdateVCSStore(t *testing.T) {
	vcsStore := &vcs.Mock{}
	srv := &Service{
		log: newTestLogger(),
		srcInfos: map[string]*gosrc.Srcinfo{
			"pkg1": {
				Packages: []gosrc.Package{
					{Pkgname: "pkg1"},
				},
				PackageBase: gosrc.PackageBase{
					Source: []gosrc.ArchString{{Value: "git+https://example.com/pkg1.git"}},
				},
			},
			"pkg2": {
				Packages: []gosrc.Package{
					{Pkgname: "pkg2"},
				},
				PackageBase: gosrc.PackageBase{
					Source: []gosrc.ArchString{{Value: "git+https://example.com/pkg2.git"}},
				},
			},
		},
		vcsStore: vcsStore,
	}

	targets := []map[string]*dep.InstallInfo{
		{
			"pkg1": {},
			"pkg2": {},
		},
		{
			"pkg1": {},
		},
	}
	ignore := map[string]error{}

	err := srv.UpdateVCSStore(context.Background(), targets, ignore)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"pkg1", "pkg2"}, vcsStore.Updates)
}
