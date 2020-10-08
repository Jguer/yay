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
	fmt.Printf("%v", gotext.Get("Usage"))
	fmt.Printf("%v\n", ":")

	fmt.Printf("%v\n", "    yay")

	fmt.Printf("%v", "    yay <")
	fmt.Printf("%v", gotext.Get("operation"))
	fmt.Printf("%v\n", "> [...]")

	fmt.Printf("%v", "    yay <")
	fmt.Printf("%v", gotext.Get("package(s)"))
	fmt.Printf("%v\n\n", ">")

	fmt.Printf("%v", gotext.Get("operations"))
	fmt.Printf("%v\n", ":")

	fmt.Printf("%v\n", "    yay {-h --help}")

	fmt.Printf("%v\n", "    yay {-V --version}")

	fmt.Printf("%v", "    yay {-D --database}    <")
	fmt.Printf("%v", gotext.Get("options"))
	fmt.Printf("%v", "> <")
	fmt.Printf("%v", gotext.Get("package(s)"))
	fmt.Printf("%v\n", ">")

	fmt.Printf("%v", "    yay {-F --files}       [")
	fmt.Printf("%v", gotext.Get("options"))
	fmt.Printf("%v", "] [")
	fmt.Printf("%v", gotext.Get("package(s)"))
	fmt.Printf("%v\n", "]")

	fmt.Printf("%v", "    yay {-Q --query}       [")
	fmt.Printf("%v", gotext.Get("options"))
	fmt.Printf("%v", "] [")
	fmt.Printf("%v", gotext.Get("package(s)"))
	fmt.Printf("%v\n", "]")

	fmt.Printf("%v", "    yay {-R --remove}      [")
	fmt.Printf("%v", gotext.Get("options"))
	fmt.Printf("%v", "] [")
	fmt.Printf("%v", gotext.Get("package(s)"))
	fmt.Printf("%v\n", "]")

	fmt.Printf("%v", "    yay {-S --sync}        [")
	fmt.Printf("%v", gotext.Get("options"))
	fmt.Printf("%v", "] [")
	fmt.Printf("%v", gotext.Get("package(s)"))
	fmt.Printf("%v\n", "]")

	fmt.Printf("%v", "    yay {-T --deptest}     [")
	fmt.Printf("%v", gotext.Get("options"))
	fmt.Printf("%v", "] [")
	fmt.Printf("%v", gotext.Get("package(s)"))
	fmt.Printf("%v\n", "]")

	fmt.Printf("%v", "    yay {-U --upgrade}     [")
	fmt.Printf("%v", gotext.Get("options"))
	fmt.Printf("%v", "] <")
	fmt.Printf("%v", gotext.Get("file(s)"))
	fmt.Printf("%v\n\n", ">")

	fmt.Printf("%v", gotext.Get("New operations"))
	fmt.Printf("%v\n", ":")

	fmt.Printf("%v", "    yay {-Y --yay}         [")
	fmt.Printf("%v", gotext.Get("options"))
	fmt.Printf("%v", "] [")
	fmt.Printf("%v", gotext.Get("package(s)"))
	fmt.Printf("%v\n", "]")

	fmt.Printf("%v", "    yay {-P --show}        [")
	fmt.Printf("%v", gotext.Get("options"))
	fmt.Printf("%v\n", "]")

	fmt.Printf("%v", "    yay {-G --getpkgbuild} [")
	fmt.Printf("%v", gotext.Get("package(s)"))
	fmt.Printf("%v", "]\n\n")

	fmt.Printf("%v\n", gotext.Get("If no arguments are provided 'yay -Syu' will be performed"))

	fmt.Printf("%v\n\n", gotext.Get("If no operation is provided -Y will be assumed"))

	fmt.Printf("%v", gotext.Get("New options"))
	fmt.Printf("%v\n", ":")

	fmt.Printf("%v", "       --repo             ")
	fmt.Printf("%v\n", gotext.Get("Assume targets are from the repositories"))

	fmt.Printf("%v", "    -a --aur              ")
	fmt.Printf("%v\n\n", gotext.Get("Assume targets are from the AUR"))

	fmt.Printf("%v", gotext.Get("Permanent configuration options"))
	fmt.Printf("%v\n", ":")

	fmt.Printf("%v", "    --save                ")
	fmt.Printf("%v\n", gotext.Get("Causes the following options to be saved back to the"))

	fmt.Printf("%v", "                          ")
	fmt.Printf("%v\n\n", gotext.Get("config file when used"))

	fmt.Printf("%v", "    --aururl      ")
	fmt.Printf("%v", gotext.Get("<url>"))
	fmt.Printf("%v", "   ")
	fmt.Printf("%v\n", gotext.Get("Set an alternative AUR URL"))

	fmt.Printf("%v", "    --builddir    ")
	fmt.Printf("%v", gotext.Get("<dir>"))
	fmt.Printf("%v", "   ")
	fmt.Printf("%v\n", gotext.Get("Directory used to download and run PKGBUILDS"))

	fmt.Printf("%v", "    --absdir      ")
	fmt.Printf("%v", gotext.Get("<dir>"))
	fmt.Printf("%v", "   ")
	fmt.Printf("%v\n", gotext.Get("Directory used to store downloads from the ABS"))

	fmt.Printf("%v", "    --editor      ")
	fmt.Printf("%v", gotext.Get("<file>"))
	fmt.Printf("%v", "  ")
	fmt.Printf("%v\n", gotext.Get("Editor to use when editing PKGBUILDs"))

	fmt.Printf("%v", "    --editorflags ")
	fmt.Printf("%v", gotext.Get("<flags>"))
	fmt.Printf("%v", " ")
	fmt.Printf("%v\n", gotext.Get("Pass arguments to editor"))

	fmt.Printf("%v", "    --makepkg     ")
	fmt.Printf("%v", gotext.Get("<file>"))
	fmt.Printf("%v", "  ")
	fmt.Printf("%v\n", gotext.Get("makepkg command to use"))

	fmt.Printf("%v", "    --mflags      ")
	fmt.Printf("%v", gotext.Get("<flags>"))
	fmt.Printf("%v", " ")
	fmt.Printf("%v\n", gotext.Get("Pass arguments to makepkg"))

	fmt.Printf("%v", "    --pacman      ")
	fmt.Printf("%v", gotext.Get("<file>"))
	fmt.Printf("%v", "  ")
	fmt.Printf("%v\n", gotext.Get("pacman command to use"))

	fmt.Printf("%v", "    --git         ")
	fmt.Printf("%v", gotext.Get("<file>"))
	fmt.Printf("%v", "  ")
	fmt.Printf("%v\n", gotext.Get("git command to use"))

	fmt.Printf("%v", "    --gitflags    ")
	fmt.Printf("%v", gotext.Get("<flags>"))
	fmt.Printf("%v", " ")
	fmt.Printf("%v\n", gotext.Get("Pass arguments to git"))

	fmt.Printf("%v", "    --gpg         ")
	fmt.Printf("%v", gotext.Get("<file>"))
	fmt.Printf("%v", "  ")
	fmt.Printf("%v\n", gotext.Get("gpg command to use"))

	fmt.Printf("%v", "    --gpgflags    ")
	fmt.Printf("%v", gotext.Get("<flags>"))
	fmt.Printf("%v", " ")
	fmt.Printf("%v\n", gotext.Get("Pass arguments to gpg"))

	fmt.Printf("%v", "    --config      ")
	fmt.Printf("%v", gotext.Get("<file>"))
	fmt.Printf("%v", "  ")
	fmt.Printf("%v\n", gotext.Get("pacman.conf file to use"))

	fmt.Printf("%v", "    --makepkgconf ")
	fmt.Printf("%v", gotext.Get("<file>"))
	fmt.Printf("%v", "  ")
	fmt.Printf("%v\n", gotext.Get("makepkg.conf file to use"))

	fmt.Printf("%v", "    --nomakepkgconf       ")
	fmt.Printf("%v\n\n", gotext.Get("Use the default makepkg.conf"))

	fmt.Printf("%v", "    --requestsplitn ")
	fmt.Printf("%v", gotext.Get("<n>"))
	fmt.Printf("%v", "   ")
	fmt.Printf("%v\n", gotext.Get("Max amount of packages to query per AUR request"))

	fmt.Printf("%v", "    --completioninterval  ")
	fmt.Printf("%v", gotext.Get("<n>"))
	fmt.Printf("%v", " ")
	fmt.Printf("%v\n", gotext.Get("Time in days to refresh completion cache"))

	fmt.Printf("%v", "    --sortby    ")
	fmt.Printf("%v", gotext.Get("<field>"))
	fmt.Printf("%v", "   ")
	fmt.Printf("%v\n", gotext.Get("Sort AUR results by a specific field during search"))

	fmt.Printf("%v", "    --searchby  ")
	fmt.Printf("%v", gotext.Get("<field>"))
	fmt.Printf("%v", "   ")
	fmt.Printf("%v\n", gotext.Get("Search for packages using a specified field"))

	fmt.Printf("%v", "    --answerclean   ")
	fmt.Printf("%v", gotext.Get("<a>"))
	fmt.Printf("%v", "   ")
	fmt.Printf("%v\n", gotext.Get("Set a predetermined answer for the clean build menu"))

	fmt.Printf("%v", "    --answerdiff    ")
	fmt.Printf("%v", gotext.Get("<a>"))
	fmt.Printf("%v", "   ")
	fmt.Printf("%v\n", gotext.Get("Set a predetermined answer for the diff menu"))

	fmt.Printf("%v", "    --answeredit    ")
	fmt.Printf("%v", gotext.Get("<a>"))
	fmt.Printf("%v", "   ")
	fmt.Printf("%v\n", gotext.Get("Set a predetermined answer for the edit pkgbuild menu"))

	fmt.Printf("%v", "    --answerupgrade ")
	fmt.Printf("%v", gotext.Get("<a>"))
	fmt.Printf("%v", "   ")
	fmt.Printf("%v\n", gotext.Get("Set a predetermined answer for the upgrade menu"))

	fmt.Printf("%v", "    --noanswerclean       ")
	fmt.Printf("%v\n", gotext.Get("Unset the answer for the clean build menu"))

	fmt.Printf("%v", "    --noanswerdiff        ")
	fmt.Printf("%v\n", gotext.Get("Unset the answer for the edit diff menu"))

	fmt.Printf("%v", "    --noansweredit        ")
	fmt.Printf("%v\n", gotext.Get("Unset the answer for the edit pkgbuild menu"))

	fmt.Printf("%v", "    --noanswerupgrade     ")
	fmt.Printf("%v\n", gotext.Get("Unset the answer for the upgrade menu"))

	fmt.Printf("%v", "    --cleanmenu           ")
	fmt.Printf("%v\n", gotext.Get("Give the option to clean build PKGBUILDS"))

	fmt.Printf("%v", "    --diffmenu            ")
	fmt.Printf("%v\n", gotext.Get("Give the option to show diffs for build files"))

	fmt.Printf("%v", "    --editmenu            ")
	fmt.Printf("%v\n", gotext.Get("Give the option to edit/view PKGBUILDS"))

	fmt.Printf("%v", "    --upgrademenu         ")
	fmt.Printf("%v\n", gotext.Get("Show a detailed list of updates with the option to skip any"))

	fmt.Printf("%v", "    --nocleanmenu         ")
	fmt.Printf("%v\n", gotext.Get("Don't clean build PKGBUILDS"))

	fmt.Printf("%v", "    --nodiffmenu          ")
	fmt.Printf("%v\n", gotext.Get("Don't show diffs for build files"))

	fmt.Printf("%v", "    --noeditmenu          ")
	fmt.Printf("%v\n", gotext.Get("Don't edit/view PKGBUILDS"))

	fmt.Printf("%v", "    --noupgrademenu       ")
	fmt.Printf("%v\n", gotext.Get("Don't show the upgrade menu"))

	fmt.Printf("%v", "    --askremovemake       ")
	fmt.Printf("%v\n", gotext.Get("Ask to remove makedepends after install"))

	fmt.Printf("%v", "    --removemake          ")
	fmt.Printf("%v\n", gotext.Get("Remove makedepends after install"))

	fmt.Printf("%v", "    --noremovemake        ")
	fmt.Printf("%v\n\n", gotext.Get("Don't remove makedepends after install"))

	fmt.Printf("%v", "    --cleanafter          ")
	fmt.Printf("%v\n", gotext.Get("Remove package sources after successful install"))

	fmt.Printf("%v", "    --nocleanafter        ")
	fmt.Printf("%v\n", gotext.Get("Do not remove package sources after successful build"))

	fmt.Printf("%v", "    --bottomup            ")
	fmt.Printf("%v\n", gotext.Get("Shows AUR's packages first and then repository's"))

	fmt.Printf("%v", "    --topdown             ")
	fmt.Printf("%v\n\n", gotext.Get("Shows repository's packages first and then AUR's"))

	fmt.Printf("%v", "    --devel               ")
	fmt.Printf("%v\n", gotext.Get("Check development packages during sysupgrade"))

	fmt.Printf("%v", "    --nodevel             ")
	fmt.Printf("%v\n", gotext.Get("Do not check development packages"))

	fmt.Printf("%v", "    --rebuild             ")
	fmt.Printf("%v\n", gotext.Get("Always build target packages"))

	fmt.Printf("%v", "    --rebuildall          ")
	fmt.Printf("%v\n", gotext.Get("Always build all AUR packages"))

	fmt.Printf("%v", "    --norebuild           ")
	fmt.Printf("%v\n", gotext.Get("Skip package build if in cache and up to date"))

	fmt.Printf("%v", "    --rebuildtree         ")
	fmt.Printf("%v\n", gotext.Get("Always build all AUR packages even if installed"))

	fmt.Printf("%v", "    --redownload          ")
	fmt.Printf("%v\n", gotext.Get("Always download pkgbuilds of targets"))

	fmt.Printf("%v", "    --noredownload        ")
	fmt.Printf("%v\n", gotext.Get("Skip pkgbuild download if in cache and up to date"))

	fmt.Printf("%v", "    --redownloadall       ")
	fmt.Printf("%v\n", gotext.Get("Always download pkgbuilds of all AUR packages"))

	fmt.Printf("%v", "    --provides            ")
	fmt.Printf("%v\n", gotext.Get("Look for matching providers when searching for packages"))

	fmt.Printf("%v", "    --noprovides          ")
	fmt.Printf("%v\n", gotext.Get("Just look for packages by pkgname"))

	fmt.Printf("%v", "    --pgpfetch            ")
	fmt.Printf("%v\n", gotext.Get("Prompt to import PGP keys from PKGBUILDs"))

	fmt.Printf("%v", "    --nopgpfetch          ")
	fmt.Printf("%v\n", gotext.Get("Don't prompt to import PGP keys"))

	fmt.Printf("%v", "    --useask              ")
	fmt.Printf("%v\n", gotext.Get("Automatically resolve conflicts using pacman's ask flag"))

	fmt.Printf("%v", "    --nouseask            ")
	fmt.Printf("%v\n", gotext.Get("Confirm conflicts manually during the install"))

	fmt.Printf("%v", "    --combinedupgrade     ")
	fmt.Printf("%v\n", gotext.Get("Refresh then perform the repo and AUR upgrade together"))

	fmt.Printf("%v", "    --nocombinedupgrade   ")
	fmt.Printf("%v\n", gotext.Get("Perform the repo upgrade and AUR upgrade separately"))

	fmt.Printf("%v", "    --batchinstall        ")
	fmt.Printf("%v\n", gotext.Get("Build multiple AUR packages then install them together"))

	fmt.Printf("%v", "    --nobatchinstall      ")
	fmt.Printf("%v\n\n", gotext.Get("Build and install each AUR package one by one"))

	fmt.Printf("%v", "    --sudo                ")
	fmt.Printf("%v", gotext.Get("<file>"))
	fmt.Printf("%v", "  ")
	fmt.Printf("%v\n", gotext.Get("sudo command to use"))

	fmt.Printf("%v", "    --sudoflags           ")
	fmt.Printf("%v", gotext.Get("<flags>"))
	fmt.Printf("%v", " ")
	fmt.Printf("%v\n", gotext.Get("Pass arguments to sudo"))

	fmt.Printf("%v", "    --sudoloop            ")
	fmt.Printf("%v\n", gotext.Get("Loop sudo calls in the background to avoid timeout"))

	fmt.Printf("%v", "    --nosudoloop          ")
	fmt.Printf("%v\n\n", gotext.Get("Do not loop sudo calls in the background"))

	fmt.Printf("%v", "    --timeupdate          ")
	fmt.Printf("%v\n", gotext.Get("Check packages' AUR page for changes during sysupgrade"))

	fmt.Printf("%v", "    --notimeupdate        ")
	fmt.Printf("%v\n\n", gotext.Get("Do not check packages' AUR page for changes"))

	fmt.Printf("%v", gotext.Get("show specific options"))
	fmt.Printf("%v\n", ":")

	fmt.Printf("%v", "    -c --complete         ")
	fmt.Printf("%v\n", gotext.Get("Used for completions"))

	fmt.Printf("%v", "    -d --defaultconfig    ")
	fmt.Printf("%v\n", gotext.Get("Print default yay configuration"))

	fmt.Printf("%v", "    -g --currentconfig    ")
	fmt.Printf("%v\n", gotext.Get("Print current yay configuration"))

	fmt.Printf("%v", "    -s --stats            ")
	fmt.Printf("%v\n", gotext.Get("Display system package statistics"))

	fmt.Printf("%v", "    -w --news             ")
	fmt.Printf("%v\n\n", gotext.Get("Print arch news"))

	fmt.Printf("%v", gotext.Get("yay specific options"))
	fmt.Printf("%v\n", ":")

	fmt.Printf("%v", "    -c --clean            ")
	fmt.Printf("%v\n", gotext.Get("Remove unneeded dependencies"))

	fmt.Printf("%v", "       --gendb            ")
	fmt.Printf("%v\n\n", gotext.Get("Generates development package DB used for updating"))

	fmt.Printf("%v", gotext.Get("getpkgbuild specific options"))
	fmt.Printf("%v\n", ":")

	fmt.Printf("%v", "    -f --force            ")
	fmt.Printf("%v\n", gotext.Get("Force download for existing ABS packages"))
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
