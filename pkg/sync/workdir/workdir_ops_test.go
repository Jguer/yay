//go:build !integration
// +build !integration

package workdir

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
)

func TestMergePkgbuilds(t *testing.T) {
	t.Parallel()

	builder := &fakeWorkdirCmdBuilder{}
	dirs := map[string]string{
		"one": "/tmp/pkg-one",
		"two": "/tmp/pkg-two",
	}

	err := mergePkgbuilds(context.Background(), builder, dirs)
	require.NoError(t, err)
	require.Len(t, builder.captureCalls, 4)
	require.True(t, strings.Contains(builder.captureCalls[0], "reset"))
	require.True(t, strings.Contains(builder.captureCalls[1], "merge"))
}

func TestCleanAfter(t *testing.T) {
	t.Parallel()

	builder := &fakeWorkdirCmdBuilder{}
	run := &runtime.Runtime{
		Logger:     text.NewLogger(&strings.Builder{}, &strings.Builder{}, strings.NewReader(""), false, "test"),
		CmdBuilder: builder,
	}
	dirs := map[string]string{
		"one": "/tmp/pkg-one",
		"two": "/tmp/pkg-two",
	}

	cleanAfter(context.Background(), run, builder, dirs)
	require.Len(t, builder.captureCalls, 2)
	require.Len(t, builder.showCalls, 2)
	require.True(t, strings.Contains(builder.captureCalls[0], "reset"))
	require.True(t, strings.Contains(builder.showCalls[0], "clean"))
}

func TestRemoveMake(t *testing.T) {
	var old bool
	old, settings.NoConfirm = settings.NoConfirm, false
	defer func() { settings.NoConfirm = old }()

	builder := &fakeWorkdirCmdBuilder{}
	args := parser.MakeArguments()
	args.AddArg("Q")
	_ = removeMake(context.Background(), &settings.Configuration{}, builder, []string{"foo", "bar"}, args)

	require.Len(t, builder.showCalls, 1)
	require.Contains(t, builder.showCalls[0], "pacman")
	require.Contains(t, builder.showCalls[0], "-R")
	require.Contains(t, builder.showCalls[0], "foo")
	require.Contains(t, builder.showCalls[0], "bar")
	require.True(t, strings.Contains(builder.showCalls[0], "--noconfirm"))
}

type fakeWorkdirCmdBuilder struct {
	captureCalls []string
	showCalls    []string
}

func (f *fakeWorkdirCmdBuilder) BuildGitCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "git", extraArgs...)
}

func (f *fakeWorkdirCmdBuilder) BuildPacmanCmd(ctx context.Context, args *parser.Arguments, mode parser.TargetMode, noConfirm bool) *exec.Cmd {
	cmdArgs := append([]string{"pacman"}, args.FormatGlobals()...)
	cmdArgs = append(cmdArgs, args.FormatArgs()...)
	cmdArgs = append(cmdArgs, args.Targets...)
	if noConfirm {
		cmdArgs = append(cmdArgs, "--noconfirm")
	}

	return exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
}

func (f *fakeWorkdirCmdBuilder) BuildMakepkgCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "makepkg", extraArgs...)
}

func (f *fakeWorkdirCmdBuilder) BuildGPGCmd(ctx context.Context, extraArgs ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "gpg", extraArgs...)
}

func (f *fakeWorkdirCmdBuilder) AddMakepkgFlag(string) {}
func (f *fakeWorkdirCmdBuilder) GetKeepSrc() bool      { return false }
func (f *fakeWorkdirCmdBuilder) SudoLoop()             {}

func (f *fakeWorkdirCmdBuilder) Capture(cmd *exec.Cmd) (string, string, error) {
	f.captureCalls = append(f.captureCalls, cmd.String())
	return "", "", nil
}

func (f *fakeWorkdirCmdBuilder) Show(cmd *exec.Cmd) error {
	f.showCalls = append(f.showCalls, cmd.String())
	return nil
}
