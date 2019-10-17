package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	alpm "github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v9/pkg/multierror"
)

// TODO: yay-next passes args through the header, use that to unify ABS and AUR
func gitDownloadABS(url string, path string, name string) (bool, error) {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(filepath.Join(path, name))
	if os.IsNotExist(err) {
		cmd := passToGit(path, "clone", "--no-progress", "--single-branch",
			"-b", "packages/"+name, url, name)
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

	return true, nil
}

func getPkgbuilds(pkgs []string) error {
	missing := false
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	pkgs = removeInvalidTargets(pkgs)
	aur, repo, err := packageSlices(pkgs)

	if err != nil {
		return err
	}

	for n := range aur {
		_, pkg := splitDBFromName(aur[n])
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
			switch {
			case err != nil && !os.IsNotExist(err):
				fmt.Fprintln(os.Stderr, bold(red(smallArrow)), err)
				continue
			case os.IsNotExist(err), cmdArgs.existsArg("f", "force"):
				if err = os.RemoveAll(filepath.Join(wd, name)); err != nil {
					fmt.Fprintln(os.Stderr, bold(red(smallArrow)), err)
					continue
				}
			}

			bases = append(bases, base)
		}

		if err = downloadPkgbuilds(bases, nil, wd, false); err != nil {
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
	var errs multierror.MultiError
	names := make(map[string]string)
	missing := make([]string, 0)
	downloaded := 0

	dbList, err := alpmHandle.SyncDBs()
	if err != nil {
		return false, err
	}

	for _, pkgN := range pkgs {
		var pkg *alpm.Package
		var err error
		var url string
		pkgDB, name := splitDBFromName(pkgN)

		if pkgDB != "" {
			if db, err := alpmHandle.SyncDBByName(pkgDB); err == nil {
				pkg = db.Pkg(name)
			}
		} else {
			_ = dbList.ForEach(func(db alpm.DB) error {
				if pkg = db.Pkg(name); pkg != nil {
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

		// TODO: Check existence with ls-remote
		// https://git.archlinux.org/svntogit/packages.git
		switch pkg.DB().Name() {
		case "core", "extra", "testing":
			url = "https://git.archlinux.org/svntogit/packages.git"
		case "community", "multilib", "community-testing", "multilib-testing":
			url = "https://git.archlinux.org/svntogit/community.git"
		default:
			missing = append(missing, name)
			continue
		}

		_, err = os.Stat(filepath.Join(path, name))
		switch {
		case err != nil && !os.IsNotExist(err):
			fmt.Fprintln(os.Stderr, bold(red(smallArrow)), err)
			continue
		case os.IsNotExist(err), cmdArgs.existsArg("f", "force"):
			if err = os.RemoveAll(filepath.Join(path, name)); err != nil {
				fmt.Fprintln(os.Stderr, bold(red(smallArrow)), err)
				continue
			}
		default:
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
		if _, err := gitDownloadABS(url, config.ABSDir, pkg); err != nil {
			errs.Add(fmt.Errorf("%s Failed to get pkgbuild: %s: %s", bold(red(arrow)), bold(cyan(pkg)), bold(red(err.Error()))))
			return
		}

		_, stderr, err := capture(exec.Command("cp", "-r", filepath.Join(config.ABSDir, pkg, "trunk"), filepath.Join(path, pkg)))
		mux.Lock()
		downloaded++
		if err != nil {
			errs.Add(fmt.Errorf("%s Failed to link %s: %s", bold(red(arrow)), bold(cyan(pkg)), bold(red(stderr))))
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
	return len(missing) != 0, errs.Return()
}
