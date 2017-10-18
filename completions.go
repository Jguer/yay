package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"

	alpm "github.com/jguer/go-alpm"
)

//CreateAURList creates a new completion file
func createAURList(out *os.File) (err error) {
	resp, err := http.Get("https://aur.archlinux.org/packages.gz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)

	scanner.Scan()
	for scanner.Scan() {
		fmt.Print(scanner.Text())
		out.WriteString(scanner.Text())
		if config.Shell == "fish" {
			fmt.Print("\tAUR\n")
			out.WriteString("\tAUR\n")
		} else {
			fmt.Print("\n")
			out.WriteString("\n")
		}
	}

	return nil
}

//CreatePackageList appends Repo packages to completion cache
func createRepoList(out *os.File) (err error) {
	dbList, err := alpmHandle.SyncDbs()
	if err != nil {
		return
	}

	_ = dbList.ForEach(func(db alpm.Db) error {
		_ = db.PkgCache().ForEach(func(pkg alpm.Package) error {
			fmt.Print(pkg.Name())
			out.WriteString(pkg.Name())
			if config.Shell == "fish" {
				fmt.Print("\t" + pkg.DB().Name() + "\n")
				out.WriteString("\t" + pkg.DB().Name() + "\n")
			} else {
				fmt.Print("\n")
				out.WriteString("\n")
			}
			return nil
		})
		return nil
	})
	return nil
}
