package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"

	alpm "github.com/Jguer/go-alpm"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/completion"
	"github.com/Jguer/yay/v10/pkg/intrange"
	"github.com/Jguer/yay/v10/pkg/news"
	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/text"
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

yay specific options:
    -c --clean            Remove unneeded dependencies
       --gendb            Generates development package DB used for updating

getpkgbuild specific options:
    -f --force            Force download for existing ABS packages`)
}

func handleCmd(args *settings.Args, alpmHandle *alpm.Handle) error {
	if config.Help {
		return handleHelp(args)
	}

	if config.SudoLoop && config.NeedRoot() {
		sudoLoopBackground()
	}

	switch config.Op {
	case settings.Version:
		handleVersion()
		return nil
	case settings.Database:
		return show(passToPacman(args))
	case settings.Files:
		return show(passToPacman(args))
	case settings.Query:
		return handleQuery(args, alpmHandle)
	case settings.Remove:
		return handleRemove(args)
	case settings.Sync:
		return handleSync(args, alpmHandle)
	case settings.DepTest:
		return show(passToPacman(args))
	case settings.Upgrade:
		return show(passToPacman(args))
	case settings.GetPkgbuild:
		return handleGetpkgbuild(args, alpmHandle)
	case settings.Show:
		return handlePrint(args, alpmHandle)
	case settings.Yay:
		return handleYay(args, alpmHandle)
	}

	return fmt.Errorf(gotext.Get("unhandled operation"))
}

func handleQuery(args *settings.Args, alpmHandle *alpm.Handle) error {
	if config.Upgrades {
		return printUpdateList(args, alpmHandle, config.SysUpgrade > 1)
	}
	return show(passToPacman(args))
}

func handleHelp(args *settings.Args) error {
	if config.Op == settings.Yay {
		usage()
		return nil
	}
	return show(passToPacman(args))
}

func handleVersion() {
	fmt.Printf("yay v%s - libalpm v%s\n", yayVersion, alpm.Version())
}

func handlePrint(args *settings.Args, alpmHandle *alpm.Handle) (err error) {
	switch {
	case config.NumUpgrades:
		err = printNumberOfUpdates(alpmHandle, config.SysUpgrade > 1)
	case config.News > 0:
		double := config.News > 1
		quiet := config.Quiet
		err = news.PrintNewsFeed(alpmHandle, config.SortMode, double, quiet)
	case config.Complete > 1:
		err = completion.Show(alpmHandle, config.AURURL, config.CompletionPath, config.CompletionInterval, true)
	case config.Complete == 1:
		err = completion.Show(alpmHandle, config.AURURL, config.CompletionPath, config.CompletionInterval, false)
	case config.Stats:
		err = localStatistics(alpmHandle)
	default:
		err = nil
	}
	return err
}

func handleYay(args *settings.Args, alpmHandle *alpm.Handle) error {
	if config.Gendb {
		return createDevelDB(config.VCSPath, alpmHandle)
	}
	if config.Clean > 1 {
		return cleanDependencies(alpmHandle, true)
	}
	if config.Clean == 1 {
		return cleanDependencies(alpmHandle, false)
	}
	if len(config.Targets) > 0 {
		return handleYogurt(args, alpmHandle)
	}
	return nil
}

func handleGetpkgbuild(args *settings.Args, alpmHandle *alpm.Handle) error {
	return getPkgbuilds(config.Targets, alpmHandle, config.Force)
}

func handleYogurt(args *settings.Args, alpmHandle *alpm.Handle) error {
	config.SearchMode = numberMenu
	return displayNumberMenu(config.Targets, alpmHandle, args)
}

func handleSync(args *settings.Args, alpmHandle *alpm.Handle) error {
	targets := config.Targets

	if config.Search {
		if config.Quiet {
			config.SearchMode = minimal
		} else {
			config.SearchMode = detailed
		}
		return syncSearch(targets, alpmHandle)
	}
	if config.Print || config.PrintFormat != "" {
		return show(passToPacman(args))
	}
	if config.Clean > 0 {
		return syncClean(args, alpmHandle)
	}
	if config.List {
		return syncList(args, alpmHandle)
	}
	if config.Groups {
		return show(passToPacman(args))
	}
	if config.Info > 0 {
		return syncInfo(targets, alpmHandle)
	}
	if config.SysUpgrade > 0 {
		return install(args, alpmHandle, false)
	}
	if len(config.Targets) > 0 {
		return install(args, alpmHandle, false)
	}
	if config.Refresh > 0 {
		return show(passToPacman(args))
	}
	return nil
}

func handleRemove(args *settings.Args) error {
	err := show(passToPacman(args))
	if err == nil {
		removeVCSPackage(config.Targets)
	}

	return err
}

// NumberMenu presents a CLI for selecting packages to install.
func displayNumberMenu(pkgS []string, alpmHandle *alpm.Handle, args *settings.Args) error {
	var (
		aurErr, repoErr error
		aq              aurQuery
		pq              repoQuery
		lenaq, lenpq    int
	)

	pkgS = query.RemoveInvalidTargets(pkgS, config.Mode)

	if config.Mode == settings.ModeAUR || config.Mode == settings.ModeAny {
		aq, aurErr = narrowSearch(pkgS, true)
		lenaq = len(aq)
	}
	if config.Mode == settings.ModeRepo || config.Mode == settings.ModeAny {
		pq, repoErr = queryRepo(pkgS, alpmHandle)
		lenpq = len(pq)
		if repoErr != nil {
			return repoErr
		}
	}

	if lenpq == 0 && lenaq == 0 {
		return fmt.Errorf(gotext.Get("no packages match search"))
	}

	switch config.SortMode {
	case "topdown":
		if config.Mode == settings.ModeRepo || config.Mode == settings.ModeAny {
			pq.printSearch(alpmHandle)
		}
		if config.Mode == settings.ModeAUR || config.Mode == settings.ModeAny {
			aq.printSearch(lenpq+1, alpmHandle)
		}
	case "bottomup":
		if config.Mode == settings.ModeAUR || config.Mode == settings.ModeAny {
			aq.printSearch(lenpq+1, alpmHandle)
		}
		if config.Mode == settings.ModeRepo || config.Mode == settings.ModeAny {
			pq.printSearch(alpmHandle)
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
	arguments := config.Globals()

	isInclude := len(exclude) == 0 && len(otherExclude) == 0

	for i, pkg := range pq {
		var target int
		switch config.SortMode {
		case "topdown":
			target = i + 1
		case "bottomup":
			target = len(pq) - i
		default:
			return fmt.Errorf(gotext.Get("invalid sort mode. Fix with yay -Y --bottomup --save"))
		}

		if (isInclude && include.Get(target)) || (!isInclude && !exclude.Get(target)) {
			arguments.Add(pkg.DB().Name() + "/" + pkg.Name())
		}
	}

	for i := range aq {
		var target int

		switch config.SortMode {
		case "topdown":
			target = i + 1 + len(pq)
		case "bottomup":
			target = len(aq) - i + len(pq)
		default:
			return fmt.Errorf(gotext.Get("invalid sort mode. Fix with yay -Y --bottomup --save"))
		}

		if (isInclude && include.Get(target)) || (!isInclude && !exclude.Get(target)) {
			arguments.Add("aur/" + aq[i].Name)
		}
	}

	if len(config.Targets) == 0 {
		fmt.Println(gotext.Get(" there is nothing to do"))
		return nil
	}

	if config.SudoLoop {
		sudoLoopBackground()
	}

	return install(arguments, alpmHandle, true)
}

func syncList(args *settings.Args, alpmHandle *alpm.Handle) error {
	aur := false

	for i := len(config.Targets) - 1; i >= 0; i-- {
		if config.Targets[i] == "aur" && (config.Mode == settings.ModeAny || config.Mode == settings.ModeAUR) {
			config.Targets = append(config.Targets[:i], config.Targets[i+1:]...)
			aur = true
		}
	}

	if (config.Mode == settings.ModeAny || config.Mode == settings.ModeAUR) && (len(config.Targets) == 0 || aur) {
		localDB, err := alpmHandle.LocalDB()
		if err != nil {
			return err
		}

		resp, err := http.Get(config.AURURL + "/packages.gz")
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)

		scanner.Scan()
		for scanner.Scan() {
			name := scanner.Text()
			if config.Quiet {
				fmt.Println(name)
			} else {
				fmt.Printf("%s %s %s", magenta("aur"), bold(name), bold(green(gotext.Get("unknown-version"))))

				if localDB.Pkg(name) != nil {
					fmt.Print(bold(blue(gotext.Get(" [Installed]"))))
				}

				fmt.Println()
			}
		}
	}

	if (config.Mode == settings.ModeAny || config.Mode == settings.ModeRepo) && (len(config.Targets) != 0 || !aur) {
		return show(passToPacman(args))
	}

	return nil
}
