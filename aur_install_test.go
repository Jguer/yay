package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v11/pkg/db/mock"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/text"
	"github.com/Jguer/yay/v11/pkg/vcs"
)

var testLogger = text.NewLogger(io.Discard, strings.NewReader(""), true, "test")

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

			mockDB := &mock.DBExecutor{IsCorrectVersionInstalledFn: isCorrectInstalledOverride}
			mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
			cmdBuilder := &exe.CmdBuilder{
				MakepkgBin:      makepkgBin,
				SudoBin:         "su",
				PacmanBin:       pacmanBin,
				Runner:          mockRunner,
				SudoLoopEnabled: false,
			}

			cmdBuilder.Runner = mockRunner

			installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny, false, testLogger)

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddArg("needed")
			cmdArgs.AddTarget("yay")

			pkgBuildDirs := map[string]string{
				"yay": tmpDir,
			}

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

			errI := installer.Install(context.Background(), cmdArgs, targets, pkgBuildDirs, []string{})
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

			mockDB := &mock.DBExecutor{IsCorrectVersionInstalledFn: isCorrectInstalledOverride}
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

			installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny, false, testLogger)

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddTarget("yay")

			pkgBuildDirs := map[string]string{
				"yay":      tmpDir,
				"jellyfin": tmpDirJfin,
			}

			errI := installer.Install(context.Background(), cmdArgs, tc.targets, pkgBuildDirs, []string{})
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

	installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny, false, testLogger)

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
		desc           string
		targets        []map[string]*dep.InstallInfo
		wantErrInstall bool
		wantErrCompile bool
		failBuild      bool
		failPkgInstall bool
	}

	tmpDir := t.TempDir()

	testCases := []testCase{
		{
			desc:           "one layer",
			wantErrInstall: false,
			wantErrCompile: true,
			failBuild:      true,
			failPkgInstall: false,
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
			desc:           "one layer -- fail install",
			wantErrInstall: true,
			wantErrCompile: false,
			failBuild:      false,
			failPkgInstall: true,
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
			desc:           "two layers",
			wantErrInstall: false,
			wantErrCompile: true,
			failBuild:      true,
			failPkgInstall: false,
			targets: []map[string]*dep.InstallInfo{
				{"bob": {
					AURBase: ptrString("yay"),
				}},
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
				if tc.failBuild && strings.Contains(cmd.String(), "makepkg -cf --noconfirm") && cmd.Dir == tmpDir {
					return errors.New("makepkg failed")
				}
				return nil
			}

			isCorrectInstalledOverride := func(string, string) bool {
				return false
			}

			mockDB := &mock.DBExecutor{IsCorrectVersionInstalledFn: isCorrectInstalledOverride}
			mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
			cmdBuilder := &exe.CmdBuilder{
				MakepkgBin:      makepkgBin,
				SudoBin:         "su",
				PacmanBin:       pacmanBin,
				Runner:          mockRunner,
				SudoLoopEnabled: false,
			}

			cmdBuilder.Runner = mockRunner

			installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny, false, testLogger)

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddArg("needed")
			cmdArgs.AddTarget("yay")

			pkgBuildDirs := map[string]string{
				"yay": tmpDir,
			}

			errI := installer.Install(context.Background(), cmdArgs, tc.targets, pkgBuildDirs, []string{})
			if tc.wantErrInstall {
				require.Error(td, errI)
			} else {
				require.NoError(td, errI)
			}
			err := installer.CompileFailedAndIgnored()
			if tc.wantErrCompile {
				require.Error(td, err)
				assert.ErrorContains(td, err, "yay")
			} else {
				require.NoError(td, err)
			}
		})
	}
}

func TestInstaller_InstallSplitPackage(t *testing.T) {
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
		wantShow    []string
		wantCapture []string
		targets     []map[string]*dep.InstallInfo
	}

	tmpDir := t.TempDir()

	testCases := []testCase{
		{
			desc: "jellyfin",
			targets: []map[string]*dep.InstallInfo{
				{"jellyfin": {
					Source:      dep.AUR,
					Reason:      dep.Explicit,
					Version:     "10.8.4-1",
					SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
					AURBase:     ptrString("jellyfin"),
				}},
				{
					"jellyfin-server": {
						Source:      dep.AUR,
						Reason:      dep.Dep,
						Version:     "10.8.4-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("jellyfin"),
					},
					"jellyfin-web": {
						Source:      dep.AUR,
						Reason:      dep.Dep,
						Version:     "10.8.4-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("jellyfin"),
					},
				},
				{
					"dotnet-runtime-6.0": {
						Source:     dep.Sync,
						Reason:     dep.Dep,
						Version:    "6.0.12.sdk112-1",
						SyncDBName: ptrString("community"),
					},
					"aspnet-runtime": {
						Source:     dep.Sync,
						Reason:     dep.Dep,
						Version:    "6.0.12.sdk112-1",
						SyncDBName: ptrString("community"),
					},
					"dotnet-sdk-6.0": {
						Source:     dep.Sync,
						Reason:     dep.MakeDep,
						Version:    "6.0.12.sdk112-1",
						SyncDBName: ptrString("community"),
					},
				},
			},
			wantShow: []string{
				"pacman -S --config /etc/pacman.conf -- community/dotnet-runtime-6.0 community/aspnet-runtime community/dotnet-sdk-6.0",
				"pacman -D -q --asdeps --config /etc/pacman.conf -- dotnet-runtime-6.0 aspnet-runtime dotnet-sdk-6.0",
				"makepkg --nobuild -fC --ignorearch",
				"makepkg -cf --noconfirm --noextract --noprepare --holdver --ignorearch",
				"makepkg --nobuild -fC --ignorearch",
				"makepkg -c --nobuild --noextract --ignorearch",
				"pacman -U --config /etc/pacman.conf -- /testdir/jellyfin-web-10.8.4-1-x86_64.pkg.tar.zst /testdir/jellyfin-server-10.8.4-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asdeps --config /etc/pacman.conf -- jellyfin-server jellyfin-web",
				"makepkg --nobuild -fC --ignorearch",
				"makepkg -c --nobuild --noextract --ignorearch",
				"pacman -U --config /etc/pacman.conf -- /testdir/jellyfin-10.8.4-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config /etc/pacman.conf -- jellyfin",
			},
			wantCapture: []string{"makepkg --packagelist", "makepkg --packagelist", "makepkg --packagelist"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(td *testing.T) {
			pkgTars := []string{
				tmpDir + "/jellyfin-10.8.4-1-x86_64.pkg.tar.zst",
				tmpDir + "/jellyfin-web-10.8.4-1-x86_64.pkg.tar.zst",
				tmpDir + "/jellyfin-server-10.8.4-1-x86_64.pkg.tar.zst",
			}

			captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
				return strings.Join(pkgTars, "\n"), "", nil
			}

			i := 0
			showOverride := func(cmd *exec.Cmd) error {
				i++
				if i == 4 {
					for _, pkgTar := range pkgTars {
						f, err := os.OpenFile(pkgTar, os.O_RDONLY|os.O_CREATE, 0o666)
						require.NoError(td, err)
						require.NoError(td, f.Close())
					}
				}
				return nil
			}

			isCorrectInstalledOverride := func(string, string) bool {
				return false
			}

			mockDB := &mock.DBExecutor{IsCorrectVersionInstalledFn: isCorrectInstalledOverride}
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

			installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny, false, testLogger)

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddTarget("jellyfin")

			pkgBuildDirs := map[string]string{
				"jellyfin": tmpDir,
			}

			errI := installer.Install(context.Background(), cmdArgs, tc.targets, pkgBuildDirs, []string{})
			require.NoError(td, errI)

			require.Len(td, mockRunner.ShowCalls, len(tc.wantShow))
			require.Len(td, mockRunner.CaptureCalls, len(tc.wantCapture))

			for i, call := range mockRunner.ShowCalls {
				show := call.Args[0].(*exec.Cmd).String()
				show = strings.ReplaceAll(show, tmpDir, "/testdir") // replace the temp dir with a static path
				show = strings.ReplaceAll(show, makepkgBin, "makepkg")
				show = strings.ReplaceAll(show, pacmanBin, "pacman")

				// options are in a different order on different systems and on CI root user is used
				assert.Subset(td, strings.Split(show, " "),
					strings.Split(tc.wantShow[i], " "),
					fmt.Sprintf("got at %d: %s \n", i, show))
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

func TestInstaller_InstallDownloadOnly(t *testing.T) {
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
				"makepkg -c --nobuild --noextract --ignorearch",
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

			mockDB := &mock.DBExecutor{IsCorrectVersionInstalledFn: isCorrectInstalledOverride}
			mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
			cmdBuilder := &exe.CmdBuilder{
				MakepkgBin:      makepkgBin,
				SudoBin:         "su",
				PacmanBin:       pacmanBin,
				Runner:          mockRunner,
				SudoLoopEnabled: false,
			}

			cmdBuilder.Runner = mockRunner

			installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny, true, testLogger)

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddTarget("yay")

			pkgBuildDirs := map[string]string{
				"yay": tmpDir,
			}

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

			errI := installer.Install(context.Background(), cmdArgs, targets, pkgBuildDirs, []string{})
			require.NoError(td, errI)

			require.Len(td, mockRunner.ShowCalls, len(tc.wantShow))
			require.Len(td, mockRunner.CaptureCalls, len(tc.wantCapture))
			require.Empty(td, installer.failedAndIgnored)

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
