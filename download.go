package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	rpc "github.com/mikkeloscar/aur"
)

func downloadFile(path string, url string) (err error) {
	// Create the file
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

// DownloadAndUnpack downloads url tgz and extracts to path.
func downloadAndUnpack(url string, path string, trim bool) (err error) {
	err = os.MkdirAll(path, 0755)
	if err != nil {
		return
	}

	tokens := strings.Split(url, "/")
	fileName := tokens[len(tokens)-1]

	tarLocation := path + fileName
	defer os.Remove(tarLocation)

	err = downloadFile(tarLocation, url)
	if err != nil {
		return
	}

	if trim {
		err = exec.Command("/bin/sh", "-c",
			config.TarBin+" --strip-components 2 --include='*/"+fileName[:len(fileName)-7]+"/trunk/' -xf "+tarLocation+" -C "+path).Run()
		os.Rename(path+"trunk", path+fileName[:len(fileName)-7]) // kurwa
	} else {
		err = exec.Command(config.TarBin, "-xf", tarLocation, "-C", path).Run()
	}
	if err != nil {
		return
	}

	return
}

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
func getPkgbuildfromABS(pkgN string, path string) (err error) {
	dbList, err := AlpmHandle.SyncDbs()
	if err != nil {
		return
	}

	for _, db := range dbList.Slice() {
		pkg, err := db.PkgByName(pkgN)
		if err == nil {
			var url string
			if db.Name() == "core" || db.Name() == "extra" {
				url = "https://projects.archlinux.org/svntogit/packages.git/snapshot/packages/" + pkg.Name() + ".tar.gz"
			} else if db.Name() == "community" {
				url = "https://projects.archlinux.org/svntogit/community.git/snapshot/community-packages/" + pkg.Name() + ".tar.gz"
			} else {
				return fmt.Errorf("Not in standard repositories")
			}
			fmt.Printf("\x1b[1;32m==>\x1b[1;33m %s \x1b[1;32mfound in ABS.\x1b[0m\n", pkgN)
			errD := downloadAndUnpack(url, path, true)
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

	fmt.Printf("\x1b[1;32m==>\x1b[1;33m %s \x1b[1;32mfound in AUR.\x1b[0m\n", pkgN)
	downloadAndUnpack(baseURL+aq[0].URLPath, dir, false)
	return
}
