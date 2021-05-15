package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	alpm "github.com/Jguer/go-alpm/v2"
	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/completion"
	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/dep"
	"github.com/Jguer/yay/v10/pkg/intrange"
	"github.com/Jguer/yay/v10/pkg/multierror"
	"github.com/Jguer/yay/v10/pkg/pgp"
	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/stringset"
	"github.com/Jguer/yay/v10/pkg/text"
)

const gitEmptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

func asdeps(cmdArgs *settings.Arguments, pkgs []string) (err error) {
	if len(pkgs) == 0 {
		return nil
	}

	cmdArgs = cmdArgs.CopyGlobal()
	_ = cmdArgs.AddArg("q", "D", "asdeps")
	cmdArgs.AddTarget(pkgs...)
	err = config.Runtime.CmdRunner.Show(passToPacman(cmdArgs))
	if err != nil {
		return fmt.Errorf(gotext.Get("error updating package install reason to dependency"))
	}

	return nil
}

func asexp(cmdArgs *settings.Arguments, pkgs []string) (err error) {
	if len(pkgs) == 0 {
		return nil
	}

	cmdArgs = cmdArgs.CopyGlobal()
	_ = cmdArgs.AddArg("q", "D", "asexplicit")
	cmdArgs.AddTarget(pkgs...)
	err = config.Runtime.CmdRunner.Show(passToPacman(cmdArgs))
	if err != nil {
		return fmt.Errorf(gotext.Get("error updating package install reason to explicit"))
	}

	return nil
}

// Install handles package installs
func install(cmdArgs *settings.Arguments, dbExecutor db.Executor, ignoreProviders bool) (err error) {
	var incompatible stringset.StringSet
	var do *dep.Order

	var srcinfos map[string]*gosrc.Srcinfo
	noDeps := cmdArgs.ExistsDouble("d", "nodeps")
	noCheck := strings.Contains(config.MFlags, "--nocheck")
	sysupgradeArg := cmdArgs.ExistsArg("u", "sysupgrade")
	refreshArg := cmdArgs.ExistsArg("y", "refresh")
	warnings := query.NewWarnings()

	if noDeps {
		config.Runtime.CmdBuilder.MakepkgFlags = append(config.Runtime.CmdBuilder.MakepkgFlags, "-d")
	}

	if config.Runtime.Mode == settings.ModeAny || config.Runtime.Mode == settings.ModeRepo {
		if config.CombinedUpgrade {
			if refreshArg {
				err = earlyRefresh(cmdArgs)
				if err != nil {
					return fmt.Errorf(gotext.Get("error refreshing databases"))
				}
			}
		} else if refreshArg || sysupgradeArg || len(cmdArgs.Targets) > 0 {
			err = earlyPacmanCall(cmdArgs, dbExecutor)
			if err != nil {
				return err
			}
		}
	}

	// we may have done -Sy, our handle now has an old
	// database.
	err = dbExecutor.RefreshHandle()
	if err != nil {
		return err
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

	if config.Runtime.Mode == settings.ModeAUR {
		arguments.DelArg("u", "sysupgrade")
	}

	// if we are doing -u also request all packages needing update
	if sysupgradeArg {
		ignore, targets, errUp := sysupgradeTargets(dbExecutor, cmdArgs.ExistsDouble("u", "sysupgrade"))
		if errUp != nil {
			return errUp
		}

		for _, up := range targets {
			cmdArgs.AddTarget(up)
			requestTargets = append(requestTargets, up)
		}

		if len(ignore) > 0 {
			arguments.CreateOrAppendOption("ignore", ignore.ToSlice()...)
		}
	}

	targets := stringset.FromSlice(cmdArgs.Targets)

	dp, err := dep.GetPool(requestTargets,
		warnings, dbExecutor, config.Runtime.Mode,
		ignoreProviders, settings.NoConfirm, config.Provides, config.ReBuild, config.RequestSplitN, noDeps, noCheck)
	if err != nil {
		return err
	}

	err = dp.CheckMissing(noDeps, noCheck)
	if err != nil {
		return err
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
		return config.Runtime.CmdRunner.Show(passToPacman(cmdArgs))
	}

	if len(dp.Aur) > 0 && os.Geteuid() == 0 {
		return fmt.Errorf(gotext.Get("refusing to install AUR packages as root, aborting"))
	}

	conflicts, err := dp.CheckConflicts(config.UseAsk, settings.NoConfirm, noDeps)
	if err != nil {
		return err
	}

	do = dep.GetOrder(dp, noDeps, noCheck)
	if err != nil {
		return err
	}

	for _, pkg := range do.Repo {
		arguments.AddTarget(pkg.DB().Name() + "/" + pkg.Name())
	}

	for _, pkg := range dp.Groups {
		arguments.AddTarget(pkg)
	}

	if len(do.Aur) == 0 && len(arguments.Targets) == 0 && (!cmdArgs.ExistsArg("u", "sysupgrade") || config.Runtime.Mode == settings.ModeAUR) {
		fmt.Println(gotext.Get(" there is nothing to do"))
		return nil
	}

	do.Print()
	fmt.Println()

	if config.CleanAfter {
		defer cleanAfter(do.Aur)
	}

	if do.HasMake() {
		switch config.RemoveMake {
		case "yes":
			defer func() {
				err = removeMake(do)
			}()

		case "no":
			break
		default:
			if text.ContinueTask(gotext.Get("Remove make dependencies after install?"), false, settings.NoConfirm) {
				defer func() {
					err = removeMake(do)
				}()
			}
		}
	}

	if config.CleanMenu {
		if anyExistInCache(do.Aur) {
			askClean := pkgbuildNumberMenu(do.Aur, remoteNamesCache)
			toClean, errClean := cleanNumberMenu(do.Aur, remoteNamesCache, askClean)
			if errClean != nil {
				return errClean
			}

			cleanBuilds(toClean)
		}
	}

	toSkip := pkgbuildsToSkip(do.Aur, targets)
	cloned, err := downloadPkgbuilds(do.Aur, toSkip, config.BuildDir)
	if err != nil {
		return err
	}

	var toDiff []dep.Base
	var toEdit []dep.Base

	if config.DiffMenu {
		pkgbuildNumberMenu(do.Aur, remoteNamesCache)
		toDiff, err = diffNumberMenu(do.Aur, remoteNamesCache)
		if err != nil {
			return err
		}

		if len(toDiff) > 0 {
			err = showPkgbuildDiffs(toDiff, cloned)
			if err != nil {
				return err
			}
		}
	}

	if len(toDiff) > 0 {
		oldValue := settings.NoConfirm
		settings.NoConfirm = false
		fmt.Println()
		if !text.ContinueTask(gotext.Get("Proceed with install?"), true, settings.NoConfirm) {
			return fmt.Errorf(gotext.Get("aborting due to user"))
		}
		err = updatePkgbuildSeenRef(toDiff)
		if err != nil {
			text.Errorln(err.Error())
		}

		settings.NoConfirm = oldValue
	}

	err = mergePkgbuilds(do.Aur)
	if err != nil {
		return err
	}

	srcinfos, err = parseSrcinfoFiles(do.Aur, true)
	if err != nil {
		return err
	}

	if config.EditMenu {
		pkgbuildNumberMenu(do.Aur, remoteNamesCache)
		toEdit, err = editNumberMenu(do.Aur, remoteNamesCache)
		if err != nil {
			return err
		}

		if len(toEdit) > 0 {
			err = editPkgbuilds(toEdit, srcinfos)
			if err != nil {
				return err
			}
		}
	}

	if len(toEdit) > 0 {
		oldValue := settings.NoConfirm
		settings.NoConfirm = false
		fmt.Println()
		if !text.ContinueTask(gotext.Get("Proceed with install?"), true, settings.NoConfirm) {
			return errors.New(gotext.Get("aborting due to user"))
		}
		settings.NoConfirm = oldValue
	}

	incompatible, err = getIncompatible(do.Aur, srcinfos, dbExecutor)
	if err != nil {
		return err
	}

	if config.PGPFetch {
		err = pgp.CheckPgpKeys(do.Aur, srcinfos, config.GpgBin, config.GpgFlags, settings.NoConfirm)
		if err != nil {
			return err
		}
	}

	if !config.CombinedUpgrade {
		arguments.DelArg("u", "sysupgrade")
	}

	if len(arguments.Targets) > 0 || arguments.ExistsArg("u") {
		if errShow := config.Runtime.CmdRunner.Show(passToPacman(arguments)); errShow != nil {
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

		if errDeps := asdeps(cmdArgs, deps); errDeps != nil {
			return errDeps
		}
		if errExp := asexp(cmdArgs, exp); errExp != nil {
			return errExp
		}
	}

	go func() {
		_ = completion.Update(dbExecutor, config.AURURL, config.Runtime.CompletionPath, config.CompletionInterval, false)
	}()

	err = downloadPkgbuildsSources(do.Aur, incompatible)
	if err != nil {
		return err
	}

	err = buildInstallPkgbuilds(cmdArgs, dbExecutor, dp, do, srcinfos, incompatible, conflicts, noDeps, noCheck)
	if err != nil {
		return err
	}

	return nil
}

func removeMake(do *dep.Order) error {
	removeArguments := settings.MakeArguments()
	err := removeArguments.AddArg("R", "u")
	if err != nil {
		return err
	}

	for _, pkg := range do.GetMake() {
		removeArguments.AddTarget(pkg)
	}

	oldValue := settings.NoConfirm
	settings.NoConfirm = true
	err = config.Runtime.CmdRunner.Show(passToPacman(removeArguments))
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

func earlyPacmanCall(cmdArgs *settings.Arguments, dbExecutor db.Executor) error {
	arguments := cmdArgs.Copy()
	arguments.Op = "S"
	targets := cmdArgs.Targets
	cmdArgs.ClearTargets()
	arguments.ClearTargets()

	if config.Runtime.Mode == settings.ModeRepo {
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
		if err := config.Runtime.CmdRunner.Show(passToPacman(arguments)); err != nil {
			return errors.New(gotext.Get("error installing repo packages"))
		}
	}

	return nil
}

func earlyRefresh(cmdArgs *settings.Arguments) error {
	arguments := cmdArgs.Copy()
	cmdArgs.DelArg("y", "refresh")
	arguments.DelArg("u", "sysupgrade")
	arguments.DelArg("s", "search")
	arguments.DelArg("i", "info")
	arguments.DelArg("l", "list")
	arguments.ClearTargets()
	return config.Runtime.CmdRunner.Show(passToPacman(arguments))
}

func alpmArchIsSupported(alpmArch []string, arch string) bool {
	if arch == "any" {
		return true
	}

	for _, a := range alpmArch {
		if a == arch {
			return true
		}
	}

	return false
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
			if alpmArchIsSupported(alpmArch, arch) {
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

		if !text.ContinueTask(gotext.Get("Try to build them anyway?"), true, settings.NoConfirm) {
			return nil, errors.New(gotext.Get("aborting due to user"))
		}
	}

	return incompatible, nil
}

func parsePackageList(dir string) (pkgdests map[string]string, pkgVersion string, err error) {
	stdout, stderr, err := config.Runtime.CmdRunner.Capture(
		config.Runtime.CmdBuilder.BuildMakepkgCmd(dir, "--packagelist"), 0)
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

func anyExistInCache(bases []dep.Base) bool {
	for _, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)

		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			return true
		}
	}

	return false
}

func pkgbuildNumberMenu(bases []dep.Base, installed stringset.StringSet) bool {
	toPrint := ""
	askClean := false

	for n, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)

		toPrint += fmt.Sprintf(text.Magenta("%3d")+" %-40s", len(bases)-n,
			text.Bold(base.String()))

		anyInstalled := false
		for _, b := range base {
			anyInstalled = anyInstalled || installed.Get(b.Name)
		}

		if anyInstalled {
			toPrint += text.Bold(text.Green(gotext.Get(" (Installed)")))
		}

		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			toPrint += text.Bold(text.Green(gotext.Get(" (Build Files Exist)")))
			askClean = true
		}

		toPrint += "\n"
	}

	fmt.Print(toPrint)

	return askClean
}

func cleanNumberMenu(bases []dep.Base, installed stringset.StringSet, hasClean bool) ([]dep.Base, error) {
	toClean := make([]dep.Base, 0)

	if !hasClean {
		return toClean, nil
	}

	text.Infoln(gotext.Get("Packages to cleanBuild?"))
	text.Infoln(gotext.Get("%s [A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)", text.Cyan(gotext.Get("[N]one"))))
	cleanInput, err := getInput(config.AnswerClean)
	if err != nil {
		return nil, err
	}

	cInclude, cExclude, cOtherInclude, cOtherExclude := intrange.ParseNumberMenu(cleanInput)
	cIsInclude := len(cExclude) == 0 && len(cOtherExclude) == 0

	if cOtherInclude.Get("abort") || cOtherInclude.Get("ab") {
		return nil, fmt.Errorf(gotext.Get("aborting due to user"))
	}

	if !cOtherInclude.Get("n") && !cOtherInclude.Get("none") {
		for i, base := range bases {
			pkg := base.Pkgbase()
			anyInstalled := false
			for _, b := range base {
				anyInstalled = anyInstalled || installed.Get(b.Name)
			}

			dir := filepath.Join(config.BuildDir, pkg)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				continue
			}

			if !cIsInclude && cExclude.Get(len(bases)-i) {
				continue
			}

			if anyInstalled && (cOtherInclude.Get("i") || cOtherInclude.Get("installed")) {
				toClean = append(toClean, base)
				continue
			}

			if !anyInstalled && (cOtherInclude.Get("no") || cOtherInclude.Get("notinstalled")) {
				toClean = append(toClean, base)
				continue
			}

			if cOtherInclude.Get("a") || cOtherInclude.Get("all") {
				toClean = append(toClean, base)
				continue
			}

			if cIsInclude && (cInclude.Get(len(bases)-i) || cOtherInclude.Get(pkg)) {
				toClean = append(toClean, base)
				continue
			}

			if !cIsInclude && (!cExclude.Get(len(bases)-i) && !cOtherExclude.Get(pkg)) {
				toClean = append(toClean, base)
				continue
			}
		}
	}

	return toClean, nil
}

func editNumberMenu(bases []dep.Base, installed stringset.StringSet) ([]dep.Base, error) {
	return editDiffNumberMenu(bases, installed, false)
}

func diffNumberMenu(bases []dep.Base, installed stringset.StringSet) ([]dep.Base, error) {
	return editDiffNumberMenu(bases, installed, true)
}

func editDiffNumberMenu(bases []dep.Base, installed stringset.StringSet, diff bool) ([]dep.Base, error) {
	toEdit := make([]dep.Base, 0)
	var editInput string
	var err error

	if diff {
		text.Infoln(gotext.Get("Diffs to show?"))
		text.Infoln(gotext.Get("%s [A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)", text.Cyan(gotext.Get("[N]one"))))
		editInput, err = getInput(config.AnswerDiff)
		if err != nil {
			return nil, err
		}
	} else {
		text.Infoln(gotext.Get("PKGBUILDs to edit?"))
		text.Infoln(gotext.Get("%s [A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)", text.Cyan(gotext.Get("[N]one"))))
		editInput, err = getInput(config.AnswerEdit)
		if err != nil {
			return nil, err
		}
	}

	eInclude, eExclude, eOtherInclude, eOtherExclude := intrange.ParseNumberMenu(editInput)
	eIsInclude := len(eExclude) == 0 && len(eOtherExclude) == 0

	if eOtherInclude.Get("abort") || eOtherInclude.Get("ab") {
		return nil, fmt.Errorf(gotext.Get("aborting due to user"))
	}

	if !eOtherInclude.Get("n") && !eOtherInclude.Get("none") {
		for i, base := range bases {
			pkg := base.Pkgbase()
			anyInstalled := false
			for _, b := range base {
				anyInstalled = anyInstalled || installed.Get(b.Name)
			}

			if !eIsInclude && eExclude.Get(len(bases)-i) {
				continue
			}

			if anyInstalled && (eOtherInclude.Get("i") || eOtherInclude.Get("installed")) {
				toEdit = append(toEdit, base)
				continue
			}

			if !anyInstalled && (eOtherInclude.Get("no") || eOtherInclude.Get("notinstalled")) {
				toEdit = append(toEdit, base)
				continue
			}

			if eOtherInclude.Get("a") || eOtherInclude.Get("all") {
				toEdit = append(toEdit, base)
				continue
			}

			if eIsInclude && (eInclude.Get(len(bases)-i) || eOtherInclude.Get(pkg)) {
				toEdit = append(toEdit, base)
			}

			if !eIsInclude && (!eExclude.Get(len(bases)-i) && !eOtherExclude.Get(pkg)) {
				toEdit = append(toEdit, base)
			}
		}
	}

	return toEdit, nil
}

func updatePkgbuildSeenRef(bases []dep.Base) error {
	var errMulti multierror.MultiError
	for _, base := range bases {
		pkg := base.Pkgbase()
		err := gitUpdateSeenRef(config.BuildDir, pkg)
		if err != nil {
			errMulti.Add(err)
		}
	}
	return errMulti.Return()
}

func showPkgbuildDiffs(bases []dep.Base, cloned stringset.StringSet) error {
	var errMulti multierror.MultiError
	for _, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)
		start, err := getLastSeenHash(config.BuildDir, pkg)
		if err != nil {
			errMulti.Add(err)
			continue
		}

		if cloned.Get(pkg) {
			start = gitEmptyTree
		} else {
			hasDiff, err := gitHasDiff(config.BuildDir, pkg)
			if err != nil {
				errMulti.Add(err)
				continue
			}

			if !hasDiff {
				text.Warnln(gotext.Get("%s: No changes -- skipping", text.Cyan(base.String())))
				continue
			}
		}

		args := []string{
			"diff",
			start + "..HEAD@{upstream}", "--src-prefix",
			dir + "/", "--dst-prefix", dir + "/", "--", ".", ":(exclude).SRCINFO",
		}
		if text.UseColor {
			args = append(args, "--color=always")
		} else {
			args = append(args, "--color=never")
		}
		_ = config.Runtime.CmdRunner.Show(config.Runtime.CmdBuilder.BuildGitCmd(dir, args...))
	}

	return errMulti.Return()
}

func editPkgbuilds(bases []dep.Base, srcinfos map[string]*gosrc.Srcinfo) error {
	pkgbuilds := make([]string, 0, len(bases))
	for _, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)
		pkgbuilds = append(pkgbuilds, filepath.Join(dir, "PKGBUILD"))

		for _, splitPkg := range srcinfos[pkg].SplitPackages() {
			if splitPkg.Install != "" {
				pkgbuilds = append(pkgbuilds, filepath.Join(dir, splitPkg.Install))
			}
		}
	}

	if len(pkgbuilds) > 0 {
		editor, editorArgs := editor()
		editorArgs = append(editorArgs, pkgbuilds...)
		editcmd := exec.Command(editor, editorArgs...)
		editcmd.Stdin, editcmd.Stdout, editcmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		err := editcmd.Run()
		if err != nil {
			return errors.New(gotext.Get("editor did not exit successfully, aborting: %s", err))
		}
	}

	return nil
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
			if alpm.VerCmp(pkgbuild.Version(), base.Version()) >= 0 {
				toSkip.Set(base.Pkgbase())
			}
		}
	}

	return toSkip
}

func mergePkgbuilds(bases []dep.Base) error {
	for _, base := range bases {
		err := gitMerge(config.BuildDir, base.Pkgbase())
		if err != nil {
			return err
		}
	}

	return nil
}

func downloadPkgbuilds(bases []dep.Base, toSkip stringset.StringSet, buildDir string) (stringset.StringSet, error) {
	cloned := make(stringset.StringSet)
	downloaded := 0
	var wg sync.WaitGroup
	var mux sync.Mutex
	var errs multierror.MultiError

	download := func(base dep.Base) {
		defer wg.Done()
		pkg := base.Pkgbase()

		if toSkip.Get(pkg) {
			mux.Lock()
			downloaded++
			text.OperationInfoln(
				gotext.Get("PKGBUILD up to date, Skipping (%d/%d): %s",
					downloaded, len(bases), text.Cyan(base.String())))
			mux.Unlock()
			return
		}

		clone, err := gitDownload(config.AURURL+"/"+pkg+".git", buildDir, pkg)
		if err != nil {
			errs.Add(err)
			return
		}
		if clone {
			mux.Lock()
			cloned.Set(pkg)
			mux.Unlock()
		}

		mux.Lock()
		downloaded++
		text.OperationInfoln(gotext.Get("Downloaded PKGBUILD (%d/%d): %s", downloaded, len(bases), text.Cyan(base.String())))
		mux.Unlock()
	}

	count := 0
	for _, base := range bases {
		wg.Add(1)
		go download(base)
		count++
		if count%25 == 0 {
			wg.Wait()
		}
	}

	wg.Wait()

	return cloned, errs.Return()
}

func downloadPkgbuildsSources(bases []dep.Base, incompatible stringset.StringSet) (err error) {
	for _, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)
		args := []string{"--verifysource", "-Ccf"}

		if incompatible.Get(pkg) {
			args = append(args, "--ignorearch")
		}

		err = config.Runtime.CmdRunner.Show(
			config.Runtime.CmdBuilder.BuildMakepkgCmd(dir, args...))
		if err != nil {
			return errors.New(gotext.Get("error downloading sources: %s", text.Cyan(base.String())))
		}
	}

	return
}

func buildInstallPkgbuilds(
	cmdArgs *settings.Arguments,
	dbExecutor db.Executor,
	dp *dep.Pool,
	do *dep.Order,
	srcinfos map[string]*gosrc.Srcinfo,
	incompatible stringset.StringSet,
	conflicts stringset.MapStringSet, noDeps, noCheck bool,
) error {
	arguments := cmdArgs.Copy()
	arguments.ClearTargets()
	arguments.Op = "U"
	arguments.DelArg("confirm")
	arguments.DelArg("noconfirm")
	arguments.DelArg("c", "clean")
	arguments.DelArg("q", "quiet")
	arguments.DelArg("q", "quiet")
	arguments.DelArg("y", "refresh")
	arguments.DelArg("u", "sysupgrade")
	arguments.DelArg("w", "downloadonly")

	deps := make([]string, 0)
	exp := make([]string, 0)
	oldConfirm := settings.NoConfirm
	settings.NoConfirm = true

	//remotenames: names of all non repo packages on the system
	localNames, remoteNames, err := query.GetPackageNamesBySource(dbExecutor)
	if err != nil {
		return err
	}

	// cache as a stringset. maybe make it return a string set in the first
	// place
	remoteNamesCache := stringset.FromSlice(remoteNames)
	localNamesCache := stringset.FromSlice(localNames)

	doInstall := func() error {
		if len(arguments.Targets) == 0 {
			return nil
		}

		if errShow := config.Runtime.CmdRunner.Show(passToPacman(arguments)); errShow != nil {
			return errShow
		}

		if errStore := config.Runtime.VCSStore.Save(); err != nil {
			fmt.Fprintln(os.Stderr, errStore)
		}

		if errDeps := asdeps(cmdArgs, deps); err != nil {
			return errDeps
		}
		if errExps := asexp(cmdArgs, exp); err != nil {
			return errExps
		}

		settings.NoConfirm = oldConfirm

		arguments.ClearTargets()
		deps = make([]string, 0)
		exp = make([]string, 0)
		settings.NoConfirm = true
		return nil
	}

	for _, base := range do.Aur {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)
		built := true

		satisfied := true
	all:
		for _, pkg := range base {
			for _, deps := range dep.ComputeCombinedDepList(pkg, noDeps, noCheck) {
				for _, dep := range deps {
					if !dp.AlpmExecutor.LocalSatisfierExists(dep) {
						satisfied = false
						text.Warnln(gotext.Get("%s not satisfied, flushing install queue", dep))
						break all
					}
				}
			}
		}

		if !satisfied || !config.BatchInstall {
			err = doInstall()
			if err != nil {
				return err
			}
		}

		srcinfo := srcinfos[pkg]

		args := []string{"--nobuild", "-fC"}

		if incompatible.Get(pkg) {
			args = append(args, "--ignorearch")
		}

		// pkgver bump
		if err = config.Runtime.CmdRunner.Show(
			config.Runtime.CmdBuilder.BuildMakepkgCmd(dir, args...)); err != nil {
			return errors.New(gotext.Get("error making: %s", base.String()))
		}

		pkgdests, pkgVersion, errList := parsePackageList(dir)
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
				err = config.Runtime.CmdRunner.Show(
					config.Runtime.CmdBuilder.BuildMakepkgCmd(
						dir, "-c", "--nobuild", "--noextract", "--ignorearch"))
				if err != nil {
					return errors.New(gotext.Get("error making: %s", err))
				}

				fmt.Fprintln(os.Stdout, gotext.Get("%s is up to date -- skipping", text.Cyan(pkg+"-"+pkgVersion)))
				continue
			}
		}

		if built {
			err = config.Runtime.CmdRunner.Show(
				config.Runtime.CmdBuilder.BuildMakepkgCmd(
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

			if errMake := config.Runtime.CmdRunner.Show(
				config.Runtime.CmdBuilder.BuildMakepkgCmd(
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

		doAddTarget := func(name string, optional bool) error {
			pkgdest, ok := pkgdests[name]
			if !ok {
				if optional {
					return nil
				}

				return errors.New(gotext.Get("could not find PKGDEST for: %s", name))
			}

			if _, errStat := os.Stat(pkgdest); os.IsNotExist(errStat) {
				if optional {
					return nil
				}

				return errors.New(
					gotext.Get(
						"the PKGDEST for %s is listed by makepkg but does not exist: %s",
						name, pkgdest))
			}

			arguments.AddTarget(pkgdest)
			if cmdArgs.ExistsArg("asdeps", "asdep") {
				deps = append(deps, name)
			} else if cmdArgs.ExistsArg("asexplicit", "asexp") {
				exp = append(exp, name)
			} else if !dp.Explicit.Get(name) && !localNamesCache.Get(name) && !remoteNamesCache.Get(name) {
				deps = append(deps, name)
			}

			return nil
		}

		for _, split := range base {
			if errAdd := doAddTarget(split.Name, false); errAdd != nil {
				return errAdd
			}

			if errAddDebug := doAddTarget(split.Name+"-debug", true); errAddDebug != nil {
				return errAddDebug
			}
		}

		var mux sync.Mutex
		var wg sync.WaitGroup
		for _, pkg := range base {
			wg.Add(1)
			go config.Runtime.VCSStore.Update(pkg.Name, srcinfo.Source, &mux, &wg)
		}

		wg.Wait()
	}

	err = doInstall()
	settings.NoConfirm = oldConfirm
	return err
}
