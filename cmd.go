package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	alpm "github.com/Jguer/go-alpm/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/completion"
	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/download"
	"github.com/Jguer/yay/v12/pkg/intrange"
	"github.com/Jguer/yay/v12/pkg/news"
	"github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/upgrade"
	"github.com/Jguer/yay/v12/pkg/vcs"
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
    yay {-B --build}       [options] [dir]
    yay {-G --getpkgbuild} [options] [package(s)]
    yay {-P --show}        [options]
    yay {-W --web}         [options] [package(s)]
    yay {-Y --yay}         [options] [package(s)]

If no operation is specified 'yay -Syu' will be performed
If no operation and no targets are provided -Y will be assumed

New options:
       --repo             Assume targets are from the repositories
    -a --aur              Assume targets are from the AUR

Permanent configuration options:
    --save                Causes the following options to be saved back to the
                          config file when used

    --aururl      <url>   Set an alternative AUR URL
    --aurrpcurl   <url>   Set an alternative URL for the AUR /rpc endpoint
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
    --singlelineresults   List each search result on its own line
    --doublelineresults   List each search result on two lines, like pacman

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

func handleCmd(ctx context.Context, cfg *settings.Configuration,
	cmdArgs *parser.Arguments, dbExecutor db.Executor,
) error {
	if cmdArgs.ExistsArg("h", "help") {
		return handleHelp(ctx, cfg, cmdArgs)
	}

	if cfg.SudoLoop && cmdArgs.NeedRoot(cfg.Mode) {
		cfg.Runtime.CmdBuilder.SudoLoop()
	}

	switch cmdArgs.Op {
	case "V", "version":
		handleVersion()

		return nil
	case "D", "database":
		return cfg.Runtime.CmdBuilder.Show(cfg.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, cfg.Mode, settings.NoConfirm))
	case "F", "files":
		return cfg.Runtime.CmdBuilder.Show(cfg.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, cfg.Mode, settings.NoConfirm))
	case "Q", "query":
		return handleQuery(ctx, cfg, cmdArgs, dbExecutor)
	case "R", "remove":
		return handleRemove(ctx, cfg, cmdArgs, cfg.Runtime.VCSStore)
	case "S", "sync":
		return handleSync(ctx, cfg, cmdArgs, dbExecutor)
	case "T", "deptest":
		return cfg.Runtime.CmdBuilder.Show(cfg.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, cfg.Mode, settings.NoConfirm))
	case "U", "upgrade":
		return handleUpgrade(ctx, cfg, cmdArgs)
	case "B", "build":
		return handleBuild(ctx, cfg, dbExecutor, cmdArgs)
	case "G", "getpkgbuild":
		return handleGetpkgbuild(ctx, cfg, cmdArgs, dbExecutor)
	case "P", "show":
		return handlePrint(ctx, cfg, cmdArgs, dbExecutor)
	case "Y", "yay":
		return handleYay(ctx, cfg, cmdArgs, cfg.Runtime.CmdBuilder,
			dbExecutor, cfg.Runtime.QueryBuilder)
	case "W", "web":
		return handleWeb(ctx, cfg, cmdArgs)
	}

	return errors.New(gotext.Get("unhandled operation"))
}

// getFilter returns filter function which can keep packages which were only
// explicitly installed or ones installed as dependencies for showing available
// updates or their count.
func getFilter(cmdArgs *parser.Arguments) (upgrade.Filter, error) {
	deps, explicit := cmdArgs.ExistsArg("d", "deps"), cmdArgs.ExistsArg("e", "explicit")

	switch {
	case deps && explicit:
		return nil, errors.New(gotext.Get("invalid option: '--deps' and '--explicit' may not be used together"))
	case deps:
		return func(pkg *upgrade.Upgrade) bool {
			return pkg.Reason == alpm.PkgReasonDepend
		}, nil
	case explicit:
		return func(pkg *upgrade.Upgrade) bool {
			return pkg.Reason == alpm.PkgReasonExplicit
		}, nil
	}

	return func(pkg *upgrade.Upgrade) bool {
		return true
	}, nil
}

func handleQuery(ctx context.Context, cfg *settings.Configuration, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	if cmdArgs.ExistsArg("u", "upgrades") {
		filter, err := getFilter(cmdArgs)
		if err != nil {
			return err
		}

		return printUpdateList(ctx, cfg, cmdArgs, dbExecutor,
			cmdArgs.ExistsDouble("u", "sysupgrade"), filter)
	}

	if err := cfg.Runtime.CmdBuilder.Show(cfg.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, cfg.Mode, settings.NoConfirm)); err != nil {
		if str := err.Error(); strings.Contains(str, "exit status") {
			// yay -Qdt should not output anything in case of error
			return fmt.Errorf("")
		}

		return err
	}

	return nil
}

func handleHelp(ctx context.Context, cfg *settings.Configuration, cmdArgs *parser.Arguments) error {
	usage()
	switch cmdArgs.Op {
	case "Y", "yay", "G", "getpkgbuild", "P", "show", "W", "web", "B", "build":
		return nil
	}

	cfg.Runtime.Logger.Println("\npacman operation specific options:")
	return cfg.Runtime.CmdBuilder.Show(cfg.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, cfg.Mode, settings.NoConfirm))
}

func handleVersion() {
	fmt.Printf("yay v%s - libalpm v%s\n", yayVersion, alpm.Version())
}

func handlePrint(ctx context.Context, cfg *settings.Configuration, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	switch {
	case cmdArgs.ExistsArg("d", "defaultconfig"):
		tmpConfig := settings.DefaultConfig(yayVersion)
		fmt.Printf("%v", tmpConfig)

		return nil
	case cmdArgs.ExistsArg("g", "currentconfig"):
		fmt.Printf("%v", cfg)

		return nil
	case cmdArgs.ExistsArg("w", "news"):
		double := cmdArgs.ExistsDouble("w", "news")
		quiet := cmdArgs.ExistsArg("q", "quiet")

		return news.PrintNewsFeed(ctx, cfg.Runtime.HTTPClient, dbExecutor.LastBuildTime(), cfg.BottomUp, double, quiet)
	case cmdArgs.ExistsArg("c", "complete"):
		return completion.Show(ctx, cfg.Runtime.HTTPClient, dbExecutor,
			cfg.AURURL, cfg.CompletionPath, cfg.CompletionInterval, cmdArgs.ExistsDouble("c", "complete"))
	case cmdArgs.ExistsArg("s", "stats"):
		return localStatistics(ctx, cfg, dbExecutor)
	}

	return nil
}

func handleYay(ctx context.Context, cfg *settings.Configuration,
	cmdArgs *parser.Arguments, cmdBuilder exe.ICmdBuilder,
	dbExecutor db.Executor, queryBuilder query.Builder,
) error {
	switch {
	case cmdArgs.ExistsArg("gendb"):
		return createDevelDB(ctx, cfg, dbExecutor)
	case cmdArgs.ExistsDouble("c"):
		return cleanDependencies(ctx, cfg, cmdBuilder, cmdArgs, dbExecutor, true)
	case cmdArgs.ExistsArg("c", "clean"):
		return cleanDependencies(ctx, cfg, cmdBuilder, cmdArgs, dbExecutor, false)
	case len(cmdArgs.Targets) > 0:
		return displayNumberMenu(ctx, cfg, cmdArgs.Targets, dbExecutor, queryBuilder, cmdArgs)
	}

	return nil
}

func handleWeb(ctx context.Context, cfg *settings.Configuration, cmdArgs *parser.Arguments) error {
	switch {
	case cmdArgs.ExistsArg("v", "vote"):
		return handlePackageVote(ctx, cmdArgs.Targets, cfg.Runtime.AURClient,
			cfg.Runtime.VoteClient, cfg.RequestSplitN, true)
	case cmdArgs.ExistsArg("u", "unvote"):
		return handlePackageVote(ctx, cmdArgs.Targets, cfg.Runtime.AURClient,
			cfg.Runtime.VoteClient, cfg.RequestSplitN, false)
	}

	return nil
}

func handleGetpkgbuild(ctx context.Context, cfg *settings.Configuration, cmdArgs *parser.Arguments, dbExecutor download.DBSearcher) error {
	if cmdArgs.ExistsArg("p", "print") {
		return printPkgbuilds(dbExecutor, cfg.Runtime.AURCache,
			cfg.Runtime.HTTPClient, cmdArgs.Targets, cfg.Mode, cfg.AURURL)
	}

	return getPkgbuilds(ctx, dbExecutor, cfg.Runtime.AURCache, cfg,
		cmdArgs.Targets, cmdArgs.ExistsArg("f", "force"))
}

func handleUpgrade(ctx context.Context,
	config *settings.Configuration, cmdArgs *parser.Arguments,
) error {
	return config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, config.Mode, settings.NoConfirm))
}

// -B* options
func handleBuild(ctx context.Context,
	config *settings.Configuration, dbExecutor db.Executor, cmdArgs *parser.Arguments,
) error {
	if cmdArgs.ExistsArg("i", "install") {
		return installLocalPKGBUILD(ctx, config, cmdArgs, dbExecutor)
	}

	return nil
}

func handleSync(ctx context.Context, cfg *settings.Configuration, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	targets := cmdArgs.Targets

	switch {
	case cmdArgs.ExistsArg("s", "search"):
		return syncSearch(ctx, targets, dbExecutor, cfg.Runtime.QueryBuilder, !cmdArgs.ExistsArg("q", "quiet"))
	case cmdArgs.ExistsArg("p", "print", "print-format"):
		return cfg.Runtime.CmdBuilder.Show(cfg.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, cfg.Mode, settings.NoConfirm))
	case cmdArgs.ExistsArg("c", "clean"):
		return syncClean(ctx, cfg, cmdArgs, dbExecutor)
	case cmdArgs.ExistsArg("l", "list"):
		return syncList(ctx, cfg, cfg.Runtime.HTTPClient, cmdArgs, dbExecutor)
	case cmdArgs.ExistsArg("g", "groups"):
		return cfg.Runtime.CmdBuilder.Show(cfg.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, cfg.Mode, settings.NoConfirm))
	case cmdArgs.ExistsArg("i", "info"):
		return syncInfo(ctx, cfg, cmdArgs, targets, dbExecutor)
	case cmdArgs.ExistsArg("u", "sysupgrade") || len(cmdArgs.Targets) > 0:
		if cfg.NewInstallEngine {
			return syncInstall(ctx, cfg, cmdArgs, dbExecutor)
		}

		return install(ctx, cfg, cmdArgs, dbExecutor, false)
	case cmdArgs.ExistsArg("y", "refresh"):
		return cfg.Runtime.CmdBuilder.Show(cfg.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, cfg.Mode, settings.NoConfirm))
	}

	return nil
}

func handleRemove(ctx context.Context, cfg *settings.Configuration, cmdArgs *parser.Arguments, localCache vcs.Store) error {
	err := cfg.Runtime.CmdBuilder.Show(cfg.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, cfg.Mode, settings.NoConfirm))
	if err == nil {
		localCache.RemovePackages(cmdArgs.Targets)
	}

	return err
}

// NumberMenu presents a CLI for selecting packages to install.
func displayNumberMenu(ctx context.Context, cfg *settings.Configuration, pkgS []string, dbExecutor db.Executor,
	queryBuilder query.Builder, cmdArgs *parser.Arguments,
) error {
	queryBuilder.Execute(ctx, dbExecutor, pkgS)

	if err := queryBuilder.Results(dbExecutor, query.NumberMenu); err != nil {
		return err
	}

	if queryBuilder.Len() == 0 {
		// no results were found
		return nil
	}

	text.Infoln(gotext.Get("Packages to install (eg: 1 2 3, 1-3 or ^4)"))

	numberBuf, err := text.GetInput(os.Stdin, "", false)
	if err != nil {
		return err
	}

	include, exclude, _, otherExclude := intrange.ParseNumberMenu(numberBuf)

	targets, err := queryBuilder.GetTargets(include, exclude, otherExclude)
	if err != nil {
		return err
	}

	// modify the arguments to pass for the install
	cmdArgs.Op = "S"
	cmdArgs.Targets = targets

	if len(cmdArgs.Targets) == 0 {
		fmt.Println(gotext.Get(" there is nothing to do"))
		return nil
	}

	if cfg.NewInstallEngine {
		return syncInstall(ctx, cfg, cmdArgs, dbExecutor)
	}

	return install(ctx, cfg, cmdArgs, dbExecutor, true)
}

func syncList(ctx context.Context, cfg *settings.Configuration,
	httpClient *http.Client, cmdArgs *parser.Arguments, dbExecutor db.Executor,
) error {
	aur := false

	for i := len(cmdArgs.Targets) - 1; i >= 0; i-- {
		if cmdArgs.Targets[i] == "aur" && cfg.Mode.AtLeastAUR() {
			cmdArgs.Targets = append(cmdArgs.Targets[:i], cmdArgs.Targets[i+1:]...)
			aur = true
		}
	}

	if cfg.Mode.AtLeastAUR() && (len(cmdArgs.Targets) == 0 || aur) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.AURURL+"/packages.gz", http.NoBody)
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

	if cfg.Mode.AtLeastRepo() && (len(cmdArgs.Targets) != 0 || !aur) {
		return cfg.Runtime.CmdBuilder.Show(cfg.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, cfg.Mode, settings.NoConfirm))
	}

	return nil
}
