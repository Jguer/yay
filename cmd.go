package main

import (
	"bufio"
	"encoding/json"
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

    New options:
    --topdown            shows repository's packages first and then aur's
    --bottomup           shows aur's packages first and then repository's
    --noconfirm          skip user input on package install
	--devel			     Check -git/-svn/-hg development version
	--nodevel			 Disable development version checking
`)
}

func init() {
	defaultSettings(&config)

	var err error
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		if info, err := os.Stat(dir); err == nil && info.IsDir() == true {
			configfile = os.Getenv("XDG_CONFIG_HOME") + "/yay/config.json"
		} else {
			configfile = os.Getenv("HOME") + "/.config/yay/config.json"
		}
	} else {
		configfile = os.Getenv("HOME") + "/.config/yay/config.json"
	}

	if _, err = os.Stat(configfile); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Dir(configfile), 0755)
		if err != nil {
			fmt.Println("Unable to create config directory:", filepath.Dir(configfile), err)
			os.Exit(2)
		}
		// Save the default config if nothing is found
		config.saveConfig()
	} else {
		file, err := os.Open(configfile)
		if err != nil {
			fmt.Println("Error reading config:", err)
		} else {
			decoder := json.NewDecoder(file)
			err = decoder.Decode(&config)
			if err != nil {
				fmt.Println("Loading default Settings\nError reading config:", err)
				defaultSettings(&config)
			}
		}
	}

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

	updated = false
	configfile = os.Getenv("HOME") + "/.config/yay/yay_vcs.json"

	if _, err := os.Stat(configfile); os.IsNotExist(err) {
		_ = os.MkdirAll(os.Getenv("HOME")+"/.config/yay", 0755)
		return
	}

	file, err := os.Open(configfile)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&savedInfo)
	if err != nil {
		fmt.Println("error:", err)
	}
}

func parser() (op string, options []string, packages []string, changedConfig bool, err error) {
	if len(os.Args) < 2 {
		err = fmt.Errorf("no operation specified")
		return
	}
	changedConfig = false
	op = "yogurt"

	for _, arg := range os.Args[1:] {
		if arg[0] == '-' && arg[1] != '-' {
			switch arg {
			default:
				op = arg
			}
			continue
		}

		if arg[0] == '-' && arg[1] == '-' {
			changedConfig = true
			switch arg {
			case "--printconfig":
				fmt.Printf("%+v", config)
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
			case "--complete":
				config.Shell = "sh"
				_ = complete()
				os.Exit(0)
			case "--fcomplete":
				config.Shell = "fish"
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
		err = localStatistics(version)
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
		aq.printSearch(numpq)
		pq.printSearch()
	} else {
		pq.printSearch()
		aq.printSearch(numpq)
	}

	fmt.Printf("\x1b[32m%s\x1b[0m\nNumbers: ", "Type numbers to install. Separate each number with a space.")
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
		num, err = strconv.Atoi(numS)
		if err != nil {
			continue
		}

		// Install package
		if num > numaq+numpq-1 || num < 0 {
			continue
		} else if num > numpq-1 {
			if config.SortMode == BottomUp {
				aurI = append(aurI, aq[numaq+numpq-num-1].Name)
			} else {
				aurI = append(aurI, aq[num-numpq].Name)
			}
		} else {
			if config.SortMode == BottomUp {
				repoI = append(repoI, pq[numpq-num-1].Name())
			} else {
				repoI = append(repoI, pq[num].Name())
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
func complete() (err error) {
	path := os.Getenv("HOME") + "/.cache/yay/aur_" + config.Shell + ".cache"

	if info, err := os.Stat(path); os.IsNotExist(err) || time.Since(info.ModTime()).Hours() > 48 {
		os.MkdirAll(os.Getenv("HOME")+"/.cache/yay/", 0755)

		out, err := os.Create(path)
		if err != nil {
			return err
		}

		if createAURList(out) != nil {
			defer os.Remove(path)
		}
		err = createRepoList(out)

		out.Close()
		return err
	}

	in, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = io.Copy(os.Stdout, in)
	return err
}
