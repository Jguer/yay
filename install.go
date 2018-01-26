package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

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
		return fmt.Errorf("Could not find all Targets")
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
		fmt.Println("Resolving Dependencies")

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
				fmt.Println(boldRedFgBlackBg(arrow+"Warning:"),
					blackBg(pkg.Name+"-"+pkg.Version+"is orphaned"))
			}
		}

		for _, pkg := range dc.Aur {
			if pkg.Maintainer == "" {
				fmt.Println(boldRedFgBlackBg(arrow+"Warning:"),
					blackBg(pkg.Name+"-"+pkg.Version+"is orphaned"))
			}
		}

		fmt.Println()

		p1 := func(a []*alpm.Package) {
			for _, v := range a {
				fmt.Print("  ", v.Name())
			}
		}

		p2 := func(a []*rpc.Pkg) {
			for _, v := range a {
				fmt.Print("  ", v.Name)
			}
		}

		fmt.Print("Repo (" + strconv.Itoa(len(dc.Repo)) + "):")
		p1(dc.Repo)
		fmt.Println()

		fmt.Print("Repo Make (" + strconv.Itoa(len(dc.RepoMake)) + "):")
		p1(dc.RepoMake)
		fmt.Println()

		fmt.Print("Aur (" + strconv.Itoa(len(dc.Aur)) + "):")
		p2(dc.Aur)
		fmt.Println()

		fmt.Print("Aur Make (" + strconv.Itoa(len(dc.AurMake)) + "):")
		p2(dc.AurMake)
		fmt.Println()

		fmt.Println()

		askCleanBuilds(dc.AurMake)
		askCleanBuilds(dc.Aur)

		fmt.Println()

		if !continueTask("Proceed with download?", "nN") {
			return fmt.Errorf("Aborting due to user")
		}

		err = dowloadPkgBuilds(dc.AurMake)
		if err != nil {
			return err
		}
		err = dowloadPkgBuilds(dc.Aur)
		if err != nil {
			return err
		}

		askEditPkgBuilds(dc.AurMake)
		askEditPkgBuilds(dc.Aur)

		if !continueTask("Proceed with install?", "nN") {
			return fmt.Errorf("Aborting due to user")
		}

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
			if continueTask("Remove make dependancies?", "yY") {
				return nil
			}

			removeArguments := makeArguments()
			removeArguments.addArg("R")

			for _, pkg := range dc.RepoMake {
				removeArguments.addTarget(pkg.Name())
			}

			for _, pkg := range dc.AurMake {
				removeArguments.addTarget(pkg.Name)
			}

			passToPacman(removeArguments)
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

func askEditPkgBuilds(pkgs []*rpc.Pkg) {
	for _, pkg := range pkgs {
		dir := config.BuildDir + pkg.PackageBase + "/"

		if !continueTask(pkg.Name+" Edit PKGBUILD?", "yY") {
			editcmd := exec.Command(editor(), dir+"PKGBUILD")
			editcmd.Stdin, editcmd.Stdout, editcmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			editcmd.Run()
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
