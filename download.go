package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Decide what download method to use:
// Use the config option when the destination does not already exits
// If .git exists in the destination uer git
// Otherwise use a tarrball
func shouldUseGit(path string) bool {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return config.GitClone
	}

	_, err = os.Stat(filepath.Join(path, ".git"))
	if os.IsNotExist(err) {
		return false
	}

	return true
}

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

func gitDownload(url string, path string, name string) error {
	_, err := os.Stat(filepath.Join(path, name, ".git"))
	if os.IsNotExist(err) {
		err = passToGit(path, "clone", url, name)
		if err != nil {
			return fmt.Errorf("error cloning %s", name)
		}

		return nil
	} else if err != nil {
		return fmt.Errorf("error reading %s", filepath.Join(path, name, ".git"))
	}

	err = passToGit(filepath.Join(path, name), "fetch")
	if err != nil {
		return fmt.Errorf("error fetching %s", name)
	}

	err = passToGit(filepath.Join(path, name), "reset", "--hard", "HEAD")
	if err != nil {
		return fmt.Errorf("error reseting %s", name)
	}

	err = passToGit(filepath.Join(path, name), "merge", "--no-edit", "--ff")
	if err != nil {
		return fmt.Errorf("error merging %s", name)
	}

	return nil
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

func getPkgbuilds(pkgs []string) error {
	//possibleAurs := make([]string, 0, 0)
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	wd = wd + "/"

	missing, err := getPkgbuildsfromABS(pkgs, wd)
	if err != nil {
		return err
	}

	err = getPkgbuildsfromAUR(missing, wd)
	return err
}

// GetPkgbuild downloads pkgbuild from the ABS.
func getPkgbuildsfromABS(pkgs []string, path string) (missing []string, err error) {
	dbList, err := alpmHandle.SyncDbs()
	if err != nil {
		return
	}

nextPkg:
	for _, pkgN := range pkgs {
		for _, db := range dbList.Slice() {
			pkg, err := db.PkgByName(pkgN)
			if err == nil {
				var url string
				name := pkg.Base()
				if name == "" {
					name = pkg.Name()
				}

				if db.Name() == "core" || db.Name() == "extra" {
					url = "https://projects.archlinux.org/svntogit/packages.git/snapshot/packages/" + name + ".tar.gz"
				} else if db.Name() == "community" || db.Name() == "multilib" {
					url = "https://projects.archlinux.org/svntogit/community.git/snapshot/community-packages/" + name + ".tar.gz"
				} else {
					fmt.Println(pkgN + " not in standard repositories")
					continue nextPkg
				}

				errD := downloadAndUnpack(url, path, true)
				if errD != nil {
					fmt.Println(bold(magenta(pkg.Name())), bold(green(errD.Error())))
				}

				fmt.Println(bold(green(arrow)), bold(green("Downloaded")), bold(magenta(pkg.Name())), bold(green("from ABS")))
				continue nextPkg
			}
		}

		missing = append(missing, pkgN)
	}

	return
}

// GetPkgbuild downloads pkgbuild from the AUR.
func getPkgbuildsfromAUR(pkgs []string, dir string) (err error) {
	aq, err := aurInfo(pkgs)
	if err != nil {
		return err
	}

	for _, pkg := range aq {
		var err error
		if shouldUseGit(filepath.Join(dir, pkg.PackageBase)) {
			err = gitDownload(baseURL+"/"+pkg.PackageBase+".git", dir, pkg.PackageBase)
		} else {
			err = downloadAndUnpack(baseURL+aq[0].URLPath, dir, false)
		}

		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(bold(green(arrow)), bold(green("Downloaded")), bold(magenta(pkg.Name)), bold(green("from AUR")))
		}
	}

	return
}
