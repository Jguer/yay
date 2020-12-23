package exe

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/Jguer/yay/v10/pkg/text"
)

type Runner interface {
	Capture(cmd *exec.Cmd, timeout int64) (stdout string, stderr string, err error)
	Show(cmd *exec.Cmd) error
}

type OSRunner struct {
}

func (r *OSRunner) Show(cmd *exec.Cmd) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func (r *OSRunner) Capture(cmd *exec.Cmd, timeout int64) (stdout, stderr string, err error) {
	var outbuf, errbuf bytes.Buffer
	var timer *time.Timer
	timedOut := false

	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	err = cmd.Start()
	if err != nil {
		stdout = strings.TrimSpace(outbuf.String())
		stderr = strings.TrimSpace(errbuf.String())
		return stdout, stderr, err
	}

	if timeout != 0 {
		timer = time.AfterFunc(time.Duration(timeout)*time.Second, func() {
			err = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			if err != nil {
				text.Errorln(err)
			}
			timedOut = true
		})
	}

	err = cmd.Wait()
	if timeout != 0 {
		timer.Stop()
	}

	stdout = strings.TrimSpace(outbuf.String())
	stderr = strings.TrimSpace(errbuf.String())
	if err != nil {
		return stdout, stderr, err
	}

	if timedOut {
		err = fmt.Errorf("command timed out")
	}

	return stdout, stderr, err
}
