package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	alpm "github.com/jguer/go-alpm"
)

//CreateAURList creates a new completion file
func createAURList(out *os.File, shell string) (err error) {
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
		if shell == "fish" {
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
func createRepoList(out *os.File, shell string) (err error) {
	dbList, err := alpmHandle.SyncDbs()
	if err != nil {
		return
	}

	_ = dbList.ForEach(func(db alpm.Db) error {
		_ = db.PkgCache().ForEach(func(pkg alpm.Package) error {
			fmt.Print(pkg.Name())
			out.WriteString(pkg.Name())
			if shell == "fish" {
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

// Complete provides completion info for shells
func complete(shell string) error {
	var path string

	if shell == "fish" {
		path = filepath.Join(completionFile, "fish"+".cache")
	} else {
		path = filepath.Join(completionFile, "sh"+".cache")
	}
	info, err := os.Stat(path)

	if os.IsNotExist(err) || time.Since(info.ModTime()).Hours() > 48 {
		os.MkdirAll(filepath.Dir(completionFile), 0755)
		out, errf := os.Create(path)
		if errf != nil {
			return errf
		}

		if createAURList(out, shell) != nil {
			defer os.Remove(path)
		}
		erra := createRepoList(out, shell)

		out.Close()
		return erra
	}

	in, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = io.Copy(os.Stdout, in)
	return err
}
