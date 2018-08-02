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
	return err == nil || os.IsExist(err)
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

func gitHasDiff(path string, name string) (bool, error) {
	stdout, stderr, err := capture(passToGit(filepath.Join(path, name), "rev-parse", "HEAD", "HEAD@{upstream}"))
	if err != nil {
		return false, fmt.Errorf("%s%s", stderr, err)
	}

	lines := strings.Split(stdout, "\n")
	head := lines[0]
	upstream := lines[1]

	return head != upstream, nil
}

func gitDownload(url string, path string, name string) (bool, error) {
	_, err := os.Stat(filepath.Join(path, name, ".git"))
	if os.IsNotExist(err) {
		_, stderr, err := capture(passToGit(path, "clone", "--no-progress", url, name))
		if err != nil {
			return false, fmt.Errorf("error cloning %s: stderr", name, stderr)
		}

		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("error reading %s", filepath.Join(path, name, ".git"))
	}

	_, stderr, err := capture(passToGit(filepath.Join(path, name), "fetch"))
	if err != nil {
		return false, fmt.Errorf("error fetching %s: %s", name, stderr)
	}

	return false, nil
}

func gitMerge(path string, name string) error {
	_, stderr, err := capture(passToGit(filepath.Join(path, name), "reset", "--hard", "HEAD"))
	if err != nil {
		return fmt.Errorf("error resetting %s: %s", name, stderr)
	}

	_, stderr, err = capture(passToGit(filepath.Join(path, name), "merge", "--no-edit", "--ff"))
	if err != nil {
		return fmt.Errorf("error merging %s: %s", name, stderr)
	}

	return nil
}

func gitDiff(path string, name string) error {
	err := show(passToGit(filepath.Join(path, name), "diff", "HEAD..HEAD@{upstream}"))

	return err
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
	missing := false
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	pkgs = removeInvalidTargets(pkgs)

	aur, repo, err := packageSlices(pkgs)

	if len(repo) > 0 {
		missing, err = getPkgbuildsfromABS(repo, wd)
		if err != nil {
			return err
		}
	}

	if len(aur) > 0 {
		_missing, err := getPkgbuildsfromAUR(aur, wd)
		if err != nil {
			return err
		}
		missing = missing || _missing
	}

	if missing {
		err = fmt.Errorf("")
	}

	return err
}

// GetPkgbuild downloads pkgbuild from the ABS.
func getPkgbuildsfromABS(pkgs []string, path string) (missing bool, err error) {
	dbList, err := alpmHandle.SyncDbs()
	if err != nil {
		return
	}

nextPkg:
	for _, pkgN := range pkgs {
		pkgDb, name := splitDbFromName(pkgN)

		for _, db := range dbList.Slice() {
			if pkgDb != "" && db.Name() != pkgDb {
				continue
			}

			pkg, err := db.PkgByName(name)
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
					fmt.Println(pkgN, "not in standard repositories")
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

		fmt.Println(pkgN, "could not find package in database")
		missing = true
	}

	if _, err := os.Stat(filepath.Join(cacheHome, "packages")); err == nil {
		os.RemoveAll(filepath.Join(cacheHome, "packages"))
	}

	return
}

// GetPkgbuild downloads pkgbuild from the AUR.
func getPkgbuildsfromAUR(pkgs []string, dir string) (bool, error) {
	missing := false
	strippedPkgs := make([]string, 0)
	for _, pkg := range pkgs {
		_, name := splitDbFromName(pkg)
		strippedPkgs = append(strippedPkgs, name)
	}

	aq, err := aurInfoPrint(strippedPkgs)
	if err != nil {
		return missing, err
	}

	for _, pkg := range aq {
		if _, err := os.Stat(filepath.Join(dir, pkg.PackageBase)); err == nil {
			fmt.Println(bold(red(arrow)), bold(cyan(pkg.Name)), "directory already exists")
			continue
		}

		if shouldUseGit(filepath.Join(dir, pkg.PackageBase)) {
			_, err = gitDownload(baseURL+"/"+pkg.PackageBase+".git", dir, pkg.PackageBase)
		} else {
			err = downloadAndUnpack(baseURL+aq[0].URLPath, dir)
		}

		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(bold(yellow(arrow)), "Downloaded", cyan(pkg.PackageBase), "from AUR")
		}
	}

	if len(aq) != len(pkgs) {
		missing = true
	}

	return missing, err
}
