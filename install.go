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
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/completion"
	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/download"
	"github.com/Jguer/yay/v11/pkg/menus"
	"github.com/Jguer/yay/v11/pkg/pgp"
	"github.com/Jguer/yay/v11/pkg/query"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/stringset"
	"github.com/Jguer/yay/v11/pkg/text"
)

func setPkgReason(ctx context.Context, cmdArgs *parser.Arguments, pkgs []string, exp bool) error {
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

	if err := config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, config.Runtime.Mode, settings.NoConfirm)); err != nil {
		return &SetPkgReasonError{exp: exp}
	}

	return nil
}

func asdeps(ctx context.Context, cmdArgs *parser.Arguments, pkgs []string) error {
	return setPkgReason(ctx, cmdArgs, pkgs, false)
}

func asexp(ctx context.Context, cmdArgs *parser.Arguments, pkgs []string) error {
	return setPkgReason(ctx, cmdArgs, pkgs, true)
}

// Install handles package installs.
func install(ctx context.Context, cmdArgs *parser.Arguments, dbExecutor db.Executor, ignoreProviders bool) error {
	var (
		incompatible    stringset.StringSet
		do              *dep.Order
		srcinfos        map[string]*gosrc.Srcinfo
		noDeps          = cmdArgs.ExistsDouble("d", "nodeps")
		noCheck         = strings.Contains(config.MFlags, "--nocheck")
		assumeInstalled = cmdArgs.GetArgs("assume-installed")
		sysupgradeArg   = cmdArgs.ExistsArg("u", "sysupgrade")
		refreshArg      = cmdArgs.ExistsArg("y", "refresh")
		warnings        = query.NewWarnings()
	)

	if noDeps {
		config.Runtime.CmdBuilder.AddMakepkgFlag("-d")
	}

	if config.Runtime.Mode.AtLeastRepo() {
		if config.CombinedUpgrade {
			if refreshArg {
				if errR := earlyRefresh(ctx, cmdArgs); errR != nil {
					return fmt.Errorf("%s - %w", gotext.Get("error refreshing databases"), errR)
				}
			}
		} else if refreshArg || sysupgradeArg || len(cmdArgs.Targets) > 0 {
			if errP := earlyPacmanCall(ctx, cmdArgs, dbExecutor); errP != nil {
				return errP
			}
		}
	}

	// we may have done -Sy, our handle now has an old
	// database.
	if errRefresh := dbExecutor.RefreshHandle(); errRefresh != nil {
		return errRefresh
	}

	localNames, remoteNames, err := query.GetPackageNamesBySource(dbExecutor)
	if err != nil {
		return err
	}

	remoteNamesCache := stringset.FromSlice(remoteNames)
	localNamesCache := stringset.FromSlice(localNames)

	requestTargets := cmdArgs.Copy().Targets

	// create the arguments to pass for the repo install
	arguments := cmdArgs.Copy()
	arguments.DelArg("asdeps", "asdep")
	arguments.DelArg("asexplicit", "asexp")
	arguments.Op = "S"
	arguments.ClearTargets()

	if config.Runtime.Mode == parser.ModeAUR {
		arguments.DelArg("u", "sysupgrade")
	}

	// if we are doing -u also request all packages needing update
	if sysupgradeArg {
		var errSysUp error
		requestTargets, errSysUp = addUpgradeTargetsToArgs(ctx, dbExecutor, cmdArgs, requestTargets, arguments)
		if errSysUp != nil {
			return errSysUp
		}
	}

	targets := stringset.FromSlice(cmdArgs.Targets)

	dp, err := dep.GetPool(ctx, requestTargets,
		warnings, dbExecutor, config.Runtime.AURClient, config.Runtime.Mode,
		ignoreProviders, settings.NoConfirm, config.Provides, config.ReBuild, config.RequestSplitN, noDeps, noCheck, assumeInstalled)
	if err != nil {
		return err
	}

	if errC := dp.CheckMissing(noDeps, noCheck); errC != nil {
		return errC
	}

	if len(dp.Aur) == 0 {
		if !config.CombinedUpgrade {
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

		return config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, config.Runtime.Mode, settings.NoConfirm))
	}

	conflicts, errCC := dp.CheckConflicts(config.UseAsk, settings.NoConfirm, noDeps)
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
		(!cmdArgs.ExistsArg("u", "sysupgrade") || config.Runtime.Mode == parser.ModeAUR) {
		fmt.Println(gotext.Get(" there is nothing to do"))
		return nil
	}

	do.Print()
	fmt.Println()

	if config.CleanAfter {
		defer func() {
			pkgbuildDirs := make([]string, 0, len(do.Aur))
			for _, base := range do.Aur {
				dir := filepath.Join(config.BuildDir, base.Pkgbase())
				if isGitRepository(dir) {
					pkgbuildDirs = append(pkgbuildDirs, dir)
				}
			}
			cleanAfter(ctx, config.Runtime.CmdBuilder, pkgbuildDirs)
		}()
	}

	if do.HasMake() {
		switch config.RemoveMake {
		case "yes":
			defer func() {
				err = removeMake(ctx, config.Runtime.CmdBuilder, do.GetMake())
			}()

		case "no":
			break
		default:
			if text.ContinueTask(os.Stdin, gotext.Get("Remove make dependencies after install?"), false, settings.NoConfirm) {
				defer func() {
					err = removeMake(ctx, config.Runtime.CmdBuilder, do.GetMake())
				}()
			}
		}
	}

	if errCleanMenu := menus.Clean(config.CleanMenu,
		config.BuildDir, do.Aur,
		remoteNamesCache, settings.NoConfirm, config.AnswerClean); errCleanMenu != nil {
		if errors.As(errCleanMenu, &settings.ErrUserAbort{}) {
			return errCleanMenu
		}

		text.Errorln(errCleanMenu)
	}

	toSkip := pkgbuildsToSkip(do.Aur, targets)
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
		config.Runtime.CmdBuilder, toClone, config.AURURL, config.BuildDir, false)
	if errA != nil {
		return errA
	}

	if errDiffMenu := menus.Diff(ctx, config.Runtime.CmdBuilder, config.BuildDir,
		config.DiffMenu, do.Aur, remoteNamesCache,
		cloned, settings.NoConfirm, config.AnswerDiff); errDiffMenu != nil {
		if errors.As(errDiffMenu, &settings.ErrUserAbort{}) {
			return errDiffMenu
		}

		text.Errorln(errDiffMenu)
	}

	if errM := mergePkgbuilds(ctx, do.Aur); errM != nil {
		return errM
	}

	srcinfos, err = parseSrcinfoFiles(do.Aur, true)
	if err != nil {
		return err
	}

	if errEditMenu := menus.Edit(config.EditMenu, config.BuildDir, do.Aur,
		config.Editor, config.EditorFlags, remoteNamesCache, srcinfos,
		settings.NoConfirm, config.AnswerEdit); errEditMenu != nil {
		if errors.As(errEditMenu, &settings.ErrUserAbort{}) {
			return errEditMenu
		}

		text.Errorln(errEditMenu)
	}

	incompatible, err = getIncompatible(do.Aur, srcinfos, dbExecutor)
	if err != nil {
		return err
	}

	if config.PGPFetch {
		if errCPK := pgp.CheckPgpKeys(do.Aur, srcinfos, config.GpgBin, config.GpgFlags, settings.NoConfirm); errCPK != nil {
			return errCPK
		}
	}

	if !config.CombinedUpgrade {
		arguments.DelArg("u", "sysupgrade")
	}

	if len(arguments.Targets) > 0 || arguments.ExistsArg("u") {
		if errShow := config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			arguments, config.Runtime.Mode, settings.NoConfirm)); errShow != nil {
			return errors.New(gotext.Get("error installing repo packages"))
		}

		deps := make([]string, 0)
		exp := make([]string, 0)

		for _, pkg := range do.Repo {
			if !dp.Explicit.Get(pkg.Name()) && !localNamesCache.Get(pkg.Name()) && !remoteNamesCache.Get(pkg.Name()) {
				deps = append(deps, pkg.Name())

				continue
			}

			if cmdArgs.ExistsArg("asdeps", "asdep") && dp.Explicit.Get(pkg.Name()) {
				deps = append(deps, pkg.Name())
			} else if cmdArgs.ExistsArg("asexp", "asexplicit") && dp.Explicit.Get(pkg.Name()) {
				exp = append(exp, pkg.Name())
			}
		}

		if errDeps := asdeps(ctx, cmdArgs, deps); errDeps != nil {
			return errDeps
		}

		if errExp := asexp(ctx, cmdArgs, exp); errExp != nil {
			return errExp
		}
	}

	go func() {
		_ = completion.Update(ctx, config.Runtime.HTTPClient, dbExecutor,
			config.AURURL, config.Runtime.CompletionPath, config.CompletionInterval, false)
	}()

	pkgBuildDirs := make(map[string]string, len(do.Aur))
	for _, base := range do.Aur {
		pkgBuildDirs[base.Pkgbase()] = filepath.Join(config.BuildDir, base.Pkgbase())
	}

	if errP := downloadPKGBUILDSourceFanout(ctx,
		config.Runtime.CmdBuilder,
		pkgBuildDirs,
		len(incompatible) > 0, config.MaxConcurrentDownloads); errP != nil {
		text.Errorln(errP)
	}

	if errB := buildInstallPkgbuilds(ctx, cmdArgs, dbExecutor, dp, do, srcinfos, incompatible, conflicts, noDeps, noCheck); errB != nil {
		return errB
	}

	return nil
}

func addUpgradeTargetsToArgs(ctx context.Context, dbExecutor db.Executor, cmdArgs *parser.Arguments, requestTargets []string, arguments *parser.Arguments) ([]string, error) {
	ignore, targets, errUp := sysupgradeTargets(ctx, dbExecutor, cmdArgs.ExistsDouble("u", "sysupgrade"))
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

func removeMake(ctx context.Context, cmdBuilder exe.ICmdBuilder, makeDeps []string) error {
	removeArguments := parser.MakeArguments()

	err := removeArguments.AddArg("R", "u")
	if err != nil {
		return err
	}

	for _, pkg := range makeDeps {
		removeArguments.AddTarget(pkg)
	}

	oldValue := settings.NoConfirm
	settings.NoConfirm = true
	err = cmdBuilder.Show(cmdBuilder.BuildPacmanCmd(ctx,
		removeArguments, config.Runtime.Mode, settings.NoConfirm))
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

func earlyPacmanCall(ctx context.Context, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	arguments := cmdArgs.Copy()
	arguments.Op = "S"
	targets := cmdArgs.Targets
	cmdArgs.ClearTargets()
	arguments.ClearTargets()

	if config.Runtime.Mode == parser.ModeRepo {
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
		if err := config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			arguments, config.Runtime.Mode, settings.NoConfirm)); err != nil {
			return errors.New(gotext.Get("error installing repo packages"))
		}
	}

	return nil
}

func earlyRefresh(ctx context.Context, cmdArgs *parser.Arguments) error {
	arguments := cmdArgs.Copy()
	cmdArgs.DelArg("y", "refresh")
	arguments.DelArg("u", "sysupgrade")
	arguments.DelArg("s", "search")
	arguments.DelArg("i", "info")
	arguments.DelArg("l", "list")
	arguments.ClearTargets()

	return config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
		arguments, config.Runtime.Mode, settings.NoConfirm))
}

func getIncompatible(bases []dep.Base, srcinfos map[string]*gosrc.Srcinfo, dbExecutor db.Executor) (stringset.StringSet, error) {
	incompatible := make(stringset.StringSet)
	basesMap := make(map[string]dep.Base)

	alpmArch, err := dbExecutor.AlpmArchitectures()
	if err != nil {
		return nil, err
	}

nextpkg:
	for _, base := range bases {
		for _, arch := range srcinfos[base.Pkgbase()].Arch {
			if db.ArchIsSupported(alpmArch, arch) {
				continue nextpkg
			}
		}

		incompatible.Set(base.Pkgbase())
		basesMap[base.Pkgbase()] = base
	}

	if len(incompatible) > 0 {
		text.Warnln(gotext.Get("The following packages are not compatible with your architecture:"))

		for pkg := range incompatible {
			fmt.Print("  " + text.Cyan(basesMap[pkg].String()))
		}

		fmt.Println()

		if !text.ContinueTask(os.Stdin, gotext.Get("Try to build them anyway?"), true, settings.NoConfirm) {
			return nil, &settings.ErrUserAbort{}
		}
	}

	return incompatible, nil
}

func parsePackageList(ctx context.Context, dir string) (pkgdests map[string]string, pkgVersion string, err error) {
	stdout, stderr, err := config.Runtime.CmdBuilder.Capture(
		config.Runtime.CmdBuilder.BuildMakepkgCmd(ctx, dir, "--packagelist"))
	if err != nil {
		return nil, "", fmt.Errorf("%s %s", stderr, err)
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

	return pkgdests, pkgVersion, nil
}

func parseSrcinfoFiles(bases []dep.Base, errIsFatal bool) (map[string]*gosrc.Srcinfo, error) {
	srcinfos := make(map[string]*gosrc.Srcinfo)

	for k, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)

		text.OperationInfoln(gotext.Get("(%d/%d) Parsing SRCINFO: %s", k+1, len(bases), text.Cyan(base.String())))

		pkgbuild, err := gosrc.ParseFile(filepath.Join(dir, ".SRCINFO"))
		if err != nil {
			if !errIsFatal {
				text.Warnln(gotext.Get("failed to parse %s -- skipping: %s", base.String(), err))
				continue
			}

			return nil, errors.New(gotext.Get("failed to parse %s: %s", base.String(), err))
		}

		srcinfos[pkg] = pkgbuild
	}

	return srcinfos, nil
}

func pkgbuildsToSkip(bases []dep.Base, targets stringset.StringSet) stringset.StringSet {
	toSkip := make(stringset.StringSet)

	for _, base := range bases {
		isTarget := false
		for _, pkg := range base {
			isTarget = isTarget || targets.Get(pkg.Name)
		}

		if (config.ReDownload == "yes" && isTarget) || config.ReDownload == "all" {
			continue
		}

		dir := filepath.Join(config.BuildDir, base.Pkgbase(), ".SRCINFO")
		pkgbuild, err := gosrc.ParseFile(dir)

		if err == nil {
			if db.VerCmp(pkgbuild.Version(), base.Version()) >= 0 {
				toSkip.Set(base.Pkgbase())
			}
		}
	}

	return toSkip
}

func gitMerge(ctx context.Context, path, name string) error {
	_, stderr, err := config.Runtime.CmdBuilder.Capture(
		config.Runtime.CmdBuilder.BuildGitCmd(ctx,
			filepath.Join(path, name), "reset", "--hard", "HEAD"))
	if err != nil {
		return errors.New(gotext.Get("error resetting %s: %s", name, stderr))
	}

	_, stderr, err = config.Runtime.CmdBuilder.Capture(
		config.Runtime.CmdBuilder.BuildGitCmd(ctx,
			filepath.Join(path, name), "merge", "--no-edit", "--ff"))
	if err != nil {
		return errors.New(gotext.Get("error merging %s: %s", name, stderr))
	}

	return nil
}

func mergePkgbuilds(ctx context.Context, bases []dep.Base) error {
	for _, base := range bases {
		err := gitMerge(ctx, config.BuildDir, base.Pkgbase())
		if err != nil {
			return err
		}
	}

	return nil
}

func buildInstallPkgbuilds(
	ctx context.Context,
	cmdArgs *parser.Arguments,
	dbExecutor db.Executor,
	dp *dep.Pool,
	do *dep.Order,
	srcinfos map[string]*gosrc.Srcinfo,
	incompatible stringset.StringSet,
	conflicts stringset.MapStringSet, noDeps, noCheck bool,
) error {
	deps := make([]string, 0)
	exp := make([]string, 0)
	pkgArchives := make([]string, 0)
	oldConfirm := settings.NoConfirm
	settings.NoConfirm = true

	// remotenames: names of all non repo packages on the system
	localNames, remoteNames, err := query.GetPackageNamesBySource(dbExecutor)
	if err != nil {
		return err
	}

	// cache as a stringset. maybe make it return a string set in the first
	// place
	remoteNamesCache := stringset.FromSlice(remoteNames)
	localNamesCache := stringset.FromSlice(localNames)

	for i, base := range do.Aur {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)
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

		if !satisfied || !config.BatchInstall {
			errArchive := installPkgArchive(ctx, cmdArgs, pkgArchives)
			errReason := setInstallReason(ctx, cmdArgs, deps, exp)

			deps = make([]string, 0)
			exp = make([]string, 0)
			pkgArchives = make([]string, 0) // reset the pkgarchives

			if errArchive != nil || errReason != nil {
				if i != 0 {
					go config.Runtime.VCSStore.RemovePackage([]string{do.Aur[i-1].String()})
				}

				if errArchive != nil {
					return errArchive
				}

				return errReason
			}
		}

		srcinfo := srcinfos[pkg]

		args := []string{"--nobuild", "-fC"}

		if incompatible.Get(pkg) {
			args = append(args, "--ignorearch")
		}

		// pkgver bump
		if err = config.Runtime.CmdBuilder.Show(
			config.Runtime.CmdBuilder.BuildMakepkgCmd(ctx, dir, args...)); err != nil {
			return errors.New(gotext.Get("error making: %s", base.String()))
		}

		pkgdests, pkgVersion, errList := parsePackageList(ctx, dir)
		if errList != nil {
			return errList
		}

		isExplicit := false
		for _, b := range base {
			isExplicit = isExplicit || dp.Explicit.Get(b.Name)
		}

		if config.ReBuild == "no" || (config.ReBuild == "yes" && !isExplicit) {
			for _, split := range base {
				pkgdest, ok := pkgdests[split.Name]
				if !ok {
					return errors.New(gotext.Get("could not find PKGDEST for: %s", split.Name))
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
				err = config.Runtime.CmdBuilder.Show(
					config.Runtime.CmdBuilder.BuildMakepkgCmd(ctx,
						dir, "-c", "--nobuild", "--noextract", "--ignorearch"))
				if err != nil {
					return errors.New(gotext.Get("error making: %s", err))
				}

				fmt.Fprintln(os.Stdout, gotext.Get("%s is up to date -- skipping", text.Cyan(pkg+"-"+pkgVersion)))

				continue
			}
		}

		if built {
			err = config.Runtime.CmdBuilder.Show(
				config.Runtime.CmdBuilder.BuildMakepkgCmd(ctx,
					dir, "-c", "--nobuild", "--noextract", "--ignorearch"))
			if err != nil {
				return errors.New(gotext.Get("error making: %s", err))
			}

			text.Warnln(gotext.Get("%s already made -- skipping build", text.Cyan(pkg+"-"+pkgVersion)))
		} else {
			args := []string{"-cf", "--noconfirm", "--noextract", "--noprepare", "--holdver"}

			if incompatible.Get(pkg) {
				args = append(args, "--ignorearch")
			}

			if errMake := config.Runtime.CmdBuilder.Show(
				config.Runtime.CmdBuilder.BuildMakepkgCmd(ctx,
					dir, args...)); errMake != nil {
				return errors.New(gotext.Get("error making: %s", base.String()))
			}
		}

		// conflicts have been checked so answer y for them
		if config.UseAsk && cmdArgs.ExistsArg("ask") {
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

		var (
			mux sync.Mutex
			wg  sync.WaitGroup
		)

		for _, pkg := range base {
			wg.Add(1)

			go config.Runtime.VCSStore.Update(ctx, pkg.Name, srcinfo.Source, &mux, &wg)
		}

		wg.Wait()
	}

	errArchive := installPkgArchive(ctx, cmdArgs, pkgArchives)
	if errArchive != nil {
		go config.Runtime.VCSStore.RemovePackage([]string{do.Aur[len(do.Aur)-1].String()})
	}

	errReason := setInstallReason(ctx, cmdArgs, deps, exp)
	if errReason != nil {
		go config.Runtime.VCSStore.RemovePackage([]string{do.Aur[len(do.Aur)-1].String()})
	}

	settings.NoConfirm = oldConfirm

	return err
}

func installPkgArchive(ctx context.Context, cmdArgs *parser.Arguments, pkgArchives []string) error {
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

	if len(pkgArchives) == 0 {
		return nil
	}

	arguments.AddTarget(pkgArchives...)

	if errShow := config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
		arguments, config.Runtime.Mode, settings.NoConfirm)); errShow != nil {
		return errShow
	}

	if errStore := config.Runtime.VCSStore.Save(); errStore != nil {
		fmt.Fprintln(os.Stderr, errStore)
	}

	return nil
}

func setInstallReason(ctx context.Context, cmdArgs *parser.Arguments, deps, exps []string) error {
	if len(deps)+len(exps) == 0 {
		return nil
	}

	if errDeps := asdeps(ctx, cmdArgs, deps); errDeps != nil {
		return errDeps
	}

	return asexp(ctx, cmdArgs, exps)
}

func doAddTarget(dp *dep.Pool, localNamesCache, remoteNamesCache stringset.StringSet,
	cmdArgs *parser.Arguments, pkgdests map[string]string,
	deps, exp []string, name string, optional bool, pkgArchives []string,
) (newDeps, newExp, newPkgArchives []string, err error) {
	pkgdest, ok := pkgdests[name]
	if !ok {
		if optional {
			return deps, exp, newPkgArchives, nil
		}

		return deps, exp, pkgArchives, errors.New(gotext.Get("could not find PKGDEST for: %s", name))
	}

	if _, errStat := os.Stat(pkgdest); os.IsNotExist(errStat) {
		if optional {
			return deps, exp, pkgArchives, nil
		}

		return deps, exp, pkgArchives, errors.New(
			gotext.Get(
				"the PKGDEST for %s is listed by makepkg but does not exist: %s",
				name, pkgdest))
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
