package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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
    --answeredit    <a>   Set a predetermined answer for the edit pkgbuild menu
    --answerupgrade <a>   Set a predetermined answer for the upgrade menu
    --noanswerclean       Unset the answer for the clean build menu
    --noansweredit        Unset the answer for the edit pkgbuild menu
    --noanswerupgrade     Unset the answer for the upgrade menu

    --afterclean          Remove package sources after successful install
    --noafterclean        Do not remove package sources after successful build
    --bottomup            Shows AUR's packages first and then repository's
    --topdown             Shows repository's packages first and then AUR's

    --devel               Check development packages during sysupgrade
    --nodevel             Do not check development packages
    --gitclone            Use git clone for PKGBUILD retrieval
    --nogitclone          Never use git clone for PKGBUILD retrieval
    --showdiffs           Show diffs for build files
    --noshowdiffs         Always show the entire PKGBUILD
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

    --sudoloop            Loop sudo calls in the background to avoid timeout
    --nosudoloop          Do not loop sudo calls in the background

    --timeupdate          Check packages' AUR page for changes during sysupgrade
    --notimeupdate        Do not check packages' AUR page for changes

Print specific options:
    -c --complete         Used for completions
    -d --defaultconfig    Print default yay configuration
    -g --config           Print current yay configuration
    -n --numberupgrades   Print number of updates
    -s --stats            Display system package statistics
    -u --upgrades         Print update list
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
		err = passToPacman(cmdArgs)
	case "F", "files":
		err = passToPacman(cmdArgs)
	case "Q", "query":
		err = handleQuery()
	case "R", "remove":
		err = handleRemove()
	case "S", "sync":
		err = handleSync()
	case "T", "deptest":
		err = passToPacman(cmdArgs)
	case "U", "upgrade":
		err = passToPacman(cmdArgs)
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
		err = passToPacman(cmdArgs)
	}

	return err
}

func handleHelp() error {
	if cmdArgs.op == "Y" || cmdArgs.op == "yay" {
		usage()
		return nil
	}

	return passToPacman(cmdArgs)
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
	case "showdiffs":
		config.ShowDiffs = true
	case "noshowdiffs":
		config.ShowDiffs = false
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
	} else if cmdArgs.existsArg("c", "clean") {
		err = syncClean(cmdArgs)
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
	removeVCSPackage(cmdArgs.targets)
	err = passToPacman(cmdArgs)
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

	argArr = append(argArr, args.targets...)

	cmd = exec.Command(argArr[0], argArr[1:]...)

	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()

	if err != nil {
		return fmt.Errorf("")
	}
	return nil
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

	argArr = append(argArr, args.targets...)

	cmd = exec.Command(argArr[0], argArr[1:]...)
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	err := cmd.Run()
	stdout := outbuf.String()
	stderr := errbuf.String()

	return stdout, stderr, err
}

// copies the contents of one directory to another
func copyDirContentsRecursive(srcDir string, destDir string, destOwnerUid int, destOwnerGid int) error {
	err := filepath.Walk(srcDir, func(myPath string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", srcDir, err)
			return err
		}

		if info.IsDir() {
			dirPath := strings.Replace(myPath, srcDir+"/", "", 1)
			destDirPath := path.Join(destDir, dirPath)
			os.Mkdir(destDirPath, 0755)
			os.Chown(destDirPath, destOwnerUid, destOwnerGid)
		} else {
			filePath := strings.Replace(myPath, srcDir+"/", "", 1)
			destFilePath := path.Join(destDir, filePath)

			srcFile, err := os.Open(myPath)
			if err != nil {
				return fmt.Errorf("Error when opening file %v: %v", myPath, err)
			}
			defer srcFile.Close()

			destFile, err := os.Create(destFilePath) // creates if file doesn't exist
			if err != nil {
				return fmt.Errorf("Error when creating file: %v", myPath, err)
			}
			defer destFile.Close()

			_, err = io.Copy(destFile, srcFile) // check first var for number of bytes copied
			if err != nil {
				return fmt.Errorf("Error when copying file data from %v to %v: %v", myPath, destFilePath, err)
			}

			err = destFile.Sync()
			if err != nil {
				return fmt.Errorf("Error when syncing file data to %v: %v", destFilePath, err)
			}

			fmt.Printf("visited file: %q\n", strings.Replace(myPath, srcDir+"/", "", 1))

			os.Chown(destFilePath, destOwnerUid, destOwnerGid)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("Error when copying files from %v: %v", srcDir, err)
	}

	return nil
}

// passToMakepkg outsources execution to makepkg binary without modifications.
func passToMakepkg(dir string, args ...string) (err error) {
	var tempDir string

	if config.NoConfirm {
		args = append(args)
	}

	mflags := strings.Fields(config.MFlags)
	args = append(args, mflags...)

	cmd := exec.Command(config.MakepkgBin, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	// Run makepkg as nobody if running yay as root
	if 0 == os.Geteuid() {
		nobodyUser, err := user.Lookup("nobody")
		if err != nil {
			return fmt.Errorf("Unable to find the UID / GID of user \"nobody\" to execute makepkg: %v", err)
		}

		nobodyUid, err := strconv.Atoi(nobodyUser.Uid)
		if err != nil {
			return fmt.Errorf("Unable to convert the UID of user \"nobody\" to a string: %v", err)
		}

		nobodyGid, err := strconv.Atoi(nobodyUser.Gid)
		if err != nil {
			return fmt.Errorf("Unable to find the GID of user \"nobody\" to a string: %v", err)
		}

		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(nobodyUid), Gid: uint32(nobodyGid)}

		// create temporary directory for nobody to build in
		tempDir, err := ioutil.TempDir("", "yay")
		if err != nil {
			return fmt.Errorf("Unable to create temporary directory for \"nobody\" to build package in: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// make the temporary directory be owned by nobody
		err = os.Chown(tempDir, nobodyUid, nobodyGid)
		if err != nil {
			return fmt.Errorf("Unable to chown temporary directory to \"nobody\": %v", err)
		}

		// copy package's files to the tempdir and chown them all to nobody
		fmt.Println(dir)
		fmt.Println(tempDir)
		copyDirContentsRecursive(dir, tempDir, nobodyUid, nobodyGid)

		cmd.Dir = tempDir
	} else {
		cmd.Dir = dir
	}

	err = cmd.Run()

	// move contents of temp dir to roots dir
	if 0 == os.Geteuid() {
		copyDirContentsRecursive(tempDir, dir, 0, 0)
	}

	if err == nil {
		_ = saveVCSInfo()
	}

	_ = tempDir

	return
}

func passToMakepkgCapture(dir string, args ...string) (string, string, error) {
	var outbuf, errbuf bytes.Buffer

	if config.NoConfirm {
		args = append(args)
	}

	mflags := strings.Fields(config.MFlags)
	args = append(args, mflags...)

	cmd := exec.Command(config.MakepkgBin, args...)
	cmd.Dir = dir
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	err := cmd.Run()
	stdout := outbuf.String()
	stderr := errbuf.String()

	if err == nil {
		_ = saveVCSInfo()
	}

	return stdout, stderr, err
}

func passToGit(dir string, _args ...string) (err error) {
	gitflags := strings.Fields(config.GitFlags)
	args := []string{"-C", dir}
	args = append(args, gitflags...)
	args = append(args, _args...)

	cmd := exec.Command(config.GitBin, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err = cmd.Run()
	return
}

func passToGitCapture(dir string, _args ...string) (string, string, error) {
	var outbuf, errbuf bytes.Buffer
	gitflags := strings.Fields(config.GitFlags)
	args := []string{"-C", dir}
	args = append(args, gitflags...)
	args = append(args, _args...)

	cmd := exec.Command(config.GitBin, args...)
	cmd.Dir = dir
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	err := cmd.Run()
	stdout := outbuf.String()
	stderr := errbuf.String()

	return stdout, stderr, err
}
