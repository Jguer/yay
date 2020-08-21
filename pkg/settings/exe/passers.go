package exe

import (
	"os"
	"os/exec"
	"strings"
)

type CmdBuilder struct {
	GitBin   string
	GitFlags []string
}

func NewCmdBuilder(gitBin, gitFlags string) *CmdBuilder {
	c := &CmdBuilder{GitBin: gitBin, GitFlags: strings.Fields(gitFlags)}

	return c
}

func (c *CmdBuilder) BuildGitCmd(dir string, extraArgs ...string) *exec.Cmd {
	args := make([]string, 0, len(c.GitFlags))
	copy(args, c.GitFlags)

	if dir != "" {
		args = append(args, "-C", dir)
	}

	args = append(args, extraArgs...)

	cmd := exec.Command(c.GitBin, args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd
}
