package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
	yay --gendb          generates development package DB used for updating.

	Permanent configuration options:
	--topdown            shows repository's packages first and then aur's
	--bottomup           shows aur's packages first and then repository's
	--devel              Check -git/-svn/-hg development version
	--nodevel            Disable development version checking
	--afterclean         Clean package sources after successful build
	--noafterclean       Disable package sources cleaning after successful build
	--timeupdate         Check package's modification date and version
	--notimeupdate       Check only package version change

	New options:
	--noconfirm          skip user input on package install
	--printconfig        Prints current yay configuration
	`)
}

func init() {
	var configHome string // configHome handles config directory home
	var cacheHome string  // cacheHome handles cache home
	var err error

	if 0 == os.Geteuid() {
		fmt.Println("Please avoid running yay as root/sudo.")
	}

	if configHome = os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		if info, err := os.Stat(configHome); err == nil && info.IsDir() {
			configHome = configHome + "/yay"
		} else {
			configHome = os.Getenv("HOME") + "/.config/yay"
		}
	} else {
		configHome = os.Getenv("HOME") + "/.config/yay"
	}

	if cacheHome = os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		if info, err := os.Stat(cacheHome); err == nil && info.IsDir() {
			cacheHome = cacheHome + "/yay"
		} else {
			cacheHome = os.Getenv("HOME") + "/.cache/yay"
		}
	} else {
		cacheHome = os.Getenv("HOME") + "/.cache/yay"
	}

	configFile = configHome + "/config.json"
	vcsFile = configHome + "/yay_vcs.json"
	completionFile = cacheHome + "/aur_"

	////////////////
	// yay config //
	////////////////
	defaultSettings(&config)

	if _, err = os.Stat(configFile); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Dir(configFile), 0755)
		if err != nil {
			fmt.Println("Unable to create config directory:",
				filepath.Dir(configFile), err)
			os.Exit(2)
		}
		// Save the default config if nothing is found
		config.saveConfig()
	} else {
		cfile, errf := os.OpenFile(configFile, os.O_RDWR|os.O_CREATE, 0644)
		if errf != nil {
			fmt.Println("Error reading config:", err)
		} else {
			defer cfile.Close()
			decoder := json.NewDecoder(cfile)
			err = decoder.Decode(&config)
			if err != nil {
				fmt.Println("Loading default Settings.\nError reading config:",
					err)
				defaultSettings(&config)
			}
		}
	}

	/////////////////
	// vcs config //
	////////////////
	updated = false

	vfile, err := os.Open(vcsFile)
	if err == nil {
		defer vfile.Close()
		decoder := json.NewDecoder(vfile)
		_ = decoder.Decode(&savedInfo)
	}

	/////////////////
	// alpm config //
	/////////////////
	alpmConf, err = readAlpmConfig(config.PacmanConf)
	if err != nil {
		fmt.Println("Unable to read Pacman conf", err)
		os.Exit(1)
	}

	alpmHandle, err = alpmConf.CreateHandle()
	if err != nil {
		fmt.Println("Unable to CreateHandle", err)
		os.Exit(1)
	}
}

func parser() (op string, options, packages []string, changedConfig bool, err error) {
	if len(os.Args) < 2 {
		err = fmt.Errorf("no operation specified")
		return
	}
	changedConfig = false
	op = "yogurt"

	for _, arg := range os.Args[1:] {
		if len(arg) < 2 {
			continue
		}
		if arg[0] == '-' && arg[1] != '-' {
			switch arg {
			case "-V":
				arg = "--version"
			case "-h":
				arg = "--help"
			default:
				op = arg
				continue
			}
		}

		if strings.HasPrefix(arg, "--") {
			changedConfig = true
			switch arg {
			case "--afterclean":
				config.CleanAfter = true
			case "--noafterclean":
				config.CleanAfter = false
			case "--printconfig":
				fmt.Printf("%#v", config)
				os.Exit(0)
			case "--gendb":
				err = createDevelDB()
				if err != nil {
					fmt.Println(err)
				}
				err = saveVCSInfo()
				if err != nil {
					fmt.Println(err)
				}
				os.Exit(0)
			case "--devel":
				config.Devel = true
			case "--nodevel":
				config.Devel = false
			case "--timeupdate":
				config.TimeUpdate = true
			case "--notimeupdate":
				config.TimeUpdate = false
			case "--topdown":
				config.SortMode = TopDown
			case "--bottomup":
				config.SortMode = BottomUp
			case "--complete":
				config.Shell = "sh"
				_ = complete()
				os.Exit(0)
			case "--fcomplete":
				config.Shell = fishShell
				_ = complete()
				os.Exit(0)
			case "--help":
				usage()
				os.Exit(0)
			case "--version":
				fmt.Printf("yay v%s\n", version)
				os.Exit(0)
			case "--noconfirm":
				config.NoConfirm = true
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
	op, options, pkgs, changedConfig, err := parser()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	switch op {
	case "-Cd":
		err = cleanDependencies(pkgs)
	case "-G":
		for _, pkg := range pkgs {
			err = getPkgbuild(pkg)
			if err != nil {
				fmt.Println(pkg+":", err)
			}
		}
	case "-Qstats":
		err = localStatistics()
	case "-Ss", "-Ssq", "-Sqs":
		if op == "-Ss" {
			config.SearchMode = Detailed
		} else {
			config.SearchMode = Minimal
		}

		if pkgs != nil {
			err = syncSearch(pkgs)
		}
	case "-S":
		err = install(pkgs, options)
	case "-Sy":
		err = passToPacman("-Sy", nil, nil)
		if err != nil {
			break
		}
		err = install(pkgs, options)
	case "-Syu", "-Suy", "-Su":
		if strings.Contains(op, "y") {
			err = passToPacman("-Sy", nil, nil)
			if err != nil {
				break
			}
		}
		err = upgradePkgs(options)
	case "-Si":
		err = syncInfo(pkgs, options)
	case "yogurt":
		config.SearchMode = NumberMenu

		if pkgs != nil {
			err = numberMenu(pkgs, options)
		}
	default:
		if op[0] == 'R' {
			removeVCSPackage(pkgs)
		}
		err = passToPacman(op, pkgs, options)
	}

	var erra error
	if updated {
		erra = saveVCSInfo()
		if erra != nil {
			fmt.Println(err)
		}
	}

	if changedConfig {
		erra = config.saveConfig()
		if erra != nil {
			fmt.Println(err)
		}

	}

	erra = alpmHandle.Release()
	if erra != nil {
		fmt.Println(err)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// BuildIntRange build the range from start to end
func BuildIntRange(rangeStart, rangeEnd int) []int {
	if rangeEnd-rangeStart == 0 {
		// rangeEnd == rangeStart, which means no range
		return []int{rangeStart}
	}
	if rangeEnd < rangeStart {
		swap := rangeEnd
		rangeEnd = rangeStart
		rangeStart = swap
	}

	final := make([]int, 0)
	for i := rangeStart; i <= rangeEnd; i++ {
		final = append(final, i)
	}
	return final
}

// BuildRange construct a range of ints from the format 1-10
func BuildRange(input string) ([]int, error) {
	multipleNums := strings.Split(input, "-")
	if len(multipleNums) != 2 {
		return nil, errors.New("Invalid range")
	}

	rangeStart, err := strconv.Atoi(multipleNums[0])
	if err != nil {
		return nil, err
	}
	rangeEnd, err := strconv.Atoi(multipleNums[1])
	if err != nil {
		return nil, err
	}

	return BuildIntRange(rangeStart, rangeEnd), err
}

// NumberMenu presents a CLI for selecting packages to install.
func numberMenu(pkgS []string, flags []string) (err error) {
	var num int

	aq, err := narrowSearch(pkgS, true)
	if err != nil {
		fmt.Println("Error during AUR search:", err)
	}
	numaq := len(aq)
	pq, numpq, err := queryRepo(pkgS)
	if err != nil {
		return
	}

	if numpq == 0 && numaq == 0 {
		return fmt.Errorf("no packages match search")
	}

	if config.SortMode == BottomUp {
		aq.printSearch(numpq + 1)
		pq.printSearch()
	} else {
		pq.printSearch()
		aq.printSearch(numpq + 1)
	}

	fmt.Printf("\x1b[32m%s %s\x1b[0m\nNumbers: ",
		"Type the numbers or ranges (e.g. 1-10) you want to install.",
		"Separate each one of them with a space.")
	reader := bufio.NewReader(os.Stdin)
	numberBuf, overflow, err := reader.ReadLine()
	if err != nil || overflow {
		fmt.Println(err)
		return
	}

	numberString := string(numberBuf)
	var aurI []string
	var repoI []string
	result := strings.Fields(numberString)
	for _, numS := range result {
		var numbers []int
		num, err = strconv.Atoi(numS)
		if err != nil {
			numbers, err = BuildRange(numS)
			if err != nil {
				continue
			}
		} else {
			numbers = []int{num}
		}

		// Install package
		for _, x := range numbers {
			if x > numaq+numpq || x <= 0 {
				continue
			} else if x > numpq {
				if config.SortMode == BottomUp {
					aurI = append(aurI, aq[numaq+numpq-x].Name)
				} else {
					aurI = append(aurI, aq[x-numpq-1].Name)
				}
			} else {
				if config.SortMode == BottomUp {
					repoI = append(repoI, pq[numpq-x].Name())
				} else {
					repoI = append(repoI, pq[x-1].Name())
				}
			}
		}
	}

	if len(repoI) != 0 {
		err = passToPacman("-S", repoI, flags)
	}

	if len(aurI) != 0 {
		err = aurInstall(aurI, flags)
	}

	return err
}

// Complete provides completion info for shells
func complete() error {
	path := completionFile + config.Shell + ".cache"
	info, err := os.Stat(path)
	if os.IsNotExist(err) || time.Since(info.ModTime()).Hours() > 48 {
		os.MkdirAll(filepath.Dir(completionFile), 0755)
		out, errf := os.Create(path)
		if errf != nil {
			return errf
		}

		if createAURList(out) != nil {
			defer os.Remove(path)
		}
		erra := createRepoList(out)

		out.Close()
		return erra
	}

	in, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = io.Copy(os.Stdout, in)
	return err
}
