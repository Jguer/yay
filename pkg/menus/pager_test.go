//go:build !integration

package menus

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/text"
)

func TestResolvePager(t *testing.T) {
	// Not parallel: t.Setenv cannot be used after t.Parallel.
	tests := []struct {
		name        string
		pagerConfig string
		pager       string
		want        string
	}{
		{name: "config wins", pagerConfig: "delta", pager: "less", want: "delta"},
		{name: "PAGER when config empty", pagerConfig: "", pager: "most", want: "most"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PAGER", tt.pager)

			assert.Equal(t, tt.want, resolvePager(tt.pagerConfig))
		})
	}
}

func TestCollectPkgbuildDiffs(t *testing.T) {
	t.Parallel()

	var warnOut bytes.Buffer
	logger := text.NewLogger(&bytes.Buffer{}, &warnOut, strings.NewReader(""), false, "test")

	builder := &fakeMenusCmdBuilder{
		captureFn: func(cmd *exec.Cmd) (string, string, error) {
			joined := strings.Join(cmd.Args, " ")

			switch {
			case strings.Contains(joined, "--quiet --verify AUR_SEEN"):
				return "", "", nil
			case strings.Contains(joined, "rev-parse AUR_SEEN HEAD@{upstream}"):
				return "aaa\nbbb\n", "", nil
			case strings.Contains(joined, "rev-parse AUR_SEEN"):
				return "aaa\n", "", nil
			case strings.Contains(joined, "--no-pager diff"):
				require.Contains(t, joined, "--no-pager")
				pkg := "alpha"
				if strings.Contains(joined, "beta/") {
					pkg = "beta"
				}

				return "diff for " + pkg, "", nil
			default:
				t.Fatalf("unexpected git args: %s", joined)

				return "", "", nil
			}
		},
	}

	dirs := map[string]string{
		"alpha": "/tmp/alpha",
		"beta":  "/tmp/beta",
	}

	out, errs := collectPkgbuildDiffs(context.Background(), builder, logger, dirs, []string{"alpha", "beta"})
	require.Empty(t, errs)
	require.Contains(t, out, "Showing diff for")
	require.Contains(t, out, "diff for alpha")
	require.Contains(t, out, "diff for beta")
	// Headers for both packages appear before their diffs in one buffer.
	alphaIdx := strings.Index(out, "diff for alpha")
	betaIdx := strings.Index(out, "diff for beta")
	require.Positive(t, alphaIdx)
	require.Greater(t, betaIdx, alphaIdx)
}

func TestShowPkgbuildDiffsPagesOnce(t *testing.T) {
	t.Parallel()

	var paged strings.Builder
	origRunPager := runPager
	origIsTTY := isStdoutTerminal
	t.Cleanup(func() {
		runPager = origRunPager
		isStdoutTerminal = origIsTTY
	})

	isStdoutTerminal = func() bool { return true }
	runPager = func(_ context.Context, content, _ string) error {
		paged.WriteString(content)

		return nil
	}

	logger := text.NewLogger(&bytes.Buffer{}, &bytes.Buffer{}, strings.NewReader(""), false, "test")
	builder := &fakeMenusCmdBuilder{
		captureFn: func(cmd *exec.Cmd) (string, string, error) {
			joined := strings.Join(cmd.Args, " ")
			switch {
			case strings.Contains(joined, "--quiet --verify AUR_SEEN"):
				return "", "", exec.ErrNotFound // no AUR_SEEN → empty tree, always show
			case strings.Contains(joined, "--no-pager diff"):
				return "+++ changed", "", nil
			default:
				return "", "", nil
			}
		},
	}

	err := showPkgbuildDiffs(context.Background(), builder, logger,
		map[string]string{"pkg": "/tmp/pkg"}, []string{"pkg"}, "")
	require.NoError(t, err)
	require.Contains(t, paged.String(), "+++ changed")
	require.Contains(t, paged.String(), "Showing diff for")
}
