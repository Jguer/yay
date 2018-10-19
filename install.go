package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	gosrc "github.com/Morganamilo/go-srcinfo"
	alpm "github.com/jguer/go-alpm"
)

// Install handles package installs
func install(parser *arguments) error {
	var err error
	var incompatible stringSet

	var aurUp upSlice
	var repoUp upSlice

	var srcinfos map[string]*gosrc.Srcinfo

	warnings := &aurWarnings{}
	removeMake := false

	if mode == ModeAny || mode == ModeRepo {
		if config.CombinedUpgrade {
			if parser.existsArg("y", "refresh") {
				err = earlyRefresh(parser)
				if err != nil {
					return fmt.Errorf("Error refreshing databases")
				}
			}
		} else if parser.existsArg("y", "refresh") || parser.existsArg("u", "sysupgrade") || len(parser.targets) > 0 {
			// If --build, don’t install nore update anything at this stage
			if config.Build {
				arguments := parser.copy()
				arguments.delArg("u", "sysupgrade")
				arguments.clearTargets()
				err = earlyPacmanCall(arguments)
				if err != nil {
					return err
				}
			} else {
				err = earlyPacmanCall(parser)
				if err != nil {
					return err
				}
			}
		}
	}

	//we may have done -Sy, our handle now has an old
	//database.
	err = initAlpmHandle()
	if err != nil {
		return err
	}

	_, _, localNames, remoteNames, err := filterPackages()
	if err != nil {
		return err
	}

	remoteNamesCache := sliceToStringSet(remoteNames)
	localNamesCache := sliceToStringSet(localNames)

	requestTargets := parser.copy().targets

	//create the arguments to pass for the repo install
	arguments := parser.copy()
	arguments.delArg("asdeps", "asdep")
	arguments.delArg("asexplicit", "asexp")
	arguments.op = "S"
	arguments.clearTargets()

	if mode == ModeAUR {
		arguments.delArg("u", "sysupgrade")
	}

	//if we are doing -u also request all packages needing update
	if parser.existsArg("u", "sysupgrade") {
		aurUp, repoUp, err = upList(warnings)
		if err != nil {
			return err
		}

		warnings.print()

		ignore, aurUp, err := upgradePkgs(aurUp, repoUp)
		if err != nil {
			return err
		}

		for _, up := range repoUp {
			if !ignore.get(up.Name) {
				requestTargets = append(requestTargets, up.Name)
				parser.addTarget(up.Name)
			}
		}

		for up := range aurUp {
			requestTargets = append(requestTargets, "aur/"+up)
			parser.addTarget("aur/" + up)
		}

		value, _, exists := cmdArgs.getArg("ignore")

		if len(ignore) > 0 {
			ignoreStr := strings.Join(ignore.toSlice(), ",")
			if exists {
				ignoreStr += "," + value
			}
			arguments.options["ignore"] = ignoreStr
		}
	}

	targets := sliceToStringSet(parser.targets)

	ds, err := getDepSolver(requestTargets, warnings)
	if err != nil {
		return err
	}

	err = ds.CheckMissing()
	if err != nil {
		return err
	}

	if len(ds.Aur) == 0 && !config.Build {
		if !config.CombinedUpgrade {
			if parser.existsArg("u", "sysupgrade") {
				fmt.Println(" there is nothing to do")
			}
			return nil
		}

		parser.op = "S"
		parser.delArg("y", "refresh")
		parser.options["ignore"] = arguments.options["ignore"]
		return show(passToPacman(parser))
	}

	if len(ds.Aur) > 0 && 0 == os.Geteuid() {
		return fmt.Errorf(bold(red(arrow)) + " Refusing to install AUR Packages as root, Aborting.")
	}

	conflicts, err := ds.CheckConflicts()
	if err != nil {
		return err
	}

	if config.Build {
		downloadABS(ds.Repo, config.BuildDir)
	}

	for _, pkg := range ds.Repo {
		arguments.addTarget(pkg.DB().Name() + "/" + pkg.Name())
	}

	for _, pkg := range ds.Groups {
		arguments.addTarget(pkg)
	}

	if len(ds.Aur) == 0 && len(arguments.targets) == 0 && (!parser.existsArg("u", "sysupgrade") || mode == ModeAUR) {
		fmt.Println(" there is nothing to do")
		return nil
	}

	ds.Print()
	fmt.Println()

	if ds.HasMake() {
		if config.RemoveMake == "yes" {
			removeMake = true
		} else if config.RemoveMake == "no" {
			removeMake = false
		} else if continueTask("Remove make dependencies after install?", false) {
			removeMake = true
		}
	}

	if config.CleanMenu {
		if anyExistInCache(ds.Aur) {
			askClean := pkgbuildNumberMenu(ds.Aur, remoteNamesCache)
			toClean, err := cleanNumberMenu(ds.Aur, remoteNamesCache, askClean)
			if err != nil {
				return err
			}

			cleanBuilds(toClean)
		}
	}

	toSkip := pkgbuildsToSkip(ds.Aur, targets)
	cloned, err := downloadPkgbuilds(ds.Aur, toSkip, config.BuildDir)
	if err != nil {
		return err
	}

	var toDiff []Base
	var toEdit []Base

	if config.DiffMenu {
		pkgbuildNumberMenu(ds.Aur, remoteNamesCache)
		toDiff, err = diffNumberMenu(ds.Aur, remoteNamesCache)
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
		if !continueTask(bold(green("Proceed with install?")), true) {
			return fmt.Errorf("Aborting due to user")
		}
		config.NoConfirm = oldValue
	}

	err = mergePkgbuilds(ds.Aur)
	if err != nil {
		return err
	}

	srcinfos, err = parseSrcinfoFiles(ds.Aur, true)
	if err != nil {
		return err
	}

	if config.EditMenu {
		pkgbuildNumberMenu(ds.Aur, remoteNamesCache)
		toEdit, err = editNumberMenu(ds.Aur, remoteNamesCache)
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
		if !continueTask(bold(green("Proceed with install?")), true) {
			return fmt.Errorf("Aborting due to user")
		}
		config.NoConfirm = oldValue
	}

	incompatible, err = getIncompatible(ds.Aur, srcinfos)
	if err != nil {
		return err
	}

	if config.PGPFetch {
		err = checkPgpKeys(ds.Aur, srcinfos)
		if err != nil {
			return err
		}
	}

	if !config.CombinedUpgrade {
		arguments.delArg("u", "sysupgrade")
	}

	if !config.Build && len(arguments.targets) > 0 || arguments.existsArg("u") {
		err := show(passToPacman(arguments))
		if err != nil {
			return fmt.Errorf("Error installing repo packages")
		}

		depArguments := makeArguments()
		depArguments.addArg("D", "asdeps")
		expArguments := makeArguments()
		expArguments.addArg("D", "asexplicit")

		for _, pkg := range ds.Repo {
			if !ds.Explicit.get(pkg.Name()) && !localNamesCache.get(pkg.Name()) && !remoteNamesCache.get(pkg.Name()) {
				depArguments.addTarget(pkg.Name())
				continue
			}

			if parser.existsArg("asdeps", "asdep") && ds.Explicit.get(pkg.Name()) {
				depArguments.addTarget(pkg.Name())
			} else if parser.existsArg("asexp", "asexplicit") && ds.Explicit.get(pkg.Name()) {
				expArguments.addTarget(pkg.Name())
			}
		}

		if len(depArguments.targets) > 0 {
			_, stderr, err := capture(passToPacman(depArguments))
			if err != nil {
				return fmt.Errorf("%s%s", stderr, err)
			}
		}

		if len(expArguments.targets) > 0 {
			_, stderr, err := capture(passToPacman(expArguments))
			if err != nil {
				return fmt.Errorf("%s%s", stderr, err)
			}
		}
	}

	go updateCompletion(false)

	err = downloadPkgbuildsSources(ds.Aur, incompatible)
	if err != nil {
		return err
	}

	err = buildInstallABS(ds, parser, incompatible, conflicts)
	if err != nil {
		return err
	}

	err = buildInstallPkgbuilds(ds, srcinfos, parser, incompatible, conflicts)
	if err != nil {
		return err
	}

	if removeMake {
		removeArguments := makeArguments()
		removeArguments.addArg("R", "u")

		for _, pkg := range ds.getMake() {
			removeArguments.addTarget(pkg)
		}

		oldValue := config.NoConfirm
		config.NoConfirm = true
		err = show(passToPacman(removeArguments))
		config.NoConfirm = oldValue

		if err != nil {
			return err
		}
	}

	if config.CleanAfter {
		cleanAfter(ds.Aur)
	}

	return nil
}

func inRepos(syncDb alpm.DbList, pkg string) bool {
	target := toTarget(pkg)

	if target.Db == "aur" {
		return false
	} else if target.Db != "" {
		return true
	}

	previousHideMenus := hideMenus
	hideMenus = false
	_, err := syncDb.FindSatisfier(target.DepString())
	hideMenus = previousHideMenus
	if err == nil {
		return true
	}

	_, err = syncDb.PkgCachebyGroup(target.Name)
	if err == nil {
		return true
	}

	return false
}

func earlyPacmanCall(parser *arguments) error {
	arguments := parser.copy()
	arguments.op = "S"
	targets := parser.targets
	parser.clearTargets()
	arguments.clearTargets()

	syncDb, err := alpmHandle.SyncDbs()
	if err != nil {
		return err
	}

	if mode == ModeRepo {
		arguments.targets = targets
	} else {
		//separate aur and repo targets
		for _, target := range targets {
			if inRepos(syncDb, target) {
				arguments.addTarget(target)
			} else {
				parser.addTarget(target)
			}
		}
	}

	if parser.existsArg("y", "refresh") || parser.existsArg("u", "sysupgrade") || len(arguments.targets) > 0 {
		err = show(passToPacman(arguments))
		if err != nil {
			return fmt.Errorf("Error installing repo packages")
		}
	}

	return nil
}

func earlyRefresh(parser *arguments) error {
	arguments := parser.copy()
	parser.delArg("y", "refresh")
	arguments.delArg("u", "sysupgrade")
	arguments.delArg("s", "search")
	arguments.delArg("i", "info")
	arguments.delArg("l", "list")
	arguments.clearTargets()
	return show(passToPacman(arguments))
}

func getIncompatible(bases []Base, srcinfos map[string]*gosrc.Srcinfo) (stringSet, error) {
	incompatible := make(stringSet)
	basesMap := make(map[string]Base)
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

		incompatible.set(base.Pkgbase())
		basesMap[base.Pkgbase()] = base
	}

	if len(incompatible) > 0 {
		fmt.Println()
		fmt.Print(bold(yellow(arrow)) + " The following packages are not compatible with your architecture:")
		for pkg := range incompatible {
			fmt.Print("  " + cyan((basesMap[pkg].String())))
		}

		fmt.Println()

		if !continueTask("Try to build them anyway?", true) {
			return nil, fmt.Errorf("Aborting due to user")
		}
	}

	return incompatible, nil
}

func parsePackageList(dir string) (map[string]string, string, error) {
	stdout, stderr, err := capture(passToMakepkg(dir, "--packagelist"))

	if err != nil {
		return nil, "", fmt.Errorf("%s%s", stderr, err)
	}

	var version string
	lines := strings.Split(stdout, "\n")
	pkgdests := make(map[string]string)

	for _, line := range lines {
		if line == "" {
			continue
		}

		fileName := filepath.Base(line)
		split := strings.Split(fileName, "-")

		if len(split) < 4 {
			return nil, "", fmt.Errorf("Can not find package name : %s", split)
		}

		// pkgname-pkgver-pkgrel-arch.pkgext
		// This assumes 3 dashes after the pkgname, Will cause an error
		// if the PKGEXT contains a dash. Please no one do that.
		pkgname := strings.Join(split[:len(split)-3], "-")
		version = strings.Join(split[len(split)-3:len(split)-1], "-")
		pkgdests[pkgname] = line
	}

	return pkgdests, version, nil
}

func anyExistInCache(bases []Base) bool {
	for _, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)

		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			return true
		}
	}

	return false
}

func pkgbuildNumberMenu(bases []Base, installed stringSet) bool {
	toPrint := ""
	askClean := false

	for n, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)

		toPrint += fmt.Sprintf(magenta("%3d")+" %-40s", len(bases)-n,
			bold(base.String()))

		anyInstalled := false
		for _, b := range base {
			anyInstalled = anyInstalled || installed.get(b.Name)
		}

		if anyInstalled {
			toPrint += bold(green(" (Installed)"))
		}

		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			toPrint += bold(green(" (Build Files Exist)"))
			askClean = true
		}

		toPrint += "\n"
	}

	fmt.Print(toPrint)

	return askClean
}

func cleanNumberMenu(bases []Base, installed stringSet, hasClean bool) ([]Base, error) {
	toClean := make([]Base, 0)

	if !hasClean {
		return toClean, nil
	}

	fmt.Println(bold(green(arrow + " Packages to cleanBuild?")))
	fmt.Println(bold(green(arrow) + cyan(" [N]one ") + "[A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)"))
	fmt.Print(bold(green(arrow + " ")))
	cleanInput, err := getInput(config.AnswerClean)
	if err != nil {
		return nil, err
	}

	cInclude, cExclude, cOtherInclude, cOtherExclude := parseNumberMenu(cleanInput)
	cIsInclude := len(cExclude) == 0 && len(cOtherExclude) == 0

	if cOtherInclude.get("abort") || cOtherInclude.get("ab") {
		return nil, fmt.Errorf("Aborting due to user")
	}

	if !cOtherInclude.get("n") && !cOtherInclude.get("none") {
		for i, base := range bases {
			pkg := base.Pkgbase()
			anyInstalled := false
			for _, b := range base {
				anyInstalled = anyInstalled || installed.get(b.Name)
			}

			dir := filepath.Join(config.BuildDir, pkg)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				continue
			}

			if !cIsInclude && cExclude.get(len(bases)-i) {
				continue
			}

			if anyInstalled && (cOtherInclude.get("i") || cOtherInclude.get("installed")) {
				toClean = append(toClean, base)
				continue
			}

			if !anyInstalled && (cOtherInclude.get("no") || cOtherInclude.get("notinstalled")) {
				toClean = append(toClean, base)
				continue
			}

			if cOtherInclude.get("a") || cOtherInclude.get("all") {
				toClean = append(toClean, base)
				continue
			}

			if cIsInclude && (cInclude.get(len(bases)-i) || cOtherInclude.get(pkg)) {
				toClean = append(toClean, base)
				continue
			}

			if !cIsInclude && (!cExclude.get(len(bases)-i) && !cOtherExclude.get(pkg)) {
				toClean = append(toClean, base)
				continue
			}
		}
	}

	return toClean, nil
}

func editNumberMenu(bases []Base, installed stringSet) ([]Base, error) {
	return editDiffNumberMenu(bases, installed, false)
}

func diffNumberMenu(bases []Base, installed stringSet) ([]Base, error) {
	return editDiffNumberMenu(bases, installed, true)
}

func editDiffNumberMenu(bases []Base, installed stringSet, diff bool) ([]Base, error) {
	toEdit := make([]Base, 0)
	var editInput string
	var err error

	fmt.Println(bold(green(arrow) + cyan(" [N]one ") + "[A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)"))

	if diff {
		fmt.Println(bold(green(arrow + " Diffs to show?")))
		fmt.Print(bold(green(arrow + " ")))
		editInput, err = getInput(config.AnswerDiff)
		if err != nil {
			return nil, err
		}
	} else {
		fmt.Println(bold(green(arrow + " PKGBUILDs to edit?")))
		fmt.Print(bold(green(arrow + " ")))
		editInput, err = getInput(config.AnswerEdit)
		if err != nil {
			return nil, err
		}
	}

	eInclude, eExclude, eOtherInclude, eOtherExclude := parseNumberMenu(editInput)
	eIsInclude := len(eExclude) == 0 && len(eOtherExclude) == 0

	if eOtherInclude.get("abort") || eOtherInclude.get("ab") {
		return nil, fmt.Errorf("Aborting due to user")
	}

	if !eOtherInclude.get("n") && !eOtherInclude.get("none") {
		for i, base := range bases {
			pkg := base.Pkgbase()
			anyInstalled := false
			for _, b := range base {
				anyInstalled = anyInstalled || installed.get(b.Name)
			}

			if !eIsInclude && eExclude.get(len(bases)-i) {
				continue
			}

			if anyInstalled && (eOtherInclude.get("i") || eOtherInclude.get("installed")) {
				toEdit = append(toEdit, base)
				continue
			}

			if !anyInstalled && (eOtherInclude.get("no") || eOtherInclude.get("notinstalled")) {
				toEdit = append(toEdit, base)
				continue
			}

			if eOtherInclude.get("a") || eOtherInclude.get("all") {
				toEdit = append(toEdit, base)
				continue
			}

			if eIsInclude && (eInclude.get(len(bases)-i) || eOtherInclude.get(pkg)) {
				toEdit = append(toEdit, base)
			}

			if !eIsInclude && (!eExclude.get(len(bases)-i) && !eOtherExclude.get(pkg)) {
				toEdit = append(toEdit, base)
			}
		}
	}

	return toEdit, nil
}

func showPkgbuildDiffs(bases []Base, cloned stringSet) error {
	for _, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)
		if shouldUseGit(dir) {
			start := "HEAD"

			if cloned.get(pkg) {
				start = gitEmptyTree
			} else {
				hasDiff, err := gitHasDiff(config.BuildDir, pkg)
				if err != nil {
					return err
				}

				if !hasDiff {
					fmt.Printf("%s %s: %s\n", bold(yellow(arrow)), cyan(base.String()), bold("No changes -- skipping"))
					continue
				}
			}

			args := []string{"diff", start + "..HEAD@{upstream}", "--src-prefix", dir + "/", "--dst-prefix", dir + "/", "--", ".", ":(exclude).SRCINFO"}
			if useColor {
				args = append(args, "--color=always")
			} else {
				args = append(args, "--color=never")
			}
			err := show(passToGit(dir, args...))
			if err != nil {
				return err
			}
		} else {
			args := []string{"diff"}
			if useColor {
				args = append(args, "--color=always")
			} else {
				args = append(args, "--color=never")
			}
			args = append(args, "--no-index", "/var/empty", dir)
			// git always returns 1. why? I have no idea
			show(passToGit(dir, args...))
		}
	}

	return nil
}

func editPkgbuilds(bases []Base, srcinfos map[string]*gosrc.Srcinfo) error {
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
			return fmt.Errorf("Editor did not exit successfully, Aborting: %s", err)
		}
	}

	return nil
}

func parseSrcinfoFiles(bases []Base, errIsFatal bool) (map[string]*gosrc.Srcinfo, error) {
	srcinfos := make(map[string]*gosrc.Srcinfo)
	for k, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)

		str := bold(cyan("::") + " Parsing SRCINFO (%d/%d): %s\n")
		fmt.Printf(str, k+1, len(bases), cyan(base.String()))

		pkgbuild, err := gosrc.ParseFile(filepath.Join(dir, ".SRCINFO"))
		if err != nil {
			if !errIsFatal {
				fmt.Printf("failed to parse %s -- skipping: %s\n", base.String(), err)
				continue
			}
			return nil, fmt.Errorf("failed to parse %s: %s", base.String(), err)
		}

		srcinfos[pkg] = pkgbuild
	}

	return srcinfos, nil
}

func pkgbuildsToSkip(bases []Base, targets stringSet) stringSet {
	toSkip := make(stringSet)

	for _, base := range bases {
		isTarget := false
		for _, pkg := range base {
			isTarget = isTarget || targets.get(pkg.Name)
		}

		if (config.ReDownload == "yes" && isTarget) || config.ReDownload == "all" {
			continue
		}

		dir := filepath.Join(config.BuildDir, base.Pkgbase(), ".SRCINFO")
		pkgbuild, err := gosrc.ParseFile(dir)

		if err == nil {
			if alpm.VerCmp(pkgbuild.Version(), base.Version()) >= 0 {
				toSkip.set(base.Pkgbase())
			}
		}
	}

	return toSkip
}

func mergePkgbuilds(bases []Base) error {
	for _, base := range bases {
		if shouldUseGit(filepath.Join(config.BuildDir, base.Pkgbase())) {
			err := gitMerge(config.BuildDir, base.Pkgbase())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func downloadABS(packages []*alpm.Package, buildDir string) (stringSet, error) {
	cloned := make(stringSet)
	downloaded := 0
	var wg sync.WaitGroup
	var mux sync.Mutex
	var errs MultiError

	download := func(k int, base *alpm.Package) {
		defer wg.Done()
		pkg := base.Name()

		var url string

		if shouldUseGit(filepath.Join(config.BuildDir, "packages", pkg)) {
			clone, err := aspDownload(filepath.Join(config.BuildDir, "packages"), pkg)
			if err != nil {
				errs.Add(err)
				return
			}
			if clone {
				mux.Lock()
				cloned.set(pkg)
				mux.Unlock()
			}
		} else {

			switch base.DB().Name() {
			case "core", "extra", "testing":
				url = "https://git.archlinux.org/svntogit/packages.git/snapshot/packages/" + pkg + ".tar.gz"
			case "community", "multilib", "community-testing", "multilib-testing":
				url = "https://git.archlinux.org/svntogit/community.git/snapshot/packages/" + pkg + ".tar.gz"
			default:
				return
			}

			err := downloadAndUnpack(url, buildDir)
			if err != nil {
				errs.Add(err)
				return
			}
		}

		mux.Lock()
		downloaded++
		str := bold(cyan("::") + "Downloaded PKGBUILD (%d/%d): %s\n")
		fmt.Printf(str, downloaded, len(packages), cyan(base.Name()))
		mux.Unlock()
	}

	count := 0
	for k, base := range packages {
		wg.Add(1)
		go download(k, base)
		count++
		if count%25 == 0 {
			wg.Wait()
		}
	}

	wg.Wait()

	return cloned, errs.Return()
}

func downloadPkgbuilds(bases []Base, toSkip stringSet, buildDir string) (stringSet, error) {
	cloned := make(stringSet)
	downloaded := 0
	var wg sync.WaitGroup
	var mux sync.Mutex
	var errs MultiError

	download := func(k int, base Base) {
		defer wg.Done()
		pkg := base.Pkgbase()

		if toSkip.get(pkg) {
			mux.Lock()
			downloaded++
			str := bold(cyan("::") + " PKGBUILD up to date, Skipping (%d/%d): %s\n")
			fmt.Printf(str, downloaded, len(bases), cyan(base.String()))
			mux.Unlock()
			return
		}

		if shouldUseGit(filepath.Join(config.BuildDir, pkg)) {
			clone, err := gitDownload(config.AURURL+"/"+pkg+".git", buildDir, pkg)
			if err != nil {
				errs.Add(err)
				return
			}
			if clone {
				mux.Lock()
				cloned.set(pkg)
				mux.Unlock()
			}
		} else {
			err := downloadAndUnpack(config.AURURL+base.URLPath(), buildDir)
			if err != nil {
				errs.Add(err)
				return
			}
		}

		mux.Lock()
		downloaded++
		str := bold(cyan("::") + " Downloaded PKGBUILD (%d/%d): %s\n")
		fmt.Printf(str, downloaded, len(bases), cyan(base.String()))
		mux.Unlock()
	}

	count := 0
	for k, base := range bases {
		wg.Add(1)
		go download(k, base)
		count++
		if count%25 == 0 {
			wg.Wait()
		}
	}

	wg.Wait()

	return cloned, errs.Return()
}

func downloadPkgbuildsSources(bases []Base, incompatible stringSet) (err error) {
	for _, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)
		args := []string{"--verifysource", "-Ccf"}

		if incompatible.get(pkg) {
			args = append(args, "--ignorearch")
		}

		err = show(passToMakepkg(dir, args...))
		if err != nil {
			return fmt.Errorf("Error downloading sources: %s", cyan(base.String()))
		}
	}

	return
}

func buildInstallABS(ds *depSolver, parser *arguments, incompatible stringSet, conflicts mapStringSet) error {
	for _, base := range ds.Repo {
		pkg := base.Name()
		dir := filepath.Join(config.BuildDir, "packages", pkg, "trunk")
		built := true

		args := []string{"--nobuild", "-fC"}

		if incompatible.get(pkg) {
			args = append(args, "--ignorearch")
		}

		//pkgver bump
		err := show(passToMakepkg(dir, args...)) // TODO: THIS FAILS
		if err != nil {
			return fmt.Errorf("Error making: %s", base.Name())
		}

		pkgdests, _, err := parsePackageList(dir)
		if err != nil {
			return err
		}

		isExplicit := ds.Explicit.get(pkg)
		if config.ReBuild == "no" || (config.ReBuild == "yes" && !isExplicit) {
			pkgdest, ok := pkgdests[pkg]
			if !ok {
				return fmt.Errorf("Could not find PKGDEST for: %s", pkg)
			}

			_, err := os.Stat(pkgdest)
			if os.IsNotExist(err) {
				built = false
			} else if err != nil {
				return err
			}
		} else {
			built = false
		}

		if cmdArgs.existsArg("needed") {
			installed := true
			if alpmpkg, err := ds.LocalDb.PkgByName(pkg); err != nil || alpmpkg.Version() != version {
				installed = false
			}

			if installed {
				fmt.Println(cyan(pkg+"-"+version) + bold(" is up to date -- skipping"))
				continue
			}
		}

		if built {
			fmt.Println(bold(yellow(arrow)),
				cyan(pkg+"-"+version)+bold(" already made -- skipping build"))
		} else {
			args := []string{"-cf", "--noconfirm", "--noextract", "--noprepare", "--holdver"}

			if incompatible.get(pkg) {
				args = append(args, "--ignorearch")
			}

			err := show(passToMakepkg(dir, args...))
			if err != nil {
				return fmt.Errorf("Error making: %s", base.Name())
			}
		}

		arguments := parser.copy()
		arguments.clearTargets()
		arguments.op = "U"
		arguments.delArg("confirm")
		arguments.delArg("noconfirm")
		arguments.delArg("build")
		arguments.delArg("c", "clean")
		arguments.delArg("q", "quiet")
		arguments.delArg("q", "quiet")
		arguments.delArg("y", "refresh")
		arguments.delArg("u", "sysupgrade")
		arguments.delArg("w", "downloadonly")

		oldConfirm := config.NoConfirm

		//conflicts have been checked so answer y for them
		if config.UseAsk {
			ask, _ := strconv.Atoi(cmdArgs.globals["ask"])
			uask := alpm.QuestionType(ask) | alpm.QuestionTypeConflictPkg
			cmdArgs.globals["ask"] = fmt.Sprint(uask)
		} else {
			conflict := false
			if _, ok := conflicts[pkg]; ok {
				conflict = true
			}

			if !conflict {
				config.NoConfirm = true
			}
		}

		depArguments := makeArguments()
		depArguments.addArg("D", "asdeps")
		expArguments := makeArguments()
		expArguments.addArg("D", "asexplicit")

		//remotenames: names of all non repo packages on the system
		_, _, localNames, remoteNames, err := filterPackages()
		if err != nil {
			return err
		}

		//cache as a stringset. maybe make it return a string set in the first
		//place
		remoteNamesCache := sliceToStringSet(remoteNames)
		localNamesCache := sliceToStringSet(localNames)

		pkgdest, ok := pkgdests[pkg]
		if !ok {
			return fmt.Errorf("Could not find PKGDEST for: %s", pkg)
		}

		arguments.addTarget(pkgdest)
		if !ds.Explicit.get(pkg) && !localNamesCache.get(pkg) && !remoteNamesCache.get(pkg) {
			depArguments.addTarget(pkg)
		}

		if ds.Explicit.get(pkg) {
			if parser.existsArg("asdeps", "asdep") {
				depArguments.addTarget(pkg)
			} else if parser.existsArg("asexplicit", "asexp") {
				expArguments.addTarget(pkg)
			}
		}

		err = show(passToPacman(arguments))
		if err != nil {
			return err
		}

		var mux sync.Mutex
		var wg sync.WaitGroup
		var src []gosrc.ArchString
		wg.Add(1)
		go updateVCSData(pkg, src, &mux, &wg)

		wg.Wait()

		err = saveVCSInfo()
		if err != nil {
			fmt.Println(err)
		}

		if len(depArguments.targets) > 0 {
			_, stderr, err := capture(passToPacman(depArguments))
			if err != nil {
				return fmt.Errorf("%s%s", stderr, err)
			}
		}
		config.NoConfirm = oldConfirm
	}

	return nil
}

func buildInstallPkgbuilds(ds *depSolver, srcinfos map[string]*gosrc.Srcinfo, parser *arguments, incompatible stringSet, conflicts mapStringSet) error {
	for _, base := range ds.Aur {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)
		built := true

		srcinfo := srcinfos[pkg]

		args := []string{"--nobuild", "-fC"}

		if incompatible.get(pkg) {
			args = append(args, "--ignorearch")
		}

		//pkgver bump
		err := show(passToMakepkg(dir, args...))
		if err != nil {
			return fmt.Errorf("Error making: %s", base.String())
		}

		pkgdests, version, err := parsePackageList(dir)
		if err != nil {
			return err
		}

		isExplicit := false
		for _, b := range base {
			isExplicit = isExplicit || ds.Explicit.get(b.Name)
		}
		if config.ReBuild == "no" || (config.ReBuild == "yes" && !isExplicit) {
			for _, split := range base {
				pkgdest, ok := pkgdests[split.Name]
				if !ok {
					return fmt.Errorf("Could not find PKGDEST for: %s", split.Name)
				}

				_, err := os.Stat(pkgdest)
				if os.IsNotExist(err) {
					built = false
				} else if err != nil {
					return err
				}
			}
		} else {
			built = false
		}

		if cmdArgs.existsArg("needed") {
			installed := true
			for _, split := range base {
				if alpmpkg, err := ds.LocalDb.PkgByName(split.Name); err != nil || alpmpkg.Version() != version {
					installed = false
				}
			}

			if installed {
				show(passToMakepkg(dir, "-c", "--nobuild", "--noextract", "--ignorearch"))
				fmt.Println(cyan(pkg+"-"+version) + bold(" is up to date -- skipping"))
				continue
			}
		}

		if built {
			show(passToMakepkg(dir, "-c", "--nobuild", "--noextract", "--ignorearch"))
			fmt.Println(bold(yellow(arrow)),
				cyan(pkg+"-"+version)+bold(" already made -- skipping build"))
		} else {
			args := []string{"-cf", "--noconfirm", "--noextract", "--noprepare", "--holdver"}

			if incompatible.get(pkg) {
				args = append(args, "--ignorearch")
			}

			err := show(passToMakepkg(dir, args...))
			if err != nil {
				return fmt.Errorf("Error making: %s", base.String())
			}
		}

		arguments := parser.copy()
		arguments.clearTargets()
		arguments.op = "U"
		arguments.delArg("confirm")
		arguments.delArg("noconfirm")
		arguments.delArg("build")
		arguments.delArg("c", "clean")
		arguments.delArg("q", "quiet")
		arguments.delArg("q", "quiet")
		arguments.delArg("y", "refresh")
		arguments.delArg("u", "sysupgrade")
		arguments.delArg("w", "downloadonly")

		oldConfirm := config.NoConfirm

		//conflicts have been checked so answer y for them
		if config.UseAsk {
			ask, _ := strconv.Atoi(cmdArgs.globals["ask"])
			uask := alpm.QuestionType(ask) | alpm.QuestionTypeConflictPkg
			cmdArgs.globals["ask"] = fmt.Sprint(uask)
		} else {
			conflict := false
			for _, split := range base {
				if _, ok := conflicts[split.Name]; ok {
					conflict = true
				}
			}

			if !conflict {
				config.NoConfirm = true
			}
		}

		depArguments := makeArguments()
		depArguments.addArg("D", "asdeps")
		expArguments := makeArguments()
		expArguments.addArg("D", "asexplicit")

		//remotenames: names of all non repo packages on the system
		_, _, localNames, remoteNames, err := filterPackages()
		if err != nil {
			return err
		}

		//cache as a stringset. maybe make it return a string set in the first
		//place
		remoteNamesCache := sliceToStringSet(remoteNames)
		localNamesCache := sliceToStringSet(localNames)

		for _, split := range base {
			pkgdest, ok := pkgdests[split.Name]
			if !ok {
				return fmt.Errorf("Could not find PKGDEST for: %s", split.Name)
			}

			arguments.addTarget(pkgdest)
			if !ds.Explicit.get(split.Name) && !localNamesCache.get(split.Name) && !remoteNamesCache.get(split.Name) {
				depArguments.addTarget(split.Name)
			}

			if ds.Explicit.get(split.Name) {
				if parser.existsArg("asdeps", "asdep") {
					depArguments.addTarget(split.Name)
				} else if parser.existsArg("asexplicit", "asexp") {
					expArguments.addTarget(split.Name)
				}
			}
		}

		err = show(passToPacman(arguments))
		if err != nil {
			return err
		}

		var mux sync.Mutex
		var wg sync.WaitGroup
		for _, pkg := range base {
			wg.Add(1)
			go updateVCSData(pkg.Name, srcinfo.Source, &mux, &wg)
		}

		wg.Wait()

		err = saveVCSInfo()
		if err != nil {
			fmt.Println(err)
		}

		if len(depArguments.targets) > 0 {
			_, stderr, err := capture(passToPacman(depArguments))
			if err != nil {
				return fmt.Errorf("%s%s", stderr, err)
			}
		}
		config.NoConfirm = oldConfirm
	}

	return nil
}
