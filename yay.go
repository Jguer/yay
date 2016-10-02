package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
)

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

func packages() ([]string, error) {
	var ps []string
	for _, arg := range os.Args[1:] {
		if arg[0] != '-' {
			ps = append(ps, arg)
		}
	}
	return ps, nil
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
	var pkgstring bytes.Buffer
	conf, err := readConfig(PacmanConf)

	op, err := operation()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	pkgs, _ := packages()

	flag, _ := flags()

	switch op {
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
		for i, pkg := range pkgs {
			pkgstring.WriteString(pkg)
			if i != len(pkgs)-1 {
				pkgstring.WriteString(" ")
			}
		}
		err = passToPacman(op, pkgstring.String(), flag)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
