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

	alpm "github.com/Jguer/go-alpm"
	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/completion"
	"github.com/Jguer/yay/v10/pkg/dep"
	"github.com/Jguer/yay/v10/pkg/intrange"
	"github.com/Jguer/yay/v10/pkg/multierror"
	"github.com/Jguer/yay/v10/pkg/pgp"
	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/stringset"
	"github.com/Jguer/yay/v10/pkg/text"
)

func asdeps(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}

	args := config.Globals()
	args.Add("D", "asdeps")
	args.AddTarget(pkgs...)
	_, stderr, err := capture(passToPacman(args))
	if err != nil {
		return fmt.Errorf("%s %s", stderr, err)
	}

	return nil
}

func asexp(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}

	args := config.Globals()
	args.Add("D", "asexplicit")
	args.AddTarget(pkgs...)
	_, stderr, err := capture(passToPacman(args))
	if err != nil {
		return fmt.Errorf("%s %s", stderr, err)
	}

	return nil
}

// Install handles package installs
func install(args *settings.Args, alpmHandle *alpm.Handle, ignoreProviders bool) (err error) {
	var incompatible stringset.StringSet
	var do *dep.Order

	var aurUp upSlice
	var repoUp upSlice

	var srcinfos map[string]*gosrc.Srcinfo

	warnings := query.NewWarnings()

	if config.Mode == settings.ModeAny || config.Mode == settings.ModeRepo {
		if config.CombinedUpgrade {
			if config.Refresh > 0 {
				err = earlyRefresh()
				if err != nil {
					return fmt.Errorf(gotext.Get("error refreshing databases"))
				}
			}
		} else if config.Refresh > 0 || config.SysUpgrade > 0 || len(config.Targets) > 0 {
			err = earlyPacmanCall(alpmHandle)
			if err != nil {
				return err
			}
		}
	}

	// we may have done -Sy, our handle now has an old
	// database.
	alpmHandle, err = initAlpmHandle(config.Pacman, alpmHandle)
	if err != nil {
		return err
	}
	config.Alpm = alpmHandle

	_, _, localNames, remoteNames, err := query.FilterPackages(alpmHandle)
	if err != nil {
		return err
	}

	remoteNamesCache := stringset.FromSlice(remoteNames)
	localNamesCache := stringset.FromSlice(localNames)

	requestTargets := append([]string{}, config.Targets...)

	// create the arguments to pass for the repo install
	repoArgs := config.Flags()
	repoArgs.Del("asdeps", "asdep")
	repoArgs.Del("asexplicit", "asexp")
	repoArgs.Op = "S"
	repoArgs.Targets = nil

	if config.Mode == settings.ModeAUR {
		repoArgs.Del("u", "sysupgrade")
	}

	// if we are doing -u also request all packages needing update
	if config.SysUpgrade > 0 {
		aurUp, repoUp, err = upList(warnings, alpmHandle, config.SysUpgrade > 1)
		if err != nil {
			return err
		}

		warnings.Print()

		ignore, aurUp, errUp := upgradePkgs(aurUp, repoUp)
		if errUp != nil {
			return errUp
		}

		for _, up := range repoUp {
			if !ignore.Get(up.Name) {
				requestTargets = append(requestTargets, up.Name)
				args.AddTarget(up.Name)
			}
		}

		for up := range aurUp {
			requestTargets = append(requestTargets, "aur/"+up)
			args.AddTarget("aur/" + up)
		}

		/*value, _, exists := args.GetArg("ignore")

		if len(ignore) > 0 {
			ignoreStr := strings.Join(ignore.ToSlice(), ",")
			if exists {
				ignoreStr += "," + value
			}
			if repoArgs.Options["ignore"] == nil {
				repoArgs.Options["ignore"] = &settings.Option{
					Args: []string{ignoreStr},
				}
			} else {
				repoArgs.Options["ignore"].Add(ignoreStr)
			}
		}*/
	}

	targets := stringset.FromSlice(config.Targets)

	dp, err := dep.GetPool(config, requestTargets,
		warnings, alpmHandle, config.Mode,
		ignoreProviders, config.NoConfirm, config.Provides, config.Rebuild, config.RequestSplitN)
	if err != nil {
		return err
	}

	if config.Nodeps <= 1 {
		err = dp.CheckMissing()
		if err != nil {
			return err
		}
	}

	if len(dp.Aur) == 0 {
		if !config.CombinedUpgrade {
			if config.SysUpgrade > 0 {
				fmt.Println(gotext.Get(" there is nothing to do"))
			}
			return nil
		}

		args.Op = "S"
		args.Del("--refresh")

		/*if repoArgs.ExistsArg("ignore") {
			if args.ExistsArg("ignore") {
				args.Options["ignore"].Args = append(args.Options["ignore"].Args, repoArgs.Options["ignore"].Args...)
			} else {
				args.Options["ignore"] = repoArgs.Options["ignore"]
			}
		}*/
		return show(passToPacman(args))
	}

	if len(dp.Aur) > 0 && os.Geteuid() == 0 {
		return fmt.Errorf(gotext.Get("refusing to install AUR packages as root, aborting"))
	}

	var conflicts stringset.MapStringSet
	if config.Nodeps <= 1 {
		conflicts, err = dp.CheckConflicts(config.UseAsk, config.NoConfirm)
		if err != nil {
			return err
		}
	}

	do = dep.GetOrder(dp)
	if err != nil {
		return err
	}

	for _, pkg := range do.Repo {
		repoArgs.AddTarget(pkg.DB().Name() + "/" + pkg.Name())
	}

	for _, pkg := range dp.Groups {
		repoArgs.AddTarget(pkg)
	}

	if len(do.Aur) == 0 && len(repoArgs.Targets) == 0 && (config.SysUpgrade == 0 || config.Mode == settings.ModeAUR) {
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
			if text.ContinueTask(gotext.Get("Remove make dependencies after install?"), false, config.NoConfirm) {
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
		oldValue := config.NoConfirm
		config.NoConfirm = false
		fmt.Println()
		if !text.ContinueTask(gotext.Get("Proceed with install?"), true, config.NoConfirm) {
			return fmt.Errorf(gotext.Get("aborting due to user"))
		}
		err = updatePkgbuildSeenRef(toDiff)
		if err != nil {
			text.Errorln(err.Error())
		}

		config.NoConfirm = oldValue
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
		oldValue := config.NoConfirm
		config.NoConfirm = false
		fmt.Println()
		if !text.ContinueTask(gotext.Get("Proceed with install?"), true, config.NoConfirm) {
			return errors.New(gotext.Get("aborting due to user"))
		}
		config.NoConfirm = oldValue
	}

	incompatible, err = getIncompatible(do.Aur, srcinfos, alpmHandle)
	if err != nil {
		return err
	}

	if config.PGPFetch {
		err = pgp.CheckPgpKeys(do.Aur, srcinfos, config.GpgBin, config.GpgFlags, config.NoConfirm)
		if err != nil {
			return err
		}
	}

	if !config.CombinedUpgrade {
		repoArgs.Del("u", "sysupgrade")
	}

	if len(repoArgs.Targets) > 0 || config.SysUpgrade > 0 {
		if errShow := show(passToPacman(repoArgs)); errShow != nil {
			return errors.New(gotext.Get("error installing repo packages"))
		}

		deps := make([]string, 0)
		exp := make([]string, 0)

		for _, pkg := range do.Repo {
			if !dp.Explicit.Get(pkg.Name()) && !localNamesCache.Get(pkg.Name()) && !remoteNamesCache.Get(pkg.Name()) {
				deps = append(deps, pkg.Name())
				continue
			}

			if config.AsDeps && dp.Explicit.Get(pkg.Name()) {
				deps = append(deps, pkg.Name())
			} else if config.AsExplicit && dp.Explicit.Get(pkg.Name()) {
				exp = append(exp, pkg.Name())
			}
		}

		if errDeps := asdeps(deps); errDeps != nil {
			return errDeps
		}
		if errExp := asexp(exp); errExp != nil {
			return errExp
		}
	}

	go exitOnError(completion.Update(alpmHandle, config.AURURL, config.CompletionPath, config.CompletionInterval, false))

	err = downloadPkgbuildsSources(do.Aur, incompatible)
	if err != nil {
		return err
	}

	err = buildInstallPkgbuilds(alpmHandle, dp, do, srcinfos, incompatible, conflicts)
	if err != nil {
		return err
	}

	return nil
}

func removeMake(do *dep.Order) error {
	removeArguments := config.Globals()
	removeArguments.Add("R", "u")

	for _, pkg := range do.GetMake() {
		removeArguments.AddTarget(pkg)
	}

	oldValue := config.NoConfirm
	config.NoConfirm = true
	err := show(passToPacman(removeArguments))
	config.NoConfirm = oldValue

	return err
}

func inRepos(syncDB alpm.DBList, pkg string) bool {
	target := dep.ToTarget(pkg)

	if target.DB == "aur" {
		return false
	} else if target.DB != "" {
		return true
	}

	previousHideMenus := config.HideMenus
	config.HideMenus = false
	_, err := syncDB.FindSatisfier(target.DepString())
	config.HideMenus = previousHideMenus
	if err == nil {
		return true
	}

	return !syncDB.FindGroupPkgs(target.Name).Empty()
}

func earlyPacmanCall(alpmHandle *alpm.Handle) error {
	arguments := config.Flags()
	arguments.Op = "S"
	targets := config.Targets
	arguments.Targets = nil
	config.Targets = nil

	syncDB, err := alpmHandle.SyncDBs()
	if err != nil {
		return err
	}

	if config.Mode == settings.ModeRepo {
		arguments.Targets = targets
	} else {
		// separate aur and repo targets
		for _, target := range targets {
			if inRepos(syncDB, target) {
				arguments.AddTarget(target)
			} else {
				config.AddTarget(target)
			}
		}
	}

	if config.Refresh > 0 || config.SysUpgrade > 0 || len(arguments.Targets) > 0 {
		err = show(passToPacman(arguments))
		if err != nil {
			return errors.New(gotext.Get("error installing repo packages"))
		}
	}

	return nil
}

func earlyRefresh() error {
	arguments := config.Flags()
	config.Refresh = 0
	arguments.Del("u", "sysupgrade")
	arguments.Del("s", "search")
	arguments.Del("i", "info")
	arguments.Del("l", "list")
	arguments.Targets = nil
	return show(passToPacman(arguments))
}

func getIncompatible(bases []dep.Base, srcinfos map[string]*gosrc.Srcinfo, alpmHandle *alpm.Handle) (stringset.StringSet, error) {
	incompatible := make(stringset.StringSet)
	basesMap := make(map[string]dep.Base)
	alpmArch, err := alpmHandle.Arch()
	if err != nil {
		return nil, err
	}

nextpkg:
	for _, base := range bases {
		for _, arch := range srcinfos[base.Pkgbase()].Arch {
			if arch == "any" || arch == alpmArch {
				continue nextpkg
			}
		}

		incompatible.Set(base.Pkgbase())
		basesMap[base.Pkgbase()] = base
	}

	if len(incompatible) > 0 {
		text.Warnln(gotext.Get("The following packages are not compatible with your architecture:"))
		for pkg := range incompatible {
			fmt.Print("  " + cyan(basesMap[pkg].String()))
		}

		fmt.Println()

		if !text.ContinueTask(gotext.Get("Try to build them anyway?"), true, config.NoConfirm) {
			return nil, errors.New(gotext.Get("aborting due to user"))
		}
	}

	return incompatible, nil
}

func parsePackageList(dir string) (pkgdests map[string]string, pkgVersion string, err error) {
	stdout, stderr, err := capture(passToMakepkg(dir, "--packagelist"))
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

		toPrint += fmt.Sprintf(magenta("%3d")+" %-40s", len(bases)-n,
			bold(base.String()))

		anyInstalled := false
		for _, b := range base {
			anyInstalled = anyInstalled || installed.Get(b.Name)
		}

		if anyInstalled {
			toPrint += bold(green(gotext.Get(" (Installed)")))
		}

		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			toPrint += bold(green(gotext.Get(" (Build Files Exist)")))
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
	text.Infoln(gotext.Get("%s [A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)", cyan(gotext.Get("[N]one"))))
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
		text.Infoln(gotext.Get("%s [A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)", cyan(gotext.Get("[N]one"))))
		editInput, err = getInput(config.AnswerDiff)
		if err != nil {
			return nil, err
		}
	} else {
		text.Infoln(gotext.Get("PKGBUILDs to edit?"))
		text.Infoln(gotext.Get("%s [A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)", cyan(gotext.Get("[N]one"))))
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
				text.Warnln(gotext.Get("%s: No changes -- skipping", cyan(base.String())))
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
		_ = show(passToGit(dir, args...))
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

		text.OperationInfoln(gotext.Get("(%d/%d) Parsing SRCINFO: %s", k+1, len(bases), cyan(base.String())))

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

		if (config.Redownload == "yes" && isTarget) || config.Redownload == "all" {
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
					downloaded, len(bases), cyan(base.String())))
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
		text.OperationInfoln(gotext.Get("Downloaded PKGBUILD (%d/%d): %s", downloaded, len(bases), cyan(base.String())))
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

		err = show(passToMakepkg(dir, args...))
		if err != nil {
			return errors.New(gotext.Get("error downloading sources: %s", cyan(base.String())))
		}
	}

	return
}

func buildInstallPkgbuilds(
	alpmHandle *alpm.Handle,
	dp *dep.Pool,
	do *dep.Order,
	srcinfos map[string]*gosrc.Srcinfo,
	incompatible stringset.StringSet,
	conflicts stringset.MapStringSet,
) error {
	arguments := config.Flags()
	arguments.Targets = nil
	arguments.Op = "U"
	arguments.Del("confirm")
	arguments.Del("noconfirm")
	arguments.Del("c", "clean")
	arguments.Del("q", "quiet")
	arguments.Del("q", "quiet")
	arguments.Del("y", "refresh")
	arguments.Del("u", "sysupgrade")
	arguments.Del("w", "downloadonly")

	deps := make([]string, 0)
	exp := make([]string, 0)
	oldConfirm := config.NoConfirm
	config.NoConfirm = true

	//remotenames: names of all non repo packages on the system
	_, _, localNames, remoteNames, err := query.FilterPackages(alpmHandle)
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

		if errShow := show(passToPacman(arguments)); errShow != nil {
			return errShow
		}

		err = saveVCSInfo(config.VCSPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}

		if errDeps := asdeps(deps); err != nil {
			return errDeps
		}
		if errExps := asexp(exp); err != nil {
			return errExps
		}

		config.NoConfirm = oldConfirm

		arguments.Targets = nil
		deps = make([]string, 0)
		exp = make([]string, 0)
		config.NoConfirm = true
		return nil
	}

	for _, base := range do.Aur {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)
		built := true

		satisfied := true
	all:
		for _, pkg := range base {
			for _, deps := range [3][]string{pkg.Depends, pkg.MakeDepends, pkg.CheckDepends} {
				for _, dep := range deps {
					if _, errSatisfier := dp.LocalDB.PkgCache().FindSatisfier(dep); errSatisfier != nil {
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
		if err = show(passToMakepkg(dir, args...)); err != nil {
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
		if config.Rebuild == "no" || (config.Rebuild == "yes" && !isExplicit) {
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

		if config.Needed {
			installed := true
			for _, split := range base {
				if alpmpkg := dp.LocalDB.Pkg(split.Name); alpmpkg == nil || alpmpkg.Version() != pkgVersion {
					installed = false
				}
			}

			if installed {
				err = show(passToMakepkg(dir, "-c", "--nobuild", "--noextract", "--ignorearch"))
				if err != nil {
					return errors.New(gotext.Get("error making: %s", err))
				}

				fmt.Fprintln(os.Stdout, gotext.Get("%s is up to date -- skipping", cyan(pkg+"-"+pkgVersion)))
				continue
			}
		}

		if built {
			err = show(passToMakepkg(dir, "-c", "--nobuild", "--noextract", "--ignorearch"))
			if err != nil {
				return errors.New(gotext.Get("error making: %s", err))
			}

			text.Warnln(gotext.Get("%s already made -- skipping build", cyan(pkg+"-"+pkgVersion)))
		} else {
			args := []string{"-cf", "--noconfirm", "--noextract", "--noprepare", "--holdver"}

			if incompatible.Get(pkg) {
				args = append(args, "--ignorearch")
			}

			if errMake := show(passToMakepkg(dir, args...)); errMake != nil {
				return errors.New(gotext.Get("error making: %s", base.String))
			}
		}

		// conflicts have been checked so answer y for them
		if config.UseAsk && config.Ask != "" {
			ask, _ := strconv.Atoi(config.Ask)
			uask := alpm.QuestionType(ask) | alpm.QuestionTypeConflictPkg
			config.Ask = fmt.Sprint(uask)
		} else {
			for _, split := range base {
				if _, ok := conflicts[split.Name]; ok {
					config.NoConfirm = false
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
			if config.AsDeps {
				deps = append(deps, name)
			} else if config.AsExplicit {
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
			go updateVCSData(config.VCSPath, pkg.Name, srcinfo.Source, &mux, &wg)
		}

		wg.Wait()
	}

	err = doInstall()
	config.NoConfirm = oldConfirm
	return err
}
