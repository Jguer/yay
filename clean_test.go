//go:build !integration
// +build !integration

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	alpm "github.com/Jguer/dyalpm"
	pacmanconf "github.com/Morganamilo/go-pacmanconf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/db/mock"
	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
)

func TestCleanHanging(t *testing.T) {
	pacmanBin := t.TempDir() + "/pacman"

	t.Parallel()

	testCases := []struct {
		name     string
		args     []string
		wantShow []string
	}{
		{
			name:     "clean",
			args:     []string{"Y", "c"},
			wantShow: []string{"pacman", "-R", "-s", "-u", "--config", "/etc/pacman.conf", "--", "lsp-plugins"},
		},
		{
			name:     "clean double",
			args:     []string{"Y", "c", "c"},
			wantShow: []string{"pacman", "-R", "-s", "-u", "--config", "/etc/pacman.conf", "--", "lsp-plugins", "linux-headers"},
		},
	}

	dbExc := &mock.DBExecutor{
		PackageOptionalDependsFn: func(i alpm.Package) []alpm.Depend {
			if i.Name() == "linux" {
				return []alpm.Depend{
					{
						Name: "linux-headers",
					},
				}
			}

			return []alpm.Depend{}
		},
		PackageProvidesFn: func(p alpm.Package) []alpm.Depend { return []alpm.Depend{} },
		PackageDependsFn:  func(p alpm.Package) []alpm.Depend { return []alpm.Depend{} },
		LocalPackagesFn: func() []mock.IPackage {
			return []mock.IPackage{
				&mock.Package{
					PReason: alpm.PkgReasonExplicit,
					PName:   "linux",
				},
				&mock.Package{
					PReason: alpm.PkgReasonDepend,
					PName:   "lsp-plugins",
				},
				&mock.Package{
					PReason: alpm.PkgReasonDepend,
					PName:   "linux-headers",
				},
			}
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRunner := &exe.MockRunner{
				CaptureFn: func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
					return "", "", nil
				},
				ShowFn: func(cmd *exec.Cmd) error { return nil },
			}
			cmdBuilder := &exe.CmdBuilder{
				SudoBin:          "su",
				PacmanBin:        pacmanBin,
				PacmanConfigPath: "/etc/pacman.conf",
				GitBin:           "git",
				Runner:           mockRunner,
				SudoLoopEnabled:  false,
			}

			run := &runtime.Runtime{CmdBuilder: cmdBuilder, Cfg: &settings.Configuration{}}
			cmdArgs := parser.MakeArguments()
			cmdArgs.AddArg(tc.args...)

			err := handleCmd(context.Background(),
				run, cmdArgs, dbExc,
			)

			require.NoError(t, err)

			for i, call := range mockRunner.ShowCalls {
				show := call.Args[0].(*exec.Cmd).String()
				show = strings.ReplaceAll(show, pacmanBin, "pacman")

				// options are in a different order on different systems and on CI root user is used
				assert.Subset(t, strings.Split(show, " "),
					strings.Split(tc.wantShow[i], " "),
					fmt.Sprintf("%d - %s", i, show))
			}
		})
	}
}

func TestIntegrationCleanAUR(t *testing.T) {
	buildDir := filepath.Join(t.TempDir(), "build")
	yayGitDir := filepath.Join(buildDir, "yay-git")
	zoomDir := filepath.Join(buildDir, "zoom")

	t.Parallel()

	testCases := []struct {
		name     string
		args     []string
		wantDirs []string
	}{
		{
			name:     "Sync clean AUR",
			args:     []string{"S", "c", "a"},
			wantDirs: []string{"zoom"},
		},
		{
			name:     "Sync clean double AUR",
			args:     []string{"S", "c", "c", "a"},
			wantDirs: []string{},
		},
	}

	dbExc := &mock.DBExecutor{
		PackageOptionalDependsFn: func(i alpm.Package) []alpm.Depend {
			if i.Name() == "linux" {
				return []alpm.Depend{
					{
						Name: "linux-headers",
					},
				}
			}

			return []alpm.Depend{}
		},
		PackageProvidesFn: func(p alpm.Package) []alpm.Depend { return []alpm.Depend{} },
		PackageDependsFn:  func(p alpm.Package) []alpm.Depend { return []alpm.Depend{} },
		InstalledRemotePackagesFn: func() map[string]alpm.Package {
			return map[string]alpm.Package{
				"zoom": &mock.Package{
					PName:    "zoom",
					PVersion: "6.5.8-1",
					PBase:    "zoom",
					PReason:  alpm.PkgReasonExplicit,
				},
			}
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRunner := &exe.MockRunner{}

			cfg := &settings.Configuration{
				BuildDir: buildDir,
				Mode:     parser.ModeAUR,
			}
			pacmanConf := &pacmanconf.Config{
				// Only testing the keep installed clean method right now
				CleanMethod: []string{"KeepInstalled"},
			}
			run := &runtime.Runtime{
				Cfg:        cfg,
				PacmanConf: pacmanConf,
				Logger:     newTestLogger(),
			}
			cmdArgs := parser.MakeArguments()
			cmdArgs.AddArg(tc.args...)

			// Create the package directories to be cleaned
			err := os.MkdirAll(yayGitDir, 0o755)
			require.NoError(t, err)
			err = os.MkdirAll(zoomDir, 0o755)
			require.NoError(t, err)

			err = handleCmd(context.Background(),
				run, cmdArgs, dbExc,
			)
			require.NoError(t, err)

			// This should only test AUR cleaning, so no calls to an external command should be made
			assert.Len(t, mockRunner.ShowCalls, 0)

			// Make sure the directories left after cleaning are the only ones we expect
			packageDirs, err := os.ReadDir(buildDir)
			require.NoError(t, err)

			var packageDirNames []string
			for _, dir := range packageDirs {
				packageDirNames = append(packageDirNames, dir.Name())
			}

			assert.ElementsMatch(t, tc.wantDirs, packageDirNames)
		})
	}
}
