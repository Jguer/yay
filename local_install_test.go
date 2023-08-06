//go:build !integration
// +build !integration

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	aur "github.com/Jguer/aur"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/db/mock"
	mockaur "github.com/Jguer/yay/v12/pkg/dep/mock"
	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/vcs"
)

func newTestLogger() *text.Logger {
	return text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
}

func TestIntegrationLocalInstall(t *testing.T) {
	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	gitBin := t.TempDir() + "/git"
	tmpDir := t.TempDir()
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(gitBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	tars := []string{
		tmpDir + "/jellyfin-10.8.4-1-x86_64.pkg.tar.zst",
		tmpDir + "/jellyfin-web-10.8.4-1-x86_64.pkg.tar.zst",
		tmpDir + "/jellyfin-server-10.8.4-1-x86_64.pkg.tar.zst",
	}

	wantShow := []string{
		"makepkg --verifysource --skippgpcheck -Ccf",
		"pacman -S --config /etc/pacman.conf -- community/dotnet-sdk-6.0 community/dotnet-runtime-6.0",
		"pacman -D -q --asdeps --config /etc/pacman.conf -- dotnet-runtime-6.0 dotnet-sdk-6.0",
		"makepkg --nobuild -fC --ignorearch",
		"makepkg -c --nobuild --noextract --ignorearch",
		"makepkg --nobuild -fC --ignorearch",
		"makepkg -c --nobuild --noextract --ignorearch",
		"pacman -U --config /etc/pacman.conf -- /testdir/jellyfin-server-10.8.4-1-x86_64.pkg.tar.zst /testdir/jellyfin-web-10.8.4-1-x86_64.pkg.tar.zst",
		"pacman -D -q --asexplicit --config /etc/pacman.conf -- jellyfin-server jellyfin-web",
		"makepkg --nobuild -fC --ignorearch",
		"makepkg -c --nobuild --noextract --ignorearch",
		"pacman -U --config /etc/pacman.conf -- /testdir/jellyfin-10.8.4-1-x86_64.pkg.tar.zst",
		"pacman -D -q --asexplicit --config /etc/pacman.conf -- jellyfin",
	}

	wantCapture := []string{
		"makepkg --packagelist",
		"git -C testdata/jfin git reset --hard HEAD",
		"git -C testdata/jfin git merge --no-edit --ff",
		"makepkg --packagelist",
		"makepkg --packagelist",
	}

	captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
		return strings.Join(tars, "\n"), "", nil
	}

	once := sync.Once{}

	showOverride := func(cmd *exec.Cmd) error {
		once.Do(func() {
			for _, tar := range tars {
				f, err := os.OpenFile(tar, os.O_RDONLY|os.O_CREATE, 0o666)
				require.NoError(t, err)
				require.NoError(t, f.Close())
			}
		})
		return nil
	}

	mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
	cmdBuilder := &exe.CmdBuilder{
		MakepkgBin:       makepkgBin,
		SudoBin:          "su",
		PacmanBin:        pacmanBin,
		PacmanConfigPath: "/etc/pacman.conf",
		GitBin:           "git",
		Runner:           mockRunner,
		SudoLoopEnabled:  false,
	}

	cmdArgs := parser.MakeArguments()
	cmdArgs.AddArg("B")
	cmdArgs.AddArg("i")
	cmdArgs.AddTarget("testdata/jfin")
	settings.NoConfirm = true
	defer func() { settings.NoConfirm = false }()
	db := &mock.DBExecutor{
		AlpmArchitecturesFn: func() ([]string, error) {
			return []string{"x86_64"}, nil
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "dotnet-sdk>=6", "dotnet-sdk<7", "dotnet-runtime>=6", "dotnet-runtime<7", "jellyfin-server=10.8.4", "jellyfin-web=10.8.4":
				return false
			}

			return true
		},
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "dotnet-runtime>=6", "dotnet-runtime<7":
				return &mock.Package{
					PName:    "dotnet-runtime-6.0",
					PBase:    "dotnet-runtime-6.0",
					PVersion: "6.0.100-1",
					PDB:      mock.NewDB("community"),
				}
			case "dotnet-sdk>=6", "dotnet-sdk<7":
				return &mock.Package{
					PName:    "dotnet-sdk-6.0",
					PBase:    "dotnet-sdk-6.0",
					PVersion: "6.0.100-1",
					PDB:      mock.NewDB("community"),
				}
			}

			return nil
		},
		LocalPackageFn:                func(s string) mock.IPackage { return nil },
		InstalledRemotePackageNamesFn: func() []string { return []string{} },
	}

	run := &runtime.Runtime{
		Cfg: &settings.Configuration{
			RemoveMake: "no",
		},
		Logger:     newTestLogger(),
		CmdBuilder: cmdBuilder,
		VCSStore:   &vcs.Mock{},
		AURClient: &mockaur.MockAUR{
			GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
				return []aur.Pkg{}, nil
			},
		},
	}

	err = handleCmd(context.Background(), run, cmdArgs, db)
	require.NoError(t, err)

	require.Len(t, mockRunner.ShowCalls, len(wantShow))
	require.Len(t, mockRunner.CaptureCalls, len(wantCapture))

	for i, call := range mockRunner.ShowCalls {
		show := call.Args[0].(*exec.Cmd).String()
		show = strings.ReplaceAll(show, tmpDir, "/testdir") // replace the temp dir with a static path
		show = strings.ReplaceAll(show, makepkgBin, "makepkg")
		show = strings.ReplaceAll(show, pacmanBin, "pacman")
		show = strings.ReplaceAll(show, gitBin, "pacman")

		// options are in a different order on different systems and on CI root user is used
		assert.Subset(t, strings.Split(show, " "), strings.Split(wantShow[i], " "), fmt.Sprintf("%d - %s", i, show))
	}
}

func TestIntegrationLocalInstallMissingDep(t *testing.T) {
	wantErr := ErrPackagesNotFound
	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	gitBin := t.TempDir() + "/git"
	tmpDir := t.TempDir()
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(gitBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	tars := []string{
		tmpDir + "/jellyfin-10.8.4-1-x86_64.pkg.tar.zst",
		tmpDir + "/jellyfin-web-10.8.4-1-x86_64.pkg.tar.zst",
		tmpDir + "/jellyfin-server-10.8.4-1-x86_64.pkg.tar.zst",
	}

	wantShow := []string{}
	wantCapture := []string{}

	captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
		return strings.Join(tars, "\n"), "", nil
	}

	once := sync.Once{}

	showOverride := func(cmd *exec.Cmd) error {
		once.Do(func() {
			for _, tar := range tars {
				f, err := os.OpenFile(tar, os.O_RDONLY|os.O_CREATE, 0o666)
				require.NoError(t, err)
				require.NoError(t, f.Close())
			}
		})
		return nil
	}

	mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
	cmdBuilder := &exe.CmdBuilder{
		MakepkgBin:       makepkgBin,
		SudoBin:          "su",
		PacmanBin:        pacmanBin,
		PacmanConfigPath: "/etc/pacman.conf",
		GitBin:           "git",
		Runner:           mockRunner,
		SudoLoopEnabled:  false,
	}

	cmdArgs := parser.MakeArguments()
	cmdArgs.AddArg("B")
	cmdArgs.AddArg("i")
	cmdArgs.AddTarget("testdata/jfin")
	settings.NoConfirm = true
	defer func() { settings.NoConfirm = false }()
	db := &mock.DBExecutor{
		AlpmArchitecturesFn: func() ([]string, error) {
			return []string{"x86_64"}, nil
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "dotnet-sdk>=6", "dotnet-sdk<7", "dotnet-runtime>=6", "dotnet-runtime<7", "jellyfin-server=10.8.4", "jellyfin-web=10.8.4":
				return false
			}

			return true
		},
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "dotnet-runtime>=6", "dotnet-runtime<7":
				return &mock.Package{
					PName:    "dotnet-runtime-6.0",
					PBase:    "dotnet-runtime-6.0",
					PVersion: "6.0.100-1",
					PDB:      mock.NewDB("community"),
				}
			}

			return nil
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}

	run := &runtime.Runtime{
		Cfg:        &settings.Configuration{},
		Logger:     newTestLogger(),
		CmdBuilder: cmdBuilder,
		VCSStore:   &vcs.Mock{},
		AURClient: &mockaur.MockAUR{
			GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
				return []aur.Pkg{}, nil
			},
		},
	}

	err = handleCmd(context.Background(), run, cmdArgs, db)
	require.ErrorContains(t, err, wantErr.Error())

	require.Len(t, mockRunner.ShowCalls, len(wantShow))
	require.Len(t, mockRunner.CaptureCalls, len(wantCapture))

	for i, call := range mockRunner.ShowCalls {
		show := call.Args[0].(*exec.Cmd).String()
		show = strings.ReplaceAll(show, tmpDir, "/testdir") // replace the temp dir with a static path
		show = strings.ReplaceAll(show, makepkgBin, "makepkg")
		show = strings.ReplaceAll(show, pacmanBin, "pacman")
		show = strings.ReplaceAll(show, gitBin, "pacman")

		// options are in a different order on different systems and on CI root user is used
		assert.Subset(t, strings.Split(show, " "), strings.Split(wantShow[i], " "), fmt.Sprintf("%d - %s", i, show))
	}
}

func TestIntegrationLocalInstallNeeded(t *testing.T) {
	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	gitBin := t.TempDir() + "/git"
	tmpDir := t.TempDir()
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(gitBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	tars := []string{
		tmpDir + "/jellyfin-10.8.4-1-x86_64.pkg.tar.zst",
		tmpDir + "/jellyfin-web-10.8.4-1-x86_64.pkg.tar.zst",
		tmpDir + "/jellyfin-server-10.8.4-1-x86_64.pkg.tar.zst",
	}

	wantShow := []string{
		"makepkg --verifysource --skippgpcheck -Ccf",
		"pacman -S --config /etc/pacman.conf -- community/dotnet-sdk-6.0 community/dotnet-runtime-6.0",
		"pacman -D -q --asdeps --config /etc/pacman.conf -- dotnet-runtime-6.0 dotnet-sdk-6.0",
		"makepkg --nobuild -fC --ignorearch",
		"makepkg -c --nobuild --noextract --ignorearch",
		"makepkg --nobuild -fC --ignorearch",
		"makepkg -c --nobuild --noextract --ignorearch",
		"makepkg --nobuild -fC --ignorearch",
		"makepkg -c --nobuild --noextract --ignorearch",
	}

	wantCapture := []string{
		"makepkg --packagelist",
		"git -C testdata/jfin git reset --hard HEAD",
		"git -C testdata/jfin git merge --no-edit --ff",
		"makepkg --packagelist",
		"makepkg --packagelist",
	}

	captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
		return strings.Join(tars, "\n"), "", nil
	}

	once := sync.Once{}

	showOverride := func(cmd *exec.Cmd) error {
		once.Do(func() {
			for _, tar := range tars {
				f, err := os.OpenFile(tar, os.O_RDONLY|os.O_CREATE, 0o666)
				require.NoError(t, err)
				require.NoError(t, f.Close())
			}
		})
		return nil
	}

	mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
	cmdBuilder := &exe.CmdBuilder{
		MakepkgBin:       makepkgBin,
		SudoBin:          "su",
		PacmanBin:        pacmanBin,
		PacmanConfigPath: "/etc/pacman.conf",
		GitBin:           "git",
		Runner:           mockRunner,
		SudoLoopEnabled:  false,
	}

	cmdArgs := parser.MakeArguments()
	cmdArgs.AddArg("B")
	cmdArgs.AddArg("i")
	cmdArgs.AddArg("needed")
	cmdArgs.AddTarget("testdata/jfin")
	settings.NoConfirm = true
	defer func() { settings.NoConfirm = false }()
	db := &mock.DBExecutor{
		AlpmArchitecturesFn: func() ([]string, error) {
			return []string{"x86_64"}, nil
		},
		IsCorrectVersionInstalledFn: func(s1, s2 string) bool {
			return true
		},
		LocalPackageFn: func(s string) mock.IPackage {
			if s == "jellyfin-server" {
				return &mock.Package{
					PName:    "jellyfin-server",
					PBase:    "jellyfin-server",
					PVersion: "10.8.4-1",
					PDB:      mock.NewDB("community"),
				}
			}
			return nil
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "dotnet-sdk>=6", "dotnet-sdk<7", "dotnet-runtime>=6", "dotnet-runtime<7", "jellyfin-server=10.8.4", "jellyfin-web=10.8.4":
				return false
			}

			return true
		},
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "dotnet-runtime>=6", "dotnet-runtime<7":
				return &mock.Package{
					PName:    "dotnet-runtime-6.0",
					PBase:    "dotnet-runtime-6.0",
					PVersion: "6.0.100-1",
					PDB:      mock.NewDB("community"),
				}
			case "dotnet-sdk>=6", "dotnet-sdk<7":
				return &mock.Package{
					PName:    "dotnet-sdk-6.0",
					PBase:    "dotnet-sdk-6.0",
					PVersion: "6.0.100-1",
					PDB:      mock.NewDB("community"),
				}
			}

			return nil
		},
		InstalledRemotePackageNamesFn: func() []string { return []string{} },
	}

	run := &runtime.Runtime{
		Cfg: &settings.Configuration{
			RemoveMake: "no",
		},
		Logger:     newTestLogger(),
		CmdBuilder: cmdBuilder,
		VCSStore:   &vcs.Mock{},
		AURClient: &mockaur.MockAUR{
			GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
				return []aur.Pkg{}, nil
			},
		},
	}

	err = handleCmd(context.Background(), run, cmdArgs, db)
	require.NoError(t, err)

	require.Len(t, mockRunner.ShowCalls, len(wantShow), "show calls: %v", mockRunner.ShowCalls)
	require.Len(t, mockRunner.CaptureCalls, len(wantCapture))

	for i, call := range mockRunner.ShowCalls {
		show := call.Args[0].(*exec.Cmd).String()
		show = strings.ReplaceAll(show, tmpDir, "/testdir") // replace the temp dir with a static path
		show = strings.ReplaceAll(show, makepkgBin, "makepkg")
		show = strings.ReplaceAll(show, pacmanBin, "pacman")
		show = strings.ReplaceAll(show, gitBin, "pacman")

		// options are in a different order on different systems and on CI root user is used
		assert.Subset(t, strings.Split(show, " "), strings.Split(wantShow[i], " "), fmt.Sprintf("%d - %s", i, show))
	}
}

func TestIntegrationLocalInstallGenerateSRCINFO(t *testing.T) {
	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	gitBin := t.TempDir() + "/git"
	tmpDir := t.TempDir()
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(gitBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	srcinfo, err := os.ReadFile("testdata/jfin/.SRCINFO")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(srcinfo), "pkgbase = jellyfin"), string(srcinfo))

	targetDir := t.TempDir()
	f, err = os.OpenFile(filepath.Join(targetDir, "PKGBUILD"), os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	tars := []string{
		tmpDir + "/jellyfin-10.8.4-1-x86_64.pkg.tar.zst",
		tmpDir + "/jellyfin-web-10.8.4-1-x86_64.pkg.tar.zst",
		tmpDir + "/jellyfin-server-10.8.4-1-x86_64.pkg.tar.zst",
	}

	wantShow := []string{
		"makepkg --verifysource --skippgpcheck -Ccf",
		"pacman -S --config /etc/pacman.conf -- community/dotnet-sdk-6.0 community/dotnet-runtime-6.0",
		"pacman -D -q --asdeps --config /etc/pacman.conf -- dotnet-runtime-6.0 dotnet-sdk-6.0",
		"makepkg --nobuild -fC --ignorearch",
		"makepkg -c --nobuild --noextract --ignorearch",
		"makepkg --nobuild -fC --ignorearch",
		"makepkg -c --nobuild --noextract --ignorearch",
		"pacman -U --config /etc/pacman.conf -- /testdir/jellyfin-server-10.8.4-1-x86_64.pkg.tar.zst /testdir/jellyfin-web-10.8.4-1-x86_64.pkg.tar.zst",
		"pacman -D -q --asexplicit --config /etc/pacman.conf -- jellyfin-server jellyfin-web",
		"makepkg --nobuild -fC --ignorearch",
		"makepkg -c --nobuild --noextract --ignorearch",
		"pacman -U --config /etc/pacman.conf -- /testdir/jellyfin-10.8.4-1-x86_64.pkg.tar.zst",
		"pacman -D -q --asexplicit --config /etc/pacman.conf -- jellyfin",
	}

	wantCapture := []string{
		"makepkg --printsrcinfo",
		"makepkg --packagelist",
		"git -C testdata/jfin git reset --hard HEAD",
		"git -C testdata/jfin git merge --no-edit --ff",
		"makepkg --packagelist",
		"makepkg --packagelist",
	}

	captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
		for _, arg := range cmd.Args {
			if arg == "--printsrcinfo" {
				return string(srcinfo), "", nil
			}
		}
		return strings.Join(tars, "\n"), "", nil
	}

	once := sync.Once{}

	showOverride := func(cmd *exec.Cmd) error {
		once.Do(func() {
			for _, tar := range tars {
				f, err := os.OpenFile(tar, os.O_RDONLY|os.O_CREATE, 0o666)
				require.NoError(t, err)
				require.NoError(t, f.Close())
			}
		})
		return nil
	}

	mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
	cmdBuilder := &exe.CmdBuilder{
		MakepkgBin:       makepkgBin,
		SudoBin:          "su",
		PacmanBin:        pacmanBin,
		PacmanConfigPath: "/etc/pacman.conf",
		GitBin:           "git",
		Runner:           mockRunner,
		SudoLoopEnabled:  false,
	}

	cmdArgs := parser.MakeArguments()
	cmdArgs.AddArg("B")
	cmdArgs.AddArg("i")
	cmdArgs.AddTarget(targetDir)
	settings.NoConfirm = true
	defer func() { settings.NoConfirm = false }()
	db := &mock.DBExecutor{
		AlpmArchitecturesFn: func() ([]string, error) {
			return []string{"x86_64"}, nil
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "dotnet-sdk>=6", "dotnet-sdk<7", "dotnet-runtime>=6", "dotnet-runtime<7", "jellyfin-server=10.8.4", "jellyfin-web=10.8.4":
				return false
			}

			return true
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "dotnet-runtime>=6", "dotnet-runtime<7":
				return &mock.Package{
					PName:    "dotnet-runtime-6.0",
					PBase:    "dotnet-runtime-6.0",
					PVersion: "6.0.100-1",
					PDB:      mock.NewDB("community"),
				}
			case "dotnet-sdk>=6", "dotnet-sdk<7":
				return &mock.Package{
					PName:    "dotnet-sdk-6.0",
					PBase:    "dotnet-sdk-6.0",
					PVersion: "6.0.100-1",
					PDB:      mock.NewDB("community"),
				}
			}

			return nil
		},
		InstalledRemotePackageNamesFn: func() []string { return []string{} },
	}

	run := &runtime.Runtime{
		Cfg: &settings.Configuration{
			RemoveMake: "no",
			Debug:      false,
		},
		Logger:     newTestLogger(),
		CmdBuilder: cmdBuilder,
		VCSStore:   &vcs.Mock{},
		AURClient: &mockaur.MockAUR{
			GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
				return []aur.Pkg{}, nil
			},
		},
	}

	err = handleCmd(context.Background(), run, cmdArgs, db)
	require.NoError(t, err)

	require.Len(t, mockRunner.ShowCalls, len(wantShow))
	require.Len(t, mockRunner.CaptureCalls, len(wantCapture))

	for i, call := range mockRunner.ShowCalls {
		show := call.Args[0].(*exec.Cmd).String()
		show = strings.ReplaceAll(show, tmpDir, "/testdir") // replace the temp dir with a static path
		show = strings.ReplaceAll(show, makepkgBin, "makepkg")
		show = strings.ReplaceAll(show, pacmanBin, "pacman")
		show = strings.ReplaceAll(show, gitBin, "pacman")

		// options are in a different order on different systems and on CI root user is used
		assert.Subset(t, strings.Split(show, " "), strings.Split(wantShow[i], " "), fmt.Sprintf("%d - %s", i, show))
	}
}

func TestIntegrationLocalInstallMissingFiles(t *testing.T) {
	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	gitBin := t.TempDir() + "/git"
	tmpDir := t.TempDir()
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(gitBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	srcinfo, err := os.ReadFile("testdata/jfin/.SRCINFO")
	require.NoError(t, err)

	targetDir := t.TempDir()

	tars := []string{
		tmpDir + "/jellyfin-10.8.4-1-x86_64.pkg.tar.zst",
		tmpDir + "/jellyfin-web-10.8.4-1-x86_64.pkg.tar.zst",
		tmpDir + "/jellyfin-server-10.8.4-1-x86_64.pkg.tar.zst",
	}

	wantShow := []string{}

	wantCapture := []string{}

	captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
		if cmd.Args[1] == "--printsrcinfo" {
			return string(srcinfo), "", nil
		}
		return strings.Join(tars, "\n"), "", nil
	}

	once := sync.Once{}

	showOverride := func(cmd *exec.Cmd) error {
		once.Do(func() {
			for _, tar := range tars {
				f, err := os.OpenFile(tar, os.O_RDONLY|os.O_CREATE, 0o666)
				require.NoError(t, err)
				require.NoError(t, f.Close())
			}
		})
		return nil
	}

	mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
	cmdBuilder := &exe.CmdBuilder{
		MakepkgBin:       makepkgBin,
		SudoBin:          "su",
		PacmanBin:        pacmanBin,
		PacmanConfigPath: "/etc/pacman.conf",
		GitBin:           "git",
		Runner:           mockRunner,
		SudoLoopEnabled:  false,
	}

	cmdArgs := parser.MakeArguments()
	cmdArgs.AddArg("B")
	cmdArgs.AddArg("i")
	cmdArgs.AddTarget(targetDir)
	settings.NoConfirm = true
	defer func() { settings.NoConfirm = false }()
	db := &mock.DBExecutor{
		AlpmArchitecturesFn: func() ([]string, error) {
			return []string{"x86_64"}, nil
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "dotnet-sdk>=6", "dotnet-sdk<7", "dotnet-runtime>=6", "dotnet-runtime<7", "jellyfin-server=10.8.4", "jellyfin-web=10.8.4":
				return false
			}

			return true
		},
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "dotnet-runtime>=6", "dotnet-runtime<7":
				return &mock.Package{
					PName:    "dotnet-runtime-6.0",
					PBase:    "dotnet-runtime-6.0",
					PVersion: "6.0.100-1",
					PDB:      mock.NewDB("community"),
				}
			case "dotnet-sdk>=6", "dotnet-sdk<7":
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

	config := &runtime.Runtime{
		Cfg: &settings.Configuration{
			RemoveMake: "no",
			Debug:      false,
		},
		Logger:     newTestLogger(),
		CmdBuilder: cmdBuilder,
		VCSStore:   &vcs.Mock{},
		AURClient: &mockaur.MockAUR{
			GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
				return []aur.Pkg{}, nil
			},
		},
	}

	err = handleCmd(context.Background(), config, cmdArgs, db)
	require.ErrorIs(t, err, ErrNoBuildFiles)

	require.Len(t, mockRunner.ShowCalls, len(wantShow))
	require.Len(t, mockRunner.CaptureCalls, len(wantCapture))

	for i, call := range mockRunner.ShowCalls {
		show := call.Args[0].(*exec.Cmd).String()
		show = strings.ReplaceAll(show, tmpDir, "/testdir") // replace the temp dir with a static path
		show = strings.ReplaceAll(show, makepkgBin, "makepkg")
		show = strings.ReplaceAll(show, pacmanBin, "pacman")
		show = strings.ReplaceAll(show, gitBin, "pacman")

		// options are in a different order on different systems and on CI root user is used
		assert.Subset(t, strings.Split(show, " "), strings.Split(wantShow[i], " "), fmt.Sprintf("%d - %s", i, show))
	}
}

func TestIntegrationLocalInstallWithDepsProvides(t *testing.T) {
	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	gitBin := t.TempDir() + "/git"
	tmpDir := t.TempDir()
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(gitBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	tars := []string{
		tmpDir + "/ceph-bin-17.2.6-2-x86_64.pkg.tar.zst",
		tmpDir + "/ceph-libs-bin-17.2.6-2-x86_64.pkg.tar.zst",
	}

	wantShow := []string{
		"makepkg --verifysource --skippgpcheck -Ccf",
		"makepkg --nobuild -fC --ignorearch",
		"makepkg -c --nobuild --noextract --ignorearch",
		"pacman -U --config /etc/pacman.conf -- /testdir/ceph-libs-bin-17.2.6-2-x86_64.pkg.tar.zst",
		"pacman -D -q --asexplicit --config /etc/pacman.conf -- ceph-libs-bin",
		"makepkg --nobuild -fC --ignorearch",
		"makepkg -c --nobuild --noextract --ignorearch",
		"pacman -U --config /etc/pacman.conf -- /testdir/ceph-bin-17.2.6-2-x86_64.pkg.tar.zst",
		"pacman -D -q --asexplicit --config /etc/pacman.conf -- ceph-bin",
	}

	wantCapture := []string{
		"git -C testdata/cephbin git reset --hard HEAD",
		"git -C testdata/cephbin git merge --no-edit --ff",
		"makepkg --packagelist",
		"makepkg --packagelist",
	}

	captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
		return strings.Join(tars, "\n"), "", nil
	}

	once := sync.Once{}

	showOverride := func(cmd *exec.Cmd) error {
		once.Do(func() {
			for _, tar := range tars {
				f, err := os.OpenFile(tar, os.O_RDONLY|os.O_CREATE, 0o666)
				require.NoError(t, err)
				require.NoError(t, f.Close())
			}
		})
		return nil
	}

	mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
	cmdBuilder := &exe.CmdBuilder{
		MakepkgBin:       makepkgBin,
		SudoBin:          "su",
		PacmanBin:        pacmanBin,
		PacmanConfigPath: "/etc/pacman.conf",
		GitBin:           "git",
		Runner:           mockRunner,
		SudoLoopEnabled:  false,
	}

	cmdArgs := parser.MakeArguments()
	cmdArgs.AddArg("B")
	cmdArgs.AddArg("i")
	cmdArgs.AddTarget("testdata/cephbin")
	settings.NoConfirm = true
	defer func() { settings.NoConfirm = false }()
	db := &mock.DBExecutor{
		AlpmArchitecturesFn: func() ([]string, error) {
			return []string{"x86_64"}, nil
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "ceph=17.2.6-2", "ceph-libs=17.2.6-2":
				return false
			}

			return true
		},
		SyncSatisfierFn: func(s string) mock.IPackage {
			return nil
		},
		LocalPackageFn:                func(s string) mock.IPackage { return nil },
		InstalledRemotePackageNamesFn: func() []string { return []string{} },
	}

	config := &runtime.Runtime{
		Cfg: &settings.Configuration{
			RemoveMake: "no",
		},
		Logger:     newTestLogger(),
		CmdBuilder: cmdBuilder,
		VCSStore:   &vcs.Mock{},
		AURClient: &mockaur.MockAUR{
			GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
				return []aur.Pkg{}, nil
			},
		},
	}

	err = handleCmd(context.Background(), config, cmdArgs, db)
	require.NoError(t, err)

	require.Len(t, mockRunner.ShowCalls, len(wantShow))
	require.Len(t, mockRunner.CaptureCalls, len(wantCapture))

	for i, call := range mockRunner.ShowCalls {
		show := call.Args[0].(*exec.Cmd).String()
		show = strings.ReplaceAll(show, tmpDir, "/testdir") // replace the temp dir with a static path
		show = strings.ReplaceAll(show, makepkgBin, "makepkg")
		show = strings.ReplaceAll(show, pacmanBin, "pacman")
		show = strings.ReplaceAll(show, gitBin, "pacman")

		// options are in a different order on different systems and on CI root user is used
		assert.Subset(t, strings.Split(show, " "), strings.Split(wantShow[i], " "), fmt.Sprintf("%d - %s", i, show))
	}
}

func TestIntegrationLocalInstallTwoSrcInfosWithDeps(t *testing.T) {
	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	gitBin := t.TempDir() + "/git"
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(gitBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	pkgsTars := []string{
		tmpDir1 + "/libzip-git-1.9.2.r166.gd2c47d0f-1-x86_64.pkg.tar.zst",
		tmpDir2 + "/gourou-0.8.1-4-x86_64.pkg.tar.zst",
	}

	wantShow := []string{
		"makepkg --verifysource --skippgpcheck -Ccf",
		"makepkg --verifysource --skippgpcheck -Ccf",
		"makepkg --nobuild -fC --ignorearch",
		"makepkg -c --nobuild --noextract --ignorearch",
		"pacman -U --config /etc/pacman.conf -- /testdir1/libzip-git-1.9.2.r166.gd2c47d0f-1-x86_64.pkg.tar.zst",
		"pacman -D -q --asexplicit --config /etc/pacman.conf -- libzip-git",
		"makepkg --nobuild -fC --ignorearch",
		"makepkg -c --nobuild --noextract --ignorearch",
		"pacman -U --config /etc/pacman.conf -- /testdir2/gourou-0.8.1-4-x86_64.pkg.tar.zst",
		"pacman -D -q --asexplicit --config /etc/pacman.conf -- gourou",
	}

	wantCapture := []string{
		"git -C testdata/gourou git reset --hard HEAD",
		"git -C testdata/gourou git merge --no-edit --ff",
		"git -C testdata/libzip-git git reset --hard HEAD",
		"git -C testdata/libzip-git git merge --no-edit --ff",
		"makepkg --packagelist",
		"makepkg --packagelist",
	}

	captureCounter := 0
	captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
		captureCounter++
		switch captureCounter {
		case 5:
			return pkgsTars[0] + "\n", "", nil
		case 6:
			return pkgsTars[1] + "\n", "", nil
		default:
			return "", "", nil
		}
	}

	once := sync.Once{}

	showOverride := func(cmd *exec.Cmd) error {
		once.Do(func() {
			for _, tar := range pkgsTars {
				f, err := os.OpenFile(tar, os.O_RDONLY|os.O_CREATE, 0o666)
				require.NoError(t, err)
				require.NoError(t, f.Close())
			}
		})
		return nil
	}

	mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
	cmdBuilder := &exe.CmdBuilder{
		MakepkgBin:       makepkgBin,
		SudoBin:          "su",
		PacmanBin:        pacmanBin,
		PacmanConfigPath: "/etc/pacman.conf",
		GitBin:           "git",
		Runner:           mockRunner,
		SudoLoopEnabled:  false,
	}

	cmdArgs := parser.MakeArguments()
	cmdArgs.AddArg("B")
	cmdArgs.AddArg("i")
	cmdArgs.AddTarget("testdata/gourou")
	cmdArgs.AddTarget("testdata/libzip-git")
	settings.NoConfirm = true
	defer func() { settings.NoConfirm = false }()
	db := &mock.DBExecutor{
		AlpmArchitecturesFn: func() ([]string, error) {
			return []string{"x86_64"}, nil
		},
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "gourou", "libzip", "libzip-git":
				return false
			}

			return true
		},
		SyncSatisfierFn: func(s string) mock.IPackage {
			return nil
		},
		LocalPackageFn:                func(s string) mock.IPackage { return nil },
		InstalledRemotePackageNamesFn: func() []string { return []string{} },
	}

	run := &runtime.Runtime{
		Cfg: &settings.Configuration{
			RemoveMake: "no",
		},
		Logger:     newTestLogger(),
		CmdBuilder: cmdBuilder,
		VCSStore:   &vcs.Mock{},
		AURClient: &mockaur.MockAUR{
			GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
				return []aur.Pkg{}, nil
			},
		},
	}

	err = handleCmd(context.Background(), run, cmdArgs, db)
	require.NoError(t, err)

	require.Len(t, mockRunner.ShowCalls, len(wantShow))
	require.Len(t, mockRunner.CaptureCalls, len(wantCapture))

	for i, call := range mockRunner.ShowCalls {
		show := call.Args[0].(*exec.Cmd).String()
		show = strings.ReplaceAll(show, tmpDir1, "/testdir1") // replace the temp dir with a static path
		show = strings.ReplaceAll(show, tmpDir2, "/testdir2") // replace the temp dir with a static path
		show = strings.ReplaceAll(show, makepkgBin, "makepkg")
		show = strings.ReplaceAll(show, pacmanBin, "pacman")
		show = strings.ReplaceAll(show, gitBin, "pacman")

		// options are in a different order on different systems and on CI root user is used
		assert.Subset(t, strings.Split(show, " "), strings.Split(wantShow[i], " "), fmt.Sprintf("%d - %s", i, show))
	}
}
