package exe

import (
	"context"
	"os/exec"

	"github.com/Jguer/yay/v11/pkg/settings/parser"
)

type Call struct {
	Res  []interface{}
	Args []interface{}
}

type MockBuilder struct {
	Runner               Runner
	BuildMakepkgCmdCalls []Call
	BuildMakepkgCmdFn    func(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd
	BuildPacmanCmdFn     func(ctx context.Context, args *parser.Arguments, mode parser.TargetMode, noConfirm bool) *exec.Cmd
}

type MockRunner struct {
	ShowCalls    []Call
	CaptureCalls []Call
	ShowFn       func(cmd *exec.Cmd) error
	CaptureFn    func(cmd *exec.Cmd) (stdout string, stderr string, err error)
}

func (m *MockBuilder) BuildMakepkgCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd {
	var res *exec.Cmd
	if m.BuildMakepkgCmdFn != nil {
		res = m.BuildMakepkgCmdFn(ctx, dir, extraArgs...)
	} else {
		res = exec.CommandContext(ctx, "makepkg", extraArgs...)
	}

	m.BuildMakepkgCmdCalls = append(m.BuildMakepkgCmdCalls, Call{
		Res: []interface{}{res},
		Args: []interface{}{
			ctx,
			dir,
			extraArgs,
		},
	})

	return res
}

func (m *MockBuilder) AddMakepkgFlag(flag string) {
}

func (m *MockBuilder) BuildGitCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "git", extraArgs...)
}

func (m *MockBuilder) BuildPacmanCmd(ctx context.Context, args *parser.Arguments, mode parser.TargetMode, noConfirm bool) *exec.Cmd {
	var res *exec.Cmd

	if m.BuildPacmanCmdFn != nil {
		res = m.BuildPacmanCmdFn(ctx, args, mode, noConfirm)
	} else {
		res = exec.CommandContext(ctx, "pacman")
	}

	return res
}

func (m *MockBuilder) SetPacmanDBPath(path string) {
}

func (m *MockBuilder) SudoLoop() {
}

func (m *MockBuilder) Capture(cmd *exec.Cmd) (stdout, stderr string, err error) {
	return m.Runner.Capture(cmd)
}

func (m *MockBuilder) Show(cmd *exec.Cmd) error {
	return m.Runner.Show(cmd)
}

func (m *MockRunner) Capture(cmd *exec.Cmd) (stdout, stderr string, err error) {
	m.CaptureCalls = append(m.CaptureCalls, Call{
		Args: []interface{}{
			cmd,
		},
	})

	if m.CaptureFn != nil {
		return m.CaptureFn(cmd)
	}

	return "", "", nil
}

func (m *MockRunner) Show(cmd *exec.Cmd) error {
	var err error
	if m.ShowFn != nil {
		err = m.ShowFn(cmd)
	}

	m.ShowCalls = append(m.ShowCalls, Call{
		Args: []interface{}{
			cmd,
		},
	})

	return err
}
