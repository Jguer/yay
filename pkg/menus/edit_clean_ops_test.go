//go:build !integration
// +build !integration

package menus

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
)

func TestEditor(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")
	editorPath, args := editor(logger, "echo", "--help", false)
	require.NotEmpty(t, editorPath)
	require.Contains(t, args, "--help")
}

func TestEditPkgbuilds(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")
	pkgb := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(pkgb, "PKGBUILD"), []byte("test"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(pkgb, "subpackage"), []byte("pkg"), 0o644))
	err := editPkgbuilds(logger, map[string]string{"pkg": pkgb}, []string{"pkg"}, "echo", "", nil, false)
	require.NoError(t, err)
}

func TestCleanFnNoDirs(t *testing.T) {
	t.Parallel()

	run := &runtime.Runtime{
		Logger: text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test"),
		Cfg:    &settings.Configuration{},
	}
	require.NoError(t, CleanFn(context.Background(), run, io.Discard, map[string]string{}, mapset.NewThreadUnsafeSet[string]()))
}

func TestCleanFnWithDirs(t *testing.T) {
	t.Parallel()

	execCmd := &fakeCmdBuilder{}
	run := &runtime.Runtime{
		Logger:     text.NewLogger(io.Discard, io.Discard, strings.NewReader("a\n"), false, "test"),
		CmdBuilder: execCmd,
		Cfg:        &settings.Configuration{},
	}

	dirs := map[string]string{
		"pkg": t.TempDir(),
	}
	installed := mapset.NewThreadUnsafeSet[string]()
	err := CleanFn(context.Background(), run, io.Discard, dirs, installed)
	require.NoError(t, err)
	require.Len(t, execCmd.showCalls, 2)
	require.Contains(t, strings.Join(execCmd.showCalls, " "), "reset")
}

type fakeCmdBuilder struct {
	showCalls []string
}

func (f *fakeCmdBuilder) BuildGitCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "git", extraArgs...)
}

func (f *fakeCmdBuilder) BuildPacmanCmd(ctx context.Context, args *parser.Arguments, mode parser.TargetMode, noConfirm bool) *exec.Cmd {
	return exec.CommandContext(ctx, "pacman", args.FormatGlobals()...)
}

func (f *fakeCmdBuilder) BuildMakepkgCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "makepkg", extraArgs...)
}

func (f *fakeCmdBuilder) BuildGPGCmd(ctx context.Context, extraArgs ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "gpg", extraArgs...)
}

func (f *fakeCmdBuilder) AddMakepkgFlag(flag string) {}
func (f *fakeCmdBuilder) GetKeepSrc() bool           { return false }
func (f *fakeCmdBuilder) SudoLoop()                  {}

func (f *fakeCmdBuilder) Capture(cmd *exec.Cmd) (string, string, error) {
	return "", "", nil
}

func (f *fakeCmdBuilder) Show(cmd *exec.Cmd) error {
	f.showCalls = append(f.showCalls, cmd.String())
	return nil
}
