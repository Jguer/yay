package main

import (
	"fmt"
	"os"
	"os/exec"
	"io/ioutil"
	"strings"

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
		//todo make pretty
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

		for _, pkg := range dc.AurMake {
			if pkg.Maintainer == "" {
				fmt.Println(boldRedFgBlackBg(arrow+" Warning:"),
					blackBg(pkg.Name+"-"+pkg.Version+" is orphaned"))
			}
		}

		for _, pkg := range dc.Aur {
			if pkg.Maintainer == "" {
				fmt.Println(boldRedFgBlackBg(arrow+" Warning:"),
					blackBg(pkg.Name+"-"+pkg.Version+" is orphaned"))
			}
		}

		printDownloadsFromRepo("Repo", dc.Repo)
		printDownloadsFromRepo("Repo Make", dc.RepoMake)
		printDownloadsFromAur("AUR", dc.Aur)
		printDownloadsFromAur("AUR Make", dc.AurMake)

		askCleanBuilds(dc.AurMake)
		askCleanBuilds(dc.Aur)
		fmt.Println()

		if !continueTask("Proceed with install?", "nN") {
			return fmt.Errorf("Aborting due to user")
		}

		if len(dc.RepoMake) + len(dc.Repo) > 0 {
			arguments := parser.copy()
			arguments.delArg("u", "sysupgrade")
			arguments.delArg("y", "refresh")
			arguments.op = "S"
			arguments.targets = make(stringSet)
			arguments.addArg("needed", "asdeps")
			for _, pkg := range dc.Repo {
				arguments.addTarget(pkg.Name())
			}
			for _, pkg := range dc.RepoMake {
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

		// if !continueTask("Proceed with download?", "nN") {
		// 	return fmt.Errorf("Aborting due to user")
		// }

		if _, ok := arguments.options["gendb"]; !ok {
			err = checkForConflicts(dc.Aur, dc.AurMake, dc.Repo, dc.RepoMake)
			if err != nil {
				return err
			}
		}

		err = dowloadPkgBuilds(dc.AurMake)
		if err != nil {
			return err
		}
		err = dowloadPkgBuilds(dc.Aur)
		if err != nil {
			return err
		}

		err = askEditPkgBuilds(dc.AurMake)
		if err != nil {
			return err
		}
		err = askEditPkgBuilds(dc.Aur)
		if err != nil {
			return err
		}
			
		if _, ok := arguments.options["gendb"]; ok {
			fmt.Println("GenDB finished. No packages were installed")
			return nil
		}

		// if !continueTask("Proceed with install?", "nN") {
		// 	return fmt.Errorf("Aborting due to user")
		// }

		err = downloadPkgBuildsSources(dc.AurMake)
		if err != nil {
			return err
		}
		err = downloadPkgBuildsSources(dc.Aur)
		if err != nil {
			return err
		}

		err = parsesrcinfos(dc.AurMake, srcinfos)
		if err != nil {
			return err
		}
		err = parsesrcinfos(dc.Aur, srcinfos)
		if err != nil {
			return err
		}

		err = buildInstallPkgBuilds(dc.AurMake, srcinfos, parser.targets, parser)
		if err != nil {
			return err
		}
		err = buildInstallPkgBuilds(dc.Aur, srcinfos, parser.targets, parser)
		if err != nil {
			return err
		}

		if len(dc.RepoMake)+len(dc.AurMake) > 0 {
			if continueTask("Remove make dependencies?", "yY") {
				return nil
			}

			removeArguments := makeArguments()
			removeArguments.addArg("R", "u")

			for _, pkg := range dc.RepoMake {
				removeArguments.addTarget(pkg.Name())
			}

			for _, pkg := range dc.AurMake {
				removeArguments.addTarget(pkg.Name)
			}

			oldValue := config.NoConfirm
			config.NoConfirm = true
			passToPacman(removeArguments)
			config.NoConfirm = oldValue
		}

		if config.CleanAfter {
			clean(dc.AurMake)
			clean(dc.Aur)
		}

		return nil
	}

	return nil
}

func askCleanBuilds(pkgs []*rpc.Pkg) {
	for _, pkg := range pkgs {
		dir := config.BuildDir + pkg.PackageBase + "/"

		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			if !continueTask(pkg.Name+" Directory exists. Clean Build?", "yY") {
				_ = os.RemoveAll(config.BuildDir + pkg.PackageBase)
			}
		}
	}
}

func checkForConflicts(aur []*rpc.Pkg, aurMake []*rpc.Pkg, repo []*alpm.Package,
	repoMake []*alpm.Package) error {

	localDb, err := alpmHandle.LocalDb()
	if err != nil {
		return err
	}
	var toRemove []string

	for _, pkg := range aur {
		for _, cpkg := range pkg.Conflicts {
			if _, err := localDb.PkgByName(cpkg); err == nil {
				toRemove = append(toRemove, cpkg)
			}
		}
	}

	for _, pkg := range aurMake {
		for _, cpkg := range pkg.Conflicts {
			if _, err := localDb.PkgByName(cpkg); err == nil {
				toRemove = append(toRemove, cpkg)
			}
		}
	}

	for _, pkg := range repo {
		pkg.Conflicts().ForEach(func(conf alpm.Depend) error {
			if _, err := localDb.PkgByName(conf.Name); err == nil {
				toRemove = append(toRemove, conf.Name)
			}
			return nil
		})
	}

	for _, pkg := range repoMake {
		pkg.Conflicts().ForEach(func(conf alpm.Depend) error {
			if _, err := localDb.PkgByName(conf.Name); err == nil {
				toRemove = append(toRemove, conf.Name)
			}
			return nil
		})
	}

	if len(toRemove) != 0 {
		fmt.Println(
			redFg("The following packages conflict with packages to install:"))
		for _, pkg := range toRemove {
			fmt.Println(yellowFg(pkg))
		}

		if !continueTask("Remove conflicting package(s)?", "nN") {
			return fmt.Errorf("Aborting due to user")
		}

		removeArguments := makeArguments()
		removeArguments.addArg("R", "d", "d")

		for _, pkg := range toRemove {
			removeArguments.addTarget(pkg)
		}

		oldValue := config.NoConfirm
		config.NoConfirm = true
		passToPacman(removeArguments)
		config.NoConfirm = oldValue
	}

	return nil
}

func askEditPkgBuilds(pkgs []*rpc.Pkg) (error)  {
	for _, pkg := range pkgs {
		dir := config.BuildDir + pkg.PackageBase + "/"

		if !continueTask(pkg.Name+" Edit PKGBUILD?", "yY") {
			editcmd := exec.Command(editor(), dir+"PKGBUILD")
			editcmd.Stdin, editcmd.Stdout, editcmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			editcmd.Run()
		}
	}

	return nil
}


func parsesrcinfos(pkgs []*rpc.Pkg, srcinfos map[string]*gopkg.PKGBUILD) (error) {

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
		if err == nil {
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
	}

	return nil
}

func dowloadPkgBuilds(pkgs []*rpc.Pkg) (err error) {
	for _, pkg := range pkgs {
		//todo make pretty
		fmt.Println("Downloading:", pkg.Name+"-"+pkg.Version)

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

func buildInstallPkgBuilds(pkgs []*rpc.Pkg, srcinfos map[string]*gopkg.PKGBUILD, targets stringSet, parser *arguments) (error) {
	//for n := len(pkgs) -1 ; n > 0; n-- {
	for n := 0; n < len(pkgs); n++ {
		pkg := pkgs[n]
		dir := config.BuildDir + pkg.PackageBase + "/"

		srcinfo := srcinfos[pkg.PackageBase]
		version := srcinfo.CompleteVersion()
		file, err := completeFileName(dir, pkg.Name + "-" + version.String())

		if file != "" {
			fmt.Println(boldRedFgBlackBg(arrow+" Warning:"),
				blackBg(pkg.Name+"-"+pkg.Version+ " Already made -- skipping build"))
		} else {
			err = passToMakepkg(dir, "-Cscf", "--noconfirm")
			if err != nil {
				return err
			}

			file, err = completeFileName(dir, pkg.Name + "-" + version.String())
			if err != nil {
				return err
			}

			if file == "" {
				return fmt.Errorf("Could not find built package")
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

		oldConfirm := config.NoConfirm
		config.NoConfirm = true

		if targets.get(pkg.Name) {
			arguments.addArg("asdeps")
		}

		arguments.addTarget(file)

		err = passToPacman(arguments)
		config.NoConfirm = oldConfirm
		if err !=nil {
			return err
		}
	}

	return nil
}

func clean(pkgs []*rpc.Pkg) {
	for _, pkg := range pkgs {
		dir := config.BuildDir + pkg.PackageBase + "/"

		fmt.Println(boldGreenFg(arrow +
			" CleanAfter enabled. Deleting " + pkg.Name  +" source folder."))
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
