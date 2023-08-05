package srcinfo

import (
	"context"
	"testing"

	gosrc "github.com/Morganamilo/go-srcinfo"

	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v12/pkg/db/mock"
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/vcs"
)

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
	srv := &Service{
		pkgBuildDirs: map[string]string{
			"pkg1": "/path/to/pkg1",
			"pkg2": "/path/to/pkg2",
		},
		srcInfos: map[string]*gosrc.Srcinfo{
			"pkg1": {
				Packages: []gosrc.Package{
					{Pkgname: "pkg1"},
				},
			},
			"pkg2": {
				Packages: []gosrc.Package{
					{Pkgname: "pkg2"},
				},
			},
		},
	}

	err := srv.CheckPGPKeys(context.Background())
	assert.NoError(t, err)
}

func TestService_UpdateVCSStore(t *testing.T) {
	srv := &Service{
		srcInfos: map[string]*gosrc.Srcinfo{
			"pkg1": {
				Packages: []gosrc.Package{
					{Pkgname: "pkg1"},
				},
			},
			"pkg2": {
				Packages: []gosrc.Package{
					{Pkgname: "pkg2"},
				},
			},
		},
		vcsStore: &vcs.Mock{},
	}

	targets := []map[string]*dep.InstallInfo{
		{
			"pkg1": {},
			"pkg2": {},
		},
	}
	ignore := map[string]error{}

	err := srv.UpdateVCSStore(context.Background(), targets, ignore)
	assert.NoError(t, err)
}
