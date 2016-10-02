package main

import (
	"fmt"
	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/aur"
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
func InstallPackage(pkg string, conf *alpm.PacmanConfig, flags string) error {
	if found, err := aur.IspkgInRepo(pkg, conf); found {
		if err != nil {
			return err
		}

		var cmd *exec.Cmd
		if flags == "" {
			cmd = exec.Command("sudo", "pacman", "-S", pkg)
		} else {
			cmd = exec.Command("sudo", "pacman", "-S", pkg, flags)
		}
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		err = cmd.Run()
	} else {
		err = aur.Install(pkg, BuildDir, conf, flags)
	}

	return nil
}

// UpdatePackages handles cache update and upgrade
func UpdatePackages(flags string) error {
	var cmd *exec.Cmd
	if flags == "" {
		cmd = exec.Command("sudo", "pacman", "-Syu")
	} else {
		cmd = exec.Command("sudo", "pacman", "-Syu", flags)
	}
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
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

func passToPacman(op string, flags string) error {
	var cmd *exec.Cmd
	if flags == "" {
		cmd = exec.Command("sudo", "pacman", op)
	} else {
		cmd = exec.Command("sudo", "pacman", op, flags)
	}
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err

}
