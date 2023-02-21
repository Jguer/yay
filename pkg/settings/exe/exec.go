package exe

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/Jguer/yay/v11/pkg/text"
)

type Runner interface {
	Capture(cmd *exec.Cmd) (stdout string, stderr string, err error)
	Show(cmd *exec.Cmd) error
}

type OSRunner struct{}

func (r *OSRunner) Show(cmd *exec.Cmd) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}
	text.Debugln("running", cmd.String())
	return cmd.Run()
}

func (r *OSRunner) Capture(cmd *exec.Cmd) (stdout, stderr string, err error) {
	text.Debugln("capturing", cmd.String())
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}

	outbuf, err := cmd.Output()
	stdout = strings.TrimSpace(string(outbuf))

	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
	}

	return stdout, stderr, err
}
