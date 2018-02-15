package main

import (
	"fmt"
	"os"
	"os/exec"

	alpm "github.com/jguer/go-alpm"
	rpc "github.com/mikkeloscar/aur"
	gopkg "github.com/mikkeloscar/gopkgbuild"
)

// Install handles package installs
func install(parser *arguments) error {
	aurs, repos, missing, err := packageSlices(parser.targets.toSlice())
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

		err = buildInstallPkgBuilds(dc.AurMake, parser.targets)
		if err != nil {
			return err
		}
		err = buildInstallPkgBuilds(dc.Aur, parser.targets)
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

			file, err := os.OpenFile(dir + ".SRCINFO", os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				return err
			}
			defer file.Close()

			cmd := exec.Command(config.MakepkgBin, "--printsrcinfo")
			cmd.Stdout, cmd.Stderr = file, os.Stderr
			cmd.Dir = dir
			err = cmd.Run()

			if err != nil {
				return err
			}
		}


		pkgbuild, err := gopkg.ParseSRCINFO(dir + ".SRCINFO")
		if err == nil {
			for _, pkgsource := range pkgbuild.Source {
				owner, repo := parseSource(pkgsource)
				if owner != "" && repo != "" {
					err = branchInfo(pkg.Name, owner, repo)
					if err != nil {
						fmt.Println(err)
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
		err = passToMakepkg(dir, "-f", "--verifysource")
		if err != nil {
			return
		}
	}

	return
}

func buildInstallPkgBuilds(pkgs []*rpc.Pkg, targets stringSet) (err error) {
	//for n := len(pkgs) -1 ; n > 0; n-- {
	for n := 0; n < len(pkgs); n++ {
		pkg := pkgs[n]

		dir := config.BuildDir + pkg.PackageBase + "/"
		if targets.get(pkg.Name) {
			err = passToMakepkg(dir, "-Cscfi", "--noconfirm")
		} else {
			err = passToMakepkg(dir, "-Cscfi", "--noconfirm", "--asdeps")
		}
		if err != nil {
			return
		}
	}

	return
}
