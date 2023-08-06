//go:build !integration
// +build !integration

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Jguer/aur"
	alpm "github.com/Jguer/go-alpm/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/db/mock"
	mockaur "github.com/Jguer/yay/v12/pkg/dep/mock"
	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/vcs"
)

func TestPrintUpdateList(t *testing.T) {
	// The current method of capturing os.Stdout hinders parallelization.
	// Setting of global settings.NoConfirm in printUpdateList also hinders parallelization.
	// t.Parallel()
	pacmanBin := t.TempDir() + "/pacman"
	f, err := os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	mockDBName := mock.NewDB("core")
	mockDB := &mock.DBExecutor{
		AlpmArchitecturesFn: func() ([]string, error) {
			return []string{"x86_64"}, nil
		},
		RefreshHandleFn: func() error {
			return nil
		},
		ReposFn: func() []string {
			return []string{"core"}
		},
		InstalledRemotePackagesFn: func() map[string]alpm.IPackage {
			return map[string]alpm.IPackage{
				"vosk-api": &mock.Package{
					PName:    "vosk-api",
					PVersion: "0.3.43-1",
					PBase:    "vosk-api",
					PReason:  alpm.PkgReasonExplicit,
				},
			}
		},
		InstalledRemotePackageNamesFn: func() []string {
			return []string{"vosk-api"}
		},
		SyncUpgradesFn: func(
			bool,
		) (map[string]db.SyncUpgrade, error) {
			return map[string]db.SyncUpgrade{
				"linux": {
					Package: &mock.Package{
						PName:    "linux",
						PVersion: "5.10.0",
						PDB:      mockDBName,
					},
					LocalVersion: "4.3.0",
					Reason:       alpm.PkgReasonExplicit,
				},
				"go": {
					Package: &mock.Package{
						PName:    "go",
						PVersion: "2:1.20.4-1",
						PDB:      mockDBName,
					},
					LocalVersion: "2:1.20.3-1",
					Reason:       alpm.PkgReasonExplicit,
				},
			}, nil
		},
		LocalPackageFn: func(s string) mock.IPackage {
			if s == "no-update-pkg" {
				return &mock.Package{
					PName:    "no-update-pkg",
					PVersion: "3.3.3",
					PDB:      mockDBName,
				}
			}
			return nil
		},
		SetLoggerFn: func(logger *text.Logger) {},
	}

	mockDBNoUpdates := &mock.DBExecutor{
		AlpmArchitecturesFn: func() ([]string, error) {
			return []string{"x86_64"}, nil
		},
		RefreshHandleFn: func() error {
			return nil
		},
		ReposFn: func() []string {
			return []string{"core"}
		},
		InstalledRemotePackagesFn: func() map[string]alpm.IPackage {
			return map[string]alpm.IPackage{
				"vosk-api": &mock.Package{
					PName:    "vosk-api",
					PVersion: "0.3.43-1",
					PBase:    "vosk-api",
					PReason:  alpm.PkgReasonExplicit,
				},
			}
		},
		InstalledRemotePackageNamesFn: func() []string {
			return []string{"vosk-api"}
		},
		SyncUpgradesFn: func(
			bool,
		) (map[string]db.SyncUpgrade, error) {
			return map[string]db.SyncUpgrade{}, nil
		},
		LocalPackageFn: func(s string) mock.IPackage {
			return nil
		},
		SetLoggerFn: func(logger *text.Logger) {},
	}

	mockAUR := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{
				{
					Name:        "vosk-api",
					PackageBase: "vosk-api",
					Version:     "0.3.45-1",
				},
			}, nil
		},
	}

	mockAURNoUpdates := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{
				{
					Name:        "vosk-api",
					PackageBase: "vosk-api",
					Version:     "0.3.43-1",
				},
			}, nil
		},
	}

	type mockData struct {
		db       *mock.DBExecutor
		aurCache *mockaur.MockAUR
	}

	testCases := []struct {
		name     string
		mockData mockData
		args     []string
		targets  []string
		wantPkgs []string
		wantErr  bool
	}{
		{
			name:     "Qu",
			mockData: mockData{mockDB, mockAUR},
			args:     []string{"Q", "u"},
			targets:  []string{},
			wantPkgs: []string{
				fmt.Sprintf("%s %s -> %s",
					text.Bold("linux"),
					text.Bold(text.Green("4.3.0")),
					text.Bold(text.Green("5.10.0")),
				),
				fmt.Sprintf("%s %s -> %s",
					text.Bold("go"),
					text.Bold(text.Green("2:1.20.3-1")),
					text.Bold(text.Green("2:1.20.4-1")),
				),
				fmt.Sprintf("%s %s -> %s",
					text.Bold("vosk-api"),
					text.Bold(text.Green("0.3.43-1")),
					text.Bold(text.Green("0.3.45-1")),
				),
			},
		},
		{
			name:     "Quq",
			mockData: mockData{mockDB, mockAUR},
			args:     []string{"Q", "u", "q"},
			targets:  []string{},
			wantPkgs: []string{"linux", "go", "vosk-api"},
		},
		{
			name:     "Quq linux",
			mockData: mockData{mockDB, mockAUR},
			args:     []string{"Q", "u", "q"},
			targets:  []string{"linux"},
			wantPkgs: []string{"linux"},
		},
		{
			name:     "Qunq",
			mockData: mockData{mockDB, mockAUR},
			args:     []string{"Q", "u", "n", "q"},
			targets:  []string{},
			wantPkgs: []string{"linux", "go"},
		},
		{
			name:     "Qumq",
			mockData: mockData{mockDB, mockAUR},
			args:     []string{"Q", "u", "m", "q"},
			targets:  []string{},
			wantPkgs: []string{"vosk-api"},
		},
		{
			name:     "Quq no-update-pkg",
			mockData: mockData{mockDB, mockAUR},
			args:     []string{"Q", "u", "q"},
			targets:  []string{"no-update-pkg"},
			wantPkgs: []string{},
			wantErr:  true,
		},
		{
			name:     "Quq non-existent-pkg",
			mockData: mockData{mockDB, mockAUR},
			args:     []string{"Q", "u", "q"},
			targets:  []string{"non-existent-pkg"},
			wantPkgs: []string{},
			wantErr:  true,
		},
		{
			name:     "Qu no-updates-any",
			mockData: mockData{mockDBNoUpdates, mockAURNoUpdates},
			args:     []string{"Q", "u"},
			targets:  []string{},
			wantPkgs: []string{},
			wantErr:  true,
		},
		{
			name:     "Qun no-updates-native",
			mockData: mockData{mockDBNoUpdates, mockAUR},
			args:     []string{"Q", "u", "n"},
			targets:  []string{},
			wantPkgs: []string{},
			wantErr:  true,
		},
		{
			name:     "Qum no-updates-foreign",
			mockData: mockData{mockDB, mockAURNoUpdates},
			args:     []string{"Q", "u", "m"},
			targets:  []string{},
			wantPkgs: []string{},
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmdBuilder := &exe.CmdBuilder{
				SudoBin:          "su",
				PacmanBin:        pacmanBin,
				PacmanConfigPath: "/etc/pacman.conf",
				Runner:           &exe.MockRunner{},
				SudoLoopEnabled:  false,
			}

			r, w, _ := os.Pipe()

			logger := text.NewLogger(w, io.Discard, strings.NewReader(""), true, "test")

			run := &runtime.Runtime{
				Cfg: &settings.Configuration{
					RemoveMake: "no",
				},
				Logger:     logger,
				CmdBuilder: cmdBuilder,
				VCSStore:   &vcs.Mock{},
				AURClient:  tc.mockData.aurCache,
			}

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddArg(tc.args...)
			cmdArgs.AddTarget(tc.targets...)

			err = handleCmd(context.Background(), run, cmdArgs, tc.mockData.db)

			w.Close()
			out, _ := io.ReadAll(r)

			if tc.wantErr {
				require.Error(t, err)
				assert.EqualError(t, err, "")
			} else {
				require.NoError(t, err)
			}

			outStr := string(out)
			outPkgs := make([]string, 0)
			if outStr != "" {
				outPkgs = strings.Split(strings.TrimSuffix(outStr, "\n"), "\n")
			}

			assert.ElementsMatch(t, outPkgs, tc.wantPkgs, "Lists of packages should match")
		})
	}
}
