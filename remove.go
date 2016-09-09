package main

import (
	"os"
	"os/exec"
	"strings"
)

func removePackage(pkg string, extra string, flags ...string) (err error) {
	cmd := exec.Command("sudo", PacmanBin, "-R"+extra, pkg, strings.Join(flags, " "))
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	return nil
}
