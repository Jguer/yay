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

func usage() {
	fmt.Println(`usage:  yay <operation> [...]
    operations:
    yay {-h --help}
    yay {-V --version}
    yay {-D --database} <options> <package(s)>
    yay {-F --files}    [options] [package(s)]
    yay {-Q --query}    [options] [package(s)]
    yay {-R --remove}   [options] <package(s)>
    yay {-S --sync}     [options] [package(s)]
    yay {-T --deptest}  [options] [package(s)]
    yay {-U --upgrade}  [options] <file(s)>

    New operations:
    yay -Qstats  -  Displays system information
`)
}

func parser() (op string, options []string, packages []string, err error) {
	if len(os.Args) < 2 {
		err = fmt.Errorf("No operation specified.")
		return
	}

	for _, arg := range os.Args[1:] {
		if arg[0] == '-' && arg[1] != '-' {
			op = arg
		}

		if arg[0] == '-' && arg[1] == '-' {
			if arg == "--help" {
				op = arg
			}
			options = append(options, arg)
		}

		if arg[0] != '-' {
			packages = append(packages, arg)
		}
	}

	if op == "" {
		op = "yogurt"
	}

	return
}

func main() {
	var err error
	conf, err := readConfig(PacmanConf)

	op, pkgs, options, err := parser()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	switch op {
	case "-Qstats":
		err = stats(&conf)
	case "-Ss":
		for _, pkg := range pkgs {
			err = searchMode(pkg, &conf)
		}
	case "-S":
		err = InstallPackage(pkgs, &conf, options)
	case "-Syu", "-Suy":
		err = updateAndInstall(&conf, options)
	case "yogurt":
		for _, pkg := range pkgs {
			err = searchAndInstall(pkg, &conf, options)
		}
	case "--help", "-h":
		usage()
	default:
		err = passToPacman(op, pkgs, options)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
