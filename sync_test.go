//go:build !integration
// +build !integration

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
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

type syncPrintTestEnv struct {
	ctx    context.Context
	run    *runtime.Runtime
	args   *parser.Arguments
	db     *mock.DBExecutor
	runner *exe.MockRunner
	aur    *mockaur.MockAUR
}

func newSyncPrintTestEnv(t *testing.T) *syncPrintTestEnv {
	t.Helper()

	mockRunner := &exe.MockRunner{}
	mockBuilder := &exe.MockBuilder{
		Runner: mockRunner,
		BuildPacmanCmdFn: func(ctx context.Context, args *parser.Arguments, mode parser.TargetMode, noConfirm bool) *exec.Cmd {
			return exec.CommandContext(ctx, "pacman")
		},
	}

	mockAUR := &mockaur.MockAUR{}

	run := &runtime.Runtime{
		Cfg:        &settings.Configuration{},
		Logger:     text.NewLogger(io.Discard, os.Stderr, strings.NewReader("\n"), true, "test"),
		CmdBuilder: mockBuilder,
		VCSStore:   &vcs.Mock{},
		AURClient:  mockAUR,
	}

	dbExec := &mock.DBExecutor{
		PackagesFromGroupFn: func(string) []mock.IPackage { return nil },
		SyncSatisfierFn:     func(string) mock.IPackage { return nil },
		LocalPackageFn:      func(string) mock.IPackage { return nil },
		InstalledRemotePackagesFn: func() map[string]alpm.IPackage {
			return map[string]alpm.IPackage{}
		},
		InstalledRemotePackageNamesFn: func() []string { return []string{} },
		RefreshHandleFn:               func() error { return nil },
	}

	args := parser.MakeArguments()
	require.NoError(t, args.AddArg("S"))

	return &syncPrintTestEnv{
		ctx:    context.Background(),
		run:    run,
		args:   args,
		db:     dbExec,
		runner: mockRunner,
		aur:    mockAUR,
	}
}

func TestSyncPrint_RepoTargetsUsePacmanOnly(t *testing.T) {
	t.Parallel()

	env := newSyncPrintTestEnv(t)
	env.db.SyncSatisfierFn = func(name string) mock.IPackage {
		if name == "vim" {
			return &mock.Package{PName: name}
		}

		return nil
	}
	env.args.AddTarget("vim")

	aurCalls := 0
	env.aur.GetFn = func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
		aurCalls++
		return []aur.Pkg{}, nil
	}

	err := syncPrint(env.ctx, env.run, env.args, env.db)
	require.NoError(t, err)
	// Make sure pacman was called and AUR was not
	require.Len(t, env.runner.ShowCalls, 1)
	assert.Equal(t, 0, aurCalls)
}

func TestSyncPrint_AURTargetsSkipPacman(t *testing.T) {
	t.Parallel()

	env := newSyncPrintTestEnv(t)
	env.args.AddTarget("yay-bin")

	var queried []string
	env.aur.GetFn = func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
		queried = append([]string(nil), query.Needles...)
		return []aur.Pkg{
			{
				Name:        query.Needles[0],
				PackageBase: query.Needles[0],
				Version:     "1.0.0",
			},
		}, nil
	}

	err := syncPrint(env.ctx, env.run, env.args, env.db)
	require.NoError(t, err)
	// Make sure pacman was not called and AUR was
	assert.Len(t, env.runner.ShowCalls, 0)
	require.Equal(t, []string{"yay-bin"}, queried)
}

func TestSyncPrint_MixedTargetsCallPacmanAndAUR(t *testing.T) {
	t.Parallel()

	env := newSyncPrintTestEnv(t)
	env.db.SyncSatisfierFn = func(name string) mock.IPackage {
		if name == "vim" {
			return &mock.Package{PName: name}
		}

		return nil
	}
	env.args.AddTarget("vim", "yay-bin")

	var queried []string
	env.aur.GetFn = func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
		queried = append([]string(nil), query.Needles...)
		return []aur.Pkg{
			{
				Name:        query.Needles[0],
				PackageBase: query.Needles[0],
				Version:     "2.0.0",
			},
		}, nil
	}

	err := syncPrint(env.ctx, env.run, env.args, env.db)
	require.NoError(t, err)
	// Make sure both pacman and AUR were called
	require.Len(t, env.runner.ShowCalls, 1)
	require.Equal(t, []string{"yay-bin"}, queried)
}

func TestSyncPrint_UpgradeChecksPacmanAndAUR(t *testing.T) {
	t.Parallel()

	env := newSyncPrintTestEnv(t)
	require.NoError(t, env.args.AddArg("u"))

	env.db.InstalledRemotePackagesFn = func() map[string]alpm.IPackage {
		return map[string]alpm.IPackage{
			"yay-bin": &mock.Package{
				PName:    "yay-bin",
				PVersion: "1.0.0",
				PBase:    "yay-bin",
				PReason:  alpm.PkgReasonExplicit,
			},
		}
	}
	env.db.InstalledRemotePackageNamesFn = func() []string {
		return []string{"yay-bin"}
	}

	var queried []string
	env.aur.GetFn = func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
		queried = append([]string(nil), query.Needles...)
		return []aur.Pkg{
			{
				Name:        "yay-bin",
				PackageBase: "yay-bin",
				Version:     "2.0.0",
			},
		}, nil
	}

	err := syncPrint(env.ctx, env.run, env.args, env.db)
	require.NoError(t, err)
	// Make sure pacman and AUR were both called
	require.Len(t, env.runner.ShowCalls, 1)
	require.Equal(t, []string{"yay-bin"}, queried)
}

func TestSyncUpgrade(t *testing.T) {
	t.Parallel()
	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	gitBin := t.TempDir() + "/git"
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(gitBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
		return "", "", nil
	}

	showOverride := func(cmd *exec.Cmd) error {
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
	cmdArgs.AddArg("S")
	cmdArgs.AddArg("y")
	cmdArgs.AddArg("u")

	dbName := mock.NewDB("core")

	db := &mock.DBExecutor{
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
			return map[string]alpm.IPackage{}
		},
		InstalledRemotePackageNamesFn: func() []string {
			return []string{}
		},
		SyncUpgradesFn: func(
			bool,
		) (map[string]db.SyncUpgrade, error) {
			return map[string]db.SyncUpgrade{
				"linux": {
					Package: &mock.Package{
						PName:    "linux",
						PVersion: "5.10.0",
						PDB:      dbName,
					},
					LocalVersion: "4.3.0",
					Reason:       alpm.PkgReasonExplicit,
				},
			}, nil
		},
	}

	run := &runtime.Runtime{
		Cfg: &settings.Configuration{
			RemoveMake: "no",
		},
		Logger:     text.NewLogger(io.Discard, os.Stderr, strings.NewReader("\n"), true, "test"),
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

	wantCapture := []string{}
	wantShow := []string{
		"pacman -S -y --config /etc/pacman.conf --",
		"pacman -S -y -u --config /etc/pacman.conf --",
	}

	require.Len(t, mockRunner.ShowCalls, len(wantShow))
	require.Len(t, mockRunner.CaptureCalls, len(wantCapture))

	for i, call := range mockRunner.ShowCalls {
		show := call.Args[0].(*exec.Cmd).String()
		show = strings.ReplaceAll(show, makepkgBin, "makepkg")
		show = strings.ReplaceAll(show, pacmanBin, "pacman")
		show = strings.ReplaceAll(show, gitBin, "pacman")

		// options are in a different order on different systems and on CI root user is used
		assert.Subset(t, strings.Split(show, " "), strings.Split(wantShow[i], " "), fmt.Sprintf("%d - %s", i, show))
	}
}

func TestSyncUpgrade_IgnoreAll(t *testing.T) {
	t.Parallel()
	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	gitBin := t.TempDir() + "/git"
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(gitBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
		return "", "", nil
	}

	showOverride := func(cmd *exec.Cmd) error {
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
	cmdArgs.AddArg("S")
	cmdArgs.AddArg("y")
	cmdArgs.AddArg("u")

	dbName := mock.NewDB("core")

	db := &mock.DBExecutor{
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
			return map[string]alpm.IPackage{}
		},
		InstalledRemotePackageNamesFn: func() []string {
			return []string{}
		},
		SyncUpgradesFn: func(
			bool,
		) (map[string]db.SyncUpgrade, error) {
			return map[string]db.SyncUpgrade{
				"linux": {
					Package: &mock.Package{
						PName:    "linux",
						PVersion: "5.10.0",
						PDB:      dbName,
					},
					LocalVersion: "4.3.0",
					Reason:       alpm.PkgReasonExplicit,
				},
			}, nil
		},
	}

	run := &runtime.Runtime{
		Cfg: &settings.Configuration{
			RemoveMake: "no",
		},
		Logger:     text.NewLogger(io.Discard, os.Stderr, strings.NewReader("1\n"), true, "test"),
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

	wantCapture := []string{}
	wantShow := []string{
		"pacman -S -y --config /etc/pacman.conf --",
	}

	require.Len(t, mockRunner.ShowCalls, len(wantShow))
	require.Len(t, mockRunner.CaptureCalls, len(wantCapture))

	for i, call := range mockRunner.ShowCalls {
		show := call.Args[0].(*exec.Cmd).String()
		show = strings.ReplaceAll(show, makepkgBin, "makepkg")
		show = strings.ReplaceAll(show, pacmanBin, "pacman")
		show = strings.ReplaceAll(show, gitBin, "pacman")

		// options are in a different order on different systems and on CI root user is used
		assert.Subset(t, strings.Split(show, " "), strings.Split(wantShow[i], " "), fmt.Sprintf("%d - %s", i, show))
	}
}

func TestSyncUpgrade_IgnoreOne(t *testing.T) {
	t.Parallel()
	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	gitBin := t.TempDir() + "/git"
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(gitBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
		return "", "", nil
	}

	showOverride := func(cmd *exec.Cmd) error {
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
	cmdArgs.AddArg("S")
	cmdArgs.AddArg("y")
	cmdArgs.AddArg("u")

	dbName := mock.NewDB("core")

	db := &mock.DBExecutor{
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
			return map[string]alpm.IPackage{}
		},
		InstalledRemotePackageNamesFn: func() []string {
			return []string{}
		},
		SyncUpgradesFn: func(
			bool,
		) (map[string]db.SyncUpgrade, error) {
			return map[string]db.SyncUpgrade{
				"gcc": {
					Package: &mock.Package{
						PName:    "gcc",
						PVersion: "6.0.0",
						PDB:      dbName,
					},
					LocalVersion: "5.0.0",
					Reason:       alpm.PkgReasonExplicit,
				},
				"linux": {
					Package: &mock.Package{
						PName:    "linux",
						PVersion: "5.10.0",
						PDB:      dbName,
					},
					LocalVersion: "4.3.0",
					Reason:       alpm.PkgReasonExplicit,
				},
				"linux-headers": {
					Package: &mock.Package{
						PName:    "linux-headers",
						PVersion: "5.10.0",
						PDB:      dbName,
					},
					LocalVersion: "4.3.0",
					Reason:       alpm.PkgReasonDepend,
				},
			}, nil
		},
	}

	run := &runtime.Runtime{
		Cfg: &settings.Configuration{
			RemoveMake: "no",
		},
		Logger:     text.NewLogger(io.Discard, os.Stderr, strings.NewReader("1\n"), true, "test"),
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

	wantCapture := []string{}
	wantShow := []string{
		"pacman -S -y --config /etc/pacman.conf --",
		"pacman -S -y -u --config /etc/pacman.conf --ignore linux-headers --",
	}

	require.Len(t, mockRunner.ShowCalls, len(wantShow))
	require.Len(t, mockRunner.CaptureCalls, len(wantCapture))

	for i, call := range mockRunner.ShowCalls {
		show := call.Args[0].(*exec.Cmd).String()
		show = strings.ReplaceAll(show, makepkgBin, "makepkg")
		show = strings.ReplaceAll(show, pacmanBin, "pacman")
		show = strings.ReplaceAll(show, gitBin, "pacman")

		// options are in a different order on different systems and on CI root user is used
		assert.Subset(t, strings.Split(show, " "), strings.Split(wantShow[i], " "), fmt.Sprintf("%d - %s", i, show))
	}
}

// Pinned deps with rollup
func TestSyncUpgradeAURPinnedSplitPackage(t *testing.T) {
	t.Parallel()
	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	tmpDir := t.TempDir()
	gitBin := t.TempDir() + "/git"
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(gitBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	pkgBuildDir := tmpDir + "/vosk-api"
	os.Mkdir(pkgBuildDir, 0o755)
	fSource, err := os.OpenFile(pkgBuildDir+"/.SRCINFO", os.O_RDWR|os.O_CREATE, 0o666)
	require.NoError(t, err)
	n, errF := fSource.WriteString(`pkgbase = vosk-api
	pkgdesc = Offline speech recognition toolkit
	pkgver = 0.3.45
	pkgrel = 1
	url = https://alphacephei.com/vosk/
	arch = x86_64
	license = Apache

pkgname = vosk-api
	pkgdesc = vosk-api

pkgname = python-vosk
	pkgdesc = Python module for vosk-api
	depends = vosk-api=0.3.45`)
	require.NoError(t, errF)
	require.Greater(t, n, 0)
	require.NoError(t, fSource.Close())

	tars := []string{
		tmpDir + "/vosk-api-0.3.45-1-x86_64.pkg.tar.zst",
		tmpDir + "/python-vosk-0.3.45-1-x86_64.pkg.tar.zst",
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
		if sanitizeCall(cmd.String(), tmpDir, makepkgBin,
			pacmanBin, gitBin) == "pacman -U --config /etc/pacman.conf -- /testdir/vosk-api-0.3.45-1-x86_64.pkg.tar.zst" {
			return errors.New("Unsatisfied dependency")
		}
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
	cmdArgs.AddArg("S")
	cmdArgs.AddArg("y")
	cmdArgs.AddArg("u")

	db := &mock.DBExecutor{
		AlpmArchitecturesFn: func() ([]string, error) {
			return []string{"x86_64"}, nil
		},
		RefreshHandleFn: func() error {
			return nil
		},
		ReposFn: func() []string {
			return []string{"core"}
		},
		SyncSatisfierFn: func(s string) mock.IPackage {
			return nil
		},
		InstalledRemotePackagesFn: func() map[string]alpm.IPackage {
			return map[string]alpm.IPackage{
				"vosk-api": &mock.Package{
					PName:    "vosk-api",
					PVersion: "0.3.43-1",
					PBase:    "vosk-api",
					PReason:  alpm.PkgReasonDepend,
				},
				"python-vosk": &mock.Package{
					PName:    "python-vosk",
					PVersion: "0.3.43-1",
					PBase:    "python-vosk",
					PReason:  alpm.PkgReasonExplicit,
					// TODO: fix mock Depends
				},
			}
		},
		InstalledRemotePackageNamesFn: func() []string {
			return []string{"vosk-api", "python-vosk"}
		},
		LocalSatisfierExistsFn: func(s string) bool {
			return false
		},
		SyncUpgradesFn: func(
			bool,
		) (map[string]db.SyncUpgrade, error) {
			return map[string]db.SyncUpgrade{}, nil
		},
	}

	run := &runtime.Runtime{
		Cfg: &settings.Configuration{
			DoubleConfirm: true,
			RemoveMake:    "no",
			BuildDir:      tmpDir,
		},
		Logger:     text.NewLogger(io.Discard, os.Stderr, strings.NewReader("\n\n\n\n"), true, "test"),
		CmdBuilder: cmdBuilder,
		VCSStore:   &vcs.Mock{},
		AURClient: &mockaur.MockAUR{
			GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
				return []aur.Pkg{
					{
						Name:        "vosk-api",
						PackageBase: "vosk-api",
						Version:     "0.3.45-1",
					},
					{
						Name:        "python-vosk",
						PackageBase: "vosk-api",
						Version:     "0.3.45-1",
						Depends: []string{
							"vosk-api=0.3.45",
						},
					},
				}, nil
			},
		},
	}

	err = handleCmd(context.Background(), run, cmdArgs, db)
	require.NoError(t, err)

	wantCapture := []string{
		"/usr/bin/git -C /testdir/vosk-api reset --hard HEAD",
		"/usr/bin/git -C /testdir/vosk-api merge --no-edit --ff",
		"makepkg --packagelist",
		"makepkg --packagelist",
	}
	wantShow := []string{
		"pacman -S -y --config /etc/pacman.conf --",
		"makepkg --verifysource --skippgpcheck -f -Cc", "makepkg --nobuild -f -C --ignorearch",
		"makepkg -c --nobuild --noextract --ignorearch",
		"pacman -U --config /etc/pacman.conf -- /testdir/vosk-api-0.3.45-1-x86_64.pkg.tar.zst",
		"makepkg --nobuild -f -C --ignorearch", "makepkg -c --nobuild --noextract --ignorearch",
		"pacman -U --config /etc/pacman.conf -- /testdir/vosk-api-0.3.45-1-x86_64.pkg.tar.zst /testdir/python-vosk-0.3.45-1-x86_64.pkg.tar.zst",
		"pacman -D -q --asdeps --config /etc/pacman.conf -- vosk-api",
		"pacman -D -q --asexplicit --config /etc/pacman.conf -- python-vosk",
	}

	require.Len(t, mockRunner.ShowCalls, len(wantShow),
		fmt.Sprintf("%#v", sanitizeCalls(mockRunner.ShowCalls, tmpDir, makepkgBin, pacmanBin, gitBin)))
	require.Len(t, mockRunner.CaptureCalls, len(wantCapture),
		fmt.Sprintf("%#v", sanitizeCalls(mockRunner.CaptureCalls, tmpDir, makepkgBin, pacmanBin, gitBin)))

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

func sanitizeCalls(calls []exe.Call, tmpDir, makepkg, pacman, git string) []string {
	san := make([]string, 0, len(calls))
	for _, c := range calls {
		s := c.Args[0].(*exec.Cmd).String()
		san = append(san, sanitizeCall(s, tmpDir, makepkg, pacman, git))
	}

	return san
}

func sanitizeCall(s, tmpDir, makepkg, pacman, git string) string {
	_, after, found := strings.Cut(s, makepkg)
	if found {
		s = "makepkg" + after
	}

	_, after, found = strings.Cut(s, pacman)
	if found {
		s = "pacman" + after
	}

	_, after, found = strings.Cut(s, git)
	if found {
		s = "git" + after
	}

	s = strings.ReplaceAll(s, tmpDir, "/testdir")

	return s
}

func TestSyncUpgrade_NoCombinedUpgrade(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		combinedUpgrade bool
		want            []string
	}{
		{
			name:            "combined upgrade",
			combinedUpgrade: true,
			want:            []string{"pacman -S -y -u --config /etc/pacman.conf --"},
		},
		{
			name:            "no combined upgrade",
			combinedUpgrade: false,
			want:            []string{"pacman -S -y --config /etc/pacman.conf --"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			makepkgBin := t.TempDir() + "/makepkg"
			pacmanBin := t.TempDir() + "/pacman"
			gitBin := t.TempDir() + "/git"
			f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
			require.NoError(t, err)
			require.NoError(t, f.Close())

			f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
			require.NoError(t, err)
			require.NoError(t, f.Close())

			f, err = os.OpenFile(gitBin, os.O_RDONLY|os.O_CREATE, 0o755)
			require.NoError(t, err)
			require.NoError(t, f.Close())

			captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
				return "", "", nil
			}

			showOverride := func(cmd *exec.Cmd) error {
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
			cmdArgs.AddArg("S")
			cmdArgs.AddArg("y")
			cmdArgs.AddArg("u")

			db := &mock.DBExecutor{
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
					return map[string]alpm.IPackage{}
				},
				InstalledRemotePackageNamesFn: func() []string {
					return []string{}
				},
				SyncUpgradesFn: func(
					bool,
				) (map[string]db.SyncUpgrade, error) {
					return map[string]db.SyncUpgrade{}, nil
				},
			}

			run := &runtime.Runtime{
				Cfg: &settings.Configuration{
					RemoveMake:      "no",
					CombinedUpgrade: false,
				},
				Logger:     text.NewLogger(io.Discard, os.Stderr, strings.NewReader("1\n"), true, "test"),
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

			require.Len(t, mockRunner.ShowCalls, len(tc.want))
			require.Len(t, mockRunner.CaptureCalls, 0)

			for i, call := range mockRunner.ShowCalls {
				show := call.Args[0].(*exec.Cmd).String()
				show = strings.ReplaceAll(show, pacmanBin, "pacman")

				// options are in a different order on different systems and on CI root user is used
				assert.Subset(t, strings.Split(show, " "), strings.Split(tc.want[i], " "), fmt.Sprintf("%d - %s", i, show))
			}
		})
	}
}
