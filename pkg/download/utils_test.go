package download

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v10/pkg/settings/exe"
)

type testRunner struct{}

func (t *testRunner) Capture(cmd *exec.Cmd, timeout int64) (stdout string, stderr string, err error) {
	return "", "", nil
}

func (t *testRunner) Show(cmd *exec.Cmd) error {
	return nil
}

type testGitBuilder struct {
	index         int
	test          *testing.T
	want          string
	parentBuilder *exe.CmdBuilder
}

func (t *testGitBuilder) BuildGitCmd(dir string, extraArgs ...string) *exec.Cmd {
	cmd := t.parentBuilder.BuildGitCmd(dir, extraArgs...)

	assert.Equal(t.test, t.want, cmd.String())

	t.index += 1
	return cmd
}

func (c *testGitBuilder) Show(cmd *exec.Cmd) error {
	return c.parentBuilder.Show(cmd)
}

func (c *testGitBuilder) Capture(cmd *exec.Cmd, timeout int64) (stdout, stderr string, err error) {
	return c.parentBuilder.Capture(cmd, timeout)
}
