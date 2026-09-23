package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	alpm "github.com/Jguer/dyalpm"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v13/pkg/completion"
	"github.com/Jguer/yay/v13/pkg/db"
	"github.com/Jguer/yay/v13/pkg/download"
	"github.com/Jguer/yay/v13/pkg/intrange"
	"github.com/Jguer/yay/v13/pkg/news"
	"github.com/Jguer/yay/v13/pkg/query"
	"github.com/Jguer/yay/v13/pkg/runtime"
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/settings/exe"
	"github.com/Jguer/yay/v13/pkg/settings/parser"
	"github.com/Jguer/yay/v13/pkg/text"
	"github.com/Jguer/yay/v13/pkg/upgrade"
	"github.com/Jguer/yay/v13/pkg/vcs"
)

type helpOption struct {
	flags       string
	description string
}

type operationHelp struct {
	short   string
	long    string
	targets string
	options []helpOption
}

type usageOption struct {
	flags       string
	description string
}

func yayOperationHelp() []operationHelp {
	strOptions := gotext.Get("options")
	strDir := gotext.Get("dir")
	strPackage := gotext.Get("package(s)")

	return []operationHelp{
		{"B", "build", fmt.Sprintf("[%s] [%s]", strOptions, strDir), []helpOption{
			{"-i --install", gotext.Get("Build and install a PKGBUILD")},
		}},
		{"G", "getpkgbuild", fmt.Sprintf("[%s] [%s]", strOptions, strPackage), []helpOption{
			{"-f --force", gotext.Get("Force download for existing ABS packages")},
			{"-p --print", gotext.Get("Print pkgbuild of packages")},
		}},
		{"P", "show", fmt.Sprintf("[%s]", strOptions), []helpOption{
			{"-c --complete", gotext.Get("Used for completions")},
			{"-d --defaultconfig", gotext.Get("Print default yay configuration")},
			{"-g --currentconfig", gotext.Get("Print current yay configuration")},
			{"-s --stats", gotext.Get("Display system package statistics")},
			{"-w --news", gotext.Get("Print arch news")},
			{"-q --quiet", gotext.Get("Only show titles when printing news")},
		}},
		{"W", "web", fmt.Sprintf("[%s] [%s]", strOptions, strPackage), []helpOption{
			{"-u --unvote", gotext.Get("Remove a vote from AUR package(s)")},
			{"-v --vote", gotext.Get("Vote for AUR package(s)")},
		}},
		{"Y", "yay", fmt.Sprintf("[%s] [%s]", strOptions, strPackage), []helpOption{
			{"-c --clean", gotext.Get("Remove unneeded dependencies (-cc to ignore optdepends)")},
			{"   --gendb", gotext.Get("Generates development package DB used for updating")},
		}},
	}
}

func (h *operationHelp) syntax() string {
	return fmt.Sprintf("yay {-%s --%s}", h.short, h.long)
}

func findYayOperationHelp(operation string) *operationHelp {
	operations := yayOperationHelp()
	for i := range operations {
		help := &operations[i]
		if operation == help.short || operation == help.long {
			return help
		}
	}

	return nil
}

func printUsageOptions(logger *text.Logger, title string, options []usageOption) {
	logger.Println("\n" + title)
	for _, option := range options {
		if option.flags == "" && option.description == "" {
			logger.Println("")
			continue
		}

		descriptionLines := strings.Split(option.description, "\n")
		logger.Printf("    %-21s %s\n", option.flags, descriptionLines[0])
		for _, line := range descriptionLines[1:] {
			logger.Printf("    %-21s %s\n", "", line)
		}
	}
}

func usage(logger *text.Logger) {
	strFile := gotext.Get("file(s)")
	strOperation := gotext.Get("operation")
	strOptions := gotext.Get("options")
	strPackage := gotext.Get("package(s)")

	logger.Println(gotext.Get("Usage:"))
	logger.Println("    yay")
	logger.Printf("    yay <%s> [...]\n", strOperation)
	logger.Printf("    yay <%s>\n", strPackage)

	logger.Println("\n" + gotext.Get("operations:"))
	logger.Println("    yay {-h --help}")
	logger.Println("    yay {-V --version}")
	logger.Printf("    yay {-D --database}    <%s> <%s>\n", strOptions, strPackage)
	logger.Printf("    yay {-F --files}       [%s] [%s]\n", strOptions, strPackage)
	logger.Printf("    yay {-Q --query}       [%s] [%s]\n", strOptions, strPackage)
	logger.Printf("    yay {-R --remove}      [%s] <%s>\n", strOptions, strPackage)
	logger.Printf("    yay {-S --sync}        [%s] [%s]\n", strOptions, strPackage)
	logger.Printf("    yay {-T --deptest}     [%s] [%s]\n", strOptions, strPackage)
	logger.Printf("    yay {-U --upgrade}     [%s] <%s>\n", strOptions, strFile)

	// Yay-specific operations
	logger.Println("\n" + gotext.Get("New operations:"))
	operations := yayOperationHelp()
	for i := range operations {
		help := &operations[i]
		logger.Printf("    %-22s %s\n", help.syntax(), help.targets)
	}
	logger.Println("\n" + gotext.Get("use 'yay {-h --help}' with an operation for available options"))

	logger.Println("\n" + gotext.Get("If no operation is specified 'yay -Syu' will be performed"))
	logger.Println(gotext.Get("If no operation is specified and targets are provided, -Y will be assumed"))

	printUsageOptions(logger, gotext.Get("New options:"), []usageOption{
		{"-N --repo", gotext.Get("Assume targets are from the repositories")},
		{"-a --aur", gotext.Get("Assume targets are from the AUR")},
	})

	printUsageOptions(logger, gotext.Get("Permanent configuration options:"), []usageOption{
		{"--save", gotext.Get("Causes the following options to be saved back to the\nconfig file when used")},
		{},
		{"--aururl <url>", gotext.Get("Set an alternative AUR URL")},
		{"--aurrpcurl <url>", gotext.Get("Set an alternative URL for the AUR /rpc endpoint")},
		{"--builddir <dir>", gotext.Get("Directory used to download and run PKGBUILDS")},
		{"--editor <file>", gotext.Get("Editor to use when editing PKGBUILDs")},
		{"--editorflags <flags>", gotext.Get("Pass arguments to editor")},
		{"--makepkg <file>", gotext.Get("makepkg command to use")},
		{"--mflags <flags>", gotext.Get("Pass arguments to makepkg")},
		{"--pacman <file>", gotext.Get("pacman command to use")},
		{"--git <file>", gotext.Get("git command to use")},
		{"--gitflags <flags>", gotext.Get("Pass arguments to git")},
		{"--gpg <file>", gotext.Get("gpg command to use")},
		{"--gpgflags <flags>", gotext.Get("Pass arguments to gpg")},
		{"--config <file>", gotext.Get("pacman.conf file to use")},
		{"--makepkgconf <file>", gotext.Get("makepkg.conf file to use")},
		{"--nomakepkgconf", gotext.Get("Use the default makepkg.conf")},
		{},
		{"--requestsplitn <n>", gotext.Get("Max amount of packages to query per AUR request")},
		{"--completioninterval <n>", gotext.Get("Time in days to refresh completion cache")},
		{"--sortby <field>", gotext.Get("Sort AUR results by a specific field during search")},
		{"--searchby <field>", gotext.Get("Search for packages using a specified field")},
		{"--answerclean <a>", gotext.Get("Set a predetermined answer for the clean build menu")},
		{"--answerdiff <a>", gotext.Get("Set a predetermined answer for the diff menu")},
		{"--answeredit <a>", gotext.Get("Set a predetermined answer for the edit pkgbuild menu")},
		{"--answerupgrade <a>", gotext.Get("Set a predetermined answer for the upgrade menu")},
		{"--noanswerclean", gotext.Get("Unset the answer for the clean build menu")},
		{"--noanswerdiff", gotext.Get("Unset the answer for the edit diff menu")},
		{"--noansweredit", gotext.Get("Unset the answer for the edit pkgbuild menu")},
		{"--noanswerupgrade", gotext.Get("Unset the answer for the upgrade menu")},
		{"--noexcludemenu", gotext.Get("Skip the package exclusion menu during upgrades")},
		{"--cleanmenu", gotext.Get("Give the option to clean build PKGBUILDS")},
		{"--diffmenu", gotext.Get("Give the option to show diffs for build files")},
		{"--editmenu", gotext.Get("Give the option to edit/view PKGBUILDS")},
		{"--askremovemake", gotext.Get("Ask to remove makedepends after install")},
		{"--askyesremovemake", gotext.Get("Ask to remove makedepends after install(\"Y\" as default)")},
		{"--removemake", gotext.Get("Remove makedepends after install")},
		{"--noremovemake", gotext.Get("Don't remove makedepends after install")},
		{},
		{"--cleanafter", gotext.Get("Remove package sources after successful install")},
		{"--keepsrc", gotext.Get("Keep pkg/ and src/ after building packages")},
		{"--bottomup", gotext.Get("Shows AUR's packages first and then repository's")},
		{"--topdown", gotext.Get("Shows repository's packages first and then AUR's")},
		{"--singlelineresults", gotext.Get("List each search result on its own line")},
		{"--doublelineresults", gotext.Get("List each search result on two lines, like pacman")},
		{},
		{"--devel", gotext.Get("Check development packages during sysupgrade")},
		{"--rebuild", gotext.Get("Always build target packages")},
		{"--rebuildall", gotext.Get("Always build all AUR packages")},
		{"--norebuild", gotext.Get("Skip package build if in cache and up to date")},
		{"--rebuildtree", gotext.Get("Always build all AUR packages even if installed")},
		{"--redownload", gotext.Get("Always download pkgbuilds of targets")},
		{"--noredownload", gotext.Get("Skip pkgbuild download if in cache and up to date")},
		{"--redownloadall", gotext.Get("Always download pkgbuilds of all AUR packages")},
		{"--provides", gotext.Get("Look for matching providers when searching for packages")},
		{"--pgpfetch", gotext.Get("Prompt to import PGP keys from PKGBUILDs")},
		{"--useask", gotext.Get("Automatically resolve conflicts using pacman's ask flag")},
		{},
		{"--sudo <file>", gotext.Get("sudo command to use")},
		{"--sudoflags <flags>", gotext.Get("Pass arguments to sudo")},
		{"--sudoloop", gotext.Get("Loop sudo calls in the background to avoid timeout")},
	})
}

func operationUsage(logger *text.Logger, operation string) {
	help := findYayOperationHelp(operation)
	if help == nil {
		return
	}

	logger.Print(gotext.Get("Usage: %s %s\noptions:\n", help.syntax(), help.targets))
	for _, option := range help.options {
		logger.Printf("    %-21s %s\n", option.flags, option.description)
	}

	logger.Println("\n" + gotext.Get("use 'yay {-h --help}' for global options"))
}

func handleCmd(ctx context.Context, run *runtime.Runtime,
	cmdArgs *parser.Arguments, dbExecutor db.Executor,
) error {
	if cmdArgs.Op == "V" || cmdArgs.Op == "version" {
		handleVersion(run.Logger)
		return nil
	}

	if cmdArgs.ExistsArg("h", "help") {
		return handleHelp(ctx, run, cmdArgs)
	}

	if run.Cfg.SudoLoop && cmdArgs.NeedRoot(run.Cfg.Mode) {
		run.CmdBuilder.SudoLoop()
	}

	switch cmdArgs.Op {
	case "D", "database":
		return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, run.Cfg.Mode, settings.NoConfirm))
	case "F", "files":
		return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, run.Cfg.Mode, settings.NoConfirm))
	case "Q", "query":
		return handleQuery(ctx, run, cmdArgs, dbExecutor)
	case "R", "remove":
		return handleRemove(ctx, run, cmdArgs, run.VCSStore)
	case "S", "sync":
		return handleSync(ctx, run, cmdArgs, dbExecutor)
	case "T", "deptest":
		return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, run.Cfg.Mode, settings.NoConfirm))
	case "U", "upgrade":
		return handleUpgrade(ctx, run, cmdArgs)
	case "B", "build":
		return handleBuild(ctx, run, dbExecutor, cmdArgs)
	case "G", "getpkgbuild":
		return handleGetpkgbuild(ctx, run, cmdArgs, dbExecutor)
	case "P", "show":
		return handlePrint(ctx, run, cmdArgs, dbExecutor)
	case "Y", "yay":
		return handleYay(ctx, run, cmdArgs, run.CmdBuilder,
			dbExecutor, run.QueryBuilder)
	case "W", "web":
		return handleWeb(ctx, run, cmdArgs)
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

func handleQuery(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	if cmdArgs.ExistsArg("u", "upgrades") {
		filter, err := getFilter(cmdArgs)
		if err != nil {
			return err
		}

		return printUpdateList(ctx, run, cmdArgs, dbExecutor,
			cmdArgs.ExistsDouble("u", "sysupgrade"), filter)
	}

	if err := run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, run.Cfg.Mode, settings.NoConfirm)); err != nil {
		if str := err.Error(); strings.Contains(str, "exit status") {
			// yay -Qdt should not output anything in case of error
			return fmt.Errorf("")
		}

		return err
	}

	return nil
}

func handleHelp(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments) error {
	switch cmdArgs.Op {
	case "Y", "yay", "G", "getpkgbuild", "P", "show", "W", "web", "B", "build":
		operationUsage(run.Logger, cmdArgs.Op)
		return nil
	case "":
		usage(run.Logger)
		return nil
	}

	return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, run.Cfg.Mode, settings.NoConfirm))
}

func handleVersion(logger *text.Logger) {
	logger.Printf("yay v%s - libalpm v%s\n", yayVersion, alpm.Version())
}

func handlePrint(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	switch {
	case cmdArgs.ExistsArg("d", "defaultconfig"):
		tmpConfig := settings.DefaultConfig(yayVersion)
		run.Logger.Printf("%v", tmpConfig)

		return nil
	case cmdArgs.ExistsArg("g", "currentconfig"):
		run.Logger.Printf("%v", run.Cfg)

		return nil
	case cmdArgs.ExistsArg("w", "news"):
		double := cmdArgs.ExistsDouble("w", "news")
		quiet := cmdArgs.ExistsArg("q", "quiet")

		return news.PrintNewsFeed(ctx, run.HTTPClient, run.Logger,
			dbExecutor.LastBuildTime(), run.Cfg.BottomUp, double, quiet)
	case cmdArgs.ExistsArg("c", "complete"):
		return completion.Show(ctx, run.HTTPClient, dbExecutor,
			run.Cfg.AURURL, run.Cfg.CompletionPath, run.Cfg.CompletionInterval, cmdArgs.ExistsDouble("c", "complete"), run.Logger)
	case cmdArgs.ExistsArg("s", "stats"):
		return localStatistics(ctx, run, dbExecutor)
	}

	return nil
}

func handleYay(ctx context.Context, run *runtime.Runtime,
	cmdArgs *parser.Arguments, cmdBuilder exe.ICmdBuilder,
	dbExecutor db.Executor, queryBuilder query.Builder,
) error {
	switch {
	case cmdArgs.ExistsArg("gendb"):
		return createDevelDB(ctx, run, dbExecutor)
	case cmdArgs.ExistsDouble("c"):
		return cleanDependencies(ctx, run.Cfg, cmdBuilder, cmdArgs, dbExecutor, true)
	case cmdArgs.ExistsArg("c", "clean"):
		return cleanDependencies(ctx, run.Cfg, cmdBuilder, cmdArgs, dbExecutor, false)
	case len(cmdArgs.Targets) > 0:
		return displayNumberMenu(ctx, run, cmdArgs.Targets, dbExecutor, queryBuilder, cmdArgs)
	}

	return nil
}

func handleWeb(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments) error {
	switch {
	case cmdArgs.ExistsArg("v", "vote"):
		return handlePackageVote(ctx, cmdArgs.Targets, run.AURClient, run.Logger,
			run.VoteClient, true)
	case cmdArgs.ExistsArg("u", "unvote"):
		return handlePackageVote(ctx, cmdArgs.Targets, run.AURClient, run.Logger,
			run.VoteClient, false)
	}

	return nil
}

func handleGetpkgbuild(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments, dbExecutor download.DBSearcher) error {
	if cmdArgs.ExistsArg("p", "print") {
		return printPkgbuilds(dbExecutor, run.AURClient,
			run.HTTPClient, run.Logger, cmdArgs.Targets, run.Cfg.Mode, run.Cfg.AURURL)
	}

	return getPkgbuilds(ctx, dbExecutor, run.AURClient, run,
		cmdArgs.Targets, cmdArgs.ExistsArg("f", "force"))
}

func handleUpgrade(ctx context.Context,
	run *runtime.Runtime, cmdArgs *parser.Arguments,
) error {
	return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, run.Cfg.Mode, settings.NoConfirm))
}

// -B* options
func handleBuild(ctx context.Context,
	run *runtime.Runtime, dbExecutor db.Executor, cmdArgs *parser.Arguments,
) error {
	return installLocalPKGBUILD(ctx, run, cmdArgs, dbExecutor)
}

func handleSync(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	targets := cmdArgs.Targets

	switch {
	case cmdArgs.ExistsArg("s", "search"):
		return syncSearch(ctx, targets, dbExecutor, run.QueryBuilder, !cmdArgs.ExistsArg("q", "quiet"))
	case cmdArgs.ExistsArg("p", "print", "print-format"):
		return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, run.Cfg.Mode, settings.NoConfirm))
	case cmdArgs.ExistsArg("c", "clean"):
		return syncClean(ctx, run, cmdArgs, dbExecutor)
	case cmdArgs.ExistsArg("l", "list"):
		return syncList(ctx, run, run.HTTPClient, cmdArgs, dbExecutor)
	case cmdArgs.ExistsArg("g", "groups"):
		return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, run.Cfg.Mode, settings.NoConfirm))
	case cmdArgs.ExistsArg("i", "info"):
		return syncInfo(ctx, run, cmdArgs, targets, dbExecutor)
	case cmdArgs.ExistsArg("u", "sysupgrade") || len(cmdArgs.Targets) > 0:
		return syncInstall(ctx, run, cmdArgs, dbExecutor)
	case cmdArgs.ExistsArg("y", "refresh"):
		return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, run.Cfg.Mode, settings.NoConfirm))
	}

	return nil
}

func handleRemove(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments, localCache vcs.Store) error {
	err := run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, run.Cfg.Mode, settings.NoConfirm))
	if err == nil {
		localCache.RemovePackages(cmdArgs.Targets)
	}

	return err
}

// NumberMenu presents a CLI for selecting packages to install.
func displayNumberMenu(ctx context.Context, run *runtime.Runtime, pkgS []string, dbExecutor db.Executor,
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

	run.Logger.Infoln(gotext.Get("Packages to install (eg: 1 2 3, 1-3 or ^4)"))

	numberBuf, err := run.Logger.GetInput("", false)
	if err != nil {
		return err
	}

	include, exclude, _, otherExclude := intrange.ParseNumberMenu(numberBuf)

	targets, err := queryBuilder.GetTargets(include, exclude, otherExclude)
	if err != nil {
		return err
	}

	// modify the arguments to pass for the install
	cmdArgs.Targets = targets

	if len(cmdArgs.Targets) == 0 {
		run.Logger.Println(gotext.Get(" there is nothing to do"))
		return nil
	}

	return syncInstall(ctx, run, cmdArgs, dbExecutor)
}

func syncList(ctx context.Context, run *runtime.Runtime,
	httpClient *http.Client, cmdArgs *parser.Arguments, dbExecutor db.Executor,
) error {
	aur := false

	for i, v := range slices.Backward(cmdArgs.Targets) {
		if v == "aur" && run.Cfg.Mode.AtLeastAUR() {
			cmdArgs.Targets = append(cmdArgs.Targets[:i], cmdArgs.Targets[i+1:]...)
			aur = true
		}
	}

	if run.Cfg.Mode.AtLeastAUR() && (len(cmdArgs.Targets) == 0 || aur) {
		scanner, err := download.GetPackageScanner(ctx, httpClient, run.Cfg.AURURL, run.Logger)
		if err != nil {
			return err
		}
		defer scanner.Close()

		for scanner.Scan() {
			name := scanner.Text()
			if cmdArgs.ExistsArg("q", "quiet") {
				run.Logger.Println(name)
			} else {
				run.Logger.Printf("%s %s %s", text.Magenta("aur"), text.Bold(name), text.Bold(text.Green(gotext.Get("unknown-version"))))

				if dbExecutor.LocalPackage(name) != nil {
					run.Logger.Print(text.Bold(text.Blue(gotext.Get(" [Installed]"))))
				}

				run.Logger.Println()
			}
		}
	}

	if run.Cfg.Mode.AtLeastRepo() && (len(cmdArgs.Targets) != 0 || !aur) {
		return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, run.Cfg.Mode, settings.NoConfirm))
	}

	return nil
}
