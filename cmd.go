package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var cmdArgs = makeArguments()

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
    yay {-P --print}       [options]
    yay {-G --getpkgbuild} [package(s)]

Permanent configuration options:
    --save               Causes the following options to be saved back to the
                         config file when used

    --builddir <dir>     Directory to use for building AUR Packages
    --editor   <file>    Editor to use when editing PKGBUILDs
    --makepkg  <file>    makepkg command to use
    --pacman   <file>    pacman command to use
    --tar      <file>    bsdtar command to use
    --git      <file>    git command to use
    --gpg      <file>    gpg command to use
    --config   <file>    pacman.conf file to use

    --requestsplitn <n>  Max amount of packages to query per AUR request

    --topdown            Shows repository's packages first and then AUR's
    --bottomup           Shows AUR's packages first and then repository's
    --devel              Check development packages during sysupgrade
    --nodevel            Do not check development packages
    --afterclean         Remove package sources after successful install
    --noafterclean       Do not remove package sources after successful build
    --timeupdate         Check package's AUR page for changes during sysupgrade
    --notimeupdate       Do not checking of AUR page changes
    --redownload         Always download pkgbuilds of targets
    --redownloadall      Always download pkgbuilds of all AUR packages
    --noredownload       Skip pkgbuild download if in cache and up to date
    --rebuild            Always build target packages
    --rebuildall         Always build all AUR packages
    --rebuildtree        Always build all AUR packages even if installed
    --norebuild          Skip package build if in cache and up to date
    --mflags <flags>     Pass arguments to makepkg
    --gpgflags <flags>   Pass arguments to gpg
    --sudoloop           Loop sudo calls in the background to avoid timeout
    --nosudoloop         Do not loop sudo calls in the background

Print specific options:
    -c --complete        Used for completions
    -d --defaultconfig   Print default yay configuration
    -g --config          Print current yay configuration
    -n --numberupgrades  Print number of updates
    -s --stats           Display system package statistics
    -u --upgrades        Print update list

Yay specific options:
    -c --clean           Remove unneeded dependencies
       --gendb           Generates development package DB used for updating

If no operation is provided -Y will be assumed`)
}

func initPaths() {
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

	configFile = configHome + "/" + configFileName
	vcsFile = cacheHome + "/" + vcsFileName
	completionFile = cacheHome + "/" + completionFilePrefix
}

func initConfig() (err error) {
	defaultSettings(&config)

	if _, err = os.Stat(configFile); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Dir(configFile), 0755)
		if err != nil {
			err = fmt.Errorf("Unable to create config directory:\n%s\n"+
				"The error was:\n%s", filepath.Dir(configFile), err)
			return
		}
		// Save the default config if nothing is found
		config.saveConfig()
	} else {
		cfile, errf := os.OpenFile(configFile, os.O_RDWR|os.O_CREATE, 0644)
		if errf != nil {
			fmt.Printf("Error reading config: %s\n", err)
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

	return
}

func initVCS() (err error) {
	if _, err = os.Stat(vcsFile); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Dir(vcsFile), 0755)
		if err != nil {
			err = fmt.Errorf("Unable to create vcs directory:\n%s\n"+
				"The error was:\n%s", filepath.Dir(configFile), err)
			return
		}
	} else {
		vfile, err := os.OpenFile(vcsFile, os.O_RDONLY|os.O_CREATE, 0644)
		if err == nil {
			defer vfile.Close()
			decoder := json.NewDecoder(vfile)
			_ = decoder.Decode(&savedInfo)
		}
	}

	return
}

func initAlpm() (err error) {
	var value string
	var exists bool
	//var double bool

	value, _, exists = cmdArgs.getArg("config")
	if exists {
		config.PacmanConf = value
	}

	alpmConf, err = readAlpmConfig(config.PacmanConf)
	if err != nil {
		err = fmt.Errorf("Unable to read Pacman conf: %s", err)
		return
	}

	value, _, exists = cmdArgs.getArg("dbpath", "b")
	if exists {
		alpmConf.DBPath = value
	}

	value, _, exists = cmdArgs.getArg("root", "r")
	if exists {
		alpmConf.RootDir = value
	}

	value, _, exists = cmdArgs.getArg("arch")
	if exists {
		alpmConf.Architecture = value
	}

	value, _, exists = cmdArgs.getArg("ignore")
	if exists {
		alpmConf.IgnorePkg = append(alpmConf.IgnorePkg, strings.Split(value, ",")...)
	}

	value, _, exists = cmdArgs.getArg("ignoregroup")
	if exists {
		alpmConf.IgnoreGroup = append(alpmConf.IgnoreGroup, strings.Split(value, ",")...)
	}

	//TODO
	//current system does not allow duplicate arguments
	//but pacman allows multiple cachdirs to be passed
	//for now only handle one cache dir
	value, _, exists = cmdArgs.getArg("cachdir")
	if exists {
		alpmConf.CacheDir = []string{value}
	}

	value, _, exists = cmdArgs.getArg("gpgdir")
	if exists {
		alpmConf.GPGDir = value
	}

	alpmHandle, err = alpmConf.CreateHandle()
	if err != nil {
		err = fmt.Errorf("Unable to CreateHandle: %s", err)
		return
	}

	alpmHandle.SetQuestionCallback(QuestionCallback)

	return
}

func main() {
	var status int
	var err error

	if 0 == os.Geteuid() {
		fmt.Println("Please avoid running yay as root/sudo.")
	}

	err = cmdArgs.parseCommandLine()
	if err != nil {
		fmt.Println(err)
		status = 1
		goto cleanup
	}

	initPaths()

	err = initConfig()
	if err != nil {
		fmt.Println(err)
		status = 1
		goto cleanup
	}

	err = initVCS()
	if err != nil {
		fmt.Println(err)
		status = 1
		goto cleanup

	}

	err = initAlpm()
	if err != nil {
		fmt.Println(err)
		status = 1
		goto cleanup
	}

	err = handleCmd()
	if err != nil {
		fmt.Println(err)
		status = 1
		goto cleanup
	}

cleanup:
	//cleanup
	//from here on out dont exit if an error occurs
	//if we fail to save the configuration
	//at least continue on and try clean up other parts

	if alpmHandle != nil {
		err = alpmHandle.Release()
		if err != nil {
			fmt.Println(err)
			status = 1
		}
	}

	os.Exit(status)
}

func sudoLoopBackground() {
	updateSudo()
	go sudoLoop()
}

func sudoLoop() {
	for {
		updateSudo()
		time.Sleep(298 * time.Second)
	}
}

func updateSudo() {
	for {
		cmd := exec.Command("sudo", "-v")
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		err := cmd.Run()
		if err != nil {
			fmt.Println(err)
		} else {
			break
		}
	}
}

func handleCmd() (err error) {
	for option, value := range cmdArgs.options {
		if handleConfig(option, value) {
			cmdArgs.delArg(option)
		}
	}

	for option, value := range cmdArgs.globals {
		if handleConfig(option, value) {
			cmdArgs.delArg(option)
		}
	}

	if shouldSaveConfig {
		config.saveConfig()
	}

	if config.SudoLoop && cmdArgs.needRoot() {
		sudoLoopBackground()
	}

	switch cmdArgs.op {
	case "V", "version":
		handleVersion()
	case "D", "database":
		passToPacman(cmdArgs)
	case "F", "files":
		passToPacman(cmdArgs)
	case "Q", "query":
		passToPacman(cmdArgs)
	case "R", "remove":
		handleRemove()
	case "S", "sync":
		err = handleSync()
	case "T", "deptest":
		passToPacman(cmdArgs)
	case "U", "upgrade":
		passToPacman(cmdArgs)
	case "G", "getpkgbuild":
		err = handleGetpkgbuild()
	case "P", "print":
		err = handlePrint()
	case "Y", "--yay":
		err = handleYay()
	default:
		//this means we allowed an op but not implement it
		//if this happens it an error in the code and not the usage
		err = fmt.Errorf("unhandled operation")
	}

	return
}

//this function should only set config options
//but currently still uses the switch left over from old code
//eventually this should be refactored out futher
//my current plan is to have yay specific operations in its own operator
//e.g. yay -Y --gendb
//e.g yay -Yg
func handleConfig(option, value string) bool {
	switch option {
	case "save":
		shouldSaveConfig = true
	case "afterclean":
		config.CleanAfter = true
	case "noafterclean":
		config.CleanAfter = false
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
	case "bottomup":
		config.SortMode = BottomUp
	case "noconfirm":
		config.NoConfirm = true
	case "redownload":
		config.ReDownload = "yes"
	case "redownloadall":
		config.ReDownload = "all"
	case "noredownload":
		config.ReDownload = "no"
	case "rebuild":
		config.ReBuild = "yes"
	case "rebuildall":
		config.ReBuild = "all"
	case "rebuildtree":
		config.ReBuild = "tree"
	case "norebuild":
		config.ReBuild = "no"
	case "gpgflags":
		config.GpgFlags = value
	case "mflags":
		config.MFlags = value
	case "builddir":
		config.BuildDir = value
	case "editor":
		config.Editor = value
	case "makepkg":
		config.MakepkgBin = value
	case "pacman":
		config.PacmanBin = value
	case "tar":
		config.TarBin = value
	case "git":
		config.GitBin = value
	case "gpg":
		config.GpgBin = value
	case "requestsplitn":
		n, err := strconv.Atoi(value)
		if err == nil && n > 0 {
			config.RequestSplitN = n
		}
	case "sudoloop":
		config.SudoLoop = true
	case "nosudoloop":
		config.SudoLoop = false
	default:
		return false
	}

	return true
}

func handleVersion() {
	fmt.Printf("yay v%s\n", version)
}

func handlePrint() (err error) {
	switch {
	case cmdArgs.existsArg("d", "defaultconfig"):
		var tmpConfig Configuration
		defaultSettings(&tmpConfig)
		fmt.Printf("%v", tmpConfig)
	case cmdArgs.existsArg("g", "config"):
		fmt.Printf("%v", config)
	case cmdArgs.existsArg("n", "numberupgrades"):
		err = printNumberOfUpdates()
	case cmdArgs.existsArg("u", "upgrades"):
		err = printUpdateList()
	case cmdArgs.existsArg("c", "complete"):
		switch {
		case cmdArgs.existsArg("f", "fish"):
			complete("fish")
		default:
			complete("sh")
		}
	case cmdArgs.existsArg("s", "stats"):
		err = localStatistics()
	default:
		err = nil
	}

	return err
}

func handleYay() (err error) {
	//_, options, targets := cmdArgs.formatArgs()
	if cmdArgs.existsArg("h", "help") {
		usage()
	} else if cmdArgs.existsArg("gendb") {
		err = createDevelDB()
		if err != nil {
			return
		}
	} else if cmdArgs.existsArg("c", "clean") {
		err = cleanDependencies()
	} else if len(cmdArgs.targets) > 0 {
		err = handleYogurt()
	}

	return
}

func handleGetpkgbuild() (err error) {
	err = getPkgbuilds(cmdArgs.formatTargets())
	return
}

func handleYogurt() (err error) {
	options := cmdArgs.formatArgs()
	targets := cmdArgs.formatTargets()

	config.SearchMode = NumberMenu
	err = numberMenu(targets, options)

	return
}

func handleSync() (err error) {
	targets := cmdArgs.formatTargets()

	if cmdArgs.existsArg("y", "refresh") {
		arguments := cmdArgs.copy()
		cmdArgs.delArg("y", "refresh")
		arguments.delArg("u", "sysupgrade")
		arguments.delArg("s", "search")
		arguments.delArg("i", "info")
		arguments.delArg("l", "list")
		arguments.targets = make(stringSet)
		err = passToPacman(arguments)
		if err != nil {
			return
		}
	}

	if cmdArgs.existsArg("s", "search") {
		if cmdArgs.existsArg("q", "quiet") {
			config.SearchMode = Minimal
		} else {
			config.SearchMode = Detailed
		}

		err = syncSearch(targets)
	} else if cmdArgs.existsArg("l", "list") {
		err = passToPacman(cmdArgs)
	} else if cmdArgs.existsArg("c", "clean") {
		err = passToPacman(cmdArgs)
	} else if cmdArgs.existsArg("i", "info") {
		err = syncInfo(targets)
	} else if cmdArgs.existsArg("u", "sysupgrade") {
		err = install(cmdArgs)
	} else if len(cmdArgs.targets) > 0 {
		err = install(cmdArgs)
	}

	return
}

func handleRemove() (err error) {
	removeVCSPackage(cmdArgs.formatTargets())
	err = passToPacman(cmdArgs)
	return
}

// NumberMenu presents a CLI for selecting packages to install.
func numberMenu(pkgS []string, flags []string) (err error) {
	aurQ, err := narrowSearch(pkgS, true)
	if err != nil {
		fmt.Println("Error during AUR search:", err)
	}
	numaq := len(aurQ)
	repoQ, numpq, err := queryRepo(pkgS)
	if err != nil {
		return
	}

	if numpq == 0 && numaq == 0 {
		return fmt.Errorf("no packages match search")
	}

	if config.SortMode == BottomUp {
		aurQ.printSearch(numpq + 1)
		repoQ.printSearch()
	} else {
		repoQ.printSearch()
		aurQ.printSearch(numpq + 1)
	}

	fmt.Println(bold(green(arrow + " Packages to install (eg: 1 2 3, 1-3 or ^4)")))
	fmt.Print(bold(green(arrow + " ")))

	reader := bufio.NewReader(os.Stdin)
	numberBuf, overflow, err := reader.ReadLine()

	if err != nil {
		return err
	}

	if overflow {
		return fmt.Errorf("Input too long")
	}

	include, exclude, _, otherExclude := parseNumberMenu(string(numberBuf))
	arguments := makeArguments()

	isInclude := len(exclude) == 0 && len(otherExclude) == 0

	for i, pkg := range repoQ {
		target := len(repoQ) - i
		if config.SortMode == TopDown {
			target = i + 1
		}

		if isInclude && include.get(target) {
			arguments.addTarget(pkg.DB().Name() + "/" + pkg.Name())
		}
		if !isInclude && !exclude.get(target) {
			arguments.addTarget(pkg.DB().Name() + "/" + pkg.Name())
		}
	}

	for i, pkg := range aurQ {
		target := len(aurQ) - i + len(repoQ)
		if config.SortMode == TopDown {
			target = i + 1 + len(repoQ)
		}

		if isInclude && include.get(target) {
			arguments.addTarget("aur/" + pkg.Name)
		}
		if !isInclude && !exclude.get(target) {
			arguments.addTarget("aur/" + pkg.Name)
		}
	}

	if config.SudoLoop {
		sudoLoopBackground()
	}

	err = install(arguments)

	return err
}

// passToPacman outsources execution to pacman binary without modifications.
func passToPacman(args *arguments) error {
	var cmd *exec.Cmd
	argArr := make([]string, 0)

	if args.needRoot() {
		argArr = append(argArr, "sudo")
	}

	argArr = append(argArr, config.PacmanBin)
	argArr = append(argArr, cmdArgs.formatGlobals()...)
	argArr = append(argArr, args.formatArgs()...)
	if config.NoConfirm {
		argArr = append(argArr, "--noconfirm")
	}

	argArr = append(argArr, "--")

	argArr = append(argArr, args.formatTargets()...)

	cmd = exec.Command(argArr[0], argArr[1:]...)

	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	return err
}

//passToPacman but return the output instead of showing the user
func passToPacmanCapture(args *arguments) (string, string, error) {
	var outbuf, errbuf bytes.Buffer
	var cmd *exec.Cmd
	argArr := make([]string, 0)

	if args.needRoot() {
		argArr = append(argArr, "sudo")
	}

	argArr = append(argArr, config.PacmanBin)
	argArr = append(argArr, cmdArgs.formatGlobals()...)
	argArr = append(argArr, args.formatArgs()...)
	if config.NoConfirm {
		argArr = append(argArr, "--noconfirm")
	}

	argArr = append(argArr, "--")

	argArr = append(argArr, args.formatTargets()...)

	cmd = exec.Command(argArr[0], argArr[1:]...)
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	err := cmd.Run()
	stdout := outbuf.String()
	stderr := errbuf.String()

	return stdout, stderr, err
}

// passToMakepkg outsources execution to makepkg binary without modifications.
func passToMakepkg(dir string, args ...string) (err error) {

	if config.NoConfirm {
		args = append(args)
	}

	mflags := strings.Fields(config.MFlags)
	args = append(args, mflags...)

	cmd := exec.Command(config.MakepkgBin, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Dir = dir
	err = cmd.Run()
	if err == nil {
		_ = saveVCSInfo()
	}
	return
}
