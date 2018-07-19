package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var cmdArgs = makeArguments()

func usage() {
	fmt.Println(`Usage:
    yay
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

New options:
       --repo             Assume targets are from the repositories
    -a --aur              Assume targets are from the AUR
Permanent configuration options:
    --save                Causes the following options to be saved back to the
                          config file when used

    --builddir    <dir>   Directory to use for building AUR Packages
    --editor      <file>  Editor to use when editing PKGBUILDs
    --editorflags <flags> Pass arguments to editor
    --makepkg     <file>  makepkg command to use
    --mflags      <flags> Pass arguments to makepkg
    --pacman      <file>  pacman command to use
    --tar         <file>  bsdtar command to use
    --git         <file>  git command to use
    --gitflags    <flags> Pass arguments to git
    --gpg         <file>  gpg command to use
    --gpgflags    <flags> Pass arguments to gpg
    --config      <file>  pacman.conf file to use

    --requestsplitn <n>   Max amount of packages to query per AUR request
    --sortby    <field>   Sort AUR results by a specific field during search
    --answerclean   <a>   Set a predetermined answer for the clean build menu
    --answerdiff    <a>   Set a predetermined answer for the diff menu
    --answeredit    <a>   Set a predetermined answer for the edit pkgbuild menu
    --answerupgrade <a>   Set a predetermined answer for the upgrade menu
    --noanswerclean       Unset the answer for the clean build menu
    --noanswerdiff        Unset the answer for the edit diff menu
    --noansweredit        Unset the answer for the edit pkgbuild menu
    --noanswerupgrade     Unset the answer for the upgrade menu
    --cleanmenu           Give the option to clean build PKGBUILDS
    --diffmenu            Give the option to show diffs for build files
    --editmenu            Give the option to edit/view PKGBUILDS
    --upgrademenu         Show a detailed list of updates with the option to skip any
    --nocleanmenu         Don't clean build PKGBUILDS
    --nodiffmenu          Don't show diffs for build files
    --noeditmenu          Don't edit/view PKGBUILDS
    --noupgrademenu       Don't show the upgrade menu

    --afterclean          Remove package sources after successful install
    --noafterclean        Do not remove package sources after successful build
    --bottomup            Shows AUR's packages first and then repository's
    --topdown             Shows repository's packages first and then AUR's

    --devel               Check development packages during sysupgrade
    --nodevel             Do not check development packages
    --gitclone            Use git clone for PKGBUILD retrieval
    --nogitclone          Never use git clone for PKGBUILD retrieval
    --rebuild             Always build target packages
    --rebuildall          Always build all AUR packages
    --norebuild           Skip package build if in cache and up to date
    --rebuildtree         Always build all AUR packages even if installed
    --redownload          Always download pkgbuilds of targets
    --noredownload        Skip pkgbuild download if in cache and up to date
    --redownloadall       Always download pkgbuilds of all AUR packages
    --provides            Look for matching provders when searching for packages
    --noprovides          Just look for packages by pkgname
    --pgpfetch            Prompt to import PGP keys from PKGBUILDs
    --nopgpfetch          Don't prompt to import PGP keys
    --useask              Automatically resolve conflicts using pacman's ask flag
    --nouseask            Confirm conflicts manually during the install
    --combinedupgrade     Refresh then perform the repo and AUR upgrade together
    --nocombinedupgrade   Perform the repo upgrade and AUR upgrade separately

    --sudoloop            Loop sudo calls in the background to avoid timeout
    --nosudoloop          Do not loop sudo calls in the background

    --timeupdate          Check packages' AUR page for changes during sysupgrade
    --notimeupdate        Do not check packages' AUR page for changes

Print specific options:
    -c --complete         Used for completions
    -d --defaultconfig    Print default yay configuration
    -g --config           Print current yay configuration
    -s --stats            Display system package statistics
    -w --news             Print arch news

Yay specific options:
    -c --clean            Remove unneeded dependencies
       --gendb            Generates development package DB used for updating

If no arguments are provided 'yay -Syu' will be performed
If no operation is provided -Y will be assumed`)
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

	if cmdArgs.existsArg("h", "help") {
		err = handleHelp()
		return
	}

	if config.SudoLoop && cmdArgs.needRoot() {
		sudoLoopBackground()
	}

	switch cmdArgs.op {
	case "V", "version":
		handleVersion()
	case "D", "database":
		err = show(passToPacman(cmdArgs))
	case "F", "files":
		err = show(passToPacman(cmdArgs))
	case "Q", "query":
		err = handleQuery()
	case "R", "remove":
		err = handleRemove()
	case "S", "sync":
		err = handleSync()
	case "T", "deptest":
		err = show(passToPacman(cmdArgs))
	case "U", "upgrade":
		err = show(passToPacman(cmdArgs))
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

func handleQuery() error {
	var err error

	if cmdArgs.existsArg("u", "upgrades") {
		err = printUpdateList(cmdArgs)
	} else {
		err = show(passToPacman(cmdArgs))
	}

	return err
}

func handleHelp() error {
	if cmdArgs.op == "Y" || cmdArgs.op == "yay" {
		usage()
		return nil
	}

	return show(passToPacman(cmdArgs))
}

//this function should only set config options
//but currently still uses the switch left over from old code
//eventually this should be refactored out further
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
	case "sortby":
		config.SortBy = value
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
	case "answerclean":
		config.AnswerClean = value
	case "noanswerclean":
		config.AnswerClean = ""
	case "answerdiff":
		config.AnswerDiff = value
	case "noanswerdiff":
		config.AnswerDiff = ""
	case "answeredit":
		config.AnswerEdit = value
	case "noansweredit":
		config.AnswerEdit = ""
	case "answerupgrade":
		config.AnswerUpgrade = value
	case "noanswerupgrade":
		config.AnswerUpgrade = ""
	case "gitclone":
		config.GitClone = true
	case "nogitclone":
		config.GitClone = false
	case "gpgflags":
		config.GpgFlags = value
	case "mflags":
		config.MFlags = value
	case "gitflags":
		config.GitFlags = value
	case "builddir":
		config.BuildDir = value
	case "editor":
		config.Editor = value
	case "editorflags":
		config.EditorFlags = value
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
	case "provides":
		config.Provides = true
	case "noprovides":
		config.Provides = false
	case "pgpfetch":
		config.PGPFetch = true
	case "nopgpfetch":
		config.PGPFetch = false
	case "upgrademenu":
		config.UpgradeMenu = true
	case "noupgrademenu":
		config.UpgradeMenu = false
	case "cleanmenu":
		config.CleanMenu = true
	case "nocleanmenu":
		config.CleanMenu = false
	case "diffmenu":
		config.DiffMenu = true
	case "nodiffmenu":
		config.DiffMenu = false
	case "editmenu":
		config.EditMenu = true
	case "noeditmenu":
		config.EditMenu = false
	case "useask":
		config.UseAsk = true
	case "nouseask":
		config.UseAsk = false
	case "combinedupgrade":
		config.CombinedUpgrade = true
	case "nocombinedupgrade":
		config.CombinedUpgrade = false
	case "a", "aur":
		mode = ModeAUR
	case "repo":
		mode = ModeRepo
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
		err = printUpdateList(cmdArgs)
	case cmdArgs.existsArg("w", "news"):
		err = printNewsFeed()
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
	if cmdArgs.existsArg("gendb") {
		err = createDevelDB()
	} else if cmdArgs.existsDouble("c") {
		err = cleanDependencies(true)
	} else if cmdArgs.existsArg("c", "clean") {
		err = cleanDependencies(false)
	} else if len(cmdArgs.targets) > 0 {
		err = handleYogurt()
	}

	return
}

func handleGetpkgbuild() (err error) {
	err = getPkgbuilds(cmdArgs.targets)
	return
}

func handleYogurt() (err error) {
	options := cmdArgs.formatArgs()

	config.SearchMode = NumberMenu
	err = numberMenu(cmdArgs.targets, options)

	return
}

func handleSync() (err error) {
	targets := cmdArgs.targets

	if cmdArgs.existsArg("y", "refresh") {
		arguments := cmdArgs.copy()
		cmdArgs.delArg("y", "refresh")
		arguments.delArg("u", "sysupgrade")
		arguments.delArg("s", "search")
		arguments.delArg("i", "info")
		arguments.delArg("l", "list")
		arguments.clearTargets()
		err = show(passToPacman(arguments))
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
	} else if cmdArgs.existsArg("c", "clean") {
		err = syncClean(cmdArgs)
	} else if cmdArgs.existsArg("l", "list") {
		err = show(passToPacman(cmdArgs))
	} else if cmdArgs.existsArg("c", "clean") {
		err = show(passToPacman(cmdArgs))
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
	removeVCSPackage(cmdArgs.targets)
	err = show(passToPacman(cmdArgs))
	return
}

// NumberMenu presents a CLI for selecting packages to install.
func numberMenu(pkgS []string, flags []string) (err error) {
	pkgS = removeInvalidTargets(pkgS)
	var aurErr error
	var repoErr error
	var aq aurQuery
	var pq repoQuery
	var lenaq int
	var lenpq int

	if mode == ModeAUR || mode == ModeAny {
		aq, aurErr = narrowSearch(pkgS, true)
		lenaq = len(aq)
	}
	if mode == ModeRepo || mode == ModeAny {
		pq, lenpq, repoErr = queryRepo(pkgS)
		if repoErr != nil {
			return err
		}
	}

	if lenpq == 0 && lenaq == 0 {
		return fmt.Errorf("No packages match search")
	}

	if config.SortMode == BottomUp {
		if mode == ModeAUR || mode == ModeAny {
			aq.printSearch(lenpq + 1)
		}
		if mode == ModeRepo || mode == ModeAny {
			pq.printSearch()
		}
	} else {
		if mode == ModeRepo || mode == ModeAny {
			pq.printSearch()
		}
		if mode == ModeAUR || mode == ModeAny {
			aq.printSearch(lenpq + 1)
		}
	}

	if aurErr != nil {
		fmt.Printf("Error during AUR search: %s\n", aurErr)
		fmt.Println("Showing repo packages only")
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

	for i, pkg := range pq {
		target := len(pq) - i
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

	for i, pkg := range aq {
		target := len(aq) - i + len(pq)
		if config.SortMode == TopDown {
			target = i + 1 + len(pq)
		}

		if isInclude && include.get(target) {
			arguments.addTarget("aur/" + pkg.Name)
		}
		if !isInclude && !exclude.get(target) {
			arguments.addTarget("aur/" + pkg.Name)
		}
	}

	if len(arguments.targets) == 0 {
		return fmt.Errorf("There is nothing to do")
	}

	if config.SudoLoop {
		sudoLoopBackground()
	}

	err = install(arguments)

	return err
}

func show(cmd *exec.Cmd) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("")
	}
	return nil
}

func capture(cmd *exec.Cmd) (string, string, error) {
	var outbuf, errbuf bytes.Buffer

	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf
	err := cmd.Run()
	stdout := outbuf.String()
	stderr := errbuf.String()

	return stdout, stderr, err
}

// passToPacman outsources execution to pacman binary without modifications.
func passToPacman(args *arguments) *exec.Cmd {
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

	argArr = append(argArr, args.targets...)

	return exec.Command(argArr[0], argArr[1:]...)
}

// passToMakepkg outsources execution to makepkg binary without modifications.
func passToMakepkg(dir string, args ...string) *exec.Cmd {
	if config.NoConfirm {
		args = append(args)
	}

	mflags := strings.Fields(config.MFlags)
	args = append(args, mflags...)

	cmd := exec.Command(config.MakepkgBin, args...)
	cmd.Dir = dir
	return cmd
}

func passToGit(dir string, _args ...string) *exec.Cmd {
	gitflags := strings.Fields(config.GitFlags)
	args := []string{"-C", dir}
	args = append(args, gitflags...)
	args = append(args, _args...)

	cmd := exec.Command(config.GitBin, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd
}
