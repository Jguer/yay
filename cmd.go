package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"

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

func opHelp(operation string, args ...string) string {
	newArgs := make([]string, 0, len(args))
	for _, arg := range args {
		newArgs = append(newArgs, fmt.Sprintf("[%s]", arg))
	}
	return fmt.Sprintf("    yay %-18s %s", operation, strings.Join(newArgs, " "))
}

func opHlp2(operation string, args ...string) string {
	newArgs := make([]string, 0, len(args))
	for _, arg := range args {
		newArgs = append(newArgs, fmt.Sprintf("<%s>", arg))
	}
	return fmt.Sprintf("    yay %-18s %s", operation, strings.Join(newArgs, " "))
}

func optionHelp(option, description string, args ...string) string {
	newArgs := make([]string, 0, len(args))
	for _, arg := range args {
		newArgs = append(newArgs, fmt.Sprintf("<%s>", arg))
	}
	return fmt.Sprintf("    %-18s %-2s %s", option, strings.Join(newArgs, " "), description)
}

func optionHlp2(option, description string, args ...string) string {
	newArgs := make([]string, 0, len(args))
	for _, arg := range args {
		newArgs = append(newArgs, fmt.Sprintf("<%s>", arg))
	}
	return fmt.Sprintf("    %-13s %-7s %s", option, strings.Join(newArgs, " "), description)
}

func optionHlp3(option, description string, args ...string) string {
	newArgs := make([]string, 0, len(args))
	for _, arg := range args {
		newArgs = append(newArgs, fmt.Sprintf("<%s>", arg))
	}
	return fmt.Sprintf("    %-15s %-5s %s", option, strings.Join(newArgs, " "), description)
}

func usage() {
	fmt.Print(gotext.Get("Usage"), ":\n    yay\n")
	fmt.Printf("    yay <%s> [...]\n", gotext.Get("operation"))
	fmt.Printf("    yay <%s>\n", gotext.Get("package(s)"))

	fmt.Print("\n", gotext.Get("operations"), ":\n")
	fmt.Println(opHelp("{-h --help}"))
	fmt.Println(opHelp("{-V --version}"))
	fmt.Println(opHlp2("{-D --database}", gotext.Get("options"), gotext.Get("package(s)")))
	fmt.Println(opHelp("{-F --files}", gotext.Get("options"), gotext.Get("package(s)")))
	fmt.Println(opHelp("{-Q --query}", gotext.Get("options"), gotext.Get("package(s)")))
	fmt.Println(opHelp("{-R --remove}", gotext.Get("options")), "<"+gotext.Get("package(s)")+">")
	fmt.Println(opHelp("{-S --sync}", gotext.Get("options"), gotext.Get("package(s)")))
	fmt.Println(opHelp("{-T --deptest}", gotext.Get("options"), gotext.Get("package(s)")))
	fmt.Println(opHelp("{-U --upgrade}", gotext.Get("options")), "<"+gotext.Get("file(s)")+">")

	fmt.Print("\n", gotext.Get("New operations"), ":\n")
	fmt.Println(opHelp("{-G --getpkgbuild}", gotext.Get("package(s)")))
	fmt.Println(opHelp("{-P --show}", gotext.Get("options")))
	fmt.Println(opHelp("{-Y --yay}", gotext.Get("options"), gotext.Get("package(s)")))

	fmt.Println()
	fmt.Println(gotext.Get("If no arguments are provided 'yay -Syu' will be performed"))
	fmt.Println(gotext.Get("If no operation is provided -Y will be assumed"))

	fmt.Print("\n", gotext.Get("New options"), ":\n")
	fmt.Println(optionHelp("--repo", gotext.Get("Assume targets are from the repositories")))
	fmt.Println(optionHelp("-a --aur", gotext.Get("Assume targets are from the AUR")))

	fmt.Print("\n", gotext.Get("Permanent configuration options"), ":\n")
	fmt.Println(optionHelp("--save", gotext.Get("Causes the following options to be saved back to the")))
	fmt.Println(optionHelp("      ", gotext.Get("config file when used")))

	fmt.Println()
	fmt.Println(optionHlp2("--aururl", gotext.Get("Set an alternative AUR URL"), gotext.Get("url")))
	fmt.Println(optionHlp2("--builddir", gotext.Get("Directory used to download and run PKGBUILDS"), gotext.Get("dir")))
	fmt.Println(optionHlp2("--absdir", gotext.Get("Directory used to store downloads from the ABS"), gotext.Get("dir")))
	fmt.Println(optionHlp2("--editor", gotext.Get("Editor to use when editing PKGBUILDs"), gotext.Get("file")))
	fmt.Println(optionHlp2("--editorflags", gotext.Get("Pass arguments to editor"), gotext.Get("flags")))
	fmt.Println(optionHlp2("--makepkg", gotext.Get("makepkg command to use"), gotext.Get("file")))
	fmt.Println(optionHlp2("--mflags", gotext.Get("Pass arguments to makepkg"), gotext.Get("flags")))
	fmt.Println(optionHlp2("--pacman", gotext.Get("pacman command to use"), gotext.Get("file")))
	fmt.Println(optionHlp2("--git", gotext.Get("git command to use"), gotext.Get("file")))
	fmt.Println(optionHlp2("--gitflags", gotext.Get("Pass arguments to git"), gotext.Get("flags")))
	fmt.Println(optionHlp2("--gpg", gotext.Get("gpg command to use"), gotext.Get("file")))
	fmt.Println(optionHlp2("--gpgflags", gotext.Get("Pass arguments to gpg"), gotext.Get("flags")))
	fmt.Println(optionHlp2("--config", gotext.Get("pacman.conf file to use"), gotext.Get("file")))
	fmt.Println(optionHlp2("--makepkgconf", gotext.Get("makepkg.conf file to use"), gotext.Get("file")))
	fmt.Println(optionHelp("--nomakepkgconf", gotext.Get("Use the default makepkg.conf")))
	fmt.Println(optionHlp3("--requestsplitn", gotext.Get("Max amount of packages to query per AUR request"), "n"))
	fmt.Println(optionHelp("--completioninterval ", gotext.Get("Time in days to refresh completion cache"), "n"))
	fmt.Println(optionHlp2("--sortby", gotext.Get("Sort AUR results by a specific field during search"), gotext.Get("field")))
	fmt.Println(optionHlp2("--searchby", gotext.Get("Search for packages using a specified field"), gotext.Get("field")))
	fmt.Println(optionHlp3("--answerclean", gotext.Get("Set a predetermined answer for the clean build menu"), "a"))
	fmt.Println(optionHlp3("--answerdiff", gotext.Get("Set a predetermined answer for the diff menu"), "a"))
	fmt.Println(optionHlp3("--answeredit", gotext.Get("Set a predetermined answer for the edit pkgbuild menu"), "a"))
	fmt.Println(optionHlp3("--answerupgrade", gotext.Get("Set a predetermined answer for the upgrade menu"), "a"))
	fmt.Println(optionHelp("--noanswerclean", gotext.Get("Unset the answer for the clean build menu")))
	fmt.Println(optionHelp("--noanswerdiff", gotext.Get("Unset the answer for the edit diff menu")))
	fmt.Println(optionHelp("--noansweredit", gotext.Get("Unset the answer for the edit pkgbuild menu")))
	fmt.Println(optionHelp("--noanswerupgrade", gotext.Get("Unset the answer for the upgrade menu")))
	fmt.Println(optionHelp("--cleanmenu", gotext.Get("Give the option to clean build PKGBUILDS")))
	fmt.Println(optionHelp("--diffmenu", gotext.Get("Give the option to show diffs for build files")))
	fmt.Println(optionHelp("--editmenu", gotext.Get("Give the option to edit/view PKGBUILDS")))
	fmt.Println(optionHelp("--upgrademenu", gotext.Get("Show a detailed list of updates with the option to skip any")))
	fmt.Println(optionHelp("--nocleanmenu", gotext.Get("Don't clean build PKGBUILDS")))
	fmt.Println(optionHelp("--nodiffmenu", gotext.Get("Don't show diffs for build files")))
	fmt.Println(optionHelp("--noeditmenu", gotext.Get("Don't edit/view PKGBUILDS")))
	fmt.Println(optionHelp("--noupgrademenu", gotext.Get("Don't show the upgrade menu")))
	fmt.Println(optionHelp("--askremovemake", gotext.Get("Ask to remove makedepends after install")))
	fmt.Println(optionHelp("--removemake", gotext.Get("Remove makedepends after install")))
	fmt.Println(optionHelp("--noremovemake", gotext.Get("Don't remove makedepends after install")))
	fmt.Println(optionHelp("--cleanafter", gotext.Get("Remove package sources after successful install")))
	fmt.Println(optionHelp("--nocleanafter", gotext.Get("Do not remove package sources after successful build")))
	fmt.Println(optionHelp("--bottomup", gotext.Get("Shows AUR's packages first and then repository's")))
	fmt.Println(optionHelp("--topdown", gotext.Get("Shows repository's packages first and then AUR's")))
	fmt.Println(optionHelp("--devel", gotext.Get("Check development packages during sysupgrade")))
	fmt.Println(optionHelp("--nodevel", gotext.Get("Do not check development packages")))
	fmt.Println(optionHelp("--rebuild", gotext.Get("Always build target packages")))
	fmt.Println(optionHelp("--rebuildall", gotext.Get("Always build all AUR packages")))
	fmt.Println(optionHelp("--norebuild", gotext.Get("Skip package build if in cache and up to date")))
	fmt.Println(optionHelp("--rebuildtree", gotext.Get("Always build all AUR packages even if installed")))
	fmt.Println(optionHelp("--redownload", gotext.Get("Always download pkgbuilds of targets")))
	fmt.Println(optionHelp("--noredownload", gotext.Get("Skip pkgbuild download if in cache and up to date")))
	fmt.Println(optionHelp("--redownloadall", gotext.Get("Always download pkgbuilds of all AUR packages")))
	fmt.Println(optionHelp("--provides", gotext.Get("Look for matching providers when searching for packages")))
	fmt.Println(optionHelp("--noprovides", gotext.Get("Just look for packages by pkgname")))
	fmt.Println(optionHelp("--pgpfetch", gotext.Get("Prompt to import PGP keys from PKGBUILDs")))
	fmt.Println(optionHelp("--nopgpfetch", gotext.Get("Don't prompt to import PGP keys")))
	fmt.Println(optionHelp("--useask", gotext.Get("Automatically resolve conflicts using pacman's ask flag")))
	fmt.Println(optionHelp("--nouseask", gotext.Get("Confirm conflicts manually during the install")))
	fmt.Println(optionHelp("--combinedupgrade", gotext.Get("Refresh then perform the repo and AUR upgrade together")))
	fmt.Println(optionHelp("--batchinstall", gotext.Get("Build multiple AUR packages then install them together")))
	fmt.Println(optionHelp("--nobatchinstall", gotext.Get("Build and install each AUR package one by one")))
	fmt.Println(optionHlp2("--sudo", gotext.Get("sudo command to use"), gotext.Get("file")))
	fmt.Println(optionHlp2("--sudoflags", gotext.Get("Pass arguments to sudo"), gotext.Get("flags")))
	fmt.Println(optionHelp("--sudoloop", gotext.Get("Loop sudo calls in the background to avoid timeout")))
	fmt.Println(optionHelp("--nosudoloop", gotext.Get("Do not loop sudo calls in the background")))
	fmt.Println(optionHelp("--timeupdate", gotext.Get("Check packages' AUR page for changes during sysupgrade")))
	fmt.Println(optionHelp("--notimeupdate", gotext.Get("Do not check packages' AUR page for changes")))

	fmt.Print("\n", gotext.Get("show specific options"), ":\n")
	fmt.Println(optionHelp("-c --complete", gotext.Get("Used for completions")))
	fmt.Println(optionHelp("-d --defaultconfig", gotext.Get("Print default yay configuration")))
	fmt.Println(optionHelp("-g --currentconfig", gotext.Get("Print current yay configuration")))
	fmt.Println(optionHelp("-s --stats", gotext.Get("Display system package statistics")))
	fmt.Println(optionHelp("-w --news", gotext.Get("Print arch news")))

	fmt.Print("\n", gotext.Get("yay specific options"), ":\n")
	fmt.Println(optionHelp("-c --clean", gotext.Get("Remove unneeded dependencies")))
	fmt.Println(optionHelp("--gendb", gotext.Get("Generates development package DB used for updating")))

	fmt.Print("\n", gotext.Get("getpkgbuild specific options"), ":\n")
	fmt.Println(optionHelp("-f --force", gotext.Get("Force download for existing ABS packages")))
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
