package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/text"
)

func sudoLoopBackground() {
	updateSudo()
	go sudoLoop()
}

func sudoLoop() {
	for {
		updateSudo()
		time.Sleep(241 * time.Second)
	}
}

func updateSudo() {
	for {
		err := config.Runtime.CmdRunner.Show(exec.Command(config.SudoBin, "-v"))
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
	argArr := make([]string, 0, 32)

	if args.NeedRoot(config.Runtime) {
		argArr = append(argArr, config.SudoBin)
		argArr = append(argArr, strings.Fields(config.SudoFlags)...)
	}

	argArr = append(argArr, config.PacmanBin)
	argArr = append(argArr, args.FormatGlobals()...)
	argArr = append(argArr, args.FormatArgs()...)
	if settings.NoConfirm {
		argArr = append(argArr, "--noconfirm")
	}

	argArr = append(argArr, "--config", config.PacmanConf, "--")
	argArr = append(argArr, args.Targets...)

	if args.NeedRoot(config.Runtime) {
		waitLock(config.Runtime.PacmanConf.DBPath)
	}
	return exec.Command(argArr[0], argArr[1:]...)
}
