package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jguer/yay/aur"
	pac "github.com/jguer/yay/pacman"
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
    --topdown            shows repository's packages first and then aur's
    --bottomup           shows aur's packages first and then repository's
    --noconfirm          skip user input on package install
`)
}

var version = "1.116"

func parser() (op string, options []string, packages []string, err error) {
	if len(os.Args) < 2 {
		err = fmt.Errorf("no operation specified")
		return
	}
	op = "yogurt"

	for _, arg := range os.Args[1:] {
		if arg[0] == '-' && arg[1] != '-' {
			switch arg {
			case "-b":
				util.Build = true
			default:
				op = arg
			}
			continue
		}

		if arg[0] == '-' && arg[1] == '-' {
			switch arg {
			case "--build":
				util.Build = true
			case "--bottomup":
				util.SortMode = util.BottomUp
			case "--topdown":
				util.SortMode = util.TopDown

			case "--complete":
				util.Shell = "sh"
				Complete()
				os.Exit(0)
			case "--fcomplete":
				util.Shell = "fish"
				Complete()
				os.Exit(0)
			case "--help":
				usage()
				os.Exit(0)
			case "--noconfirm":
				util.NoConfirm = true
				fallthrough
			default:
				options = append(options, arg)
			}
			continue
		}

		packages = append(packages, arg)
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
		err = CleanDependencies(pkgs)
	case "-G":
		for _, pkg := range pkgs {
			err = GetPkgbuild(pkg)
			if err != nil {
				fmt.Println(pkg+":", err)
			}
		}
	case "-Qstats":
		err = LocalStatistics(version)
	case "-Ss", "-Ssq", "-Sqs":
		if op == "-Ss" {
			util.SearchVerbosity = util.Detailed
		} else {
			util.SearchVerbosity = util.Minimal
		}

		if pkgs != nil {
			err = SyncSearch(pkgs)
		}
	case "-S":
		err = Install(pkgs, options)
	case "-Syu", "-Suy":
		err = Upgrade(options)
	case "-Si":
		err = SyncInfo(pkgs, options)
	case "yogurt":
		util.SearchVerbosity = util.NumberMenu

		if pkgs != nil {
			err = numberMenu(pkgs, options)
		}
	default:
		err = PassToPacman(op, pkgs, options)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// NumberMenu presents a CLI for selecting packages to install.
func numberMenu(pkgS []string, flags []string) (err error) {
	var num int

	aq, numaq, err := aur.Search(pkgS, true)
	if err != nil {
		fmt.Println("Error during AUR search:", err)
	}
	pq, numpq, err := pac.Search(pkgS)
	if err != nil {
		return
	}

	if numpq == 0 && numaq == 0 {
		return fmt.Errorf("no packages match search")
	}

	if util.SortMode == util.BottomUp {
		aq.PrintSearch(numpq)
		pq.PrintSearch()
	} else {
		pq.PrintSearch()
		aq.PrintSearch(numpq)
	}

	fmt.Printf("\x1b[32m%s\x1b[0m\nNumbers:", "Type numbers to install. Separate each number with a space.")
	reader := bufio.NewReader(os.Stdin)
	numberBuf, overflow, err := reader.ReadLine()
	if err != nil || overflow {
		fmt.Println(err)
		return
	}

	numberString := string(numberBuf)
	var aurInstall []string
	var repoInstall []string
	result := strings.Fields(numberString)
	for _, numS := range result {
		num, err = strconv.Atoi(numS)
		if err != nil {
			continue
		}

		// Install package
		if num > numaq+numpq-1 || num < 0 {
			continue
		} else if num > numpq-1 {
			if util.SortMode == util.BottomUp {
				aurInstall = append(aurInstall, aq[numaq+numpq-num-1].Name)
			} else {
				aurInstall = append(aurInstall, aq[num-numpq].Name)
			}
		} else {
			if util.SortMode == util.BottomUp {
				repoInstall = append(repoInstall, pq[numpq-num-1].Name)
			} else {
				repoInstall = append(repoInstall, pq[num].Name)
			}
		}
	}

	if len(repoInstall) != 0 {
		pac.Install(repoInstall, flags)
	}

	if len(aurInstall) != 0 {
		q, n, err := aur.MultiInfo(aurInstall)
		if err != nil {
			return err
		} else if n != len(aurInstall) {
			q.MissingPackage(aurInstall)
		}

		var finalrm []string
		for _, aurpkg := range q {
			finalmdeps, err := aurpkg.Install(flags)
			finalrm = append(finalrm, finalmdeps...)
			if err != nil {
				// Do not abandon program, we might still be able to install the rest
				fmt.Println(err)
			}
		}

		if len(finalrm) != 0 {
			aur.RemoveMakeDeps(finalrm)
		}
	}

	return nil
}
