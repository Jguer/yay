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
	fmt.Println(`Usage:
    yay <operation> [...]
    yay <package(s)>
    
operations:
    yay {-h --help}
    yay {-V --version}
    yay {-D --database}    <options> <package(s)>
    yay {-F --files}       [options] [package(s)]
    yay {-Q --query}       [options] [package(s)]
    yay {-R --remove}      [options] <package(s)>
    yay {-S --sync}        [options] [package(s)]
    yay {-T --deptest}     [options] [package(s)]
    yay {-U --upgrade}     [options] <file(s)>

New operations:
    yay {-Y --yay}         [options] [package(s)]
    yay {-G --getpkgbuild} [package(s)]

Permanent configuration options:
    --topdown            Shows repository's packages first and then aur's
    --bottomup           Shows aur's packages first and then repository's
    --devel              Check -git/-svn/-hg development version
    --nodevel            Disable development version checking
    --afterclean         Clean package sources after successful build
    --noafterclean       Disable package sources cleaning after successful build
    --timeupdate         Check package's modification date and version
    --notimeupdate       Check only package version change

Yay specific options:
    --printconfig        Prints current yay configuration
    --stats              Displays system information
    --cleandeps          Remove unneeded dependencies
    --gendb              Generates development package DB used for updating.

If no operation is provided -Y will be assumed
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
			fmt.Println("Unable to create config directory:", filepath.Dir(configFile), err)
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
				fmt.Println("Loading default Settings.\nError reading config:", err)
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

func main() {
	status := run()
	
	err := alpmHandle.Release()
	if err != nil {
		fmt.Println(err)
		status = 1
	}
	
	os.Exit(status)
}

func run() (status int) {
	var err error
	var changedConfig bool
	
	parser := makeArgParser();
	err = parser.parseCommandLine();
	
	if err != nil {
		fmt.Println(err)
		status = 1
		return
	}
	
	if parser.existsArg("-") {
		err = parser.parseStdin();

		if err != nil {
			fmt.Println(err)
			status = 1
			return
		}
	}
	
	changedConfig, err = handleCmd(parser)
	
	if err != nil {
		fmt.Println(err)
		status = 1
		//try continue onward
	}
	
	if updated {
		err = saveVCSInfo()
		
		if err != nil {
			fmt.Println(err)
			status = 1
		}
	}

	if changedConfig {
		err = config.saveConfig()
		
		if err != nil {
			fmt.Println(err)
			status = 1
		}

	}
	
	return
	
}


func handleCmd(parser *argParser) (changedConfig bool, err error) {
	var _changedConfig bool
	
	for option, _ := range parser.options {
		_changedConfig, err = handleConfig(option)
		
		if err != nil {
			return
		}
		
		if _changedConfig {
			changedConfig = true
		}
	}

	switch parser.op {
	case "V", "version":
		handleVersion(parser)
	case "D", "database":
		passToPacman(parser)
	case "F", "files":
		passToPacman(parser)
	case "Q", "query":
		passToPacman(parser)
	case "R", "remove":
		passToPacman(parser)
	case "S", "sync":
		err = handleSync(parser)
	case "T", "deptest":
		passToPacman(parser)
	case "U", "upgrade":
		passToPacman(parser)
	case "Y", "--yay":
		err = handleYay(parser)
	default:
		//this means we allowed an op but not implement it
		//if this happens it an error in the code and not the usage
		err = fmt.Errorf("unhandled operation")
		}
	
	return
}

//this function should only set config options
//but currently still uses the switch left over from old code
//eventuall this should be refactored out futher
//my current plan is to have yay specific operations in its own operator
//e.g. yay -Y --gendb
//e.g yay -Yg
func handleConfig(option string) (changedConfig bool, err error) {
	switch option {
		case "afterclean":
			config.CleanAfter = true
		case "noafterclean":
			config.CleanAfter = false
//		case "printconfig":
//			fmt.Printf("%#v", config)
//			os.Exit(0)
//		case "gendb":
//			err = createDevelDB()
//			if err != nil {
//				fmt.Println(err)
//			}
//			err = saveVCSInfo()
//			if err != nil {
//				fmt.Println(err)
//			}
//			os.Exit(0)
		case "devel":
			config.Devel = true
		case "nodevel":
			config.Devel = false
		case "timeupdate":
			config.TimeUpdate = true
		case "notimeupdate":
			config.TimeUpdate = false
		case "topdown":
			config.SortMode = TopDown
//		case "complete":
//			config.Shell = "sh"
//			complete()
//			os.Exit(0)
//		case "fcomplete":
//			config.Shell = fishShell
//			complete()
//			os.Exit(0)
//		case "help":
//			usage()
//			os.Exit(0)
//		case "version":
//			fmt.Printf("yay v%s\n", version)
//			os.Exit(0)
		case "noconfirm":
			config.NoConfirm = true
		default:
			return
		}
	
	changedConfig = true
	return
}

func handleVersion(parser *argParser) {
	fmt.Printf("yay v%s\n", version)
}

func handleYay(parser *argParser) (err error) {
	//_, options, targets := parser.formatArgs()
	if parser.existsArg("h") || parser.existsArg("help") {
		usage()
	} else if parser.existsArg("printconfig") {
		fmt.Printf("%#v", config)
	} else if parser.existsArg("gendb") {
		err = createDevelDB()
		if err != nil {
			return
		}
		err = saveVCSInfo()
		if err != nil {
			return
		}
	} else if parser.existsArg("complete") {
		config.Shell = "sh"
		complete()
	} else if parser.existsArg("fcomplete") {
		config.Shell = "fish"
		complete()
	} else if parser.existsArg("stats") {
		err = localStatistics()
	} else if parser.existsArg("cleandeps") {
		//TODO
		//_,_,targets := parser.formatArgs()
		//err = cleanDependencies(targets)
	} else {
		err = handleYogurt(parser)
	}
	
	return
}

func handleYogurt(parser *argParser) (err error) {
//	TODO
//	_, options, targets := parser.formatArgs()
//	
//	config.SearchMode = NumberMenu
//	err = numberMenu(targets, options)
//	
	return
}

func handleSync(parser *argParser) (err error) {
//TODO
//	if parser.existsArg("y") || parser.existsArg("refresh") {
//		err = passToPacman(parser)
//		if err != nil {
//			return
//		}
//	}
//	
//	if parser.existsArg("s") {
//		if parser.existsArg("i") {
//			config.SearchMode = Detailed
//		} else {
//			config.SortMode = Minimal
//		}
//		
//		err = syncSearch(targets)
//	}
//	
//	if len(targets) > 0 {
//		err = install(targets, options)
//	}
	
	return
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
		//TODO
		//err = passToPacman("-S", repoI, flags)
	}

	if len(aurI) != 0 {
		err = aurInstall(aurI, flags)
	}

	return err
}

// Complete provides completion info for shells
func complete() error {
	path := completionFile + config.Shell + ".cache"

	if info, err := os.Stat(path); os.IsNotExist(err) || time.Since(info.ModTime()).Hours() > 48 {
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
