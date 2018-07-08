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

// Generates aur or repo completion cache file
func completePart(shell string, path string, aur bool) error {
	info, err := os.Stat(path)

	// Cache is old or missing. Generate and print
	if os.IsNotExist(err) || time.Since(info.ModTime()).Hours() > 48 {
		os.MkdirAll(filepath.Dir(path), 0755)
		out, errf := os.Create(path)
		if errf != nil {
			return errf
		}

		var erra error
		if aur {
			erra = createAURList(out, shell)
		} else {
			erra = createRepoList(out, shell)
		}
		out.Close()
		if erra != nil {
			defer os.Remove(path)
		}
		return erra
	}
	// Cache is good. Open and print
	in, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = io.Copy(os.Stdout, in)
	return err
}

// Complete provides completion info for shells
func complete(shell string) error {
	var path_aur string
	var path_repo string

	var err error

	if shell == "fish" {
		path_aur = filepath.Join(cacheHome, "aur_fish"+".cache")
		path_repo = filepath.Join(cacheHome, "repo_fish"+".cache")
	} else {
		path_aur = filepath.Join(cacheHome, "aur_sh"+".cache")
		path_repo = filepath.Join(cacheHome, "repo_sh"+".cache")
	}

	// Repo
	err = completePart(shell, path_repo, false)
	if err != nil {
		return err
	}

	// AUR
	err = completePart(shell, path_aur, true)
	if err != nil {
		return err
	}

	return nil
}
