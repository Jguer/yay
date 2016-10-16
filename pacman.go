package main

import (
	"fmt"
	"github.com/jguer/go-alpm"
	"github.com/jguer/yay/aur"
	"os"
	"os/exec"
	"strings"
)

// RepoSearch describes a Repository search.
type RepoSearch struct {
	Results []Result
}

// Result describes a pkg.
type Result struct {
	Name        string
	Repository  string
	Version     string
	Description string
	Installed   bool
}

func readConfig(pacmanconf string) (conf alpm.PacmanConfig, err error) {
	file, err := os.Open(pacmanconf)
	if err != nil {
		return
	}
	conf, err = alpm.ParseConfig(file)
	if err != nil {
		return
	}
	return
}

// InstallPackage handles package install
func InstallPackage(pkgs []string, conf *alpm.PacmanConfig, flags []string) error {
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
		return err
	}

	dbList, err := h.SyncDbs()
	if err != nil {
		return err
	}

	var foreign []string
	var args []string
	repocnt := 0
	args = append(args, "pacman")
	args = append(args, "-S")

	for _, pkg := range pkgs {
		found := false
		for _, db := range dbList.Slice() {
			_, err = db.PkgByName(pkg)
			if err == nil {
				found = true
				args = append(args, pkg)
				repocnt++
				break
			}
		}

		if !found {
			foreign = append(foreign, pkg)
		}
	}

	args = append(args, flags...)

	if repocnt != 0 {
		var cmd *exec.Cmd
		cmd = exec.Command("sudo", args...)
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		err = cmd.Run()
	}

	for _, aurpkg := range foreign {
		err = aur.Install(aurpkg, BuildDir, conf, flags)
	}

	return nil
}

// UpdatePackages handles cache update and upgrade
func UpdatePackages(flags []string) error {
	var cmd *exec.Cmd
	var args []string

	args = append(args, "pacman", "-Syu")
	args = append(args, flags...)

	cmd = exec.Command("sudo", args...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}

// SearchRepos searches and prints packages in repo
func SearchRepos(pkgName string, conf *alpm.PacmanConfig, mode int) (err error) {
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
	}

	dbList, err := h.SyncDbs()
	localdb, err := h.LocalDb()

	var installed bool
	var i int
	for _, db := range dbList.Slice() {
		for _, pkg := range db.PkgCache().Slice() {
			if strings.Contains(pkg.Name(), pkgName) {
				if r, _ := localdb.PkgByName(pkg.Name()); r != nil {
					installed = true
				} else {
					installed = false
				}

				switch {
				case mode != SearchMode && installed == true:
					fmt.Printf("%d \033[1m%s/\x1B[33m%s \x1B[36m%s \x1B[32;40mInstalled\033[0m\n%s\n",
						i, db.Name(), pkg.Name(), pkg.Version(), pkg.Description())
				case mode != SearchMode && installed != true:
					fmt.Printf("%d \033[1m%s/\x1B[33m%s \x1B[36m%s\033[0m\n%s\n",
						i, db.Name(), pkg.Name(), pkg.Version(), pkg.Description())
				case mode == SearchMode && installed == true:
					fmt.Printf("\033[1m%s/\x1B[33m%s \x1B[36m%s \x1B[32;40mInstalled\033[0m\n%s\n",
						db.Name(), pkg.Name(), pkg.Version(), pkg.Description())
				case mode == SearchMode && installed != true:
					fmt.Printf("\033[1m%s/\x1B[33m%s \x1B[36m%s\033[0m\n%s\n",
						db.Name(), pkg.Name(), pkg.Version(), pkg.Description())
				}
				i++
			}
		}
	}
	return
}

// SearchPackages handles repo searches. Creates a RepoSearch struct.
func SearchPackages(pkgName string, conf *alpm.PacmanConfig) (s RepoSearch, err error) {
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
	}

	dbList, err := h.SyncDbs()
	localdb, err := h.LocalDb()

	var installed bool
	for _, db := range dbList.Slice() {
		for _, pkg := range db.PkgCache().Slice() {
			if strings.Contains(pkg.Name(), pkgName) {
				if r, _ := localdb.PkgByName(pkg.Name()); r != nil {
					installed = true
				} else {
					installed = false
				}

				s.Results = append(s.Results, Result{
					Name:        pkg.Name(),
					Description: pkg.Description(),
					Version:     pkg.Version(),
					Repository:  db.Name(),
					Installed:   installed,
				})
			}
		}
	}
	return
}

//PrintSearch receives a RepoSearch type and outputs pretty text.
func (s *RepoSearch) PrintSearch(mode int) {
	for i, pkg := range s.Results {
		switch {
		case mode != SearchMode && pkg.Installed == true:
			fmt.Printf("%d \033[1m%s/\x1B[33m%s \x1B[36m%s \x1B[32;40mInstalled\033[0m\n%s\n",
				i, pkg.Repository, pkg.Name, pkg.Version, pkg.Description)
		case mode != SearchMode && pkg.Installed != true:
			fmt.Printf("%d \033[1m%s/\x1B[33m%s \x1B[36m%s\033[0m\n%s\n",
				i, pkg.Repository, pkg.Name, pkg.Version, pkg.Description)
		case mode == SearchMode && pkg.Installed == true:
			fmt.Printf("\033[1m%s/\x1B[33m%s \x1B[36m%s \x1B[32;40mInstalled\033[0m\n%s\n",
				pkg.Repository, pkg.Name, pkg.Version, pkg.Description)
		case mode == SearchMode && pkg.Installed != true:
			fmt.Printf("\033[1m%s/\x1B[33m%s \x1B[36m%s\033[0m\n%s\n",
				pkg.Repository, pkg.Name, pkg.Version, pkg.Description)
		}
	}
}

func passToPacman(op string, pkgs []string, flags []string) error {
	var cmd *exec.Cmd
	var args []string

	args = append(args, op)
	if len(pkgs) != 0 {
		args = append(args, pkgs...)
	}

	if len(flags) != 0 {
		args = append(args, flags...)
	}

	if strings.Contains(op, "-Q") {
		cmd = exec.Command("pacman", args...)
	} else {
		args = append([]string{"pacman"}, args...)
		cmd = exec.Command("sudo", args...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err

}
