package main

import (
	"fmt"

	alpm "github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/download"
	"github.com/Jguer/yay/v10/pkg/exec"
	"github.com/Jguer/yay/v10/pkg/install"
	"github.com/Jguer/yay/v10/pkg/lookup"
	"github.com/Jguer/yay/v10/pkg/lookup/news"
	"github.com/Jguer/yay/v10/pkg/runtime"
	"github.com/Jguer/yay/v10/pkg/runtime/completion"
	"github.com/Jguer/yay/v10/pkg/types"
	"github.com/Jguer/yay/v10/pkg/vcs"
	pacmanconf "github.com/Morganamilo/go-pacmanconf"
)

func handleCmd(config *runtime.Configuration, pacmanConf *pacmanconf.Config, cmdArgs *types.Arguments, alpmHandle *alpm.Handle, savedInfo vcs.InfoStore) (err error) {
	if cmdArgs.ExistsArg("h", "help") {
		err = handleHelp(config, cmdArgs, pacmanConf)
		return
	}

	if config.SudoLoop && cmdArgs.NeedRoot(config.Mode) {
		exec.SudoLoopBackground()
	}

	switch cmdArgs.Op {
	case "V", "version":
		handleVersion()
	case "D", "database":
		err = exec.Show(exec.PassToPacman(config, pacmanConf, cmdArgs, config.NoConfirm))
	case "F", "files":
		err = exec.Show(exec.PassToPacman(config, pacmanConf, cmdArgs, config.NoConfirm))
	case "Q", "query":
		err = handleQuery(config, cmdArgs, pacmanConf, alpmHandle, savedInfo)
	case "R", "remove":
		err = handleRemove(config, cmdArgs, pacmanConf, savedInfo)
	case "S", "sync":
		err = handleSync(config, cmdArgs, pacmanConf, alpmHandle, savedInfo)
	case "T", "deptest":
		err = exec.Show(exec.PassToPacman(config, pacmanConf, cmdArgs, config.NoConfirm))
	case "U", "upgrade":
		err = exec.Show(exec.PassToPacman(config, pacmanConf, cmdArgs, config.NoConfirm))
	case "G", "getpkgbuild":
		err = handleGetpkgbuild(config, cmdArgs, pacmanConf, alpmHandle)
	case "P", "show":
		err = handlePrint(config, cmdArgs, alpmHandle, savedInfo)
	case "Y", "--yay":
		err = handleYay(config, pacmanConf, cmdArgs, alpmHandle, savedInfo)
	default:
		//this means we allowed an op but not implement it
		//if this happens it an error in the code and not the usage
		err = fmt.Errorf("unhandled operation")
	}

	return
}

func handleGetpkgbuild(config *runtime.Configuration, cmdArgs *types.Arguments, pacmanConf *pacmanconf.Config, alpmHandle *alpm.Handle) error {
	return download.GetPkgbuilds(config, cmdArgs, alpmHandle, cmdArgs.Targets)
}

func handleHelp(config *runtime.Configuration, cmdArgs *types.Arguments, pacmanConf *pacmanconf.Config) error {
	if cmdArgs.Op == "Y" || cmdArgs.Op == "yay" {
		usage()
		return nil
	}
	return exec.Show(exec.PassToPacman(config, pacmanConf, cmdArgs, config.NoConfirm))
}

func handlePrint(config *runtime.Configuration, cmdArgs *types.Arguments, alpmHandle *alpm.Handle, savedInfo vcs.InfoStore) (err error) {
	switch {
	case cmdArgs.ExistsArg("d", "defaultconfig"):
		tmpConfig := runtime.DefaultSettings()
		tmpConfig.ExpandEnv()
		fmt.Printf("%v", tmpConfig)
	case cmdArgs.ExistsArg("g", "currentconfig"):
		fmt.Printf("%v", config)
	case cmdArgs.ExistsArg("n", "numberupgrades"):
		err = lookup.PrintNumberOfUpdates(cmdArgs, alpmHandle, config, savedInfo)
	case cmdArgs.ExistsArg("u", "upgrades"):
		err = lookup.PrintUpdateList(cmdArgs, alpmHandle, config, savedInfo)
	case cmdArgs.ExistsArg("w", "news"):
		err = news.PrintFeed(cmdArgs, alpmHandle, config.SortMode)
	case cmdArgs.ExistsDouble("c", "complete"):
		err = completion.Show(alpmHandle, config.AURURL, config.BuildDir, config.CompletionInterval, true)
	case cmdArgs.ExistsArg("c", "complete"):
		err = completion.Show(alpmHandle, config.AURURL, config.BuildDir, config.CompletionInterval, false)
	case cmdArgs.ExistsArg("s", "stats"):
		err = lookup.LocalStatistics(config, alpmHandle)
	default:
		err = nil
	}
	return err
}

func handleQuery(config *runtime.Configuration, cmdArgs *types.Arguments, pacmanConf *pacmanconf.Config, alpmHandle *alpm.Handle, savedInfo vcs.InfoStore) error {
	if cmdArgs.ExistsArg("u", "upgrades") {
		return lookup.PrintUpdateList(cmdArgs, alpmHandle, config, savedInfo)
	}
	return exec.Show(exec.PassToPacman(config, pacmanConf, cmdArgs, config.NoConfirm))
}

func handleRemove(config *runtime.Configuration, cmdArgs *types.Arguments, pacmanConf *pacmanconf.Config, savedInfo vcs.InfoStore) error {
	savedInfo.RemovePackage(cmdArgs.Targets)
	return exec.Show(exec.PassToPacman(config, pacmanConf, cmdArgs, config.NoConfirm))
}

func handleSync(config *runtime.Configuration, cmdArgs *types.Arguments, pacmanConf *pacmanconf.Config, alpmHandle *alpm.Handle, savedInfo vcs.InfoStore) error {
	targets := cmdArgs.Targets

	if cmdArgs.ExistsArg("s", "search") {
		if cmdArgs.ExistsArg("q", "quiet") {
			config.SearchMode = runtime.Minimal
		} else {
			config.SearchMode = runtime.Detailed
		}
		return lookup.PrintSearch(config, alpmHandle, targets)
	}
	if cmdArgs.ExistsArg("p", "print", "print-format") {
		return exec.Show(exec.PassToPacman(config, pacmanConf, cmdArgs, config.NoConfirm))
	}
	if cmdArgs.ExistsArg("c", "clean") {
		return lookup.SyncClean(config, pacmanConf, alpmHandle, cmdArgs)
	}
	if cmdArgs.ExistsArg("l", "list") {
		return lookup.SyncList(config, pacmanConf, alpmHandle, cmdArgs)
	}
	if cmdArgs.ExistsArg("g", "groups") {
		return exec.Show(exec.PassToPacman(config, pacmanConf, cmdArgs, config.NoConfirm))
	}
	if cmdArgs.ExistsArg("i", "info") {
		return lookup.SyncInfo(config, pacmanConf, cmdArgs, alpmHandle, targets)
	}
	if cmdArgs.ExistsArg("u", "sysupgrade") {
		return install.Install(config, pacmanConf, alpmHandle, cmdArgs, savedInfo)
	}
	if len(cmdArgs.Targets) > 0 {
		return install.Install(config, pacmanConf, alpmHandle, cmdArgs, savedInfo)
	}
	if cmdArgs.ExistsArg("y", "refresh") {
		return exec.Show(exec.PassToPacman(config, pacmanConf, cmdArgs, config.NoConfirm))
	}
	return nil
}

func handleVersion() {
	fmt.Printf("yay v%s - libalpm v%s\n", runtime.Version, alpm.Version())
}

func handleYay(config *runtime.Configuration, pacmanConf *pacmanconf.Config, cmdArgs *types.Arguments, alpmHandle *alpm.Handle, savedInfo vcs.InfoStore) error {
	//_, options, targets := cmdArgs.formatArgs()
	if cmdArgs.ExistsArg("gendb") {
		return install.CreateDevelDB(alpmHandle, config, savedInfo)
	}
	if cmdArgs.ExistsDouble("c") {
		return lookup.CleanDependencies(config, alpmHandle, pacmanConf, cmdArgs, true)
	}
	if cmdArgs.ExistsArg("c", "clean") {
		return lookup.CleanDependencies(config, alpmHandle, pacmanConf, cmdArgs, false)
	}
	if len(cmdArgs.Targets) > 0 {
		return handleYogurt(config, cmdArgs, pacmanConf, alpmHandle, savedInfo)
	}
	return nil
}

func handleYogurt(config *runtime.Configuration, cmdArgs *types.Arguments, pacmanConf *pacmanconf.Config, alpmHandle *alpm.Handle, savedInfo vcs.InfoStore) error {
	config.SearchMode = runtime.NumberMenu
	return install.DisplayNumberMenu(config, pacmanConf, alpmHandle, savedInfo, cmdArgs, cmdArgs.Targets)
}

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
    --tar         <file>  bsdtar command to use
    --git         <file>  git command to use
    --gitflags    <flags> Pass arguments to git
    --gpg         <file>  gpg command to use
    --gpgflags    <flags> Pass arguments to gpg
    --config      <file>  pacman.conf file to use
    --makepkgconf <file>  makepkg.conf file to use
    --nomakepkgconf       Use the default makepkg.conf

    --requestsplitn <n>   Max amount of packages to query per AUR request
    --completioninterval  <n> Time in days to to refresh completion cache
    --sortby    <field>   Sort AUR results by a specific field during search
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
    --gitclone            Use git clone for PKGBUILD retrieval
    --nogitclone          Never use git clone for PKGBUILD retrieval
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
    -f --force            Force download for existing tar packages

If no arguments are provided 'yay -Syu' will be performed
If no operation is provided -Y will be assumed`)
}
