package exe

import (
	"os"
	"os/exec"
)

type CmdBuilder struct {
	GitBin          string
	GitFlags        []string
	MakepkgFlags    []string
	MakepkgConfPath string
	MakepkgBin      string
}

func (c *CmdBuilder) BuildGitCmd(dir string, extraArgs ...string) *exec.Cmd {
	args := make([]string, len(c.GitFlags), len(c.GitFlags)+len(extraArgs))
	copy(args, c.GitFlags)

	if dir != "" {
		args = append(args, "-C", dir)
	}

	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}

	cmd := exec.Command(c.GitBin, args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd
}

func (c *CmdBuilder) BuildMakepkgCmd(dir string, extraArgs ...string) *exec.Cmd {
	args := make([]string, len(c.MakepkgFlags), len(c.MakepkgFlags)+len(extraArgs))
	copy(args, c.MakepkgFlags)

	if c.MakepkgConfPath != "" {
		args = append(args, "--config", c.MakepkgConfPath)
	}

	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}

	cmd := exec.Command(c.MakepkgBin, args...)
	cmd.Dir = dir
	return cmd
}
