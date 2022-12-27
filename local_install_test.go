package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	aur "github.com/Jguer/aur"
	"github.com/Jguer/aur/metadata"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v11/pkg/db/mock"
	mockaur "github.com/Jguer/yay/v11/pkg/dep/mock"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
)

func TestIntegrationLocalInstall(t *testing.T) {
	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	pkgTar := "testdata/yay-91.0.0-1-x86_64.pkg.tar.zst"

	captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
		return pkgTar, "", nil
	}

	showOverride := func(cmd *exec.Cmd) error {
		tars := []string{
			"jellyfin-10.8.4-1-x86_64.pkg.tar.zst",
			"jellyfin-web-10.8.4-1-x86_64.pkg.tar.zst",
			"jellyfin-server-10.8.4-1-x86_64.pkg.tar.zst",
		}
		for _, tar := range tars {
			f, err := os.OpenFile(tar, os.O_RDONLY|os.O_CREATE, 0o666)
			require.NoError(t, err)
			require.NoError(t, f.Close())
		}
		return nil
	}

	mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
	cmdBuilder := &exe.CmdBuilder{
		MakepkgBin:      makepkgBin,
		SudoBin:         "su",
		PacmanBin:       pacmanBin,
		Runner:          mockRunner,
		SudoLoopEnabled: false,
	}

	cmdArgs := parser.MakeArguments()
	cmdArgs.AddArg("B")
	cmdArgs.AddArg("i")
	cmdArgs.AddTarget("testdata/jfin")
	db := &mock.DBExecutor{
		AlpmArchitecturesFn: func() ([]string, error) {
			return []string{"amd64"}, nil
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "dotnet-sdk-6.0", "dotnet-runtime-6.0", "jellyfin-server=10.8.8", "jellyfin-web=10.8.8":
				return false
			}

			return true
		},
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "dotnet-runtime-6.0":
				return &mock.Package{
					PName:    "dotnet-runtime-6.0",
					PBase:    "dotnet-runtime-6.0",
					PVersion: "6.0.100-1",
					PDB:      mock.NewDB("community"),
				}
			case "dotnet-sdk-6.0":
				return &mock.Package{
					PName:    "dotnet-sdk-6.0",
					PBase:    "dotnet-sdk-6.0",
					PVersion: "6.0.100-1",
					PDB:      mock.NewDB("community"),
				}
			}

			return nil
		},
	}

	config := &settings.Configuration{
		Runtime: &settings.Runtime{
			CmdBuilder: cmdBuilder,
			AURCache: &mockaur.MockAUR{
				GetFn: func(ctx context.Context, query *metadata.AURQuery) ([]*aur.Pkg, error) {
					fmt.Println(query.Needles)
					return []*aur.Pkg{}, nil
				},
			},
		},
	}

	err = handleCmd(context.Background(), config, cmdArgs, db)
	require.NoError(t, err)
}
