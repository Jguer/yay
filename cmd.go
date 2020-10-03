package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"

	"github.com/Jguer/go-alpm/v2"
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
	fmt.Printf(gotext.Get("Usage"))
	fmt.Printf(":\n")

	fmt.Printf("    yay\n")

	fmt.Printf("    yay <")
	fmt.Printf(gotext.Get("operation"))
	fmt.Printf("> [...]\n")

	fmt.Printf("    yay <")
	fmt.Printf(gotext.Get("package(s)"))
	fmt.Printf(">\n\n")

	fmt.Printf(gotext.Get("operations"))
	fmt.Printf(":\n")

	fmt.Printf("    yay {-h --help}\n")
	fmt.Printf("    yay {-V --version}\n")

	fmt.Printf("    yay {-D --database}    <")
	fmt.Printf(gotext.Get("options"))
	fmt.Printf("> <")
	fmt.Printf(gotext.Get("package(s)"))
	fmt.Printf(">\n")

	fmt.Printf("    yay {-F --files}       [")
	fmt.Printf(gotext.Get("options"))
	fmt.Printf("] [")
	fmt.Printf(gotext.Get("package(s)"))
	fmt.Printf("]\n")

	fmt.Printf("    yay {-Q --query}       [")
	fmt.Printf(gotext.Get("options"))
	fmt.Printf("] [")
	fmt.Printf(gotext.Get("package(s)"))
	fmt.Printf("]\n")

	fmt.Printf("    yay {-R --remove}      [")
	fmt.Printf(gotext.Get("options"))
	fmt.Printf("] [")
	fmt.Printf(gotext.Get("package(s)"))
	fmt.Printf("]\n")

	fmt.Printf("    yay {-S --sync}        [")
	fmt.Printf(gotext.Get("options"))
	fmt.Printf("] [")
	fmt.Printf(gotext.Get("package(s)"))
	fmt.Printf("]\n")

	fmt.Printf("    yay {-T --deptest}     [")
	fmt.Printf(gotext.Get("options"))
	fmt.Printf("] [")
	fmt.Printf(gotext.Get("package(s)"))
	fmt.Printf("]\n")

	fmt.Printf("    yay {-U --upgrade}     [")
	fmt.Printf(gotext.Get("options"))
	fmt.Printf("] <")
	fmt.Printf(gotext.Get("file(s)"))
	fmt.Printf(">\n\n")

	fmt.Printf(gotext.Get("New operations"))
	fmt.Printf(":\n")

	fmt.Printf("    yay {-Y --yay}         [")
	fmt.Printf(gotext.Get("options"))
	fmt.Printf("] [")
	fmt.Printf(gotext.Get("package(s)"))
	fmt.Printf("]\n")

	fmt.Printf("    yay {-P --show}        [")
	fmt.Printf(gotext.Get("options"))
	fmt.Printf("]\n")

	fmt.Printf("    yay {-G --getpkgbuild} [")
	fmt.Printf(gotext.Get("package(s)"))
	fmt.Printf("]\n\n")

	fmt.Printf(gotext.Get("If no arguments are provided 'yay -Syu' will be performed"))
	fmt.Printf("\n")

	fmt.Printf(gotext.Get("If no operation is provided -Y will be assumed"))
	fmt.Printf("\n\n")

	fmt.Printf(gotext.Get("New options"))
	fmt.Printf(":\n")

	fmt.Printf("       --repo             ")
	fmt.Printf(gotext.Get("Assume targets are from the repositories"))
	fmt.Printf("\n")

	fmt.Printf("    -a --aur              ")
	fmt.Printf(gotext.Get("Assume targets are from the AUR"))
	fmt.Printf("\n\n")

	fmt.Printf(gotext.Get("Permanent configuration options"))
	fmt.Printf(":\n")

	fmt.Printf("    --save                ")
	fmt.Printf(gotext.Get("Causes the following options to be saved back to the"))
	fmt.Printf("\n")

	fmt.Printf("                          ")
	fmt.Printf(gotext.Get("config file when used"))
	fmt.Printf("\n\n")

	fmt.Printf("    --aururl      ")
	fmt.Printf(gotext.Get("<url>"))
	fmt.Printf("   ")
	fmt.Printf(gotext.Get("Set an alternative AUR URL"))
	fmt.Printf("\n")

	fmt.Printf("    --builddir    ")
	fmt.Printf(gotext.Get("<dir>"))
	fmt.Printf("   ")
	fmt.Printf(gotext.Get("Directory used to download and run PKGBUILDS"))
	fmt.Printf("\n")

	fmt.Printf("    --absdir      ")
	fmt.Printf(gotext.Get("<dir>"))
	fmt.Printf("   ")
	fmt.Printf(gotext.Get("Directory used to store downloads from the ABS"))
	fmt.Printf("\n")

	fmt.Printf("    --editor      ")
	fmt.Printf(gotext.Get("<file>"))
	fmt.Printf("  ")
	fmt.Printf(gotext.Get("Editor to use when editing PKGBUILDs"))
	fmt.Printf("\n")

	fmt.Printf("    --editorflags ")
	fmt.Printf(gotext.Get("<flags>"))
	fmt.Printf(" ")
	fmt.Printf(gotext.Get("Pass arguments to editor"))
	fmt.Printf("\n")

	fmt.Printf("    --makepkg     ")
	fmt.Printf(gotext.Get("<file>"))
	fmt.Printf("  ")
	fmt.Printf(gotext.Get("makepkg command to use"))
	fmt.Printf("\n")

	fmt.Printf("    --mflags      ")
	fmt.Printf(gotext.Get("<flags>"))
	fmt.Printf(" ")
	fmt.Printf(gotext.Get("Pass arguments to makepkg"))
	fmt.Printf("\n")

	fmt.Printf("    --pacman      ")
	fmt.Printf(gotext.Get("<file>"))
	fmt.Printf("  ")
	fmt.Printf(gotext.Get("pacman command to use"))
	fmt.Printf("\n")

	fmt.Printf("    --git         ")
	fmt.Printf(gotext.Get("<file>"))
	fmt.Printf("  ")
	fmt.Printf(gotext.Get("git command to use"))
	fmt.Printf("\n")

	fmt.Printf("    --gitflags    ")
	fmt.Printf(gotext.Get("<flags>"))
	fmt.Printf(" ")
	fmt.Printf(gotext.Get("Pass arguments to git"))
	fmt.Printf("\n")

	fmt.Printf("    --gpg         ")
	fmt.Printf(gotext.Get("<file>"))
	fmt.Printf("  ")
	fmt.Printf(gotext.Get("gpg command to use"))
	fmt.Printf("\n")

	fmt.Printf("    --gpgflags    ")
	fmt.Printf(gotext.Get("<flags>"))
	fmt.Printf(" ")
	fmt.Printf(gotext.Get("Pass arguments to gpg"))
	fmt.Printf("\n")

	fmt.Printf("    --config      ")
	fmt.Printf(gotext.Get("<file>"))
	fmt.Printf("  ")
	fmt.Printf(gotext.Get("pacman.conf file to use"))
	fmt.Printf("\n")

	fmt.Printf("    --makepkgconf ")
	fmt.Printf(gotext.Get("<file>"))
	fmt.Printf("  ")
	fmt.Printf(gotext.Get("makepkg.conf file to use"))
	fmt.Printf("\n")

	fmt.Printf("    --nomakepkgconf       ")
	fmt.Printf(gotext.Get("Use the default makepkg.conf"))
	fmt.Printf("\n\n")

	fmt.Printf("    --requestsplitn ")
	fmt.Printf(gotext.Get("<n>"))
	fmt.Printf("   ")
	fmt.Printf(gotext.Get("Max amount of packages to query per AUR request"))
	fmt.Printf("\n")

	fmt.Printf("    --completioninterval  ")
	fmt.Printf(gotext.Get("<n>"))
	fmt.Printf(" ")
	fmt.Printf(gotext.Get("Time in days to refresh completion cache"))
	fmt.Printf("\n")

	fmt.Printf("    --sortby    ")
	fmt.Printf(gotext.Get("<field>"))
	fmt.Printf("   ")
	fmt.Printf(gotext.Get("Sort AUR results by a specific field during search"))
	fmt.Printf("\n")

	fmt.Printf("    --searchby  ")
	fmt.Printf(gotext.Get("<field>"))
	fmt.Printf("   ")
	fmt.Printf(gotext.Get("Search for packages using a specified field"))
	fmt.Printf("\n")

	fmt.Printf("    --answerclean   ")
	fmt.Printf(gotext.Get("<a>"))
	fmt.Printf("   ")
	fmt.Printf(gotext.Get("Set a predetermined answer for the clean build menu"))
	fmt.Printf("\n")

	fmt.Printf("    --answerdiff    ")
	fmt.Printf(gotext.Get("<a>"))
	fmt.Printf("   ")
	fmt.Printf(gotext.Get("Set a predetermined answer for the diff menu"))
	fmt.Printf("\n")

	fmt.Printf("    --answeredit    ")
	fmt.Printf(gotext.Get("<a>"))
	fmt.Printf("   ")
	fmt.Printf(gotext.Get("Set a predetermined answer for the edit pkgbuild menu"))
	fmt.Printf("\n")

	fmt.Printf("    --answerupgrade ")
	fmt.Printf(gotext.Get("<a>"))
	fmt.Printf("   ")
	fmt.Printf(gotext.Get("Set a predetermined answer for the upgrade menu"))
	fmt.Printf("\n")

	fmt.Printf("    --noanswerclean       ")
	fmt.Printf(gotext.Get("Unset the answer for the clean build menu"))
	fmt.Printf("\n")

	fmt.Printf("    --noanswerdiff        ")
	fmt.Printf(gotext.Get("Unset the answer for the edit diff menu"))
	fmt.Printf("\n")

	fmt.Printf("    --noansweredit        ")
	fmt.Printf(gotext.Get("Unset the answer for the edit pkgbuild menu"))
	fmt.Printf("\n")

	fmt.Printf("    --noanswerupgrade     ")
	fmt.Printf(gotext.Get("Unset the answer for the upgrade menu"))
	fmt.Printf("\n")

	fmt.Printf("    --cleanmenu           ")
	fmt.Printf(gotext.Get("Give the option to clean build PKGBUILDS"))
	fmt.Printf("\n")

	fmt.Printf("    --diffmenu            ")
	fmt.Printf(gotext.Get("Give the option to show diffs for build files"))
	fmt.Printf("\n")

	fmt.Printf("    --editmenu            ")
	fmt.Printf(gotext.Get("Give the option to edit/view PKGBUILDS"))
	fmt.Printf("\n")

	fmt.Printf("    --upgrademenu         ")
	fmt.Printf(gotext.Get("Show a detailed list of updates with the option to skip any"))
	fmt.Printf("\n")

	fmt.Printf("    --nocleanmenu         ")
	fmt.Printf(gotext.Get("Don't clean build PKGBUILDS"))
	fmt.Printf("\n")

	fmt.Printf("    --nodiffmenu          ")
	fmt.Printf(gotext.Get("Don't show diffs for build files"))
	fmt.Printf("\n")

	fmt.Printf("    --noeditmenu          ")
	fmt.Printf(gotext.Get("Don't edit/view PKGBUILDS"))
	fmt.Printf("\n")

	fmt.Printf("    --noupgrademenu       ")
	fmt.Printf(gotext.Get("Don't show the upgrade menu"))
	fmt.Printf("\n")

	fmt.Printf("    --askremovemake       ")
	fmt.Printf(gotext.Get("Ask to remove makedepends after install"))
	fmt.Printf("\n")

	fmt.Printf("    --removemake          ")
	fmt.Printf(gotext.Get("Remove makedepends after install"))
	fmt.Printf("\n")

	fmt.Printf("    --noremovemake        ")
	fmt.Printf(gotext.Get("Don't remove makedepends after install"))
	fmt.Printf("\n\n")

	fmt.Printf("    --cleanafter          ")
	fmt.Printf(gotext.Get("Remove package sources after successful install"))
	fmt.Printf("\n")

	fmt.Printf("    --nocleanafter        ")
	fmt.Printf(gotext.Get("Do not remove package sources after successful build"))
	fmt.Printf("\n")

	fmt.Printf("    --bottomup            ")
	fmt.Printf(gotext.Get("Shows AUR's packages first and then repository's"))
	fmt.Printf("\n")

	fmt.Printf("    --topdown             ")
	fmt.Printf(gotext.Get("Shows repository's packages first and then AUR's"))
	fmt.Printf("\n\n")

	fmt.Printf("    --devel               ")
	fmt.Printf(gotext.Get("Check development packages during sysupgrade"))
	fmt.Printf("\n")

	fmt.Printf("    --nodevel             ")
	fmt.Printf(gotext.Get("Do not check development packages"))
	fmt.Printf("\n")

	fmt.Printf("    --rebuild             ")
	fmt.Printf(gotext.Get("Always build target packages"))
	fmt.Printf("\n")

	fmt.Printf("    --rebuildall          ")
	fmt.Printf(gotext.Get("Always build all AUR packages"))
	fmt.Printf("\n")

	fmt.Printf("    --norebuild           ")
	fmt.Printf(gotext.Get("Skip package build if in cache and up to date"))
	fmt.Printf("\n")

	fmt.Printf("    --rebuildtree         ")
	fmt.Printf(gotext.Get("Always build all AUR packages even if installed"))
	fmt.Printf("\n")

	fmt.Printf("    --redownload          ")
	fmt.Printf(gotext.Get("Always download pkgbuilds of targets"))
	fmt.Printf("\n")

	fmt.Printf("    --noredownload        ")
	fmt.Printf(gotext.Get("Skip pkgbuild download if in cache and up to date"))
	fmt.Printf("\n")

	fmt.Printf("    --redownloadall       ")
	fmt.Printf(gotext.Get("Always download pkgbuilds of all AUR packages"))
	fmt.Printf("\n")

	fmt.Printf("    --provides            ")
	fmt.Printf(gotext.Get("Look for matching providers when searching for packages"))
	fmt.Printf("\n")

	fmt.Printf("    --noprovides          ")
	fmt.Printf(gotext.Get("Just look for packages by pkgname"))
	fmt.Printf("\n")

	fmt.Printf("    --pgpfetch            ")
	fmt.Printf(gotext.Get("Prompt to import PGP keys from PKGBUILDs"))
	fmt.Printf("\n")

	fmt.Printf("    --nopgpfetch          ")
	fmt.Printf(gotext.Get("Don't prompt to import PGP keys"))
	fmt.Printf("\n")

	fmt.Printf("    --useask              ")
	fmt.Printf(gotext.Get("Automatically resolve conflicts using pacman's ask flag"))
	fmt.Printf("\n")

	fmt.Printf("    --nouseask            ")
	fmt.Printf(gotext.Get("Confirm conflicts manually during the install"))
	fmt.Printf("\n")

	fmt.Printf("    --combinedupgrade     ")
	fmt.Printf(gotext.Get("Refresh then perform the repo and AUR upgrade together"))
	fmt.Printf("\n")

	fmt.Printf("    --nocombinedupgrade   ")
	fmt.Printf(gotext.Get("Perform the repo upgrade and AUR upgrade separately"))
	fmt.Printf("\n")

	fmt.Printf("    --batchinstall        ")
	fmt.Printf(gotext.Get("Build multiple AUR packages then install them together"))
	fmt.Printf("\n")

	fmt.Printf("    --nobatchinstall      ")
	fmt.Printf(gotext.Get("Build and install each AUR package one by one"))
	fmt.Printf("\n\n")

	fmt.Printf("    --sudo                ")
	fmt.Printf(gotext.Get("<file>"))
	fmt.Printf("  ")
	fmt.Printf(gotext.Get("sudo command to use"))
	fmt.Printf("\n")

	fmt.Printf("    --sudoflags           ")
	fmt.Printf(gotext.Get("<flags>"))
	fmt.Printf(" ")
	fmt.Printf(gotext.Get("Pass arguments to sudo"))
	fmt.Printf("\n")

	fmt.Printf("    --sudoloop            ")
	fmt.Printf(gotext.Get("Loop sudo calls in the background to avoid timeout"))
	fmt.Printf("\n")

	fmt.Printf("    --nosudoloop          ")
	fmt.Printf(gotext.Get("Do not loop sudo calls in the background"))
	fmt.Printf("\n\n")

	fmt.Printf("    --timeupdate          ")
	fmt.Printf(gotext.Get("Check packages' AUR page for changes during sysupgrade"))
	fmt.Printf("\n")

	fmt.Printf("    --notimeupdate        ")
	fmt.Printf(gotext.Get("Do not check packages' AUR page for changes"))
	fmt.Printf("\n\n")

	fmt.Printf(gotext.Get("show specific options"))
	fmt.Printf(":")
	fmt.Printf("\n")

	fmt.Printf("    -c --complete         ")
	fmt.Printf(gotext.Get("Used for completions"))
	fmt.Printf("\n")

	fmt.Printf("    -d --defaultconfig    ")
	fmt.Printf(gotext.Get("Print default yay configuration"))
	fmt.Printf("\n")

	fmt.Printf("    -g --currentconfig    ")
	fmt.Printf(gotext.Get("Print current yay configuration"))
	fmt.Printf("\n")

	fmt.Printf("    -s --stats            ")
	fmt.Printf(gotext.Get("Display system package statistics"))
	fmt.Printf("\n")

	fmt.Printf("    -w --news             ")
	fmt.Printf(gotext.Get("Print arch news"))
	fmt.Printf("\n\n")

	fmt.Printf(gotext.Get("yay specific options"))
	fmt.Printf(":\n")

	fmt.Printf("    -c --clean            ")
	fmt.Printf(gotext.Get("Remove unneeded dependencies"))
	fmt.Printf("\n")

	fmt.Printf("       --gendb            ")
	fmt.Printf(gotext.Get("Generates development package DB used for updating"))
	fmt.Printf("\n\n")

	fmt.Printf(gotext.Get("getpkgbuild specific options"))
	fmt.Printf(":\n")

	fmt.Printf("    -f --force            ")
	fmt.Printf(gotext.Get("Force download for existing ABS packages"))
	fmt.Printf("\n")
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
