package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	alpm "github.com/Jguer/go-alpm/v2"
	gosrc "github.com/Morganamilo/go-srcinfo"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/completion"
	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/download"
	"github.com/Jguer/yay/v12/pkg/menus"
	"github.com/Jguer/yay/v12/pkg/pgp"
	"github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/srcinfo"
	"github.com/Jguer/yay/v12/pkg/stringset"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/vcs"
)

func setPkgReason(ctx context.Context,
	cmdBuilder exe.ICmdBuilder,
	mode parser.TargetMode,
	cmdArgs *parser.Arguments, pkgs []string, exp bool,
) error {
	if len(pkgs) == 0 {
		return nil
	}

	cmdArgs = cmdArgs.CopyGlobal()
	if exp {
		if err := cmdArgs.AddArg("q", "D", "asexplicit"); err != nil {
			return err
		}
	} else {
		if err := cmdArgs.AddArg("q", "D", "asdeps"); err != nil {
			return err
		}
	}

	for _, compositePkgName := range pkgs {
		pkgSplit := strings.Split(compositePkgName, "/")

		pkgName := pkgSplit[0]
		if len(pkgSplit) > 1 {
			pkgName = pkgSplit[1]
		}

		cmdArgs.AddTarget(pkgName)
	}

	if err := cmdBuilder.Show(cmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, mode, settings.NoConfirm)); err != nil {
		return &SetPkgReasonError{exp: exp}
	}

	return nil
}

func asdeps(ctx context.Context,
	cmdBuilder exe.ICmdBuilder,
	mode parser.TargetMode, cmdArgs *parser.Arguments, pkgs []string,
) error {
	return setPkgReason(ctx, cmdBuilder, mode, cmdArgs, pkgs, false)
}

func asexp(ctx context.Context,
	cmdBuilder exe.ICmdBuilder,
	mode parser.TargetMode, cmdArgs *parser.Arguments, pkgs []string,
) error {
	return setPkgReason(ctx, cmdBuilder, mode, cmdArgs, pkgs, true)
}

// Install handles package installs.
func install(ctx context.Context, cfg *settings.Configuration,
	cmdArgs *parser.Arguments, dbExecutor db.Executor, ignoreProviders bool,
) error {
	var (
		do              *dep.Order
		srcinfos        map[string]*gosrc.Srcinfo
		noDeps          = cmdArgs.ExistsDouble("d", "nodeps")
		noCheck         = strings.Contains(cfg.MFlags, "--nocheck")
		assumeInstalled = cmdArgs.GetArgs("assume-installed")
		sysupgradeArg   = cmdArgs.ExistsArg("u", "sysupgrade")
		refreshArg      = cmdArgs.ExistsArg("y", "refresh")
		warnings        = query.NewWarnings(cfg.Runtime.Logger)
	)

	if noDeps {
		cfg.Runtime.CmdBuilder.AddMakepkgFlag("-d")
	}

	if cfg.Mode.AtLeastRepo() {
		if cfg.CombinedUpgrade {
			if refreshArg {
				if errR := earlyRefresh(ctx, cfg, cfg.Runtime.CmdBuilder, cmdArgs); errR != nil {
					return fmt.Errorf("%s - %w", gotext.Get("error refreshing databases"), errR)
				}
				cmdArgs.DelArg("y", "refresh")
			}
		} else if refreshArg || sysupgradeArg || len(cmdArgs.Targets) > 0 {
			if errP := earlyPacmanCall(ctx, cfg, cmdArgs, dbExecutor); errP != nil {
				return errP
			}
		}
	}

	// we may have done -Sy, our handle now has an old
	// database.
	if errRefresh := dbExecutor.RefreshHandle(); errRefresh != nil {
		return errRefresh
	}

	remoteNames := dbExecutor.InstalledRemotePackageNames()
	localNames := dbExecutor.InstalledSyncPackageNames()

	remoteNamesCache := mapset.NewThreadUnsafeSet(remoteNames...)
	localNamesCache := stringset.FromSlice(localNames)

	requestTargets := cmdArgs.Copy().Targets

	// create the arguments to pass for the repo install
	arguments := cmdArgs.Copy()
	arguments.DelArg("asdeps", "asdep")
	arguments.DelArg("asexplicit", "asexp")
	arguments.Op = "S"
	arguments.ClearTargets()

	if cfg.Mode == parser.ModeAUR {
		arguments.DelArg("u", "sysupgrade")
	}

	// if we are doing -u also request all packages needing update
	if sysupgradeArg {
		var errSysUp error

		requestTargets, errSysUp = addUpgradeTargetsToArgs(ctx, cfg, dbExecutor, cmdArgs, requestTargets, arguments)
		if errSysUp != nil {
			return errSysUp
		}
	}

	targets := stringset.FromSlice(cmdArgs.Targets)

	dp, err := dep.GetPool(ctx, requestTargets,
		warnings, dbExecutor, cfg.Runtime.AURClient, cfg.Mode,
		ignoreProviders, settings.NoConfirm, cfg.Provides, cfg.ReBuild, cfg.RequestSplitN, noDeps, noCheck, assumeInstalled)
	if err != nil {
		return err
	}

	if errC := dp.CheckMissing(noDeps, noCheck); errC != nil {
		return errC
	}

	if len(dp.Aur) == 0 {
		if !cfg.CombinedUpgrade {
			if sysupgradeArg {
				fmt.Println(gotext.Get(" there is nothing to do"))
			}

			return nil
		}

		cmdArgs.Op = "S"
		cmdArgs.DelArg("y", "refresh")

		if arguments.ExistsArg("ignore") {
			cmdArgs.CreateOrAppendOption("ignore", arguments.GetArgs("ignore")...)
		}

		return cfg.Runtime.CmdBuilder.Show(cfg.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, cfg.Mode, settings.NoConfirm))
	}

	conflicts, errCC := dp.CheckConflicts(cfg.UseAsk, settings.NoConfirm, noDeps)
	if errCC != nil {
		return errCC
	}

	do = dep.GetOrder(dp, noDeps, noCheck)

	for _, pkg := range do.Repo {
		arguments.AddTarget(pkg.DB().Name() + "/" + pkg.Name())
	}

	for _, pkg := range dp.Groups {
		arguments.AddTarget(pkg)
	}

	if len(do.Aur) == 0 && len(arguments.Targets) == 0 &&
		(!cmdArgs.ExistsArg("u", "sysupgrade") || cfg.Mode == parser.ModeAUR) {
		fmt.Println(gotext.Get(" there is nothing to do"))
		return nil
	}

	do.Print()
	fmt.Println()

	pkgbuildDirs := make(map[string]string, len(do.Aur))

	for _, base := range do.Aur {
		dir := filepath.Join(cfg.BuildDir, base.Pkgbase())
		pkgbuildDirs[base.Pkgbase()] = dir
	}

	if cfg.CleanAfter {
		defer func() {
			cleanAfter(ctx, cfg, cfg.Runtime.CmdBuilder, pkgbuildDirs)
		}()
	}

	if do.HasMake() {
		switch cfg.RemoveMake {
		case "yes":
			defer func() {
				err = removeMake(ctx, cfg, cfg.Runtime.CmdBuilder, do.GetMake(), cmdArgs)
			}()

		case "no":
			break
		default:
			if text.ContinueTask(os.Stdin, gotext.Get("Remove make dependencies after install?"), false, settings.NoConfirm) {
				defer func() {
					err = removeMake(ctx, cfg, cfg.Runtime.CmdBuilder, do.GetMake(), cmdArgs)
				}()
			}
		}
	}

	if errCleanMenu := menus.Clean(os.Stdout, cfg.CleanMenu,
		pkgbuildDirs,
		remoteNamesCache, settings.NoConfirm, cfg.AnswerClean); errCleanMenu != nil {
		if errors.As(errCleanMenu, &settings.ErrUserAbort{}) {
			return errCleanMenu
		}

		text.Errorln(errCleanMenu)
	}

	toSkip := pkgbuildsToSkip(cfg, do.Aur, targets)
	toClone := make([]string, 0, len(do.Aur))

	for _, base := range do.Aur {
		if !toSkip.Get(base.Pkgbase()) {
			toClone = append(toClone, base.Pkgbase())
		}
	}

	if toSkipSlice := toSkip.ToSlice(); len(toSkipSlice) != 0 {
		text.OperationInfoln(
			gotext.Get("PKGBUILD up to date, Skipping (%d/%d): %s",
				len(toSkipSlice), len(toClone), text.Cyan(strings.Join(toSkipSlice, ", "))))
	}

	cloned, errA := download.AURPKGBUILDRepos(ctx,
		cfg.Runtime.CmdBuilder, toClone, cfg.AURURL, cfg.BuildDir, false)
	if errA != nil {
		return errA
	}

	if errDiffMenu := menus.Diff(ctx, cfg.Runtime.CmdBuilder, os.Stdout, pkgbuildDirs,
		cfg.DiffMenu, remoteNamesCache,
		cloned, settings.NoConfirm, cfg.AnswerDiff); errDiffMenu != nil {
		if errors.As(errDiffMenu, &settings.ErrUserAbort{}) {
			return errDiffMenu
		}

		text.Errorln(errDiffMenu)
	}

	if errM := mergePkgbuilds(ctx, cfg.Runtime.CmdBuilder, pkgbuildDirs); errM != nil {
		return errM
	}

	srcinfos, err = srcinfo.ParseSrcinfoFilesByBase(pkgbuildDirs, true)
	if err != nil {
		return err
	}

	if errEditMenu := menus.Edit(os.Stdout, cfg.EditMenu, pkgbuildDirs,
		cfg.Editor, cfg.EditorFlags, remoteNamesCache, srcinfos,
		settings.NoConfirm, cfg.AnswerEdit); errEditMenu != nil {
		if errors.As(errEditMenu, &settings.ErrUserAbort{}) {
			return errEditMenu
		}

		text.Errorln(errEditMenu)
	}

	if errI := confirmIncompatibleInstall(srcinfos, dbExecutor); errI != nil {
		return errI
	}

	if cfg.PGPFetch {
		if _, errCPK := pgp.CheckPgpKeys(ctx, pkgbuildDirs, srcinfos, cfg.Runtime.CmdBuilder, settings.NoConfirm); errCPK != nil {
			return errCPK
		}
	}

	if !cfg.CombinedUpgrade {
		arguments.DelArg("u", "sysupgrade")
	}

	if len(arguments.Targets) > 0 || arguments.ExistsArg("u") {
		if errShow := cfg.Runtime.CmdBuilder.Show(cfg.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			arguments, cfg.Mode, settings.NoConfirm)); errShow != nil {
			return errors.New(gotext.Get("error installing repo packages"))
		}

		deps := make([]string, 0)
		exp := make([]string, 0)

		for _, pkg := range do.Repo {
			if !dp.Explicit.Get(pkg.Name()) && !localNamesCache.Get(pkg.Name()) && !remoteNamesCache.Contains(pkg.Name()) {
				deps = append(deps, pkg.Name())

				continue
			}

			if cmdArgs.ExistsArg("asdeps", "asdep") && dp.Explicit.Get(pkg.Name()) {
				deps = append(deps, pkg.Name())
			} else if cmdArgs.ExistsArg("asexp", "asexplicit") && dp.Explicit.Get(pkg.Name()) {
				exp = append(exp, pkg.Name())
			}
		}

		if errDeps := asdeps(ctx, cfg.Runtime.CmdBuilder, cfg.Mode, cmdArgs, deps); errDeps != nil {
			return errDeps
		}

		if errExp := asexp(ctx, cfg.Runtime.CmdBuilder, cfg.Mode, cmdArgs, exp); errExp != nil {
			return errExp
		}
	}

	go func() {
		_ = completion.Update(ctx, cfg.Runtime.HTTPClient, dbExecutor,
			cfg.AURURL, cfg.CompletionPath, cfg.CompletionInterval, false)
	}()

	if errP := downloadPKGBUILDSourceFanout(ctx,
		cfg.Runtime.CmdBuilder,
		pkgbuildDirs,
		true, cfg.MaxConcurrentDownloads); errP != nil {
		text.Errorln(errP)
	}

	if errB := buildInstallPkgbuilds(ctx, cfg, cmdArgs, dbExecutor, dp, do,
		srcinfos, true, conflicts, noDeps, noCheck); errB != nil {
		return errB
	}

	return nil
}

func addUpgradeTargetsToArgs(ctx context.Context, cfg *settings.Configuration, dbExecutor db.Executor,
	cmdArgs *parser.Arguments, requestTargets []string, arguments *parser.Arguments,
) ([]string, error) {
	ignore, targets, errUp := sysupgradeTargets(ctx, cfg, dbExecutor, cmdArgs.ExistsDouble("u", "sysupgrade"))
	if errUp != nil {
		return nil, errUp
	}

	for _, up := range targets {
		cmdArgs.AddTarget(up)
		requestTargets = append(requestTargets, up)
	}

	if len(ignore) > 0 {
		arguments.CreateOrAppendOption("ignore", ignore.ToSlice()...)
	}

	return requestTargets, nil
}

func removeMake(ctx context.Context, config *settings.Configuration,
	cmdBuilder exe.ICmdBuilder, makeDeps []string, cmdArgs *parser.Arguments,
) error {
	removeArguments := cmdArgs.CopyGlobal()

	err := removeArguments.AddArg("R", "s", "u")
	if err != nil {
		return err
	}

	for _, pkg := range makeDeps {
		removeArguments.AddTarget(pkg)
	}

	oldValue := settings.NoConfirm
	settings.NoConfirm = true
	err = cmdBuilder.Show(cmdBuilder.BuildPacmanCmd(ctx,
		removeArguments, config.Mode, settings.NoConfirm))
	settings.NoConfirm = oldValue

	return err
}

func inRepos(dbExecutor db.Executor, pkg string) bool {
	target := dep.ToTarget(pkg)

	if target.DB == "aur" {
		return false
	} else if target.DB != "" {
		return true
	}

	previousHideMenus := settings.HideMenus
	settings.HideMenus = true
	exists := dbExecutor.SyncSatisfierExists(target.DepString())
	settings.HideMenus = previousHideMenus

	return exists || len(dbExecutor.PackagesFromGroup(target.Name)) > 0
}

func earlyPacmanCall(ctx context.Context, cfg *settings.Configuration,
	cmdArgs *parser.Arguments, dbExecutor db.Executor,
) error {
	arguments := cmdArgs.Copy()
	arguments.Op = "S"
	targets := cmdArgs.Targets
	cmdArgs.ClearTargets()
	arguments.ClearTargets()

	if cfg.Mode == parser.ModeRepo {
		arguments.Targets = targets
	} else {
		// separate aur and repo targets
		for _, target := range targets {
			if inRepos(dbExecutor, target) {
				arguments.AddTarget(target)
			} else {
				cmdArgs.AddTarget(target)
			}
		}
	}

	if cmdArgs.ExistsArg("y", "refresh") || cmdArgs.ExistsArg("u", "sysupgrade") || len(arguments.Targets) > 0 {
		if err := cfg.Runtime.CmdBuilder.Show(cfg.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			arguments, cfg.Mode, settings.NoConfirm)); err != nil {
			return errors.New(gotext.Get("error installing repo packages"))
		}
	}

	return nil
}

func earlyRefresh(ctx context.Context, cfg *settings.Configuration, cmdBuilder exe.ICmdBuilder, cmdArgs *parser.Arguments) error {
	arguments := cmdArgs.Copy()
	if cfg.CombinedUpgrade {
		arguments.DelArg("u", "sysupgrade")
	}
	arguments.DelArg("s", "search")
	arguments.DelArg("i", "info")
	arguments.DelArg("l", "list")
	arguments.ClearTargets()

	return cmdBuilder.Show(cmdBuilder.BuildPacmanCmd(ctx,
		arguments, cfg.Mode, settings.NoConfirm))
}

func confirmIncompatibleInstall(srcinfos map[string]*gosrc.Srcinfo, dbExecutor db.Executor) error {
	incompatible := []string{}

	alpmArch, err := dbExecutor.AlpmArchitectures()
	if err != nil {
		return err
	}

nextpkg:
	for base, srcinfo := range srcinfos {
		for _, arch := range srcinfo.Arch {
			if db.ArchIsSupported(alpmArch, arch) {
				continue nextpkg
			}
		}
		incompatible = append(incompatible, base)
	}

	if len(incompatible) > 0 {
		text.Warnln(gotext.Get("The following packages are not compatible with your architecture:"))

		for _, pkg := range incompatible {
			fmt.Print("  " + text.Cyan(pkg))
		}

		fmt.Println()

		if !text.ContinueTask(os.Stdin, gotext.Get("Try to build them anyway?"), true, settings.NoConfirm) {
			return &settings.ErrUserAbort{}
		}
	}

	return nil
}

func parsePackageList(ctx context.Context, cmdBuilder exe.ICmdBuilder,
	dir string,
) (pkgdests map[string]string, pkgVersion string, err error) {
	stdout, stderr, err := cmdBuilder.Capture(
		cmdBuilder.BuildMakepkgCmd(ctx, dir, "--packagelist"))
	if err != nil {
		return nil, "", fmt.Errorf("%s %w", stderr, err)
	}

	lines := strings.Split(stdout, "\n")
	pkgdests = make(map[string]string)

	for _, line := range lines {
		if line == "" {
			continue
		}

		fileName := filepath.Base(line)
		split := strings.Split(fileName, "-")

		if len(split) < 4 {
			return nil, "", errors.New(gotext.Get("cannot find package name: %v", split))
		}

		// pkgname-pkgver-pkgrel-arch.pkgext
		// This assumes 3 dashes after the pkgname, Will cause an error
		// if the PKGEXT contains a dash. Please no one do that.
		pkgName := strings.Join(split[:len(split)-3], "-")
		pkgVersion = strings.Join(split[len(split)-3:len(split)-1], "-")
		pkgdests[pkgName] = line
	}

	if len(pkgdests) == 0 {
		return nil, "", &NoPkgDestsFoundError{dir}
	}

	return pkgdests, pkgVersion, nil
}

func pkgbuildsToSkip(cfg *settings.Configuration, bases []dep.Base, targets stringset.StringSet) stringset.StringSet {
	toSkip := make(stringset.StringSet)

	for _, base := range bases {
		isTarget := false
		for _, pkg := range base {
			isTarget = isTarget || targets.Get(pkg.Name)
		}

		if (cfg.ReDownload == "yes" && isTarget) || cfg.ReDownload == "all" {
			continue
		}

		dir := filepath.Join(cfg.BuildDir, base.Pkgbase(), ".SRCINFO")
		pkgbuild, err := gosrc.ParseFile(dir)

		if err == nil {
			if db.VerCmp(pkgbuild.Version(), base.Version()) >= 0 {
				toSkip.Set(base.Pkgbase())
			}
		}
	}

	return toSkip
}

func gitMerge(ctx context.Context, cmdBuilder exe.ICmdBuilder, dir string) error {
	_, stderr, err := cmdBuilder.Capture(
		cmdBuilder.BuildGitCmd(ctx,
			dir, "reset", "--hard", "HEAD"))
	if err != nil {
		return errors.New(gotext.Get("error resetting %s: %s", dir, stderr))
	}

	_, stderr, err = cmdBuilder.Capture(
		cmdBuilder.BuildGitCmd(ctx,
			dir, "merge", "--no-edit", "--ff"))
	if err != nil {
		return errors.New(gotext.Get("error merging %s: %s", dir, stderr))
	}

	return nil
}

func mergePkgbuilds(ctx context.Context, cmdBuilder exe.ICmdBuilder, pkgbuildDirs map[string]string) error {
	for _, dir := range pkgbuildDirs {
		err := gitMerge(ctx, cmdBuilder, dir)
		if err != nil {
			return err
		}
	}

	return nil
}

func buildInstallPkgbuilds(
	ctx context.Context,
	cfg *settings.Configuration,
	cmdArgs *parser.Arguments,
	dbExecutor db.Executor,
	dp *dep.Pool,
	do *dep.Order,
	srcinfos map[string]*gosrc.Srcinfo,
	incompatible bool,
	conflicts stringset.MapStringSet, noDeps, noCheck bool,
) error {
	deps := make([]string, 0)
	exp := make([]string, 0)
	pkgArchives := make([]string, 0)
	oldConfirm := settings.NoConfirm
	settings.NoConfirm = true

	// remotenames: names of all non repo packages on the system
	remoteNames := dbExecutor.InstalledRemotePackageNames()
	localNames := dbExecutor.InstalledSyncPackageNames()

	// cache as a stringset. maybe make it return a string set in the first
	// place
	remoteNamesCache := stringset.FromSlice(remoteNames)
	localNamesCache := stringset.FromSlice(localNames)

	for i, base := range do.Aur {
		pkg := base.Pkgbase()
		dir := filepath.Join(cfg.BuildDir, pkg)
		built := true

		satisfied := true
	all:
		for _, pkg := range base {
			for _, dep := range dep.ComputeCombinedDepList(pkg, noDeps, noCheck) {
				if !dp.AlpmExecutor.LocalSatisfierExists(dep) {
					satisfied = false
					text.Warnln(gotext.Get("%s not satisfied, flushing install queue", dep))

					break all
				}
			}
		}

		if !satisfied || !cfg.BatchInstall {
			text.Debugln("non batch installing archives:", pkgArchives)
			errArchive := installPkgArchive(ctx, cfg.Runtime.CmdBuilder,
				cfg.Mode, cfg.Runtime.VCSStore, cmdArgs, pkgArchives)
			errReason := setInstallReason(ctx, cfg.Runtime.CmdBuilder, cfg.Mode, cmdArgs, deps, exp)

			deps = make([]string, 0)
			exp = make([]string, 0)
			pkgArchives = make([]string, 0) // reset the pkgarchives

			if errArchive != nil || errReason != nil {
				if i != 0 {
					go cfg.Runtime.VCSStore.RemovePackages([]string{do.Aur[i-1].String()})
				}

				if errArchive != nil {
					return errArchive
				}

				return errReason
			}
		}

		srcInfo := srcinfos[pkg]

		args := []string{"--nobuild", "-fC"}

		if incompatible {
			args = append(args, "--ignorearch")
		}

		// pkgver bump
		if err := cfg.Runtime.CmdBuilder.Show(
			cfg.Runtime.CmdBuilder.BuildMakepkgCmd(ctx, dir, args...)); err != nil {
			return errors.New(gotext.Get("error making: %s", base.String()))
		}

		pkgdests, pkgVersion, errList := parsePackageList(ctx, cfg.Runtime.CmdBuilder, dir)
		if errList != nil {
			return errList
		}

		isExplicit := false
		for _, b := range base {
			isExplicit = isExplicit || dp.Explicit.Get(b.Name)
		}

		if cfg.ReBuild == "no" || (cfg.ReBuild == "yes" && !isExplicit) {
			for _, split := range base {
				pkgdest, ok := pkgdests[split.Name]
				if !ok {
					return &PkgDestNotInListError{split.Name}
				}

				if _, errStat := os.Stat(pkgdest); os.IsNotExist(errStat) {
					built = false
				} else if errStat != nil {
					return errStat
				}
			}
		} else {
			built = false
		}

		if cmdArgs.ExistsArg("needed") {
			installed := true
			for _, split := range base {
				installed = dp.AlpmExecutor.IsCorrectVersionInstalled(split.Name, pkgVersion)
			}

			if installed {
				err := cfg.Runtime.CmdBuilder.Show(
					cfg.Runtime.CmdBuilder.BuildMakepkgCmd(ctx,
						dir, "-c", "--nobuild", "--noextract", "--ignorearch"))
				if err != nil {
					return errors.New(gotext.Get("error making: %s", err))
				}

				fmt.Fprintln(os.Stdout, gotext.Get("%s is up to date -- skipping", text.Cyan(pkg+"-"+pkgVersion)))

				continue
			}
		}

		if built {
			err := cfg.Runtime.CmdBuilder.Show(
				cfg.Runtime.CmdBuilder.BuildMakepkgCmd(ctx,
					dir, "-c", "--nobuild", "--noextract", "--ignorearch"))
			if err != nil {
				return errors.New(gotext.Get("error making: %s", err))
			}

			text.Warnln(gotext.Get("%s already made -- skipping build", text.Cyan(pkg+"-"+pkgVersion)))
		} else {
			args := []string{"-cf", "--noconfirm", "--noextract", "--noprepare", "--holdver"}

			if incompatible {
				args = append(args, "--ignorearch")
			}

			if errMake := cfg.Runtime.CmdBuilder.Show(
				cfg.Runtime.CmdBuilder.BuildMakepkgCmd(ctx,
					dir, args...)); errMake != nil {
				return errors.New(gotext.Get("error making: %s", base.String()))
			}
		}

		// conflicts have been checked so answer y for them
		if cfg.UseAsk && cmdArgs.ExistsArg("ask") {
			ask, _ := strconv.Atoi(cmdArgs.Options["ask"].First())
			uask := alpm.QuestionType(ask) | alpm.QuestionTypeConflictPkg
			cmdArgs.Options["ask"].Set(fmt.Sprint(uask))
		} else {
			for _, split := range base {
				if _, ok := conflicts[split.Name]; ok {
					settings.NoConfirm = false

					break
				}
			}
		}

		var errAdd error

		for _, split := range base {
			for suffix, optional := range map[string]bool{"": false, "-debug": true} {
				deps, exp, pkgArchives, errAdd = doAddTarget(dp, localNamesCache, remoteNamesCache,
					cmdArgs, pkgdests, deps, exp, split.Name+suffix, optional, pkgArchives)
				if errAdd != nil {
					return errAdd
				}
			}
		}
		text.Debugln("deps:", deps, "exp:", exp, "pkgArchives:", pkgArchives)

		var wg sync.WaitGroup

		for _, pkg := range base {
			if srcInfo == nil {
				text.Errorln(gotext.Get("could not find srcinfo for: %s", pkg.Name))
				break
			}

			wg.Add(1)

			text.Debugln("checking vcs store for:", pkg.Name)
			go func(name string) {
				cfg.Runtime.VCSStore.Update(ctx, name, srcInfo.Source)
				wg.Done()
			}(pkg.Name)
		}

		wg.Wait()
	}

	text.Debugln("installing archives:", pkgArchives)
	errArchive := installPkgArchive(ctx, cfg.Runtime.CmdBuilder, cfg.Mode, cfg.Runtime.VCSStore, cmdArgs, pkgArchives)
	if errArchive != nil {
		go cfg.Runtime.VCSStore.RemovePackages([]string{do.Aur[len(do.Aur)-1].String()})
	}

	errReason := setInstallReason(ctx, cfg.Runtime.CmdBuilder, cfg.Mode, cmdArgs, deps, exp)
	if errReason != nil {
		go cfg.Runtime.VCSStore.RemovePackages([]string{do.Aur[len(do.Aur)-1].String()})
	}

	settings.NoConfirm = oldConfirm

	return nil
}

func installPkgArchive(ctx context.Context,
	cmdBuilder exe.ICmdBuilder,
	mode parser.TargetMode,
	vcsStore vcs.Store,
	cmdArgs *parser.Arguments,
	pkgArchives []string,
) error {
	if len(pkgArchives) == 0 {
		return nil
	}

	arguments := cmdArgs.Copy()
	arguments.ClearTargets()
	arguments.Op = "U"
	arguments.DelArg("confirm")
	arguments.DelArg("noconfirm")
	arguments.DelArg("c", "clean")
	arguments.DelArg("i", "install")
	arguments.DelArg("q", "quiet")
	arguments.DelArg("y", "refresh")
	arguments.DelArg("u", "sysupgrade")
	arguments.DelArg("w", "downloadonly")
	arguments.DelArg("asdeps", "asdep")
	arguments.DelArg("asexplicit", "asexp")

	arguments.AddTarget(pkgArchives...)

	if errShow := cmdBuilder.Show(cmdBuilder.BuildPacmanCmd(ctx,
		arguments, mode, settings.NoConfirm)); errShow != nil {
		return errShow
	}

	if errStore := vcsStore.Save(); errStore != nil {
		fmt.Fprintln(os.Stderr, errStore)
	}

	return nil
}

func setInstallReason(ctx context.Context,
	cmdBuilder exe.ICmdBuilder, mode parser.TargetMode,
	cmdArgs *parser.Arguments, deps, exps []string,
) error {
	if len(deps)+len(exps) == 0 {
		return nil
	}

	if errDeps := asdeps(ctx, cmdBuilder, mode, cmdArgs, deps); errDeps != nil {
		return errDeps
	}

	return asexp(ctx, cmdBuilder, mode, cmdArgs, exps)
}

func doAddTarget(dp *dep.Pool, localNamesCache, remoteNamesCache stringset.StringSet,
	cmdArgs *parser.Arguments, pkgdests map[string]string,
	deps, exp []string, name string, optional bool, pkgArchives []string,
) (newDeps, newExp, newPkgArchives []string, err error) {
	pkgdest, ok := pkgdests[name]
	if !ok {
		if optional {
			return deps, exp, pkgArchives, nil
		}

		return deps, exp, pkgArchives, &PkgDestNotInListError{name}
	}

	if _, errStat := os.Stat(pkgdest); os.IsNotExist(errStat) {
		if optional {
			return deps, exp, pkgArchives, nil
		}

		return deps, exp, pkgArchives, &FindPkgDestError{pkgDest: pkgdest, name: name}
	}

	pkgArchives = append(pkgArchives, pkgdest)

	switch {
	case cmdArgs.ExistsArg("asdeps", "asdep"):
		deps = append(deps, name)
	case cmdArgs.ExistsArg("asexplicit", "asexp"):
		exp = append(exp, name)
	case !dp.Explicit.Get(name) && !localNamesCache.Get(name) && !remoteNamesCache.Get(name):
		deps = append(deps, name)
	default:
		exp = append(exp, name)
	}

	return deps, exp, pkgArchives, nil
}
