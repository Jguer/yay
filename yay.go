package main

import (
	"fmt"
	"os"
	"strings"
)

// PacmanBin describes the default installation point of pacman
const PacmanBin string = "/usr/bin/pacman"

// PacmanConf describes the default pacman config file
const PacmanConf string = "/etc/pacman.conf"

// SearchMode is search without numbers
const SearchMode bool = true

// BuildDir is the root for package building
const BuildDir string = "/tmp/yaytmp/"

func main() {
	var err error
	conf, err := readConfig(PacmanConf)

	if os.Args[1] == "-Ss" {
		err = searchMode(strings.Join(os.Args[2:], " "), conf)

	} else if os.Args[1] == "-S" {
		err = InstallPackage(os.Args[2], conf, os.Args[3:]...)
	} else {
		err = searchAndInstall(os.Args[1], conf, os.Args[3:]...)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
