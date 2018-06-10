package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	alpm "github.com/jguer/go-alpm"
	rpc "github.com/mikkeloscar/aur"
	gopkg "github.com/mikkeloscar/gopkgbuild"
)

// Install handles package installs
func install(parser *arguments) error {
	var err error
	var incompatible stringSet
	var do *depOrder
	var toClean []*rpc.Pkg
	var toEdit []*rpc.Pkg

	var aurUp upSlice
	var repoUp upSlice

	requestTargets := parser.copy().targets
	warnings := &aurWarnings{}

	removeMake := false
	srcinfosStale := make(map[string]*gopkg.PKGBUILD)

	//remotenames: names of all non repo packages on the system
	_, _, localNames, remoteNames, err := filterPackages()
	if err != nil {
		return err
	}

	//cache as a stringset. maybe make it return a string set in the first
	//place
	remoteNamesCache := sliceToStringSet(remoteNames)
	localNamesCache := sliceToStringSet(localNames)

	//create the arguments to pass for the repo install
	arguments := parser.copy()
	arguments.delArg("y", "refresh")
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
			requestTargets = append(requestTargets, up)
		}

		value, _, exists := cmdArgs.getArg("ignore")

		if len(ignore) > 0 {
			ignoreStr := strings.Join(ignore.toSlice(), ",")
			if exists {
				ignoreStr += "," + value
			}
			arguments.options["ignore"] = ignoreStr
		}

		fmt.Println()

		for pkg := range aurUp {
			parser.addTarget(pkg)
		}
	}

	targets := sliceToStringSet(parser.targets)

	dp, err := getDepPool(requestTargets, warnings)
	if err != nil {
		return err
	}

	err = dp.CheckMissing()
	if err != nil {
		return err
	}

	err = dp.CheckConflicts()
	if err != nil {
		return err
	}

	hasAur := len(dp.Aur) > 0

	if hasAur && 0 == os.Geteuid() {
		return fmt.Errorf(bold(red(arrow)) + " Refusing to install AUR Packages as root, Aborting.")
	}

	do = getDepOrder(dp)
	if err != nil {
		return err
	}

	for _, pkg := range do.Repo {
		arguments.addTarget(pkg.DB().Name() + "/" + pkg.Name())
	}

	for _, pkg := range dp.Groups {
		arguments.addTarget(pkg)
	}

	if len(do.Aur) == 0 && len(arguments.targets) == 0 && (!parser.existsArg("u", "sysupgrade") || mode == ModeAUR) {
		fmt.Println("There is nothing to do")
		return nil
	}

	if hasAur {
		hasAur = len(do.Aur) != 0

		do.Print()
		fmt.Println()

		if do.HasMake() {
			if !continueTask("Remove make dependencies after install?", "yY") {
				removeMake = true
			}
		}

		toClean, toEdit, err = cleanEditNumberMenu(do.Aur, do.Bases, remoteNamesCache)
		if err != nil {
			return err
		}

		cleanBuilds(toClean)

		toSkip := pkgBuildsToSkip(do.Aur, targets)
		err = downloadPkgBuilds(do.Aur, do.Bases, toSkip)
		if err != nil {
			return err
		}
	
		if len(toEdit) > 0 {
			if config.ShowDiffs {
				err = showPkgBuildDiffs(toEdit, do.Bases)
			} else {
				err = editPkgBuilds(toEdit, do.Bases)
			}
			if err != nil {
				return err
			}

			oldValue := config.NoConfirm
			config.NoConfirm = false
			if !continueTask(bold(green("Proceed with install?")), "nN") {
				return fmt.Errorf("Aborting due to user")
			}
			config.NoConfirm = oldValue
		}

		err = mergePkgBuilds(do.Aur)
		if err != nil {
			return err
		}


		//initial srcinfo parse before pkgver() bump
		err = parseSRCINFOFiles(do.Aur, srcinfosStale, do.Bases)
		if err != nil {
			return err
		}

		incompatible, err = getIncompatible(do.Aur, srcinfosStale, do.Bases)
		if err != nil {
			return err
		}

		if config.PGPFetch {
			err = checkPgpKeys(do.Aur, do.Bases, srcinfosStale)
			if err != nil {
				return err
			}
		}
	}

	if len(arguments.targets) > 0 || arguments.existsArg("u") {
		err := passToPacman(arguments)
		if err != nil {
			return fmt.Errorf("Error installing repo packages")
		}

		depArguments := makeArguments()
		depArguments.addArg("D", "asdeps")
		expArguments := makeArguments()
		expArguments.addArg("D", "asexplicit")

		for _, pkg := range do.Repo {
			if !dp.Explicit.get(pkg.Name()) && !localNamesCache.get(pkg.Name()) && !remoteNamesCache.get(pkg.Name()) {
				depArguments.addTarget(pkg.Name())
				continue
			}

			if parser.existsArg("asdeps", "asdep") && dp.Explicit.get(pkg.Name()) {
				depArguments.addTarget(pkg.Name())
			} else if parser.existsArg("asexp", "asexplicit") && dp.Explicit.get(pkg.Name()) {
				expArguments.addTarget(pkg.Name())
			}
		}

		if len(depArguments.targets) > 0 {
			_, stderr, err := passToPacmanCapture(depArguments)
			if err != nil {
				return fmt.Errorf("%s%s", stderr, err)
			}
		}

		if len(expArguments.targets) > 0 {
			_, stderr, err := passToPacmanCapture(expArguments)
			if err != nil {
				return fmt.Errorf("%s%s", stderr, err)
			}
		}
	}

	if hasAur {
		//conflicts have been checked so answer y for them
		ask, _ := strconv.Atoi(cmdArgs.globals["ask"])
		uask := alpm.QuestionType(ask) | alpm.QuestionTypeConflictPkg
		cmdArgs.globals["ask"] = fmt.Sprint(uask)

		err = downloadPkgBuildsSources(do.Aur, do.Bases, incompatible)
		if err != nil {
			return err
		}

		err = buildInstallPkgBuilds(dp, do, srcinfosStale, parser, incompatible)
		if err != nil {
			return err
		}

		if removeMake {
			removeArguments := makeArguments()
			removeArguments.addArg("R", "u")

			for _, pkg := range do.getMake() {
				removeArguments.addTarget(pkg)
			}

			oldValue := config.NoConfirm
			config.NoConfirm = true
			err = passToPacman(removeArguments)
			config.NoConfirm = oldValue

			if err != nil {
				return err
			}
		}

		if config.CleanAfter {
			clean(do.Aur)
		}

		return nil
	}

	return nil
}

func getIncompatible(pkgs []*rpc.Pkg, srcinfos map[string]*gopkg.PKGBUILD, bases map[string][]*rpc.Pkg) (stringSet, error) {
	incompatible := make(stringSet)
	alpmArch, err := alpmHandle.Arch()
	if err != nil {
		return nil, err
	}

nextpkg:
	for _, pkg := range pkgs {
		for _, arch := range srcinfos[pkg.PackageBase].Arch {
			if arch == "any" || arch == alpmArch {
				continue nextpkg
			}
		}

		incompatible.set(pkg.PackageBase)
	}

	if len(incompatible) > 0 {
		fmt.Println()
		fmt.Print(bold(yellow(arrow)) + " The following packages are not compatible with your architecture:")
		for pkg := range incompatible {
			fmt.Print("  " + cyan(pkg))
		}

		fmt.Println()

		if !continueTask("Try to build them anyway?", "nN") {
			return nil, fmt.Errorf("Aborting due to user")
		}
	}

	return incompatible, nil
}

func parsePackageList(dir string) (map[string]string, string, error) {
	stdout, stderr, err := passToMakepkgCapture(dir, "--packagelist")

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
		version = strings.Join(split[len(split)-3:len(split)-2], "-")
		pkgdests[pkgname] = line
	}

	return pkgdests, version, nil
}

func cleanEditNumberMenu(pkgs []*rpc.Pkg, bases map[string][]*rpc.Pkg, installed stringSet) ([]*rpc.Pkg, []*rpc.Pkg, error) {
	toPrint := ""
	askClean := false

	toClean := make([]*rpc.Pkg, 0)
	toEdit := make([]*rpc.Pkg, 0)

	for n, pkg := range pkgs {
		dir := filepath.Join(config.BuildDir, pkg.PackageBase)

		toPrint += fmt.Sprintf(magenta("%3d")+" %-40s", len(pkgs)-n,
			bold(formatPkgbase(pkg, bases)))
		if installed.get(pkg.Name) {
			toPrint += bold(green(" (Installed)"))
		}

		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			toPrint += bold(green(" (Build Files Exist)"))
			askClean = true
		}

		toPrint += "\n"
	}

	fmt.Print(toPrint)

	if askClean {
		fmt.Println(bold(green(arrow + " Packages to cleanBuild?")))
		fmt.Println(bold(green(arrow) + cyan(" [N]one ") + "[A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)"))
		fmt.Print(bold(green(arrow + " ")))
		cleanInput, err := getInput(config.AnswerClean)
		if err != nil {
			return nil, nil, err
		}

		cInclude, cExclude, cOtherInclude, cOtherExclude := parseNumberMenu(cleanInput)
		cIsInclude := len(cExclude) == 0 && len(cOtherExclude) == 0

		if cOtherInclude.get("abort") || cOtherInclude.get("ab") {
			return nil, nil, fmt.Errorf("Aborting due to user")
		}

		if !cOtherInclude.get("n") && !cOtherInclude.get("none") {
			for i, pkg := range pkgs {
				dir := filepath.Join(config.BuildDir, pkg.PackageBase)
				if _, err := os.Stat(dir); os.IsNotExist(err) {
					continue
				}

				if !cIsInclude && cExclude.get(len(pkgs)-i) {
					continue
				}

				if installed.get(pkg.Name) && (cOtherInclude.get("i") || cOtherInclude.get("installed")) {
					toClean = append(toClean, pkg)
					continue
				}

				if !installed.get(pkg.Name) && (cOtherInclude.get("no") || cOtherInclude.get("notinstalled")) {
					toClean = append(toClean, pkg)
					continue
				}

				if cOtherInclude.get("a") || cOtherInclude.get("all") {
					toClean = append(toClean, pkg)
					continue
				}

				if cIsInclude && (cInclude.get(len(pkgs)-i) || cOtherInclude.get(pkg.PackageBase)) {
					toClean = append(toClean, pkg)
					continue
				}

				if !cIsInclude && (!cExclude.get(len(pkgs)-i) && !cOtherExclude.get(pkg.PackageBase)) {
					toClean = append(toClean, pkg)
					continue
				}
			}
		}
	}

	if config.ShowDiffs {
		fmt.Println(bold(green(arrow + " Diffs to show?")))
	} else {
		fmt.Println(bold(green(arrow + " PKGBUILDs to edit?")))
	}
	fmt.Println(bold(green(arrow) + cyan(" [N]one ") + "[A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)"))

	fmt.Print(bold(green(arrow + " ")))

	editInput, err := getInput(config.AnswerEdit)
	if err != nil {
		return nil, nil, err
	}

	eInclude, eExclude, eOtherInclude, eOtherExclude := parseNumberMenu(editInput)
	eIsInclude := len(eExclude) == 0 && len(eOtherExclude) == 0

	if eOtherInclude.get("abort") || eOtherInclude.get("ab") {
		return nil, nil, fmt.Errorf("Aborting due to user")
	}

	if !eOtherInclude.get("n") && !eOtherInclude.get("none") {
		for i, pkg := range pkgs {
			if !eIsInclude && eExclude.get(len(pkgs)-i) {
				continue
			}

			if installed.get(pkg.Name) && (eOtherInclude.get("i") || eOtherInclude.get("installed")) {
				toEdit = append(toEdit, pkg)
				continue
			}

			if !installed.get(pkg.Name) && (eOtherInclude.get("no") || eOtherInclude.get("notinstalled")) {
				toEdit = append(toEdit, pkg)
				continue
			}

			if eOtherInclude.get("a") || eOtherInclude.get("all") {
				toEdit = append(toEdit, pkg)
				continue
			}

			if eIsInclude && (eInclude.get(len(pkgs)-i) || eOtherInclude.get(pkg.PackageBase)) {
				toEdit = append(toEdit, pkg)
			}

			if !eIsInclude && (!eExclude.get(len(pkgs)-i) && !eOtherExclude.get(pkg.PackageBase)) {
				toEdit = append(toEdit, pkg)
			}
		}
	}

	return toClean, toEdit, nil
}

func cleanBuilds(pkgs []*rpc.Pkg) {
	for i, pkg := range pkgs {
		dir := filepath.Join(config.BuildDir, pkg.PackageBase)
		fmt.Printf(bold(cyan("::")+" Deleting (%d/%d): %s\n"), i+1, len(pkgs), cyan(dir))
		os.RemoveAll(dir)
	}
}

func showPkgBuildDiffs(pkgs []*rpc.Pkg, bases map[string][]*rpc.Pkg) error {
	for _, pkg := range pkgs {
		dir := filepath.Join(config.BuildDir, pkg.PackageBase)
		if shouldUseGit(dir) {
			hasDiff, err := gitHasDiff(config.BuildDir, pkg.PackageBase)
			if err != nil {
				return err
			}

			if !hasDiff {
				fmt.Printf("%s %s: %s\n", bold(yellow(arrow)), cyan(formatPkgbase(pkg, bases)), bold("No changes -- skipping"))
				continue
			}

			args := []string{"diff", "HEAD..HEAD@{upstream}", "--src-prefix", dir + "/", "--dst-prefix", dir + "/"}
			if useColor {
				args = append(args, "--color=always")
			} else {
				args = append(args, "--color=never")
			}
			err = passToGit(dir, args...)
			if err != nil {
				return err
			}
		} else {
			editor, editorArgs := editor()
			editorArgs = append(editorArgs, filepath.Join(dir, "PKGBUILD"))
			editcmd := exec.Command(editor, editorArgs...)
			editcmd.Stdin, editcmd.Stdout, editcmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			err := editcmd.Run()
			if err != nil {
				return fmt.Errorf("Editor did not exit successfully, Aborting: %s", err)
			}
		}
	}

	return nil
}

func editPkgBuilds(pkgs []*rpc.Pkg, bases map[string][]*rpc.Pkg) error {
	pkgbuilds := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		dir := filepath.Join(config.BuildDir, pkg.PackageBase)
		pkgbuilds = append(pkgbuilds, filepath.Join(dir, "PKGBUILD"))
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

func parseSRCINFOFiles(pkgs []*rpc.Pkg, srcinfos map[string]*gopkg.PKGBUILD, bases map[string][]*rpc.Pkg) error {
	for k, pkg := range pkgs {
		dir := filepath.Join(config.BuildDir, pkg.PackageBase)

		str := bold(cyan("::") + " Parsing SRCINFO (%d/%d): %s\n")
		fmt.Printf(str, k+1, len(pkgs), cyan(formatPkgbase(pkg, bases)))

		pkgbuild, err := gopkg.ParseSRCINFO(filepath.Join(dir, ".SRCINFO"))
		if err != nil {
			return fmt.Errorf("%s: %s", pkg.Name, err)
		}

		srcinfos[pkg.PackageBase] = pkgbuild
	}

	return nil
}

func tryParsesrcinfosFile(pkgs []*rpc.Pkg, srcinfos map[string]*gopkg.PKGBUILD, bases map[string][]*rpc.Pkg) {
	for k, pkg := range pkgs {
		dir := filepath.Join(config.BuildDir, pkg.PackageBase)

		str := bold(cyan("::") + " Parsing SRCINFO (%d/%d): %s\n")
		fmt.Printf(str, k+1, len(pkgs), cyan(formatPkgbase(pkg, bases)))

		pkgbuild, err := gopkg.ParseSRCINFO(filepath.Join(dir, ".SRCINFO"))
		if err != nil {
			fmt.Printf("cannot parse %s skipping: %s\n", pkg.Name, err)
			continue
		}

		srcinfos[pkg.PackageBase] = pkgbuild
	}
}

func pkgBuildsToSkip(pkgs []*rpc.Pkg, targets stringSet) stringSet {
	toSkip := make(stringSet)

	for _, pkg := range pkgs {
		if config.ReDownload == "no" || (config.ReDownload == "yes" && !targets.get(pkg.Name)) {
			dir := filepath.Join(config.BuildDir, pkg.PackageBase, ".SRCINFO")
			pkgbuild, err := gopkg.ParseSRCINFO(dir)

			if err == nil {
				versionRPC, errR := gopkg.NewCompleteVersion(pkg.Version)
				versionPKG, errP := gopkg.NewCompleteVersion(pkgbuild.Version())
				if errP == nil && errR == nil {
					if !versionRPC.Newer(versionPKG) {
						toSkip.set(pkg.PackageBase)
					}
				}
			}
		}
	}

	return toSkip
}

func mergePkgBuilds(pkgs []*rpc.Pkg) error {
	for _, pkg := range pkgs {
		if shouldUseGit(filepath.Join(config.BuildDir, pkg.PackageBase)) {
			err := gitMerge(baseURL + "/" + pkg.PackageBase + ".git", config.BuildDir, pkg.PackageBase)
			if err != nil {
				return err
			}
		}
	}

	return nil
}


func downloadPkgBuilds(pkgs []*rpc.Pkg, bases map[string][]*rpc.Pkg, toSkip stringSet) error {
	for k, pkg := range pkgs {
		if toSkip.get(pkg.PackageBase) {
			str := bold(cyan("::") + " PKGBUILD up to date, Skipping (%d/%d): %s\n")
			fmt.Printf(str, k+1, len(pkgs), cyan(formatPkgbase(pkg, bases)))
			continue
		}

		str := bold(cyan("::") + " Downloading PKGBUILD (%d/%d): %s\n")

		fmt.Printf(str, k+1, len(pkgs), cyan(formatPkgbase(pkg, bases)))

		if shouldUseGit(filepath.Join(config.BuildDir, pkg.PackageBase)) {
			err := gitDownload(baseURL + "/" + pkg.PackageBase + ".git", config.BuildDir, pkg.PackageBase)
			if err != nil {
				return err
			}
		} else {
			err := downloadAndUnpack(baseURL+pkg.URLPath, config.BuildDir)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func downloadPkgBuildsSources(pkgs []*rpc.Pkg, bases map[string][]*rpc.Pkg, incompatible stringSet) (err error) {
	for _, pkg := range pkgs {
		dir := filepath.Join(config.BuildDir, pkg.PackageBase)
		args := []string{"--verifysource", "-Ccf"}

		if incompatible.get(pkg.PackageBase) {
			args = append(args, "--ignorearch")
		}

		err = passToMakepkg(dir, args...)
		if err != nil {
			return fmt.Errorf("Error downloading sources: %s", cyan(formatPkgbase(pkg, bases)))
		}
	}

	return
}

func buildInstallPkgBuilds(dp *depPool, do *depOrder, srcinfos map[string]*gopkg.PKGBUILD, parser *arguments, incompatible stringSet) error {
	for _, pkg := range do.Aur {
		dir := filepath.Join(config.BuildDir, pkg.PackageBase)
		built := true

		srcinfo := srcinfos[pkg.PackageBase]

		args := []string{"--nobuild", "-fC"}

		if incompatible.get(pkg.PackageBase) {
			args = append(args, "--ignorearch")
		}

		//pkgver bump
		err := passToMakepkg(dir, args...)
		if err != nil {
			return fmt.Errorf("Error making: %s", pkg.Name)
		}

		pkgdests, version, err := parsePackageList(dir)
		if err != nil {
			return err
		}

		if config.ReBuild == "no" || (config.ReBuild == "yes" && !dp.Explicit.get(pkg.Name)) {
			for _, split := range do.Bases[pkg.PackageBase] {
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

		if built {
			fmt.Println(bold(yellow(arrow)),
				cyan(pkg.Name+"-"+version)+bold(" Already made -- skipping build"))
		} else {
			args := []string{"-cf", "--noconfirm", "--noextract", "--noprepare", "--holdver"}

			if incompatible.get(pkg.PackageBase) {
				args = append(args, "--ignorearch")
			}

			err := passToMakepkg(dir, args...)
			if err != nil {
				return fmt.Errorf("Error making: %s", pkg.Name)
			}
		}

		arguments := parser.copy()
		arguments.clearTargets()
		arguments.op = "U"
		arguments.delArg("confirm")
		arguments.delArg("c", "clean")
		arguments.delArg("q", "quiet")
		arguments.delArg("q", "quiet")
		arguments.delArg("y", "refresh")
		arguments.delArg("u", "sysupgrade")
		arguments.delArg("w", "downloadonly")

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

		for _, split := range do.Bases[pkg.PackageBase] {
			pkgdest, ok := pkgdests[split.Name]
			if !ok {
				return fmt.Errorf("Could not find PKGDEST for: %s", split.Name)
			}

			arguments.addTarget(pkgdest)
			if !dp.Explicit.get(split.Name) && !localNamesCache.get(split.Name) && !remoteNamesCache.get(split.Name) {
				depArguments.addTarget(split.Name)
			}

			if dp.Explicit.get(split.Name) {
				if parser.existsArg("asdeps", "asdep") {
					depArguments.addTarget(split.Name)
				} else if parser.existsArg("asexplicit", "asexp") {
					expArguments.addTarget(split.Name)
				}
			}
		}

		oldConfirm := config.NoConfirm
		config.NoConfirm = true
		err = passToPacman(arguments)
		if err != nil {
			return err
		}

		for _, pkg := range do.Bases[pkg.PackageBase] {
			updateVCSData(pkg.Name, srcinfo.Source)
		}

		if len(depArguments.targets) > 0 {
			_, stderr, err := passToPacmanCapture(depArguments)
			if err != nil {
				return fmt.Errorf("%s%s", stderr, err)
			}
		}
		config.NoConfirm = oldConfirm
	}

	return nil
}

func clean(pkgs []*rpc.Pkg) {
	for _, pkg := range pkgs {
		dir := filepath.Join(config.BuildDir, pkg.PackageBase)

		fmt.Println(bold(green(arrow +
			" CleanAfter enabled. Deleting " + pkg.Name + " source folder.")))
		os.RemoveAll(dir)
	}
}
