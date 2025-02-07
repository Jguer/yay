//go:build !integration
// +build !integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Jguer/yay/v12/pkg/db/mock"
	mockaur "github.com/Jguer/yay/v12/pkg/dep/mock"
	"github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"

	"github.com/Jguer/aur"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getFromFile(t *testing.T, filePath string) mockaur.GetFunc {
	f, err := os.Open(filePath)
	require.NoError(t, err)

	fBytes, err := io.ReadAll(f)
	require.NoError(t, err)

	pkgs := []aur.Pkg{}
	err = json.Unmarshal(fBytes, &pkgs)
	require.NoError(t, err)

	return func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
		return pkgs, nil
	}
}

func TestSyncInfo(t *testing.T) {
	t.Parallel()
	pacmanBin := t.TempDir() + "/pacman"

	testCases := []struct {
		name     string
		args     []string
		targets  []string
		wantShow []string
		wantErr  bool
	}{
		{
			name:     "Si linux",
			args:     []string{"S", "i"},
			targets:  []string{"linux"},
			wantShow: []string{"pacman", "-S", "-i", "--config", "/etc/pacman.conf", "--", "linux"},
		},
		{
			name:     "Si jellyfin",
			args:     []string{"S", "i"},
			targets:  []string{"jellyfin"},
			wantShow: []string{},
		},
		{
			name:     "Si linux jellyfin",
			args:     []string{"S", "i"},
			targets:  []string{"linux", "jellyfin"},
			wantShow: []string{"pacman", "-S", "-i", "--config", "/etc/pacman.conf", "--", "linux"},
		},
		{
			name:     "Si jellyfin",
			args:     []string{"S", "i"},
			targets:  []string{"jellyfin"},
			wantShow: []string{},
		},
		{
			name:     "Si missing",
			args:     []string{"S", "i"},
			targets:  []string{"missing"},
			wantShow: []string{},
			wantErr:  true,
		},
		{
			name:     "Si arduino",
			args:     []string{"S", "i"},
			targets:  []string{"arduino"},
			wantShow: []string{"pacman", "-S", "-i", "--config", "/etc/pacman.conf", "--", "arduino-cli"},
		},
	}

	dbExc := &mock.DBExecutor{
		SyncSatisfierFn: func(s string) mock.IPackage {
			if s == "linux" {
				return &mock.Package{
					PName: "linux",
					PBase: "linux",
				}
			}
			if s == "arduino-cli" {
				return &mock.Package{
					PName: "arduino-cli",
					PBase: "arduino-cli",
				}
			}
			return nil
		},
		PackagesFromGroupFn: func(s string) []mock.IPackage {
			if s == "arduino" {
				return []mock.IPackage{
					&mock.Package{PName: "arduino-cli"},
				}
			}
			return nil
		},
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
		if query.Needles[0] == "jellyfin" {
			jfinFn := getFromFile(t, "pkg/dep/testdata/jellyfin.json")
			return jfinFn(ctx, query)
		}

		return nil, fmt.Errorf("not found")
	}}

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

			run := &runtime.Runtime{
				CmdBuilder: cmdBuilder,
				AURClient:  mockAUR,
				Logger:     newTestLogger(),
				Cfg:        &settings.Configuration{},
			}

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddArg(tc.args...)
			cmdArgs.AddTarget(tc.targets...)

			err := handleCmd(context.Background(),
				run, cmdArgs, dbExc,
			)

			if tc.wantErr {
				require.Error(t, err)
				assert.EqualError(t, err, "")
			} else {
				require.NoError(t, err)
			}
			if len(tc.wantShow) == 0 {
				assert.Empty(t, mockRunner.ShowCalls)
				return
			}
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

// Should not error when there is a DB called aur
func TestSyncSearchAURDB(t *testing.T) {
	t.Parallel()

	pacmanBin := t.TempDir() + "/pacman"
	testCases := []struct {
		name       string
		args       []string
		targets    []string
		wantShow   []string
		wantErr    bool
		bottomUp   bool
		singleLine bool
		mixed      bool
	}{
		{
			name:     "Ss jellyfin false false",
			args:     []string{"S", "s"},
			targets:  []string{"jellyfin"},
			wantShow: []string{},
		},
		{
			name:       "Ss jellyfin true false",
			args:       []string{"S", "s"},
			targets:    []string{"jellyfin"},
			wantShow:   []string{},
			singleLine: true,
		},
		{
			name:       "Ss jellyfin true true",
			args:       []string{"S", "s"},
			targets:    []string{"jellyfin"},
			wantShow:   []string{},
			singleLine: true,
			mixed:      true,
		},
		{
			name:       "Ss jellyfin false true",
			args:       []string{"S", "s"},
			targets:    []string{"jellyfin"},
			wantShow:   []string{},
			singleLine: false,
			mixed:      true,
		},
		{
			name:       "Ss jellyfin true true - bottomup",
			args:       []string{"S", "s"},
			targets:    []string{"jellyfin"},
			wantShow:   []string{},
			singleLine: true,
			mixed:      true,
			bottomUp:   true,
		},
	}

	dbExc := &mock.DBExecutor{
		SyncPackagesFn: func(s ...string) []mock.IPackage {
			return []mock.IPackage{
				&mock.Package{
					PName: "jellyfin",
					PBase: "jellyfin",
					PDB:   mock.NewDB("aur"),
				},
			}
		},
		LocalPackageFn: func(s string) mock.IPackage {
			return &mock.Package{
				PName: "jellyfin",
				PBase: "jellyfin",
				PDB:   mock.NewDB("aur"),
			}
		},
		PackagesFromGroupFn: func(s string) []mock.IPackage {
			return nil
		},
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
		if query.Needles[0] == "jellyfin" {
			jfinFn := getFromFile(t, "pkg/dep/testdata/jellyfin.json")
			return jfinFn(ctx, query)
		}

		return nil, fmt.Errorf("not found")
	}}

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

			run := &runtime.Runtime{
				CmdBuilder: cmdBuilder,
				AURClient:  mockAUR,
				QueryBuilder: query.NewSourceQueryBuilder(mockAUR, newTestLogger(), "votes", parser.ModeAny, "name",
					tc.bottomUp, tc.singleLine, tc.mixed),
				Logger: newTestLogger(),
				Cfg:    &settings.Configuration{},
			}

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddArg(tc.args...)
			cmdArgs.AddTarget(tc.targets...)

			err := handleCmd(context.Background(),
				run, cmdArgs, dbExc,
			)

			if tc.wantErr {
				require.Error(t, err)
				assert.EqualError(t, err, "")
			} else {
				require.NoError(t, err)
			}
			if len(tc.wantShow) == 0 {
				assert.Empty(t, mockRunner.ShowCalls)
				return
			}
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
