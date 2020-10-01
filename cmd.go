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
	fmt.Println(gotext.Get("Usage:"))
	fmt.Println("    yay")
	fmt.Println(gotext.Get("    yay <operation> [...]"))
	fmt.Println(gotext.Get("    yay <package(s)>"))
	fmt.Println()
	fmt.Println(gotext.Get("operations:"))
	fmt.Println("    yay {-h --help}")
	fmt.Println("    yay {-V --version}")
	fmt.Println(gotext.Get("    yay {-D --database}    <options> <package(s)>"))
	fmt.Println(gotext.Get("    yay {-F --files}       [options] [package(s)]"))
	fmt.Println(gotext.Get("    yay {-Q --query}       [options] [package(s)]"))
	fmt.Println(gotext.Get("    yay {-R --remove}      [options] <package(s)>"))
	fmt.Println(gotext.Get("    yay {-S --sync}        [options] [package(s)]"))
	fmt.Println(gotext.Get("    yay {-T --deptest}     [options] [package(s)]"))
	fmt.Println(gotext.Get("    yay {-U --upgrade}     [options] <file(s)>"))
	fmt.Println()
	fmt.Println(gotext.Get("New operations:"))
	fmt.Println(gotext.Get("    yay {-Y --yay}         [options] [package(s)]"))
	fmt.Println(gotext.Get("    yay {-P --show}        [options]"))
	fmt.Println(gotext.Get("    yay {-G --getpkgbuild} [package(s)]"))
	fmt.Println()
	fmt.Println(gotext.Get("If no arguments are provided 'yay -Syu' will be performed"))
	fmt.Println(gotext.Get("If no operation is provided -Y will be assumed"))
	fmt.Println()
	fmt.Println(gotext.Get("New options:"))
	fmt.Println(gotext.Get("       --repo             Assume targets are from the repositories"))
	fmt.Println(gotext.Get("    -a --aur              Assume targets are from the AUR"))
	fmt.Println()
	fmt.Println(gotext.Get("Permanent configuration options:"))
	fmt.Println(gotext.Get("    --save                Causes the following options to be saved back to the"))
	fmt.Println(gotext.Get("                          config file when used"))
	fmt.Println()
	fmt.Println(gotext.Get("    --aururl      <url>   Set an alternative AUR URL"))
	fmt.Println(gotext.Get("    --builddir    <dir>   Directory used to download and run PKGBUILDS"))
	fmt.Println(gotext.Get("    --absdir      <dir>   Directory used to store downloads from the ABS"))
	fmt.Println(gotext.Get("    --editor      <file>  Editor to use when editing PKGBUILDs"))
	fmt.Println(gotext.Get("    --editorflags <flags> Pass arguments to editor"))
	fmt.Println(gotext.Get("    --makepkg     <file>  makepkg command to use"))
	fmt.Println(gotext.Get("    --mflags      <flags> Pass arguments to makepkg"))
	fmt.Println(gotext.Get("    --pacman      <file>  pacman command to use"))
	fmt.Println(gotext.Get("    --git         <file>  git command to use"))
	fmt.Println(gotext.Get("    --gitflags    <flags> Pass arguments to git"))
	fmt.Println(gotext.Get("    --gpg         <file>  gpg command to use"))
	fmt.Println(gotext.Get("    --gpgflags    <flags> Pass arguments to gpg"))
	fmt.Println(gotext.Get("    --config      <file>  pacman.conf file to use"))
	fmt.Println(gotext.Get("    --makepkgconf <file>  makepkg.conf file to use"))
	fmt.Println(gotext.Get("    --nomakepkgconf       Use the default makepkg.conf"))
	fmt.Println()
	fmt.Println(gotext.Get("    --requestsplitn <n>   Max amount of packages to query per AUR request"))
	fmt.Println(gotext.Get("    --completioninterval  <n> Time in days to refresh completion cache"))
	fmt.Println(gotext.Get("    --sortby    <field>   Sort AUR results by a specific field during search"))
	fmt.Println(gotext.Get("    --searchby  <field>   Search for packages using a specified field"))
	fmt.Println(gotext.Get("    --answerclean   <a>   Set a predetermined answer for the clean build menu"))
	fmt.Println(gotext.Get("    --answerdiff    <a>   Set a predetermined answer for the diff menu"))
	fmt.Println(gotext.Get("    --answeredit    <a>   Set a predetermined answer for the edit pkgbuild menu"))
	fmt.Println(gotext.Get("    --answerupgrade <a>   Set a predetermined answer for the upgrade menu"))
	fmt.Println(gotext.Get("    --noanswerclean       Unset the answer for the clean build menu"))
	fmt.Println(gotext.Get("    --noanswerdiff        Unset the answer for the edit diff menu"))
	fmt.Println(gotext.Get("    --noansweredit        Unset the answer for the edit pkgbuild menu"))
	fmt.Println(gotext.Get("    --noanswerupgrade     Unset the answer for the upgrade menu"))
	fmt.Println(gotext.Get("    --cleanmenu           Give the option to clean build PKGBUILDS"))
	fmt.Println(gotext.Get("    --diffmenu            Give the option to show diffs for build files"))
	fmt.Println(gotext.Get("    --editmenu            Give the option to edit/view PKGBUILDS"))
	fmt.Println(gotext.Get("    --upgrademenu         Show a detailed list of updates with the option to skip any"))
	fmt.Println(gotext.Get("    --nocleanmenu         Don't clean build PKGBUILDS"))
	fmt.Println(gotext.Get("    --nodiffmenu          Don't show diffs for build files"))
	fmt.Println(gotext.Get("    --noeditmenu          Don't edit/view PKGBUILDS"))
	fmt.Println(gotext.Get("    --noupgrademenu       Don't show the upgrade menu"))
	fmt.Println(gotext.Get("    --askremovemake       Ask to remove makedepends after install"))
	fmt.Println(gotext.Get("    --removemake          Remove makedepends after install"))
	fmt.Println(gotext.Get("    --noremovemake        Don't remove makedepends after install"))
	fmt.Println()
	fmt.Println(gotext.Get("    --cleanafter          Remove package sources after successful install"))
	fmt.Println(gotext.Get("    --nocleanafter        Do not remove package sources after successful build"))
	fmt.Println(gotext.Get("    --bottomup            Shows AUR's packages first and then repository's"))
	fmt.Println(gotext.Get("    --topdown             Shows repository's packages first and then AUR's"))
	fmt.Println()
	fmt.Println(gotext.Get("    --devel               Check development packages during sysupgrade"))
	fmt.Println(gotext.Get("    --nodevel             Do not check development packages"))
	fmt.Println(gotext.Get("    --rebuild             Always build target packages"))
	fmt.Println(gotext.Get("    --rebuildall          Always build all AUR packages"))
	fmt.Println(gotext.Get("    --norebuild           Skip package build if in cache and up to date"))
	fmt.Println(gotext.Get("    --rebuildtree         Always build all AUR packages even if installed"))
	fmt.Println(gotext.Get("    --redownload          Always download pkgbuilds of targets"))
	fmt.Println(gotext.Get("    --noredownload        Skip pkgbuild download if in cache and up to date"))
	fmt.Println(gotext.Get("    --redownloadall       Always download pkgbuilds of all AUR packages"))
	fmt.Println(gotext.Get("    --provides            Look for matching providers when searching for packages"))
	fmt.Println(gotext.Get("    --noprovides          Just look for packages by pkgname"))
	fmt.Println(gotext.Get("    --pgpfetch            Prompt to import PGP keys from PKGBUILDs"))
	fmt.Println(gotext.Get("    --nopgpfetch          Don't prompt to import PGP keys"))
	fmt.Println(gotext.Get("    --useask              Automatically resolve conflicts using pacman's ask flag"))
	fmt.Println(gotext.Get("    --nouseask            Confirm conflicts manually during the install"))
	fmt.Println(gotext.Get("    --combinedupgrade     Refresh then perform the repo and AUR upgrade together"))
	fmt.Println(gotext.Get("    --nocombinedupgrade   Perform the repo upgrade and AUR upgrade separately"))
	fmt.Println(gotext.Get("    --batchinstall        Build multiple AUR packages then install them together"))
	fmt.Println(gotext.Get("    --nobatchinstall      Build and install each AUR package one by one"))
	fmt.Println()
	fmt.Println(gotext.Get("    --sudo                <file>  sudo command to use"))
	fmt.Println(gotext.Get("    --sudoflags           <flags> Pass arguments to sudo"))
	fmt.Println(gotext.Get("    --sudoloop            Loop sudo calls in the background to avoid timeout"))
	fmt.Println(gotext.Get("    --nosudoloop          Do not loop sudo calls in the background"))
	fmt.Println()
	fmt.Println(gotext.Get("    --timeupdate          Check packages' AUR page for changes during sysupgrade"))
	fmt.Println(gotext.Get("    --notimeupdate        Do not check packages' AUR page for changes"))
	fmt.Println()
	fmt.Println(gotext.Get("show specific options:"))
	fmt.Println(gotext.Get("    -c --complete         Used for completions"))
	fmt.Println(gotext.Get("    -d --defaultconfig    Print default yay configuration"))
	fmt.Println(gotext.Get("    -g --currentconfig    Print current yay configuration"))
	fmt.Println(gotext.Get("    -s --stats            Display system package statistics"))
	fmt.Println(gotext.Get("    -w --news             Print arch news"))
	fmt.Println()
	fmt.Println(gotext.Get("yay specific options:"))
	fmt.Println(gotext.Get("    -c --clean            Remove unneeded dependencies"))
	fmt.Println(gotext.Get("       --gendb            Generates development package DB used for updating"))
	fmt.Println()
	fmt.Println(gotext.Get("getpkgbuild specific options:"))
	fmt.Println(gotext.Get("    -f --force            Force download for existing ABS packages"))
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
