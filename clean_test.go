//go:build !integration
// +build !integration

package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/Jguer/go-alpm/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/db/mock"
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
		PackageOptionalDependsFn: func(i alpm.IPackage) []alpm.Depend {
			if i.Name() == "linux" {
				return []alpm.Depend{
					{
						Name: "linux-headers",
					},
				}
			}

			return []alpm.Depend{}
		},
		PackageProvidesFn: func(p alpm.IPackage) []alpm.Depend { return []alpm.Depend{} },
		PackageDependsFn:  func(p alpm.IPackage) []alpm.Depend { return []alpm.Depend{} },
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

			runtime := &settings.Runtime{CmdBuilder: cmdBuilder, Cfg: &settings.Configuration{}}
			cmdArgs := parser.MakeArguments()
			cmdArgs.AddArg(tc.args...)

			err := handleCmd(context.Background(),
				runtime, cmdArgs, dbExc,
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
