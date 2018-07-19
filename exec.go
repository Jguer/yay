package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func show(cmd *exec.Cmd) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("")
	}
	return nil
}

func capture(cmd *exec.Cmd) (string, string, error) {
	var outbuf, errbuf bytes.Buffer

	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf
	err := cmd.Run()
	stdout := outbuf.String()
	stderr := errbuf.String()

	return stdout, stderr, err
}

func sudoLoopBackground() {
	updateSudo()
	go sudoLoop()
}

func sudoLoop() {
	for {
		updateSudo()
		time.Sleep(298 * time.Second)
	}
}

func updateSudo() {
	for {
		err := show(exec.Command("sudo", "-v"))
		if err != nil {
			fmt.Println(err)
		} else {
			break
		}
	}
}

func passToPacman(args *arguments) *exec.Cmd {
	argArr := make([]string, 0)

	if args.needRoot() {
		argArr = append(argArr, "sudo")
	}

	argArr = append(argArr, config.PacmanBin)
	argArr = append(argArr, cmdArgs.formatGlobals()...)
	argArr = append(argArr, args.formatArgs()...)
	if config.NoConfirm {
		argArr = append(argArr, "--noconfirm")
	}

	argArr = append(argArr, "--")

	argArr = append(argArr, args.targets...)

	return exec.Command(argArr[0], argArr[1:]...)
}

func passToMakepkg(dir string, args ...string) *exec.Cmd {
	if config.NoConfirm {
		args = append(args)
	}

	mflags := strings.Fields(config.MFlags)
	args = append(args, mflags...)

	cmd := exec.Command(config.MakepkgBin, args...)
	cmd.Dir = dir
	return cmd
}

func passToGit(dir string, _args ...string) *exec.Cmd {
	gitflags := strings.Fields(config.GitFlags)
	args := []string{"-C", dir}
	args = append(args, gitflags...)
	args = append(args, _args...)

	cmd := exec.Command(config.GitBin, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd
}
