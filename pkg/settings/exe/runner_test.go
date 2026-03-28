//go:build !integration
// +build !integration

package exe

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/text"
)

func TestOSRunnerCapture(t *testing.T) {
	t.Parallel()

	runner := NewOSRunner(text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test"))
	cmd := exec.CommandContext(context.Background(), "sh", "-c", "printf 'ok'; printf 'err' >&2")
	out, errOut, err := runner.Capture(cmd)

	require.NoError(t, err)
	require.Equal(t, "ok", out)
	require.Empty(t, errOut)

	cmdFail := exec.CommandContext(context.Background(), "sh", "-c", "exit 1")
	_, stderr, err := runner.Capture(cmdFail)
	require.Error(t, err)
	require.Empty(t, stderr)
}
