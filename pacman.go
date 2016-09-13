package main

import (
	"fmt"
	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/aur"
	"os"
	"os/exec"
	"strings"
)

// RepoResult describes a Repository package
type RepoResult struct {
	Description string
	Repository  string
	Version     string
	Name        string
}

// RepoSearch describes a Repository search
type RepoSearch struct {
	Resultcount int
	Results     []RepoResult
}

// InstallPackage handles package install
func InstallPackage(pkg string, conf alpm.PacmanConfig, flags ...string) (err error) {
	if found, err := aur.IspkgInRepo(pkg, conf); found {
		if err != nil {
			return err
		}
		var args string
		if len(flags) != 0 {
			args = fmt.Sprintf(" %s", strings.Join(flags, " "))
		}
		cmd := exec.Command("sudo", "pacman", "-S", pkg+args)
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		err = cmd.Run()
	} else {
		err = aur.Install(os.Args[2], BuildDir, conf, os.Args[3:]...)
	}

	return nil
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

// SearchPackages handles repo searches
func SearchPackages(pkgName string, conf alpm.PacmanConfig) (search RepoSearch, err error) {
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
	}

	dbList, _ := h.SyncDbs()

	for _, db := range dbList.Slice() {
		for _, pkg := range db.PkgCache().Slice() {
			if strings.Contains(pkg.Name(), pkgName) {
				fmt.Println(pkg.Name())
			}
		}
	}
	return
}

func (s RepoSearch) printSearch(index int) (err error) {
	for i, result := range s.Results {
		if index != SearchMode {
			fmt.Printf("%d \033[1m%s/\x1B[33m%s \x1B[36m%s\033[0m\n%s\n",
				i, result.Repository, result.Name, result.Version, result.Description)
		} else {
			fmt.Printf("\033[1m%s/\x1B[33m%s \x1B[36m%s\033[0m\n%s\n",
				result.Repository, result.Name, result.Version, result.Description)
		}
	}

	return nil
}
