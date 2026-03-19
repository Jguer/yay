//go:build !integration
// +build !integration

package exe

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
)

func TestBuildGitCmd(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")
	runner := &MockRunner{}
	builder := &CmdBuilder{
		GitBin:  "git",
		Runner:  runner,
		SudoBin: "su",
		Log:     logger,
	}

	cmd := builder.BuildGitCmd(context.Background(), "repo", "status")
	if filepath.Base(cmd.Path) == "git" {
		require.Equal(t, []string{"git", "-C", "repo", "status"}, cmd.Args)
		require.NotContains(t, strings.Join(cmd.Env, "|"), "GIT_WORK_TREE=")
		require.NotContains(t, strings.Join(cmd.Env, "|"), "GIT_DIR=")
	} else {
		require.Equal(t, "systemd-run", filepath.Base(cmd.Path))
	}
}

func TestBuildPacmanCmd(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")
	builder := &CmdBuilder{
		PacmanBin:        "pacman",
		PacmanConfigPath: "/etc/pacman.conf",
		PacmanDBPath:     t.TempDir(),
		Runner:           &MockRunner{},
		Log:              logger,
	}

	args := parser.MakeArguments()
	args.AddArg("Q")
	cmd := builder.BuildPacmanCmd(context.Background(), args, parser.ModeAny, true)
	require.Equal(t, "pacman", filepath.Base(cmd.Path))
	require.Contains(t, strings.Join(cmd.Args, " "), "--noconfirm")
	require.Contains(t, strings.Join(cmd.Args, " "), "--config /etc/pacman.conf")
	require.Contains(t, strings.Join(cmd.Args, " "), "--")
}

func TestBuildPrivilegeElevatorCommand(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")
	builder := &CmdBuilder{
		SudoBin: "su",
		Runner:  &MockRunner{},
		Log:     logger,
	}
	cmd := builder.buildPrivilegeElevatorCommand(context.Background(), []string{"echo", "hello"})
	require.Equal(t, "su", filepath.Base(cmd.Path))
	require.Equal(t, []string{"su", "-c", "echo hello"}, cmd.Args)
}
