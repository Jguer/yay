package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	return os.IsExist(err)
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
		return fmt.Errorf("error resetting %s", name)
	}

	err = passToGit(filepath.Join(path, name), "merge", "--no-edit", "--ff")
	if err != nil {
		return fmt.Errorf("error merging %s", name)
	}

	return nil
}

// DownloadAndUnpack downloads url tgz and extracts to path.
func downloadAndUnpack(url string, path string) (err error) {
	err = os.MkdirAll(path, 0755)
	if err != nil {
		return
	}

	fileName := filepath.Base(url)

	tarLocation := filepath.Join(path, fileName)
	defer os.Remove(tarLocation)

	err = downloadFile(tarLocation, url)
	if err != nil {
		return
	}

	err = exec.Command(config.TarBin, "-xf", tarLocation, "-C", path).Run()
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

				if _, err := os.Stat(filepath.Join(path, name)); err == nil {
					fmt.Println(bold(red(arrow)), bold(cyan(name)), "directory already exists")
					continue nextPkg
				}

				switch db.Name() {
				case "core", "extra":
					url = "https://git.archlinux.org/svntogit/packages.git/snapshot/packages/" + name + ".tar.gz"
				case "community", "multilib":
					url = "https://git.archlinux.org/svntogit/community.git/snapshot/packages/" + name + ".tar.gz"
				default:
					fmt.Println(name + " not in standard repositories")
					continue nextPkg
				}

				errD := downloadAndUnpack(url, cacheHome)
				if errD != nil {
					fmt.Println(bold(red(arrow)), bold(cyan(pkg.Name())), bold(red(errD.Error())))
				}

				errD = exec.Command("mv", filepath.Join(cacheHome, "packages", name, "trunk"), filepath.Join(path, name)).Run()
				if errD != nil {
					fmt.Println(bold(red(arrow)), bold(cyan(pkg.Name())), bold(red(errD.Error())))
				} else {
					fmt.Println(bold(yellow(arrow)), "Downloaded", cyan(pkg.Name()), "from ABS")
				}

				continue nextPkg
			}
		}

		missing = append(missing, pkgN)
	}

	if _, err := os.Stat(filepath.Join(cacheHome, "packages")); err == nil {
		os.RemoveAll(filepath.Join(cacheHome, "packages"))
	}

	return
}

// GetPkgbuild downloads pkgbuild from the AUR.
func getPkgbuildsfromAUR(pkgs []string, dir string) (err error) {
	aq, err := aurInfoPrint(pkgs)
	if err != nil {
		return err
	}

	if (len(aq) != len(pkgs)) {
		return fmt.Errorf("Could not find all required packages");
	}

	for _, pkg := range aq {
		var err error
		if shouldUseGit(filepath.Join(dir, pkg.PackageBase)) {
			err = gitDownload(baseURL+"/"+pkg.PackageBase+".git", dir, pkg.PackageBase)
		} else {
			err = downloadAndUnpack(baseURL+aq[0].URLPath, dir)
		}

		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(bold(yellow(arrow)), "Downloaded", cyan(pkg.PackageBase), "from AUR")
		}
	}

	return
}
