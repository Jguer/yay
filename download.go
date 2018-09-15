package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	alpm "github.com/jguer/go-alpm"
)

// Decide what download method to use:
// Use the config option when the destination does not already exits
// If .git exists in the destination uer git
// Otherwise use a tarrball
func shouldUseGit(path string) bool {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return config.boolean["GitClone"]
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
		cmd := passToGit(path, "clone", "--no-progress", url, name)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		_, stderr, err := capture(cmd)
		if err != nil {
			return false, fmt.Errorf("error cloning %s: %s", name, stderr)
		}

		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("error reading %s", filepath.Join(path, name, ".git"))
	}

	cmd := passToGit(filepath.Join(path, name), "fetch")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	_, stderr, err := capture(cmd)
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
func downloadAndUnpack(url string, path string) error {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return err
	}

	fileName := filepath.Base(url)

	tarLocation := filepath.Join(path, fileName)
	defer os.Remove(tarLocation)

	err = downloadFile(tarLocation, url)
	if err != nil {
		return err
	}

	_, stderr, err := capture(exec.Command(config.value["TarCommand"], "-xf", tarLocation, "-C", path))
	if err != nil {
		return fmt.Errorf("%s", stderr)
	}

	return nil
}

func getPkgbuilds(pkgs []string) error {
	missing := false
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	pkgs = removeInvalidTargets(pkgs)
	aur, repo, err := packageSlices(pkgs)

	for n := range aur {
		_, pkg := splitDbFromName(aur[n])
		aur[n] = pkg
	}

	info, err := aurInfoPrint(aur)
	if err != nil {
		return err
	}

	if len(repo) > 0 {
		missing, err = getPkgbuildsfromABS(repo, wd)
		if err != nil {
			return err
		}
	}

	if len(aur) > 0 {
		allBases := getBases(info)
		bases := make([]Base, 0)

		for _, base := range allBases {
			name := base.Pkgbase()
			_, err = os.Stat(filepath.Join(wd, name))
			if err != nil && !os.IsNotExist(err) {
				fmt.Println(bold(red(smallArrow)), err)
				continue
			} else if os.IsNotExist(err) || cmdArgs.existsArg("f", "force") || shouldUseGit(filepath.Join(wd, name)) {
				if err = os.RemoveAll(filepath.Join(wd, name)); err != nil {
					fmt.Println(bold(red(smallArrow)), err)
					continue
				}
			} else {
				fmt.Printf("%s %s %s\n", yellow(smallArrow), cyan(name), "already downloaded -- use -f to overwrite")
				continue
			}

			bases = append(bases, base)
		}

		if _, err = downloadPkgbuilds(bases, nil, wd); err != nil {
			return err
		}

		missing = missing || len(aur) != len(info)
	}

	if missing {
		err = fmt.Errorf("")
	}

	return err
}

// GetPkgbuild downloads pkgbuild from the ABS.
func getPkgbuildsfromABS(pkgs []string, path string) (bool, error) {
	var wg sync.WaitGroup
	var mux sync.Mutex
	var errs MultiError
	names := make(map[string]string)
	missing := make([]string, 0)
	downloaded := 0

	dbList, err := alpmHandle.SyncDbs()
	if err != nil {
		return false, err
	}

	for _, pkgN := range pkgs {
		var pkg *alpm.Package
		var err error
		var url string
		pkgDb, name := splitDbFromName(pkgN)

		if pkgDb != "" {
			if db, err := alpmHandle.SyncDbByName(pkgDb); err == nil {
				pkg, err = db.PkgByName(name)
			}
		} else {
			dbList.ForEach(func(db alpm.Db) error {
				if pkg, err = db.PkgByName(name); err == nil {
					return fmt.Errorf("")
				}
				return nil
			})
		}

		if pkg == nil {
			missing = append(missing, name)
			continue
		}

		name = pkg.Base()
		if name == "" {
			name = pkg.Name()
		}

		switch pkg.DB().Name() {
		case "core", "extra", "testing":
			url = "https://git.archlinux.org/svntogit/packages.git/snapshot/packages/" + name + ".tar.gz"
		case "community", "multilib", "community-testing", "multilib-testing":
			url = "https://git.archlinux.org/svntogit/community.git/snapshot/packages/" + name + ".tar.gz"
		default:
			missing = append(missing, name)
			continue
		}

		_, err = os.Stat(filepath.Join(path, name))
		if err != nil && !os.IsNotExist(err) {
			fmt.Println(bold(red(smallArrow)), err)
			continue
		} else if os.IsNotExist(err) || cmdArgs.existsArg("f", "force") {
			if err = os.RemoveAll(filepath.Join(path, name)); err != nil {
				fmt.Println(bold(red(smallArrow)), err)
				continue
			}
		} else {
			fmt.Printf("%s %s %s\n", yellow(smallArrow), cyan(name), "already downloaded -- use -f to overwrite")
			continue
		}

		names[name] = url
	}

	if len(missing) != 0 {
		fmt.Println(yellow(bold(smallArrow)), "Missing ABS packages: ", cyan(strings.Join(missing, "  ")))
	}

	download := func(pkg string, url string) {
		defer wg.Done()
		if err := downloadAndUnpack(url, config.cacheHome); err != nil {
			errs.Add(fmt.Errorf("%s Failed to get pkgbuild: %s: %s", bold(red(arrow)), bold(cyan(pkg)), bold(red(err.Error()))))
			return
		}

		_, stderr, err := capture(exec.Command("mv", filepath.Join(config.cacheHome, "packages", pkg, "trunk"), filepath.Join(path, pkg)))
		mux.Lock()
		downloaded++
		if err != nil {
			errs.Add(fmt.Errorf("%s Failed to move %s: %s", bold(red(arrow)), bold(cyan(pkg)), bold(red(string(stderr)))))
		} else {
			fmt.Printf(bold(cyan("::"))+" Downloaded PKGBUILD from ABS (%d/%d): %s\n", downloaded, len(names), cyan(pkg))
		}
		mux.Unlock()
	}

	count := 0
	for name, url := range names {
		wg.Add(1)
		go download(name, url)
		count++
		if count%25 == 0 {
			wg.Wait()
		}
	}

	wg.Wait()
	errs.Add(os.RemoveAll(filepath.Join(config.cacheHome, "packages")))
	return len(missing) != 0, errs.Return()
}
