package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	stdout := strings.TrimSpace(outbuf.String())
	stderr := strings.TrimSpace(errbuf.String())

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

// waitLock will lock yay checking the status of db.lck until it does not exist
func waitLock() {
	if _, err := os.Stat(filepath.Join(pacmanConf.DBPath, "db.lck")); err != nil {
		return
	}

	fmt.Println(bold(yellow(smallArrow)), filepath.Join(pacmanConf.DBPath, "db.lck"), "is present.")

	fmt.Print(bold(yellow(smallArrow)), " There may be another Pacman instance running. Waiting...")

	for {
		time.Sleep(3 * time.Second)
		if _, err := os.Stat(filepath.Join(pacmanConf.DBPath, "db.lck")); err != nil {
			fmt.Println()
			return
		}
	}
}

func passToPacman(args *arguments) *exec.Cmd {
	argArr := make([]string, 0)

	if args.needRoot() {
		argArr = append(argArr, "sudo")
	}

	argArr = append(argArr, config.value["pacmancommand"])
	argArr = append(argArr, cmdArgs.formatGlobals()...)
	argArr = append(argArr, args.formatArgs()...)
	if config.noConfirm {
		argArr = append(argArr, "--noconfirm")
	}

	argArr = append(argArr, "--config", config.value["pacmanconf"])
	argArr = append(argArr, "--")
	argArr = append(argArr, args.targets...)

	if args.needRoot() {
		waitLock()
	}
	return exec.Command(argArr[0], argArr[1:]...)
}

func passToMakepkg(dir string, args ...string) *exec.Cmd {
	if config.noConfirm {
		args = append(args)
	}

	mflags := strings.Fields(config.value["makepkgflags"])
	args = append(args, mflags...)

	if config.value["makepkgconf"] != "" {
		args = append(args, "--config", config.value["makepkgconf"])
	}

	cmd := exec.Command(config.value["makepkgcommand"], args...)
	cmd.Dir = dir
	return cmd
}

func passToGit(dir string, _args ...string) *exec.Cmd {
	gitflags := strings.Fields(config.value["gitflags"])
	args := []string{"-C", dir}
	args = append(args, gitflags...)
	args = append(args, _args...)

	cmd := exec.Command(config.value["gitcommand"], args...)
	return cmd
}

func isTty() bool {
	cmd := exec.Command("test", "-t", "1")
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	return err == nil
}
