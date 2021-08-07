package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"
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
