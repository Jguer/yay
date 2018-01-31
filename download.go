package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	rpc "github.com/mikkeloscar/aur"
)

func getPkgbuild(pkg string) (err error) {
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	wd = wd + "/"

	err = getPkgbuildfromABS(pkg, wd)
	if err == nil {
		return
	}

	err = getPkgbuildfromAUR(pkg, wd)
	return
}

// GetPkgbuild downloads pkgbuild from the ABS.
func getPkgbuildfromABS(pkgN string, dir string) (err error) {
	dbList, err := alpmHandle.SyncDbs()
	if err != nil {
		return
	}

	for _, db := range dbList.Slice() {
		pkg, err := db.PkgByName(pkgN)
		if err == nil {
			var url string
			if db.Name() == "core" || db.Name() == "extra" {
				url = "https://git.archlinux.org/svntogit/packages.git"
			} else if db.Name() == "community" {
				url = "https://git.archlinux.org/svntogit/community.git"
			} else {
				return fmt.Errorf("Not in standard repositories")
			}
			fmt.Println(boldGreenFg(arrow), boldYellowFg(pkgN), boldGreenFg("found in ABS."))
			errD := exec.Command("git", "clone", "--single-branch", "--branch", "packages/"+pkg.Name(), url, path.Join(dir, pkg.Name())).Run()
			return errD
		}
	}
	return fmt.Errorf("package not found")
}

// GetPkgbuild downloads pkgbuild from the AUR.
func getPkgbuildfromAUR(pkgN string, dir string) (err error) {
	aq, err := rpc.Info([]string{pkgN})
	if err != nil {
		return err
	}

	if len(aq) == 0 {
		return fmt.Errorf("no results")
	}

	fmt.Println(boldGreenFg(arrow), boldYellowFg(pkgN), boldGreenFg("found in AUR."))
	url := "aur@aur.archlinux.org/" + aq[0].Name + ".git"
	err = exec.Command("git", "clone", "ssh+git://"+url, path.Join(dir, aq[0].Name)).Run()
	if err != nil {
		// attempt https protocol which does not require SSH key setup
		err = exec.Command("git", "clone", "https://"+url, path.Join(dir, aq[0].Name)).Run()
	}
	return
}
