//go:build !integration
// +build !integration

package menus

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
)

func TestSelectionMenu(t *testing.T) {
	t.Parallel()

	installed := mapset.NewThreadUnsafeSet[string]()
	installed.Add("alpha")

	basenames := []string{"alpha", "beta"}
	dirs := map[string]string{
		"alpha": "existing-alpha",
		"beta":  "missing-beta",
	}

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{name: "all", input: "a\n", expected: 2},
		{name: "none", input: "n\n", expected: 0},
		{name: "installed only", input: "i\n", expected: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := text.NewLogger(&bytes.Buffer{}, &bytes.Buffer{},
				strings.NewReader(tc.input), false, "test")
			selected, err := selectionMenu(logger, dirs, basenames, installed, "choose", false, "", nil)
			require.NoError(t, err)
			require.Len(t, selected, tc.expected)
		})
	}
}

func TestPkgbuildNumberMenu(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	logger := text.NewLogger(&out, io.Discard, strings.NewReader(""), false, "test")
	dirs := map[string]string{
		"alpha": t.TempDir(),
		"beta":  t.TempDir(),
	}
	installed := mapset.NewThreadUnsafeSet[string]()
	installed.Add("alpha")

	basenames := []string{"alpha", "beta"}
	pkgbuildNumberMenu(logger, dirs, basenames, installed)

	require.Contains(t, out.String(), "alpha")
	require.Contains(t, out.String(), "beta")
}

func TestShowPkgbuildDiffHelpers(t *testing.T) {
	t.Parallel()

	builder := &fakeMenusCmdBuilder{
		captureFn: func(cmd *exec.Cmd) (string, string, error) {
			if strings.Contains(cmd.String(), "--quiet --verify AUR_SEEN") {
				return "", "", nil
			}

			return "deadbeef\n", "", nil
		},
	}

	require.True(t, gitHasLastSeenRef(context.Background(), builder, "/tmp"))
	ref, err := getLastSeenHash(context.Background(), builder, "/tmp")
	require.NoError(t, err)
	require.Equal(t, "deadbeef", ref)
}

type fakeMenusCmdBuilder struct {
	captureFn  func(*exec.Cmd) (string, string, error)
	captureCnt int
}

func (f *fakeMenusCmdBuilder) BuildGitCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "git", extraArgs...)
}

func (f *fakeMenusCmdBuilder) BuildMakepkgCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "makepkg", extraArgs...)
}

func (f *fakeMenusCmdBuilder) BuildGPGCmd(ctx context.Context, extraArgs ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "gpg", extraArgs...)
}

func (f *fakeMenusCmdBuilder) BuildPacmanCmd(ctx context.Context, args *parser.Arguments, mode parser.TargetMode, noConfirm bool) *exec.Cmd {
	return exec.CommandContext(ctx, "pacman")
}

func (f *fakeMenusCmdBuilder) AddMakepkgFlag(flag string) {}

func (f *fakeMenusCmdBuilder) GetKeepSrc() bool { return false }

func (f *fakeMenusCmdBuilder) SudoLoop() {}

func (f *fakeMenusCmdBuilder) Capture(cmd *exec.Cmd) (string, string, error) {
	f.captureCnt++
	if f.captureFn != nil {
		return f.captureFn(cmd)
	}

	return "", "", nil
}

func (f *fakeMenusCmdBuilder) Show(cmd *exec.Cmd) error {
	return nil
}
