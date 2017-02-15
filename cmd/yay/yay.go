package main

import (
	"fmt"
	"os"

	"github.com/jguer/yay"
	"github.com/jguer/yay/util"
)

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
    yay -Qstats          displays system information
    yay -Cd              remove unneeded dependencies
    yay -G [package(s)]  get pkgbuild from ABS or AUR

    New options:
    --topdown     shows repository's packages first and then aur's
    --bottomup     shows aur's packages first and then repository's
    --noconfirm   skip user input on package install
`)
}

var version = "1.92"

func parser() (op string, options []string, packages []string, err error) {
	if len(os.Args) < 2 {
		err = fmt.Errorf("no operation specified")
		return
	}

	for _, arg := range os.Args[1:] {
		if arg[0] == '-' && arg[1] != '-' {
			op = arg
		}

		if arg[0] == '-' && arg[1] == '-' {
			if arg == "--help" {
				op = arg
			} else if arg == "--topdown" {
				util.SortMode = util.TopDown
			} else if arg == "--bottomup" {
				util.SortMode = util.BottomUp
			} else if arg == "--noconfirm" {
				util.NoConfirm = true
				options = append(options, arg)
			} else {
				options = append(options, arg)
			}
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
	op, options, pkgs, err := parser()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	switch op {
	case "-Cd":
		err = yay.CleanDependencies(pkgs)
	case "-G":
		for _, pkg := range pkgs {
			err = yay.GetPkgbuild(pkg)
			if err != nil {
				fmt.Println(pkg+":", err)
			}
		}
	case "-Qstats":
		err = yay.LocalStatistics(version)
	case "-Ss", "-Ssq", "-Sqs":
		if op == "-Ss" {
			util.SearchVerbosity = util.Detailed
		} else {
			util.SearchVerbosity = util.Minimal
		}

		if pkgs != nil {
			err = yay.Search(pkgs[0], pkgs[1:])
		}
	case "-S":
		err = yay.Install(pkgs, options)
	case "-Syu", "-Suy":
		err = yay.Upgrade(options)
	case "-Si":
		err = yay.SingleSearch(pkgs, options)
	case "yogurt":
		util.SearchVerbosity = util.NumberMenu

		if pkgs != nil {
			err = yay.NumberMenu(pkgs[0], pkgs[1:], options)
		}
	case "--help", "-h":
		usage()
	default:
		err = yay.PassToPacman(op, pkgs, options)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
