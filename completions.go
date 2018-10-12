package main

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	alpm "github.com/jguer/go-alpm"
)

//CreateAURList creates a new completion file
func createAURList(out *os.File) (err error) {
	resp, err := http.Get(config.value["aururl"] + "/packages.gz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)

	scanner.Scan()
	for scanner.Scan() {
		out.WriteString(scanner.Text())
		out.WriteString("\tAUR\n")
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
			out.WriteString(pkg.Name())
			out.WriteString("\t" + pkg.DB().Name() + "\n")
			return nil
		})
		return nil
	})
	return nil
}

func updateCompletion(force bool) error {
	path := filepath.Join(config.cacheDir, "completion.cache")
	info, err := os.Stat(path)

	if os.IsNotExist(err) || (config.num["completioninterval"] != -1 &&
		time.Since(info.ModTime()).Hours() >= float64(config.num["completioninterval"]*24)) ||
		force {
		os.MkdirAll(filepath.Dir(path), 0755)
		out, errf := os.Create(path)
		if errf != nil {
			return errf
		}

		if createAURList(out) != nil {
			defer os.Remove(path)
		}
		erra := createRepoList(out)

		out.Close()
		return erra
	}

	return nil
}

// Complete provides completion info for shells
func complete(force bool) error {
	path := filepath.Join(config.cacheDir, "completion.cache")

	err := updateCompletion(force)
	if err != nil {
		return err
	}

	in, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = io.Copy(os.Stdout, in)
	return err
}
