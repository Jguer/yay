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
	aurs, repos, missing, err := packageSlices(parser.targets.toSlice())
	srcinfos := make(map[string]*gopkg.PKGBUILD)
	if err != nil {
		return err
	}

	if len(missing) > 0 {
		fmt.Println(missing)
		fmt.Println("Could not find all Targets")
	}

	arguments := parser.copy()
	arguments.delArg("u", "sysupgrade")
	arguments.delArg("y", "refresh")
	arguments.op = "S"
	arguments.targets = make(stringSet)
	arguments.addTarget(repos...)

	if len(repos) != 0 {
		err := passToPacman(arguments)
		if err != nil {
			fmt.Println("Error installing repo packages.")
		}
	}

	if len(aurs) != 0 {
		//todo mamakeke pretty
		fmt.Println(greenFg(arrow), greenFg("Resolving Dependencies"))

		dt, err := getDepTree(aurs)
		if err != nil {
			return err
		}

		if len(dt.Missing) > 0 {
			fmt.Println(dt.Missing)
			return fmt.Errorf("Could not find all Deps")
		}

		dc, err := getDepCatagories(aurs, dt)
		if err != nil {
			return err
		}

		for _, pkg := range dc.Aur {
			if pkg.Maintainer == "" {
				fmt.Println(boldRedFgBlackBg(arrow+" Warning:"),
					blackBg(pkg.Name+"-"+pkg.Version+" is orphaned"))
			}
		}



		//printDownloadsFromRepo("Repo", dc.Repo)
		//printDownloadsFromRepo("Repo Make", dc.RepoMake)
		//printDownloadsFromAur("AUR", dc.Aur)
		//printDownloadsFromAur("AUR Make", dc.AurMake)

		//fmt.Println(dc.MakeOnly)
		//fmt.Println(dc.AurSet)

		printDepCatagories(dc)
		fmt.Println()

		if !arguments.existsArg("gendb") {
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

		if len(dc.Repo) > 0 {
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
		}

		if arguments.existsArg("gendb") {
			fmt.Println("GenDB finished. No packages were installed")
			return nil
		}

		// if !continueTask("Proceed with install?", "nN") {
		// 	return fmt.Errorf("Aborting due to user")
		// }

		err = downloadPkgBuildsSources(dc.Aur)
		if err != nil {
			return err
		}

		err = parsesrcinfos(dc.Aur, srcinfos)
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

func parsesrcinfos(pkgs []*rpc.Pkg, srcinfos map[string]*gopkg.PKGBUILD) error {
	for _, pkg := range pkgs {
		dir := config.BuildDir + pkg.PackageBase + "/"

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

		for _, pkgsource := range pkgbuild.Source {
			owner, repo := parseSource(pkgsource)
			if owner != "" && repo != "" {
				err = branchInfo(pkg.Name, owner, repo)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func dowloadPkgBuilds(pkgs []*rpc.Pkg, bases map[string][]*rpc.Pkg) (err error) {
	for _, pkg := range pkgs {
		//todo make pretty
		str := "Downloading: " + pkg.PackageBase + "-" + pkg.Version
		if len(bases[pkg.PackageBase]) > 1 || pkg.PackageBase != pkg.Name {
			str += " ("
			for _, split := range bases[pkg.PackageBase] {
				str += split.Name + " "
			}
			str = str[:len(str)-1] + ")"
		}
		fmt.Println(str)

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
			err := passToMakepkg(dir, "-Cscf", "--noconfirm")
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
