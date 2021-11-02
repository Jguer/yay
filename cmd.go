package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"

	alpm "github.com/Jguer/go-alpm/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/completion"
	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/download"
	"github.com/Jguer/yay/v11/pkg/intrange"
	"github.com/Jguer/yay/v11/pkg/news"
	"github.com/Jguer/yay/v11/pkg/query"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/text"
	"github.com/Jguer/yay/v11/pkg/upgrade"
	"github.com/Jguer/yay/v11/pkg/vcs"
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
    yay {-P --show}        [options]
    yay {-G --getpkgbuild} [options] [package(s)]

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

yay specific options:
    -c --clean            Remove unneeded dependencies
       --gendb            Generates development package DB used for updating

getpkgbuild specific options:
    -f --force            Force download for existing ABS packages
    -p --print            Print pkgbuild of packages`)
}

func handleCmd(ctx context.Context, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	if cmdArgs.ExistsArg("h", "help") {
		return handleHelp(ctx, cmdArgs)
	}

	if config.SudoLoop && cmdArgs.NeedRoot(config.Runtime.Mode) {
		config.Runtime.CmdBuilder.SudoLoop()
	}

	switch cmdArgs.Op {
	case "V", "version":
		handleVersion()

		return nil
	case "D", "database":
		return config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, config.Runtime.Mode, settings.NoConfirm))
	case "F", "files":
		return config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, config.Runtime.Mode, settings.NoConfirm))
	case "Q", "query":
		return handleQuery(ctx, cmdArgs, dbExecutor)
	case "R", "remove":
		return handleRemove(ctx, cmdArgs, config.Runtime.VCSStore)
	case "S", "sync":
		return handleSync(ctx, cmdArgs, dbExecutor)
	case "T", "deptest":
		return config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, config.Runtime.Mode, settings.NoConfirm))
	case "U", "upgrade":
		return config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, config.Runtime.Mode, settings.NoConfirm))
	case "G", "getpkgbuild":
		return handleGetpkgbuild(ctx, cmdArgs, dbExecutor)
	case "P", "show":
		return handlePrint(ctx, cmdArgs, dbExecutor)
	case "Y", "--yay":
		return handleYay(ctx, cmdArgs, dbExecutor)
	}

	return fmt.Errorf(gotext.Get("unhandled operation"))
}

// getFilter returns filter function which can keep packages which were only
// explicitly installed or ones installed as dependencies for showing available
// updates or their count.
func getFilter(cmdArgs *parser.Arguments) (upgrade.Filter, error) {
	deps, explicit := cmdArgs.ExistsArg("d", "deps"), cmdArgs.ExistsArg("e", "explicit")

	switch {
	case deps && explicit:
		return nil, fmt.Errorf(gotext.Get("invalid option: '--deps' and '--explicit' may not be used together"))
	case deps:
		return func(pkg upgrade.Upgrade) bool {
			return pkg.Reason == alpm.PkgReasonDepend
		}, nil
	case explicit:
		return func(pkg upgrade.Upgrade) bool {
			return pkg.Reason == alpm.PkgReasonExplicit
		}, nil
	}

	return func(pkg upgrade.Upgrade) bool {
		return true
	}, nil
}

func handleQuery(ctx context.Context, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	if cmdArgs.ExistsArg("u", "upgrades") {
		filter, err := getFilter(cmdArgs)
		if err != nil {
			return err
		}

		return printUpdateList(ctx, cmdArgs, dbExecutor, cmdArgs.ExistsDouble("u", "sysupgrade"), filter)
	}

	if err := config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, config.Runtime.Mode, settings.NoConfirm)); err != nil {
		if str := err.Error(); strings.Contains(str, "exit status") {
			// yay -Qdt should not output anything in case of error
			return fmt.Errorf("")
		}

		return err
	}

	return nil
}

func handleHelp(ctx context.Context, cmdArgs *parser.Arguments) error {
	switch cmdArgs.Op {
	case "Y", "yay", "G", "getpkgbuild", "P", "show":
		usage()
		return nil
	}

	return config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, config.Runtime.Mode, settings.NoConfirm))
}

func handleVersion() {
	fmt.Printf("yay v%s - libalpm v%s\n", yayVersion, alpm.Version())
}

func handlePrint(ctx context.Context, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	switch {
	case cmdArgs.ExistsArg("d", "defaultconfig"):
		tmpConfig := settings.DefaultConfig()
		fmt.Printf("%v", tmpConfig)

		return nil
	case cmdArgs.ExistsArg("g", "currentconfig"):
		fmt.Printf("%v", config)

		return nil
	case cmdArgs.ExistsArg("n", "numberupgrades"):
		filter, err := getFilter(cmdArgs)
		if err != nil {
			return err
		}

		return printNumberOfUpdates(ctx, dbExecutor, cmdArgs.ExistsDouble("u", "sysupgrade"), filter)
	case cmdArgs.ExistsArg("w", "news"):
		double := cmdArgs.ExistsDouble("w", "news")
		quiet := cmdArgs.ExistsArg("q", "quiet")

		return news.PrintNewsFeed(ctx, config.Runtime.HTTPClient, dbExecutor.LastBuildTime(), config.SortMode, double, quiet)
	case cmdArgs.ExistsDouble("c", "complete"):
		return completion.Show(ctx, config.Runtime.HTTPClient, dbExecutor,
			config.AURURL, config.Runtime.CompletionPath, config.CompletionInterval, true)
	case cmdArgs.ExistsArg("c", "complete"):
		return completion.Show(ctx, config.Runtime.HTTPClient, dbExecutor,
			config.AURURL, config.Runtime.CompletionPath, config.CompletionInterval, false)
	case cmdArgs.ExistsArg("s", "stats"):
		return localStatistics(ctx, dbExecutor)
	}

	return nil
}

func handleYay(ctx context.Context, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	switch {
	case cmdArgs.ExistsArg("gendb"):
		return createDevelDB(ctx, config, dbExecutor)
	case cmdArgs.ExistsDouble("c"):
		return cleanDependencies(ctx, cmdArgs, dbExecutor, true)
	case cmdArgs.ExistsArg("c", "clean"):
		return cleanDependencies(ctx, cmdArgs, dbExecutor, false)
	case len(cmdArgs.Targets) > 0:
		return handleYogurt(ctx, cmdArgs, dbExecutor)
	}

	return nil
}

func handleGetpkgbuild(ctx context.Context, cmdArgs *parser.Arguments, dbExecutor download.DBSearcher) error {
	if cmdArgs.ExistsArg("p", "print") {
		return printPkgbuilds(dbExecutor, config.Runtime.HTTPClient, cmdArgs.Targets, config.Runtime.Mode, config.AURURL)
	}

	return getPkgbuilds(ctx, dbExecutor, config, cmdArgs.Targets, cmdArgs.ExistsArg("f", "force"))
}

func handleYogurt(ctx context.Context, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	return displayNumberMenu(ctx, cmdArgs.Targets, dbExecutor, cmdArgs)
}

func handleSync(ctx context.Context, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	targets := cmdArgs.Targets

	switch {
	case cmdArgs.ExistsArg("s", "search"):
		return syncSearch(ctx, targets, config.Runtime.AURClient, dbExecutor, !cmdArgs.ExistsArg("q", "quiet"))
	case cmdArgs.ExistsArg("p", "print", "print-format"):
		return config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, config.Runtime.Mode, settings.NoConfirm))
	case cmdArgs.ExistsArg("c", "clean"):
		return syncClean(ctx, cmdArgs, dbExecutor)
	case cmdArgs.ExistsArg("l", "list"):
		return syncList(ctx, config.Runtime.HTTPClient, cmdArgs, dbExecutor)
	case cmdArgs.ExistsArg("g", "groups"):
		return config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, config.Runtime.Mode, settings.NoConfirm))
	case cmdArgs.ExistsArg("i", "info"):
		return syncInfo(ctx, cmdArgs, targets, dbExecutor)
	case cmdArgs.ExistsArg("u", "sysupgrade"):
		return install(ctx, cmdArgs, dbExecutor, false)
	case len(cmdArgs.Targets) > 0:
		return install(ctx, cmdArgs, dbExecutor, false)
	case cmdArgs.ExistsArg("y", "refresh"):
		return config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, config.Runtime.Mode, settings.NoConfirm))
	}

	return nil
}

func handleRemove(ctx context.Context, cmdArgs *parser.Arguments, localCache *vcs.InfoStore) error {
	err := config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, config.Runtime.Mode, settings.NoConfirm))
	if err == nil {
		localCache.RemovePackage(cmdArgs.Targets)
	}

	return err
}

// NumberMenu presents a CLI for selecting packages to install.
func displayNumberMenu(ctx context.Context, pkgS []string, dbExecutor db.Executor, cmdArgs *parser.Arguments) error {
	queryBuilder := query.NewSourceQueryBuilder(config.SortMode, config.SortBy, config.Runtime.Mode, config.SearchBy)

	queryBuilder.Execute(ctx, dbExecutor, config.Runtime.AURClient, pkgS)

	if err := queryBuilder.Results(dbExecutor, query.NumberMenu); err != nil {
		return err
	}

	if queryBuilder.Len() == 0 {
		// no results were found
		return nil
	}

	text.Infoln(gotext.Get("Packages to install (eg: 1 2 3, 1-3 or ^4)"))

	numberBuf, err := text.GetInput("", false)
	if err != nil {
		return err
	}

	include, exclude, _, otherExclude := intrange.ParseNumberMenu(numberBuf)

	targets, err := queryBuilder.GetTargets(include, exclude, otherExclude)
	if err != nil {
		return err
	}

	arguments := cmdArgs.CopyGlobal()
	arguments.AddTarget(targets...)

	if len(arguments.Targets) == 0 {
		fmt.Println(gotext.Get(" there is nothing to do"))
		return nil
	}

	return install(ctx, arguments, dbExecutor, true)
}

func syncList(ctx context.Context, httpClient *http.Client, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	aur := false

	for i := len(cmdArgs.Targets) - 1; i >= 0; i-- {
		if cmdArgs.Targets[i] == "aur" && config.Runtime.Mode.AtLeastAUR() {
			cmdArgs.Targets = append(cmdArgs.Targets[:i], cmdArgs.Targets[i+1:]...)
			aur = true
		}
	}

	if config.Runtime.Mode.AtLeastAUR() && (len(cmdArgs.Targets) == 0 || aur) {
		req, err := http.NewRequestWithContext(ctx, "GET", config.AURURL+"/packages.gz", nil)
		if err != nil {
			return err
		}

		resp, err := httpClient.Do(req)
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

	if config.Runtime.Mode.AtLeastRepo() && (len(cmdArgs.Targets) != 0 || !aur) {
		return config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, config.Runtime.Mode, settings.NoConfirm))
	}

	return nil
}
