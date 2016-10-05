package main

import (
	"fmt"
	"os"
)

var version string

// PacmanConf describes the default pacman config file
const PacmanConf string = "/etc/pacman.conf"

// BuildDir is the root for package building
const BuildDir string = "/tmp/yaytmp/"

// SearchMode is search without numbers.
const SearchMode int = -1

func operation() (operation string, err error) {
	if len(os.Args) < 2 {
		return "noop", fmt.Errorf("No operation specified.")
	}
	for _, arg := range os.Args[1:] {
		if arg[0] == '-' && arg[1] != '-' {
			return arg, nil
		}
	}
	return "yogurt", nil
}

func packages() ([]string, error) {
	var ps []string
	for _, arg := range os.Args[1:] {
		if arg[0] != '-' {
			ps = append(ps, arg)
		}
	}
	return ps, nil
}

func flags() (fs []string, err error) {
	for _, arg := range os.Args[1:] {
		if arg[0] == '-' && arg[1] == '-' {
			fs = append(fs, arg)
		}
	}

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

	pkgs, _ := packages()

	flag, _ := flags()

	switch op {
	case "-Qstats":
		err = stats(&conf)
	case "-Ss":
		for _, pkg := range pkgs {
			err = searchMode(pkg, &conf)
		}
	case "-S":
		err = InstallPackage(pkgs, &conf, flag)
	case "-Syu":
		err = updateAndInstall(&conf, flag)
	case "yogurt":
		for _, pkg := range pkgs {
			err = searchAndInstall(pkg, &conf, flag)
		}
	default:
		err = passToPacman(op, pkgs, flag)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
