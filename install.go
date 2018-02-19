package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"strconv"

	alpm "github.com/jguer/go-alpm"
	rpc "github.com/mikkeloscar/aur"
	gopkg "github.com/mikkeloscar/gopkgbuild"
)

// Install handles package installs
func install(parser *arguments) error {
	aur, repo, err := packageSlices(parser.targets.toSlice())
	if err != nil {
		return  err
	}

	srcinfos := make(map[string]*gopkg.PKGBUILD)
	var dc *depCatagories

	//fmt.Println(greenFg(arrow), greenFg("Resolving Dependencies"))
	requestTargets := aur

	//remotenames: names of all non repo packages on the system
	_, _, _, remoteNames, err := filterPackages()
	if err != nil {
		return  err
	}

	//cache as a stringset. maybe make it return a string set in the first
	//place
	remoteNamesCache := make(stringSet)
	for _, name := range remoteNames {
		remoteNamesCache.set(name)
	}

	//if we are doing -u also request every non repo package on the system
	if parser.existsArg("u", "sysupgrade") {
		requestTargets = append(requestTargets, remoteNames...)
	}

	if len(requestTargets) > 0 {
		fmt.Println(boldCyanFg("::"), boldFg("Querying AUR..."))
	}
	dt , err := getDepTree(requestTargets)
	if err != nil {
		return err
	}

	//only error if direct targets or deps are missing
	for missingName := range dt.Missing {
		if !remoteNamesCache.get(missingName) {
			return fmt.Errorf(boldRedFgBlackBg(arrow+" Error: ") +
				blackBg("Could not find all required package"))
		}
	}

	//create the arguments to pass for the repo install
	arguments := parser.copy()
	arguments.delArg("u", "sysupgrade")
	arguments.delArg("y", "refresh")
	arguments.op = "S"
	arguments.targets = make(stringSet)
	
	if parser.existsArg("u", "sysupgrade") {
		repoUp, aurUp, err  := upgradePkgs(dt)
		if err != nil {
			return err
		}

		for pkg := range aurUp {
			parser.addTarget(pkg)
		}
		
		for pkg := range repoUp {
			arguments.addTarget(pkg)
		}

	
		//discard stuff thats
		//not a target and
		//not an upgrade and
		//is installed
		for pkg := range dt.Aur {
			if !parser.targets.get(pkg) && remoteNamesCache.get(pkg) {
				delete(dt.Aur, pkg)
			}
		}
	}

	for _, pkg := range repo {
		arguments.addTarget(pkg)
	}


	hasAur := len(dt.Aur) != 0
	if hasAur {
		dc, err = getDepCatagories(parser.formatTargets(), dt)
		if err != nil {
			return err
		}

		//printDownloadsFromRepo("Repo", dc.Repo)
		//printDownloadsFromRepo("Repo Make", dc.RepoMake)
		//printDownloadsFromAur("AUR", dc.Aur)
		//printDownloadsFromAur("AUR Make", dc.AurMake)

		//fmt.Println(dc.MakeOnly)
		//fmt.Println(dc.AurSet)

		printDepCatagories(dc)
		fmt.Println()
	}
		
	if len(arguments.targets) > 0 {
		err := passToPacman(arguments)
		if err != nil {
			fmt.Errorf("Error installing repo packages.")
		}
	}

	if hasAur {
		if !parser.existsArg("gendb") {
			err = checkForConflicts(dc)
			if err != nil {
				return err
			}
		}

		askCleanBuilds(dc.Aur, dc.Bases)
		fmt.Println()

		if !continueTask("Proceed with install?", "nN") {
			return fmt.Errorf("Aborting due to user")
		}

		// if !continueTask("Proceed with download?", "nN") {
		// 	return fmt.Errorf("Aborting due to user")
		// }	

		err = dowloadPkgBuilds(dc.Aur, dc.Bases)
		if err != nil {
			return err
		}

		err = askEditPkgBuilds(dc.Aur, dc.Bases)
		if err != nil {
			return err
		}

		/*if len(dc.Repo) > 0 {
			arguments := parser.copy()
			arguments.delArg("u", "sysupgrade")
			arguments.delArg("y", "refresh")
			arguments.op = "S"
			arguments.targets = make(stringSet)
			arguments.addArg("needed", "asdeps")
			for _, pkg := range dc.Repo {
				arguments.addTarget(pkg.Name())
			}

			oldConfirm := config.NoConfirm
			config.NoConfirm = true
			passToPacman(arguments)
			config.NoConfirm = oldConfirm
			if err != nil {
				return err
			}
		}*/
	
		// if !continueTask("Proceed with install?", "nN") {
		// 	return fmt.Errorf("Aborting due to user")
		// }

		//this downloads the package build sources but also causes
		//a version bumb for vsc packages
		//that should not edit the sources so we should be safe to skip
		//it and parse the srcinfo at the current version
		if arguments.existsArg("gendb") {	
			err = parsesrcinfosFile(dc.Aur, srcinfos, dc.Bases)
			if err != nil {
				return err
			}


			fmt.Println(boldGreenFg(arrow+" GenDB finished. No packages were installed"))
			return nil
		}

		err = downloadPkgBuildsSources(dc.Aur)
		if err != nil {
			return err
		}


		err = parsesrcinfosGenerate(dc.Aur, srcinfos, dc.Bases)
		if err != nil {
			return err
		}

		err = buildInstallPkgBuilds(dc.Aur, srcinfos, parser.targets, parser, dc.Bases)
		if err != nil {
			return err
		}

		if len(dc.MakeOnly) > 0 {
			if continueTask("Remove make dependencies?", "yY") {
				return nil
			}

			removeArguments := makeArguments()
			removeArguments.addArg("R", "u")

			for pkg := range dc.MakeOnly {
				removeArguments.addTarget(pkg)
			}

			oldValue := config.NoConfirm
			config.NoConfirm = true
			passToPacman(removeArguments)
			config.NoConfirm = oldValue
		}

		if config.CleanAfter {
			clean(dc.Aur)
		}

		return nil
	}

	return nil
}

func askCleanBuilds(pkgs []*rpc.Pkg, bases map[string][]*rpc.Pkg) {
	for _, pkg := range pkgs {
		dir := config.BuildDir + pkg.PackageBase + "/"

		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			str := pkg.Name
			if len(bases[pkg.PackageBase]) > 1 || pkg.PackageBase != pkg.Name {
				str += " ("
				for _, split := range bases[pkg.PackageBase] {
					str += split.Name + " "
				}
				str = str[:len(str)-1] + ")"
			}

			if !continueTask(str+" Directory exists. Clean Build?", "yY") {
				_ = os.RemoveAll(config.BuildDir + pkg.PackageBase)
			}
		}
	}
}

func checkForConflicts(dc *depCatagories) error {
	localDb, err := alpmHandle.LocalDb()
	if err != nil {
		return err
	}
	toRemove := make(map[string]stringSet)

	for _, pkg := range dc.Aur {
		for _, cpkg := range pkg.Conflicts {
			if _, err := localDb.PkgByName(cpkg); err == nil {
				_, ok := toRemove[pkg.Name]
				if !ok {
					toRemove[pkg.Name] = make(stringSet)
				}
				toRemove[pkg.Name].set(cpkg)
			}
		}
	}

	for _, pkg := range dc.Repo {
		pkg.Conflicts().ForEach(func(conf alpm.Depend) error {
			if _, err := localDb.PkgByName(conf.Name); err == nil {
				_, ok := toRemove[pkg.Name()]
				if !ok {
					toRemove[pkg.Name()] = make(stringSet)
				}
				toRemove[pkg.Name()].set(conf.Name)
			}
			return nil
		})
	}

	if len(toRemove) != 0 {
		fmt.Println(
			redFg("Package conflicts found:"))
		for name, pkgs := range toRemove {
			str := yellowFg("\t" + name) + " Replaces"
			for pkg := range pkgs {
				str += " " + yellowFg(pkg)
			}

			fmt.Println(str)
		}

		if !continueTask("Continue with install?", "nN") {
			return fmt.Errorf("Aborting due to user")
		}

		ask, _ := strconv.Atoi(cmdArgs.globals["ask"])
		uask := alpm.Question(ask) | alpm.QuestionConflictPkg
		cmdArgs.globals["ask"] = fmt.Sprint(uask)
	}

	return nil
}

func askEditPkgBuilds(pkgs []*rpc.Pkg, bases map[string][]*rpc.Pkg) error {
	for _, pkg := range pkgs {
		dir := config.BuildDir + pkg.PackageBase + "/"

		str := "Edit PKGBUILD? " + pkg.PackageBase
		if len(bases[pkg.PackageBase]) > 1 || pkg.PackageBase != pkg.Name {
			str += " ("
			for _, split := range bases[pkg.PackageBase] {
				str += split.Name + " "
			}
			str = str[:len(str)-1] + ")"
		}

		if !continueTask(str, "yY") {
			editcmd := exec.Command(editor(), dir+"PKGBUILD")
			editcmd.Stdin, editcmd.Stdout, editcmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			editcmd.Run()
		}
	}

	return nil
}

func updateVSCdb(pkgbuild *gopkg.PKGBUILD) {
	for _, pkgsource := range pkgbuild.Source {
		owner, repo := parseSource(pkgsource)
		if owner != "" && repo != "" {
			err := branchInfo(pkgbuild.Pkgbase, owner, repo)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}

func parsesrcinfosFile(pkgs []*rpc.Pkg, srcinfos map[string]*gopkg.PKGBUILD, bases map[string][]*rpc.Pkg) error {
	for k, pkg := range pkgs {
		dir := config.BuildDir + pkg.PackageBase + "/"

		str := boldCyanFg("::") + boldFg(" Parsing SRCINFO (%d/%d): %s-%s")
		str2 := ""
		if len(bases[pkg.PackageBase]) > 1 || pkg.PackageBase != pkg.Name {
			str2 += " ("
			for _, split := range bases[pkg.PackageBase] {
				str2 += split.Name + " "
			}
			str2 = str2[:len(str2)-1] + ")"
		}
		fmt.Printf(str, k+1, len(pkgs), pkg.PackageBase, pkg.Version)
		fmt.Print(str2)
		fmt.Println()


		pkgbuild, err := gopkg.ParseSRCINFO(dir + ".SRCINFO")
		if err != nil {
			return fmt.Errorf("%s: %s", pkg.Name, err)
		}

		srcinfos[pkg.PackageBase] = pkgbuild
		updateVSCdb(pkgbuild)
	}

	return nil
}

func parsesrcinfosGenerate(pkgs []*rpc.Pkg, srcinfos map[string]*gopkg.PKGBUILD, bases map[string][]*rpc.Pkg) error {
	for k, pkg := range pkgs {
		dir := config.BuildDir + pkg.PackageBase + "/"

		str := "Parsing SRCINFO (%d/%d): %s-%s"
		str2 := ""
		if len(bases[pkg.PackageBase]) > 1 || pkg.PackageBase != pkg.Name {
			str2 += " ("
			for _, split := range bases[pkg.PackageBase] {
				str2 += split.Name + " "
			}
			str2 = str2[:len(str2)-1] + ")"
		}
		fmt.Printf(str, k+1, len(pkgs), pkg.PackageBase, pkg.Version)
		fmt.Print(str2)
		fmt.Println()


		
		cmd := exec.Command(config.MakepkgBin, "--printsrcinfo")
		cmd.Stderr = os.Stderr
		cmd.Dir = dir
		srcinfo, err := cmd.Output()

		if err != nil {
			return err
		}

		pkgbuild, err := gopkg.ParseSRCINFOContent(srcinfo)
		if err != nil {
			return fmt.Errorf("%s: %s", pkg.Name, err)
		}

		srcinfos[pkg.PackageBase] = pkgbuild
		updateVSCdb(pkgbuild)
	}

	return nil
}

func dowloadPkgBuilds(pkgs []*rpc.Pkg, bases map[string][]*rpc.Pkg) (err error) {
	for k, pkg := range pkgs {
		//todo make pretty
		str := boldCyanFg("::") + boldFg(" Downloading (%d/%d): %s-%s")
		str2 := ""
		if len(bases[pkg.PackageBase]) > 1 || pkg.PackageBase != pkg.Name {
			str2 += " ("
			for _, split := range bases[pkg.PackageBase] {
				str2 += split.Name + " "
			}
			str2 = str2[:len(str2)-1] + ")"
		}
		fmt.Printf(str, k+1, len(pkgs), pkg.PackageBase, pkg.Version)
		fmt.Print(str2)
		fmt.Println()

		err = downloadAndUnpack(baseURL+pkg.URLPath, config.BuildDir, false)
		if err != nil {
			return
		}
	}

	return
}

func downloadPkgBuildsSources(pkgs []*rpc.Pkg) (err error) {
	for _, pkg := range pkgs {
		dir := config.BuildDir + pkg.PackageBase + "/"
		err = passToMakepkg(dir, "--nobuild", "--nocheck", "--noprepare", "--nodeps")
		if err != nil {
			return
		}
	}

	return
}

func buildInstallPkgBuilds(pkgs []*rpc.Pkg, srcinfos map[string]*gopkg.PKGBUILD, targets stringSet, parser *arguments, bases map[string][]*rpc.Pkg) error {
	//for n := len(pkgs) -1 ; n > 0; n-- {
	for n := 0; n < len(pkgs); n++ {
		pkg := pkgs[n]

		dir := config.BuildDir + pkg.PackageBase + "/"
		built := true

		srcinfo := srcinfos[pkg.PackageBase]
		version := srcinfo.CompleteVersion()

		for _, split := range bases[pkg.PackageBase] {
			file, err := completeFileName(dir, split.Name+"-"+version.String())
			if err != nil {
				return err
			}

			if file == "" {
				built = false
			}
		}

		if built {
			fmt.Println(boldRedFgBlackBg(arrow+" Warning:"),
				blackBg(pkg.Name+"-"+pkg.Version+" Already made -- skipping build"))
		} else {
			err := passToMakepkg(dir, "-Ccf", "--noconfirm")
			if err != nil {
				return err
			}
		}

		arguments := parser.copy()
		arguments.targets = make(stringSet)
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

		for _, split := range bases[pkg.PackageBase] {
			file, err := completeFileName(dir, split.Name+"-"+version.String())
			if err != nil {
				return err
			}

			if file == "" {
				return fmt.Errorf("Could not find built package " + split.Name + "-" + version.String())
			}

			arguments.addTarget(file)
			if !targets.get(split.Name) {
				depArguments.addTarget(split.Name)
			}
		}

		oldConfirm := config.NoConfirm
		config.NoConfirm = true
		err := passToPacman(arguments)
		if err != nil {
			return err
		}
		if len(depArguments.targets) > 0 {
			err = passToPacman(depArguments)
			if err != nil {
				return err
			}
		}
		config.NoConfirm = oldConfirm
	}

	return nil
}

func clean(pkgs []*rpc.Pkg) {
	for _, pkg := range pkgs {
		dir := config.BuildDir + pkg.PackageBase + "/"

		fmt.Println(boldGreenFg(arrow +
			" CleanAfter enabled. Deleting " + pkg.Name + " source folder."))
		os.RemoveAll(dir)
	}
}

func completeFileName(dir, name string) (string, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if strings.HasPrefix(file.Name(), name) {
			return dir + file.Name(), nil
		}
	}

	return "", nil
}
