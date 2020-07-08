package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/text"
)

func show(cmd *exec.Cmd) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("")
	}
	return nil
}

func capture(cmd *exec.Cmd) (stdout, stderr string, err error) {
	var outbuf, errbuf bytes.Buffer

	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf
	err = cmd.Run()
	stdout = strings.TrimSpace(outbuf.String())
	stderr = strings.TrimSpace(errbuf.String())

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
func waitLock(dbPath string) {
	lockDBPath := filepath.Join(dbPath, "db.lck")
	if _, err := os.Stat(lockDBPath); err != nil {
		return
	}

	text.Warnln(gotext.Get("%s is present.", lockDBPath))
	text.Warn(gotext.Get("There may be another Pacman instance running. Waiting..."))

	for {
		time.Sleep(3 * time.Second)
		if _, err := os.Stat(lockDBPath); err != nil {
			fmt.Println()
			return
		}
	}
}

func passToPacman(args *settings.Arguments) *exec.Cmd {
	argArr := make([]string, 0)

	mSudoFlags := strings.Fields(config.SudoFlags)

	if args.NeedRoot(config.Runtime) {
		argArr = append(argArr, config.SudoBin)
		argArr = append(argArr, mSudoFlags...)
	}

	argArr = append(argArr, config.PacmanBin)
	argArr = append(argArr, args.FormatGlobals()...)
	argArr = append(argArr, args.FormatArgs()...)
	if config.NoConfirm {
		argArr = append(argArr, "--noconfirm")
	}

	argArr = append(argArr, "--config", config.PacmanConf, "--")
	argArr = append(argArr, args.Targets...)

	if args.NeedRoot(config.Runtime) {
		waitLock(config.Runtime.PacmanConf.DBPath)
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
