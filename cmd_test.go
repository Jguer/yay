//go:build !integration

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/Jguer/aur"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/db"
	"github.com/Jguer/yay/v13/pkg/db/mock"
	mockaur "github.com/Jguer/yay/v13/pkg/dep/mock"
	"github.com/Jguer/yay/v13/pkg/intrange"
	"github.com/Jguer/yay/v13/pkg/query"
	"github.com/Jguer/yay/v13/pkg/runtime"
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/settings/exe"
	settingslua "github.com/Jguer/yay/v13/pkg/settings/lua"
	"github.com/Jguer/yay/v13/pkg/settings/parser"
	"github.com/Jguer/yay/v13/pkg/text"
	"github.com/Jguer/yay/v13/pkg/vcs"
)

func TestHandleHelp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		operation  string
		wantOutput string
		wantCalls  int
	}{
		{"top-level", "", "Usage:\n    yay", 0},
		{"yay operation", "Y", "Usage: yay {-Y --yay}", 0},
		{"pacman operation", "S", "", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := text.NewLogger(&output, &output, strings.NewReader(""), false, "test")
			runner := &exe.MockRunner{}
			cmdArgs := parser.MakeArguments()
			cmdArgs.Op = tt.operation
			require.NoError(t, cmdArgs.AddArg("help"))
			run := &runtime.Runtime{
				Cfg: &settings.Configuration{Mode: parser.ModeAny},
				CmdBuilder: &exe.CmdBuilder{PacmanBin: "pacman", PacmanConfigPath: "/etc/pacman.conf",
					PacmanDBPath: t.TempDir(), Runner: runner, Log: logger},
				Logger: logger,
			}

			require.NoError(t, handleHelp(t.Context(), run, cmdArgs))
			assert.Contains(t, output.String(), tt.wantOutput)
			assert.Len(t, runner.ShowCalls, tt.wantCalls)
		})
	}
}

func TestYogurtMenuAURDB(t *testing.T) {
	t.Skip("skip until Operation service is an interface")
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
	cmdArgs.AddArg("Y")
	cmdArgs.AddTarget("yay")

	db := &mock.DBExecutor{
		AlpmArchitecturesFn: func() ([]string, error) {
			return []string{"x86_64"}, nil
		},
		RefreshHandleFn: func() error {
			return nil
		},
		ReposFn: func() []string {
			return []string{"aur"}
		},
		SyncPackagesFn: func(s ...string) []mock.IPackage {
			return []mock.IPackage{
				&mock.Package{
					PName:    "yay",
					PBase:    "yay",
					PVersion: "10.0.0",
					PDB:      mock.NewDB("aur"),
				},
			}
		},
		LocalPackageFn: func(s string) mock.IPackage {
			return nil
		},
	}
	aurCache := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{
				{
					Name:        "yay",
					PackageBase: "yay",
					Version:     "10.0.0",
				},
			}, nil
		},
	}
	logger := text.NewLogger(io.Discard, os.Stderr, strings.NewReader("1\n"), true, "test")

	run := &runtime.Runtime{
		Cfg: &settings.Configuration{
			RemoveMake: "no",
		},
		Logger:     logger,
		CmdBuilder: cmdBuilder,
		VCSStore:   &vcs.Mock{},
		QueryBuilder: query.NewSourceQueryBuilder(aurCache, logger, "votes", parser.ModeAny, "name",
			true, false, true),
		AURClient: aurCache,
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
		show = strings.ReplaceAll(show, gitBin, "git")

		// options are in a different order on different systems and on CI root user is used
		assert.Subset(t, strings.Split(show, " "), strings.Split(wantShow[i], " "), fmt.Sprintf("%d - %s", i, show))
	}
}

type fakeMenuBuilder struct{ names []string }

func (f *fakeMenuBuilder) Len() int { return len(f.names) }

func (f *fakeMenuBuilder) Execute(context.Context, db.Executor, []string) {}

func (f *fakeMenuBuilder) Results(db.Executor, query.SearchVerbosity) error { return nil }

func (f *fakeMenuBuilder) SetLua(*settingslua.Engine) {}

func (f *fakeMenuBuilder) GetTargets(include, _ intrange.IntRanges, _ mapset.Set[string]) ([]string, error) {
	targets := []string{}

	for i, name := range f.names {
		if include.Get(i + 1) {
			if !strings.Contains(name, "/") {
				name = "extra/" + name
			}

			targets = append(targets, name)
		}
	}

	return targets, nil
}

func TestRemovePackages(t *testing.T) {
	t.Parallel()

	installed := map[string]bool{"p1": true, "p3": true, "p4": true, "p5": true, "p6": true}

	tests := []struct {
		desc        string
		removal     string
		rest        string
		wantShow    []string
		answers     string
		wantFlags   []string
		wantWarned  []string
		wantErr     bool
		wantNothing bool
	}{
		{desc: "single", removal: "3", wantShow: []string{"p3"}},
		{desc: "all options", removal: "3", answers: "y\ny\ny\n", wantShow: []string{"p3"}, wantFlags: []string{"--recursive", "--cascade", "--nosave"}},
		{desc: "recursive only", removal: "3", answers: "y\nn\nn\n", wantShow: []string{"p3"}, wantFlags: []string{"--recursive"}},
		{desc: "range", removal: "3-4", wantShow: []string{"p3", "p4"}},
		{desc: "range with exclusion", removal: "1-5", rest: "^3", wantShow: []string{"p1", "p4", "p5"}, wantWarned: []string{"p2"}},
		{desc: "not installed is not blocking", removal: "2 6", wantShow: []string{"p6"}, wantWarned: []string{"p2"}},
		{desc: "only not installed", removal: "2", wantWarned: []string{"p2"}, wantNothing: true},
		{desc: "invalid range", removal: "1-a", wantErr: true},
		{desc: "free word", removal: "3", rest: "all", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			var shown []string

			mockRunner := &exe.MockRunner{
				ShowFn: func(cmd *exec.Cmd) error {
					shown = append(shown, strings.Split(cmd.String(), " --")[0])
					return nil
				},
			}
			cmdBuilder := &exe.CmdBuilder{
				PacmanBin:        "pacman",
				PacmanConfigPath: "/etc/pacman.conf",
				Runner:           mockRunner,
			}
			dbExecutor := &mock.DBExecutor{
				LocalPackageFn: func(s string) mock.IPackage {
					if installed[s] {
						return &mock.Package{PName: s}
					}

					return nil
				},
			}

			stderr := &bytes.Buffer{}
			logger := text.NewLogger(stderr, stderr, strings.NewReader(tc.answers+"\n\n\n"), false, "test")
			run := &runtime.Runtime{
				Cfg:        &settings.Configuration{},
				Logger:     logger,
				CmdBuilder: cmdBuilder,
				VCSStore:   &vcs.Mock{},
			}
			builder := &fakeMenuBuilder{names: []string{"p1", "p2", "p3", "p4", "p5", "p6"}}

			removeTargets, _, err := splitMenuSelection(builder, tc.removal, tc.rest)
			if err == nil {
				_, err = removePackages(context.Background(), run, dbExecutor, removeTargets)
			}

			if tc.wantErr {
				require.Error(t, err)
				assert.Empty(t, mockRunner.ShowCalls)

				return
			}

			require.NoError(t, err)

			for _, name := range tc.wantWarned {
				assert.Contains(t, stderr.String(), name)
			}

			if tc.wantNothing {
				assert.Empty(t, mockRunner.ShowCalls)
				return
			}

			require.Len(t, mockRunner.ShowCalls, 1)
			assert.Contains(t, shown[0], "-R")

			args := mockRunner.ShowCalls[0].Args[0].(*exec.Cmd).Args
			assert.Equal(t, tc.wantShow, args[len(args)-len(tc.wantShow):])

			for _, flag := range []string{"--recursive", "--cascade", "--nosave"} {
				assert.Equal(t, slices.Contains(tc.wantFlags, flag), slices.Contains(args, flag), flag)
			}
		})
	}
}

func TestRemovePackagesAUR(t *testing.T) {
	t.Parallel()

	mockRunner := &exe.MockRunner{ShowFn: func(*exec.Cmd) error { return nil }}
	vcsStore := &vcs.Mock{}
	run := &runtime.Runtime{
		Cfg:    &settings.Configuration{},
		Logger: text.NewLogger(io.Discard, io.Discard, strings.NewReader("n\nn\nn\n"), false, "test"),
		CmdBuilder: &exe.CmdBuilder{
			PacmanBin:        "pacman",
			PacmanConfigPath: "/etc/pacman.conf",
			Runner:           mockRunner,
		},
		VCSStore: vcsStore,
	}
	dbExecutor := &mock.DBExecutor{
		LocalPackageFn: func(s string) mock.IPackage { return &mock.Package{PName: s} },
	}
	builder := &fakeMenuBuilder{names: []string{"aur/foo-git", "extra/bar"}}

	removeTargets, _, err := splitMenuSelection(builder, "1-2", "")
	require.NoError(t, err)

	removed, err := removePackages(context.Background(), run, dbExecutor, removeTargets)
	require.NoError(t, err)
	assert.Equal(t, 2, removed)

	require.Len(t, mockRunner.ShowCalls, 1)

	args := mockRunner.ShowCalls[0].Args[0].(*exec.Cmd).Args
	assert.Equal(t, []string{"foo-git", "bar"}, args[len(args)-2:])
}

func TestSplitMenuSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc, removal, rest string
		wantRemove          []string
		wantInstall         []string
		wantErr             bool
	}{
		{desc: "install and remove", removal: "2", rest: "1", wantRemove: []string{"extra/p2"}, wantInstall: []string{"extra/p1"}},
		{desc: "ranges", removal: "4-5", rest: "1-2", wantRemove: []string{"extra/p4", "extra/p5"}, wantInstall: []string{"extra/p1", "extra/p2"}},
		{desc: "exclusion applies to both", removal: "4-6", rest: "1-3 ^2 ^5", wantRemove: []string{"extra/p4", "extra/p6"}, wantInstall: []string{"extra/p1", "extra/p3"}},
		{desc: "removal only with exclusion", removal: "1-5", rest: "^3", wantRemove: []string{"extra/p1", "extra/p2", "extra/p4", "extra/p5"}},
		{desc: "removal only", removal: "2", wantRemove: []string{"extra/p2"}},
		{desc: "invalid removal", removal: "1-a", wantErr: true},
		{desc: "free word", removal: "2", rest: "all", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			builder := &fakeMenuBuilder{names: []string{"p1", "p2", "p3", "p4", "p5", "p6"}}

			remove, install, err := splitMenuSelection(builder, tc.removal, tc.rest)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantRemove, remove)
			assert.Equal(t, tc.wantInstall, install)
		})
	}
}
