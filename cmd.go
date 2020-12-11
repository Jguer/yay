package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"

	alpm "github.com/Jguer/go-alpm/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/completion"
	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/intrange"
	"github.com/Jguer/yay/v10/pkg/news"
	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/vcs"
)

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
    yay {-P --show}        [options] [package(s)]
    yay {-G --getpkgbuild} [package(s)]

If no arguments are provided 'yay -Syu' will be performed
If no operation is provided -Y will be assumed

New options:
       --repo             Assume targets are from the repositories
    -a --aur              Assume targets are from the AUR

Permanent configuration options:
    --save                Causes the following options to be saved back to the
                          config file when used

    --aururl      <url>   Set an alternative AUR URL
    --builddir    <dir>   Directory used to download and run PKGBUILDS
    --absdir      <dir>   Directory used to store downloads from the ABS
    --editor      <file>  Editor to use when editing PKGBUILDs
    --editorflags <flags> Pass arguments to editor
    --makepkg     <file>  makepkg command to use
    --mflags      <flags> Pass arguments to makepkg
    --pacman      <file>  pacman command to use
    --git         <file>  git command to use
    --gitflags    <flags> Pass arguments to git
    --gpg         <file>  gpg command to use
    --gpgflags    <flags> Pass arguments to gpg
    --config      <file>  pacman.conf file to use
    --makepkgconf <file>  makepkg.conf file to use
    --nomakepkgconf       Use the default makepkg.conf

    --requestsplitn <n>   Max amount of packages to query per AUR request
    --completioninterval  <n> Time in days to refresh completion cache
    --sortby    <field>   Sort AUR results by a specific field during search
    --searchby  <field>   Search for packages using a specified field
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
    --askremovemake       Ask to remove makedepends after install
    --removemake          Remove makedepends after install
    --noremovemake        Don't remove makedepends after install

    --cleanafter          Remove package sources after successful install
    --nocleanafter        Do not remove package sources after successful build
    --bottomup            Shows AUR's packages first and then repository's
    --topdown             Shows repository's packages first and then AUR's

    --devel               Check development packages during sysupgrade
    --nodevel             Do not check development packages
    --rebuild             Always build target packages
    --rebuildall          Always build all AUR packages
    --norebuild           Skip package build if in cache and up to date
    --rebuildtree         Always build all AUR packages even if installed
    --redownload          Always download pkgbuilds of targets
    --noredownload        Skip pkgbuild download if in cache and up to date
    --redownloadall       Always download pkgbuilds of all AUR packages
    --provides            Look for matching providers when searching for packages
    --noprovides          Just look for packages by pkgname
    --pgpfetch            Prompt to import PGP keys from PKGBUILDs
    --nopgpfetch          Don't prompt to import PGP keys
    --useask              Automatically resolve conflicts using pacman's ask flag
    --nouseask            Confirm conflicts manually during the install
    --combinedupgrade     Refresh then perform the repo and AUR upgrade together
    --nocombinedupgrade   Perform the repo upgrade and AUR upgrade separately
    --batchinstall        Build multiple AUR packages then install them together
    --nobatchinstall      Build and install each AUR package one by one

    --sudo                <file>  sudo command to use
    --sudoflags           <flags> Pass arguments to sudo
    --sudoloop            Loop sudo calls in the background to avoid timeout
    --nosudoloop          Do not loop sudo calls in the background

    --timeupdate          Check packages' AUR page for changes during sysupgrade
    --notimeupdate        Do not check packages' AUR page for changes

show specific options:
    -c --complete         Used for completions
    -d --defaultconfig    Print default yay configuration
    -g --currentconfig    Print current yay configuration
    -s --stats            Display system package statistics
    -w --news             Print arch news
    -p --pkgbuild         Print pkgbuild of packages

yay specific options:
    -c --clean            Remove unneeded dependencies
       --gendb            Generates development package DB used for updating

getpkgbuild specific options:
    -f --force            Force download for existing ABS packages`)
}

func handleCmd(cmdArgs *settings.Arguments, dbExecutor db.Executor) error {
	if cmdArgs.ExistsArg("h", "help") {
		return handleHelp(cmdArgs)
	}

	if config.SudoLoop && cmdArgs.NeedRoot(config.Runtime) {
		sudoLoopBackground()
	}

	switch cmdArgs.Op {
	case "V", "version":
		handleVersion()
		return nil
	case "D", "database":
		return config.Runtime.CmdRunner.Show(passToPacman(cmdArgs))
	case "F", "files":
		return config.Runtime.CmdRunner.Show(passToPacman(cmdArgs))
	case "Q", "query":
		return handleQuery(cmdArgs, dbExecutor)
	case "R", "remove":
		return handleRemove(cmdArgs, config.Runtime.VCSStore)
	case "S", "sync":
		return handleSync(cmdArgs, dbExecutor)
	case "T", "deptest":
		return config.Runtime.CmdRunner.Show(passToPacman(cmdArgs))
	case "U", "upgrade":
		return config.Runtime.CmdRunner.Show(passToPacman(cmdArgs))
	case "G", "getpkgbuild":
		return handleGetpkgbuild(cmdArgs, dbExecutor)
	case "P", "show":
		return handlePrint(cmdArgs, dbExecutor)
	case "Y", "--yay":
		return handleYay(cmdArgs, dbExecutor)
	}

	return fmt.Errorf(gotext.Get("unhandled operation"))
}

func handleQuery(cmdArgs *settings.Arguments, dbExecutor db.Executor) error {
	if cmdArgs.ExistsArg("u", "upgrades") {
		return printUpdateList(cmdArgs, dbExecutor, cmdArgs.ExistsDouble("u", "sysupgrade"))
	}
	return config.Runtime.CmdRunner.Show(passToPacman(cmdArgs))
}

func handleHelp(cmdArgs *settings.Arguments) error {
	if cmdArgs.Op == "Y" || cmdArgs.Op == "yay" {
		usage()
		return nil
	}
	return config.Runtime.CmdRunner.Show(passToPacman(cmdArgs))
}

func handleVersion() {
	fmt.Printf("yay v%s - libalpm v%s\n", yayVersion, alpm.Version())
}

func handlePrint(cmdArgs *settings.Arguments, dbExecutor db.Executor) (err error) {
	switch {
	case cmdArgs.ExistsArg("d", "defaultconfig"):
		tmpConfig := settings.DefaultConfig()
		fmt.Printf("%v", tmpConfig)
	case cmdArgs.ExistsArg("g", "currentconfig"):
		fmt.Printf("%v", config)
	case cmdArgs.ExistsArg("n", "numberupgrades"):
		err = printNumberOfUpdates(dbExecutor, cmdArgs.ExistsDouble("u", "sysupgrade"))
	case cmdArgs.ExistsArg("w", "news"):
		double := cmdArgs.ExistsDouble("w", "news")
		quiet := cmdArgs.ExistsArg("q", "quiet")
		err = news.PrintNewsFeed(dbExecutor.LastBuildTime(), config.SortMode, double, quiet)
	case cmdArgs.ExistsDouble("c", "complete"):
		err = completion.Show(dbExecutor, config.AURURL, config.Runtime.CompletionPath, config.CompletionInterval, true)
	case cmdArgs.ExistsArg("c", "complete"):
		err = completion.Show(dbExecutor, config.AURURL, config.Runtime.CompletionPath, config.CompletionInterval, false)
	case cmdArgs.ExistsArg("s", "stats"):
		err = localStatistics(dbExecutor)
	case cmdArgs.ExistsArg("p", "pkgbuild"):
		err = printPkgbuilds(dbExecutor, cmdArgs.Targets)
	default:
		err = nil
	}
	return err
}

func handleYay(cmdArgs *settings.Arguments, dbExecutor db.Executor) error {
	if cmdArgs.ExistsArg("gendb") {
		return createDevelDB(config, dbExecutor)
	}
	if cmdArgs.ExistsDouble("c") {
		return cleanDependencies(cmdArgs, dbExecutor, true)
	}
	if cmdArgs.ExistsArg("c", "clean") {
		return cleanDependencies(cmdArgs, dbExecutor, false)
	}
	if len(cmdArgs.Targets) > 0 {
		return handleYogurt(cmdArgs, dbExecutor)
	}
	return nil
}

func handleGetpkgbuild(cmdArgs *settings.Arguments, dbExecutor db.Executor) error {
	return getPkgbuilds(cmdArgs.Targets, dbExecutor, cmdArgs.ExistsArg("f", "force"))
}

func handleYogurt(cmdArgs *settings.Arguments, dbExecutor db.Executor) error {
	config.SearchMode = numberMenu
	return displayNumberMenu(cmdArgs.Targets, dbExecutor, cmdArgs)
}

func handleSync(cmdArgs *settings.Arguments, dbExecutor db.Executor) error {
	targets := cmdArgs.Targets

	if cmdArgs.ExistsArg("s", "search") {
		if cmdArgs.ExistsArg("q", "quiet") {
			config.SearchMode = minimal
		} else {
			config.SearchMode = detailed
		}
		return syncSearch(targets, dbExecutor)
	}
	if cmdArgs.ExistsArg("p", "print", "print-format") {
		return config.Runtime.CmdRunner.Show(passToPacman(cmdArgs))
	}
	if cmdArgs.ExistsArg("c", "clean") {
		return syncClean(cmdArgs, dbExecutor)
	}
	if cmdArgs.ExistsArg("l", "list") {
		return syncList(cmdArgs, dbExecutor)
	}
	if cmdArgs.ExistsArg("g", "groups") {
		return config.Runtime.CmdRunner.Show(passToPacman(cmdArgs))
	}
	if cmdArgs.ExistsArg("i", "info") {
		return syncInfo(cmdArgs, targets, dbExecutor)
	}
	if cmdArgs.ExistsArg("u", "sysupgrade") {
		return install(cmdArgs, dbExecutor, false)
	}
	if len(cmdArgs.Targets) > 0 {
		return install(cmdArgs, dbExecutor, false)
	}
	if cmdArgs.ExistsArg("y", "refresh") {
		return config.Runtime.CmdRunner.Show(passToPacman(cmdArgs))
	}
	return nil
}

func handleRemove(cmdArgs *settings.Arguments, localCache *vcs.InfoStore) error {
	err := config.Runtime.CmdRunner.Show(passToPacman(cmdArgs))
	if err == nil {
		localCache.RemovePackage(cmdArgs.Targets)
	}

	return err
}

// NumberMenu presents a CLI for selecting packages to install.
func displayNumberMenu(pkgS []string, dbExecutor db.Executor, cmdArgs *settings.Arguments) error {
	var (
		aurErr, repoErr error
		aq              aurQuery
		pq              repoQuery
		lenaq, lenpq    int
	)

	pkgS = query.RemoveInvalidTargets(pkgS, config.Runtime.Mode)

	if config.Runtime.Mode == settings.ModeAUR || config.Runtime.Mode == settings.ModeAny {
		aq, aurErr = narrowSearch(pkgS, true)
		lenaq = len(aq)
	}
	if config.Runtime.Mode == settings.ModeRepo || config.Runtime.Mode == settings.ModeAny {
		pq = queryRepo(pkgS, dbExecutor)
		lenpq = len(pq)
		if repoErr != nil {
			return repoErr
		}
	}

	if lenpq == 0 && lenaq == 0 {
		return fmt.Errorf(gotext.Get("no packages match search"))
	}

	switch config.SortMode {
	case settings.TopDown:
		if config.Runtime.Mode == settings.ModeRepo || config.Runtime.Mode == settings.ModeAny {
			pq.printSearch(dbExecutor)
		}
		if config.Runtime.Mode == settings.ModeAUR || config.Runtime.Mode == settings.ModeAny {
			aq.printSearch(lenpq+1, dbExecutor)
		}
	case settings.BottomUp:
		if config.Runtime.Mode == settings.ModeAUR || config.Runtime.Mode == settings.ModeAny {
			aq.printSearch(lenpq+1, dbExecutor)
		}
		if config.Runtime.Mode == settings.ModeRepo || config.Runtime.Mode == settings.ModeAny {
			pq.printSearch(dbExecutor)
		}
	default:
		return fmt.Errorf(gotext.Get("invalid sort mode. Fix with yay -Y --bottomup --save"))
	}

	if aurErr != nil {
		text.Errorln(gotext.Get("Error during AUR search: %s\n", aurErr))
		text.Warnln(gotext.Get("Showing repo packages only"))
	}

	text.Infoln(gotext.Get("Packages to install (eg: 1 2 3, 1-3 or ^4)"))
	text.Info()

	reader := bufio.NewReader(os.Stdin)

	numberBuf, overflow, err := reader.ReadLine()
	if err != nil {
		return err
	}
	if overflow {
		return fmt.Errorf(gotext.Get("input too long"))
	}

	include, exclude, _, otherExclude := intrange.ParseNumberMenu(string(numberBuf))
	arguments := cmdArgs.CopyGlobal()

	isInclude := len(exclude) == 0 && len(otherExclude) == 0

	for i, pkg := range pq {
		var target int
		switch config.SortMode {
		case settings.TopDown:
			target = i + 1
		case settings.BottomUp:
			target = len(pq) - i
		default:
			return fmt.Errorf(gotext.Get("invalid sort mode. Fix with yay -Y --bottomup --save"))
		}

		if (isInclude && include.Get(target)) || (!isInclude && !exclude.Get(target)) {
			arguments.AddTarget(pkg.DB().Name() + "/" + pkg.Name())
		}
	}

	for i := range aq {
		var target int

		switch config.SortMode {
		case settings.TopDown:
			target = i + 1 + len(pq)
		case settings.BottomUp:
			target = len(aq) - i + len(pq)
		default:
			return fmt.Errorf(gotext.Get("invalid sort mode. Fix with yay -Y --bottomup --save"))
		}

		if (isInclude && include.Get(target)) || (!isInclude && !exclude.Get(target)) {
			arguments.AddTarget("aur/" + aq[i].Name)
		}
	}

	if len(arguments.Targets) == 0 {
		fmt.Println(gotext.Get(" there is nothing to do"))
		return nil
	}

	if config.SudoLoop {
		sudoLoopBackground()
	}

	return install(arguments, dbExecutor, true)
}

func syncList(cmdArgs *settings.Arguments, dbExecutor db.Executor) error {
	aur := false

	for i := len(cmdArgs.Targets) - 1; i >= 0; i-- {
		if cmdArgs.Targets[i] == "aur" && (config.Runtime.Mode == settings.ModeAny || config.Runtime.Mode == settings.ModeAUR) {
			cmdArgs.Targets = append(cmdArgs.Targets[:i], cmdArgs.Targets[i+1:]...)
			aur = true
		}
	}

	if (config.Runtime.Mode == settings.ModeAny || config.Runtime.Mode == settings.ModeAUR) && (len(cmdArgs.Targets) == 0 || aur) {
		resp, err := http.Get(config.AURURL + "/packages.gz")
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)

		scanner.Scan()
		for scanner.Scan() {
			name := scanner.Text()
			if cmdArgs.ExistsArg("q", "quiet") {
				fmt.Println(name)
			} else {
				fmt.Printf("%s %s %s", text.Magenta("aur"), text.Bold(name), text.Bold(text.Green(gotext.Get("unknown-version"))))

				if dbExecutor.LocalPackage(name) != nil {
					fmt.Print(text.Bold(text.Blue(gotext.Get(" [Installed]"))))
				}

				fmt.Println()
			}
		}
	}

	if (config.Runtime.Mode == settings.ModeAny || config.Runtime.Mode == settings.ModeRepo) && (len(cmdArgs.Targets) != 0 || !aur) {
		return config.Runtime.CmdRunner.Show(passToPacman(cmdArgs))
	}

	return nil
}
