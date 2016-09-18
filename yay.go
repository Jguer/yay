package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// PacmanBin describes the default installation point of pacman
const PacmanBin string = "/usr/bin/pacman"

// PacmanConf describes the default pacman config file
const PacmanConf string = "/etc/pacman.conf"

// BuildDir is the root for package building
const BuildDir string = "/tmp/yaytmp/"

// SearchMode is search without numbers.
const SearchMode int = -1

func operation() (operation string, err error) {
	if len(os.Args) < 2 {
		return "noop", errors.New("No operation specified.")
	}
	for _, arg := range os.Args[1:] {
		if arg[0] == '-' && arg[1] != '-' {
			return arg, nil
		}
	}
	return "yogurt", nil
}

func packages() (packages string, err error) {
	var ps []string
	for _, arg := range os.Args[1:] {
		if arg[0] != '-' {
			ps = append(ps, arg)
		}
	}

	if len(ps) == 0 {
		return "", nil
	}
	packages = strings.Join(ps, " ")

	return
}

func flags() (flags string, err error) {
	var fs []string
	for _, arg := range os.Args[1:] {
		if arg[0] == '-' && arg[1] == '-' {
			fs = append(fs, arg)
		}
	}

	if len(fs) == 0 {
		return "", nil
	}

	flags = strings.Join(fs, " ")
	return
}

func main() {
	var err error
	conf, err := readConfig(PacmanConf)

	op, err := operation()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	pkg, _ := packages()

	flag, _ := flags()

	switch op {
	case "-Ss":
		err = searchMode(pkg, conf)
	case "-S":
		err = InstallPackage(pkg, conf, flag)
	case "yogurt":
		err = searchAndInstall(pkg, conf, flag)
	default:
		fmt.Println("Pass to pacman")
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
