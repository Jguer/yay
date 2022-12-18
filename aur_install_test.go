package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v11/pkg/db/mock"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/vcs"
)

func ptrString(s string) *string {
	return &s
}

func TestInstaller_InstallNeeded(t *testing.T) {
	t.Parallel()

	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	type testCase struct {
		desc        string
		isInstalled bool
		isBuilt     bool
		wantShow    []string
		wantCapture []string
	}

	testCases := []testCase{
		{
			desc:        "not installed and not built",
			isInstalled: false,
			isBuilt:     false,
			wantShow: []string{
				"makepkg --nobuild -fC --ignorearch",
				"makepkg -cf --noconfirm --noextract --noprepare --holdver --ignorearch",
				"pacman -U --needed --config  -- /testdir/yay-91.0.0-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config  -- yay",
			},
			wantCapture: []string{"makepkg --packagelist"},
		},
		{
			desc:        "not installed and built",
			isInstalled: false,
			isBuilt:     true,
			wantShow: []string{
				"makepkg --nobuild -fC --ignorearch",
				"makepkg -c --nobuild --noextract --ignorearch",
				"pacman -U --needed --config  -- /testdir/yay-91.0.0-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config  -- yay",
			},
			wantCapture: []string{"makepkg --packagelist"},
		},
		{
			desc:        "installed",
			isInstalled: true,
			isBuilt:     false,
			wantShow: []string{
				"makepkg --nobuild -fC --ignorearch",
				"makepkg -c --nobuild --noextract --ignorearch",
			},
			wantCapture: []string{"makepkg --packagelist"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(td *testing.T) {
			tmpDir := td.TempDir()
			pkgTar := tmpDir + "/yay-91.0.0-1-x86_64.pkg.tar.zst"

			captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
				return pkgTar, "", nil
			}

			i := 0
			showOverride := func(cmd *exec.Cmd) error {
				i++
				if i == 2 {
					if !tc.isBuilt {
						f, err := os.OpenFile(pkgTar, os.O_RDONLY|os.O_CREATE, 0o666)
						require.NoError(td, err)
						require.NoError(td, f.Close())
					}
				}
				return nil
			}

			// create a mock file
			if tc.isBuilt {
				f, err := os.OpenFile(pkgTar, os.O_RDONLY|os.O_CREATE, 0o666)
				require.NoError(td, err)
				require.NoError(td, f.Close())
			}

			isCorrectInstalledOverride := func(string, string) bool {
				return tc.isInstalled
			}

			mockDB := &mock.DBExecutor{IsCorrectVersionInstalledFunc: isCorrectInstalledOverride}
			mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
			cmdBuilder := &exe.CmdBuilder{
				MakepkgBin:      makepkgBin,
				SudoBin:         "su",
				PacmanBin:       pacmanBin,
				Runner:          mockRunner,
				SudoLoopEnabled: false,
			}

			cmdBuilder.Runner = mockRunner

			installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny)

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddArg("needed")
			cmdArgs.AddTarget("yay")

			pkgBuildDirs := map[string]string{
				"yay": tmpDir,
			}

			srcInfos := map[string]*gosrc.Srcinfo{"yay": {}}

			targets := []map[string]*dep.InstallInfo{
				{
					"yay": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yay"),
					},
				},
			}

			errI := installer.Install(context.Background(), cmdArgs, targets, pkgBuildDirs, srcInfos)
			require.NoError(td, errI)

			require.Len(td, mockRunner.ShowCalls, len(tc.wantShow))
			require.Len(td, mockRunner.CaptureCalls, len(tc.wantCapture))

			for i, call := range mockRunner.ShowCalls {
				show := call.Args[0].(*exec.Cmd).String()
				show = strings.ReplaceAll(show, tmpDir, "/testdir") // replace the temp dir with a static path
				show = strings.ReplaceAll(show, makepkgBin, "makepkg")
				show = strings.ReplaceAll(show, pacmanBin, "pacman")

				// options are in a different order on different systems and on CI root user is used
				assert.Subset(td, strings.Split(show, " "), strings.Split(tc.wantShow[i], " "), show)
			}

			for i, call := range mockRunner.CaptureCalls {
				capture := call.Args[0].(*exec.Cmd).String()
				capture = strings.ReplaceAll(capture, tmpDir, "/testdir") // replace the temp dir with a static path
				capture = strings.ReplaceAll(capture, makepkgBin, "makepkg")
				capture = strings.ReplaceAll(capture, pacmanBin, "pacman")
				assert.Subset(td, strings.Split(capture, " "), strings.Split(tc.wantCapture[i], " "), capture)
			}
		})
	}
}

func TestInstaller_InstallMixedSourcesAndLayers(t *testing.T) {
	t.Parallel()

	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	type testCase struct {
		desc        string
		targets     []map[string]*dep.InstallInfo
		wantShow    []string
		wantCapture []string
	}

	tmpDir := t.TempDir()
	tmpDirJfin := t.TempDir()

	testCases := []testCase{
		{
			desc: "same layer -- different sources",
			wantShow: []string{
				"pacman -S --config /etc/pacman.conf -- core/linux",
				"pacman -D -q --asdeps --config /etc/pacman.conf -- linux",
				"makepkg --nobuild -fC --ignorearch",
				"makepkg -cf --noconfirm --noextract --noprepare --holdver --ignorearch",
				"pacman -U --config /etc/pacman.conf -- /testdir/yay-91.0.0-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config /etc/pacman.conf -- yay",
			},
			wantCapture: []string{"makepkg --packagelist"},
			targets: []map[string]*dep.InstallInfo{
				{
					"yay": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yay"),
					},
					"linux": {
						Source:     dep.Sync,
						Reason:     dep.Dep,
						Version:    "17.0.0-1",
						SyncDBName: ptrString("core"),
					},
				},
			},
		},
		{
			desc: "different layer -- different sources",
			wantShow: []string{
				"pacman -S --config /etc/pacman.conf -- core/linux",
				"pacman -D -q --asdeps --config /etc/pacman.conf -- linux",
				"makepkg --nobuild -fC --ignorearch",
				"makepkg -cf --noconfirm --noextract --noprepare --holdver --ignorearch",
				"pacman -U --config /etc/pacman.conf -- /testdir/yay-91.0.0-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config /etc/pacman.conf -- yay",
			},
			wantCapture: []string{"makepkg --packagelist"},
			targets: []map[string]*dep.InstallInfo{
				{
					"yay": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yay"),
					},
				}, {
					"linux": {
						Source:     dep.Sync,
						Reason:     dep.Dep,
						Version:    "17.0.0-1",
						SyncDBName: ptrString("core"),
					},
				},
			},
		},
		{
			desc: "same layer -- sync",
			wantShow: []string{
				"pacman -S --config /etc/pacman.conf -- extra/linux-zen core/linux",
				"pacman -D -q --asexplicit --config /etc/pacman.conf -- linux-zen linux",
			},
			wantCapture: []string{},
			targets: []map[string]*dep.InstallInfo{
				{
					"linux-zen": {
						Source:     dep.Sync,
						Reason:     dep.Explicit,
						Version:    "18.0.0-1",
						SyncDBName: ptrString("extra"),
					},
					"linux": {
						Source:     dep.Sync,
						Reason:     dep.Explicit,
						Version:    "17.0.0-1",
						SyncDBName: ptrString("core"),
					},
				},
			},
		},
		{
			desc: "same layer -- aur",
			wantShow: []string{
				"makepkg --nobuild -fC --ignorearch",
				"makepkg -cf --noconfirm --noextract --noprepare --holdver --ignorearch",
				"makepkg --nobuild -fC --ignorearch",
				"makepkg -cf --noconfirm --noextract --noprepare --holdver --ignorearch",
				"pacman -U --config /etc/pacman.conf -- pacman -U --config /etc/pacman.conf -- /testdir/yay-91.0.0-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config /etc/pacman.conf -- yay",
			},
			wantCapture: []string{"makepkg --packagelist", "makepkg --packagelist"},
			targets: []map[string]*dep.InstallInfo{
				{
					"yay": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yay"),
					},
					"jellyfin-server": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "10.8.8-1",
						SrcinfoPath: ptrString(tmpDirJfin + "/.SRCINFO"),
						AURBase:     ptrString("jellyfin"),
					},
				},
			},
		},
		{
			desc: "different layer -- aur",
			wantShow: []string{
				"makepkg --nobuild -fC --ignorearch",
				"makepkg -cf --noconfirm --noextract --noprepare --holdver --ignorearch",
				"pacman -U --config /etc/pacman.conf -- pacman -U --config /etc/pacman.conf -- /testdir/jellyfin-server-10.8.8-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asdeps --config /etc/pacman.conf -- jellyfin-server",
				"makepkg --nobuild -fC --ignorearch",
				"makepkg -cf --noconfirm --noextract --noprepare --holdver --ignorearch",
				"pacman -U --config /etc/pacman.conf -- pacman -U --config /etc/pacman.conf -- /testdir/yay-91.0.0-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config /etc/pacman.conf -- yay",
			},
			wantCapture: []string{"makepkg --packagelist", "makepkg --packagelist"},
			targets: []map[string]*dep.InstallInfo{
				{
					"yay": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yay"),
					},
				}, {
					"jellyfin-server": {
						Source:      dep.AUR,
						Reason:      dep.MakeDep,
						Version:     "10.8.8-1",
						SrcinfoPath: ptrString(tmpDirJfin + "/.SRCINFO"),
						AURBase:     ptrString("jellyfin"),
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(td *testing.T) {
			pkgTar := tmpDir + "/yay-91.0.0-1-x86_64.pkg.tar.zst"
			jfinPkgTar := tmpDirJfin + "/jellyfin-server-10.8.8-1-x86_64.pkg.tar.zst"

			captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
				if cmd.Dir == tmpDirJfin {
					return jfinPkgTar, "", nil
				}

				if cmd.Dir == tmpDir {
					return pkgTar, "", nil
				}

				return "", "", fmt.Errorf("unexpected command: %s - %s", cmd.String(), cmd.Dir)
			}

			showOverride := func(cmd *exec.Cmd) error {
				if strings.Contains(cmd.String(), "makepkg -cf --noconfirm") && cmd.Dir == tmpDir {
					f, err := os.OpenFile(pkgTar, os.O_RDONLY|os.O_CREATE, 0o666)
					require.NoError(td, err)
					require.NoError(td, f.Close())
				}

				if strings.Contains(cmd.String(), "makepkg -cf --noconfirm") && cmd.Dir == tmpDirJfin {
					f, err := os.OpenFile(jfinPkgTar, os.O_RDONLY|os.O_CREATE, 0o666)
					require.NoError(td, err)
					require.NoError(td, f.Close())
				}

				return nil
			}
			defer os.Remove(pkgTar)
			defer os.Remove(jfinPkgTar)

			isCorrectInstalledOverride := func(string, string) bool {
				return false
			}

			mockDB := &mock.DBExecutor{IsCorrectVersionInstalledFunc: isCorrectInstalledOverride}
			mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
			cmdBuilder := &exe.CmdBuilder{
				MakepkgBin:       makepkgBin,
				SudoBin:          "su",
				PacmanBin:        pacmanBin,
				PacmanConfigPath: "/etc/pacman.conf",
				Runner:           mockRunner,
				SudoLoopEnabled:  false,
			}

			cmdBuilder.Runner = mockRunner

			installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny)

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddTarget("yay")

			pkgBuildDirs := map[string]string{
				"yay":      tmpDir,
				"jellyfin": tmpDirJfin,
			}

			srcInfos := map[string]*gosrc.Srcinfo{"yay": {}, "jellyfin": {}}

			errI := installer.Install(context.Background(), cmdArgs, tc.targets, pkgBuildDirs, srcInfos)
			require.NoError(td, errI)

			require.Len(td, mockRunner.ShowCalls, len(tc.wantShow))
			require.Len(td, mockRunner.CaptureCalls, len(tc.wantCapture))

			for i, call := range mockRunner.ShowCalls {
				show := call.Args[0].(*exec.Cmd).String()
				show = strings.ReplaceAll(show, tmpDir, "/testdir")     // replace the temp dir with a static path
				show = strings.ReplaceAll(show, tmpDirJfin, "/testdir") // replace the temp dir with a static path
				show = strings.ReplaceAll(show, makepkgBin, "makepkg")
				show = strings.ReplaceAll(show, pacmanBin, "pacman")

				// options are in a different order on different systems and on CI root user is used
				assert.Subset(td, strings.Split(show, " "), strings.Split(tc.wantShow[i], " "), show)
			}

			for i, call := range mockRunner.CaptureCalls {
				capture := call.Args[0].(*exec.Cmd).String()
				capture = strings.ReplaceAll(capture, tmpDir, "/testdir") // replace the temp dir with a static path
				capture = strings.ReplaceAll(capture, tmpDirJfin, "/testdir")
				capture = strings.ReplaceAll(capture, makepkgBin, "makepkg")
				capture = strings.ReplaceAll(capture, pacmanBin, "pacman")
				assert.Subset(td, strings.Split(capture, " "), strings.Split(tc.wantCapture[i], " "), capture)
			}
		})
	}
}

func TestInstaller_RunPostHooks(t *testing.T) {
	mockDB := &mock.DBExecutor{}
	mockRunner := &exe.MockRunner{}
	cmdBuilder := &exe.CmdBuilder{
		MakepkgBin:       "makepkg",
		SudoBin:          "su",
		PacmanBin:        "pacman",
		PacmanConfigPath: "/etc/pacman.conf",
		Runner:           mockRunner,
		SudoLoopEnabled:  false,
	}

	cmdBuilder.Runner = mockRunner

	installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny)

	called := false
	hook := func(ctx context.Context) error {
		called = true
		return nil
	}

	installer.AddPostInstallHook(hook)
	installer.RunPostInstallHooks(context.Background())

	assert.True(t, called)
}

func TestInstaller_CompileFailed(t *testing.T) {
	t.Parallel()

	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	type testCase struct {
		desc      string
		targets   []map[string]*dep.InstallInfo
		lastLayer bool
	}

	tmpDir := t.TempDir()

	testCases := []testCase{
		{
			desc:      "last layer",
			lastLayer: true,
			targets: []map[string]*dep.InstallInfo{
				{
					"yay": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yay"),
					},
				},
			},
		},
		{
			desc:      "not last layer",
			lastLayer: false,
			targets: []map[string]*dep.InstallInfo{
				{"bob": {}},
				{
					"yay": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yay"),
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(td *testing.T) {
			pkgTar := tmpDir + "/yay-91.0.0-1-x86_64.pkg.tar.zst"

			captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
				return pkgTar, "", nil
			}

			showOverride := func(cmd *exec.Cmd) error {
				if strings.Contains(cmd.String(), "makepkg -cf --noconfirm") && cmd.Dir == tmpDir {
					return errors.New("makepkg failed")
				}
				return nil
			}

			isCorrectInstalledOverride := func(string, string) bool {
				return false
			}

			mockDB := &mock.DBExecutor{IsCorrectVersionInstalledFunc: isCorrectInstalledOverride}
			mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
			cmdBuilder := &exe.CmdBuilder{
				MakepkgBin:      makepkgBin,
				SudoBin:         "su",
				PacmanBin:       pacmanBin,
				Runner:          mockRunner,
				SudoLoopEnabled: false,
			}

			cmdBuilder.Runner = mockRunner

			installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny)

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddArg("needed")
			cmdArgs.AddTarget("yay")

			pkgBuildDirs := map[string]string{
				"yay": tmpDir,
			}

			srcInfos := map[string]*gosrc.Srcinfo{"yay": {}}
			errI := installer.Install(context.Background(), cmdArgs, tc.targets, pkgBuildDirs, srcInfos)
			if tc.lastLayer {
				require.NoError(td, errI) // last layer error
			} else {
				require.Error(td, errI)
			}
			err := installer.CompileFailedAndIgnored()
			if tc.lastLayer {
				require.Error(td, err)
				assert.ErrorContains(td, err, "yay")
			} else {
				require.NoError(td, err)
			}
		})
	}
}
