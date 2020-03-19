package main

import (
	"bytes"
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
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
		mSudoFlags := strings.Fields(config.SudoFlags)
		mSudoFlags = append([]string{"-v"}, mSudoFlags...)
		err := show(exec.Command(config.SudoBin, mSudoFlags...))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
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

	mSudoFlags := strings.Fields(config.SudoFlags)

	if args.needRoot() {
		argArr = append(argArr, config.SudoBin)
		argArr = append(argArr, mSudoFlags...)
	}

	argArr = append(argArr, config.PacmanBin)
	argArr = append(argArr, cmdArgs.formatGlobals()...)
	argArr = append(argArr, args.formatArgs()...)
	if config.NoConfirm {
		argArr = append(argArr, "--noconfirm")
	}

	argArr = append(argArr, "--config", config.PacmanConf)
	argArr = append(argArr, "--")
	argArr = append(argArr, args.targets...)

	if args.needRoot() {
		waitLock()
	}
	return exec.Command(argArr[0], argArr[1:]...)
}

func passToMakepkg(dir string, args ...string) *exec.Cmd {
	mflags := strings.Fields(config.MFlags)
	args = append(args, mflags...)

	if config.MakepkgConf != "" {
		args = append(args, "--config", config.MakepkgConf)
	}

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
	return cmd
}

func isTty() bool {
	return terminal.IsTerminal(int(os.Stdout.Fd()))
}
