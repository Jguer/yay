package exe

import (
	"os"
	"os/exec"
	"strings"
)

type Runner interface {
	Capture(cmd *exec.Cmd) (stdout string, stderr string, err error)
	Show(cmd *exec.Cmd) error
}

type OSRunner struct{}

func (r *OSRunner) Show(cmd *exec.Cmd) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func (r *OSRunner) Capture(cmd *exec.Cmd) (stdout, stderr string, err error) {
	outbuf, err := cmd.Output()
	stdout = strings.TrimSpace(string(outbuf))

	if err != nil {
		if exitErr, isExitError := err.(*exec.ExitError); isExitError {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
	}

	return stdout, stderr, err
}
