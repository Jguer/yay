package pacmanconf

import (
	"bytes"
	"os/exec"
)

func pacmanconf(args []string) (string, string, error) {
	var outbuf, errbuf bytes.Buffer
	cmd := exec.Command("pacman-conf", args...)

	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	err := cmd.Run()
	stdout := outbuf.String()
	stderr := errbuf.String()

	return stdout, stderr, err
}
