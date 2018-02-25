package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
			errD := gitCheckout(url, "packages/"+pkg.Name(), filepath.Join(dir, pkg.Name()))
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
	err = gitCheckout("ssh+git://"+url, "master", filepath.Join(dir, aq[0].Name))
	if err != nil {
		// attempt https protocol which does not require SSH key setup
		err = gitCheckout("https://"+url, "master", filepath.Join(dir, aq[0].Name))
	}
	return
}

func gitCheckout(repo, branch, path string) error {
	// based on https://github.com/golang/tools/blob/master/cmd/tip/tip.go checkout
	// Clone git repo if it does not exist.
	if _, err := os.Stat(filepath.Join(path, ".git")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("mkdir: %v", err)
		}
		if err := exec.Command("git", "clone", "--single-branch", "--branch", branch, repo, path).Run(); err != nil {
			return fmt.Errorf("clone: %v", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("stat .git: %v", err)
	}

	// Pull down changes and update to hash.
	cmd := exec.Command("git", "fetch")
	cmd.Dir = path
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("fetch: %v", err)
	}
	cmd = exec.Command("git", "reset", "--hard", "origin/"+branch)
	cmd.Dir = path
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reset: %v", err)
	}
	cmd = exec.Command("git", "clean", "-d", "-f", "-x")
	cmd.Dir = path
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("clean: %v", err)
	}
	return nil
}
