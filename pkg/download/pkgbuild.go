package download

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/exec"
	"github.com/Jguer/yay/v10/pkg/lookup/query"
	"github.com/Jguer/yay/v10/pkg/runtime"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"
)

const smallArrow = " ->"
const arrow = "==>"

func gitDownload(bin string, flags string, url string, path string, name string) (bool, error) {
	_, err := os.Stat(filepath.Join(path, name, ".git"))
	if os.IsNotExist(err) {
		cmd := exec.PassToGit(bin, flags, path, "clone", "--no-progress", url, name)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		_, stderr, err := exec.Capture(cmd)
		if err != nil {
			return false, fmt.Errorf("error cloning %s: %s", name, stderr)
		}

		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("error reading %s", filepath.Join(path, name, ".git"))
	}

	cmd := exec.PassToGit(bin, flags, filepath.Join(path, name), "fetch")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	_, stderr, err := exec.Capture(cmd)
	if err != nil {
		return false, fmt.Errorf("error fetching %s: %s", name, stderr)
	}

	return false, nil
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

// DownloadAndUnpack downloads url tgz and extracts to path.
func downloadAndUnpack(tarBin string, url string, path string) error {
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

	_, stderr, err := exec.CaptureBin(tarBin, "-xf", tarLocation, "-C", path)
	if err != nil {
		return fmt.Errorf("%s", stderr)
	}

	return nil
}

// Pkgbuilds downloads a set of package PKGBUILDs from the AUR.
func Pkgbuilds(config *runtime.Configuration, bases []types.Base, toSkip types.StringSet, dir string) (types.StringSet, error) {
	cloned := make(types.StringSet)
	downloaded := 0
	var wg sync.WaitGroup
	var mux sync.Mutex
	var errs types.MultiError

	download := func(k int, base types.Base) {
		defer wg.Done()
		pkg := base.Pkgbase()

		if toSkip.Get(pkg) {
			mux.Lock()
			downloaded++
			str := text.Bold(text.Cyan("::") + " PKGBUILD up to date, Skipping (%d/%d): %s\n")
			fmt.Printf(str, downloaded, len(bases), text.Cyan(base.String()))
			mux.Unlock()
			return
		}

		if exec.ShouldUseGit(filepath.Join(dir, pkg), config.GitClone) {
			clone, err := gitDownload(config.GitBin, config.GitFlags, config.AURURL+"/"+pkg+".git", dir, pkg)
			if err != nil {
				errs.Add(err)
				return
			}
			if clone {
				mux.Lock()
				cloned.Set(pkg)
				mux.Unlock()
			}
		} else {
			err := downloadAndUnpack(config.TarBin, config.AURURL+base.URLPath(), dir)
			if err != nil {
				errs.Add(err)
				return
			}
		}

		mux.Lock()
		downloaded++
		str := text.Bold(text.Cyan("::") + " Downloaded PKGBUILD (%d/%d): %s\n")
		fmt.Printf(str, downloaded, len(bases), text.Cyan(base.String()))
		mux.Unlock()
	}

	count := 0
	for k, base := range bases {
		wg.Add(1)
		go download(k, base)
		count++
		if count%25 == 0 {
			wg.Wait()
		}
	}

	wg.Wait()

	return cloned, errs.Return()
}

// GetPkgbuild downloads pkgbuild from the ABS.
func getPkgbuildsfromABS(alpmHandle *alpm.Handle, cmdArgs *types.Arguments, tarBin string, cacheDir string, pkgs []string, path string) (bool, error) {
	var wg sync.WaitGroup
	var mux sync.Mutex
	var errs types.MultiError
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
		pkgDB, name := query.SplitDBFromName(pkgN)

		if pkgDB != "" {
			if db, err := alpmHandle.SyncDBByName(pkgDB); err == nil {
				pkg = db.Pkg(name)
			}
		} else {
			dbList.ForEach(func(db alpm.DB) error {
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
		switch {
		case err != nil && !os.IsNotExist(err):
			fmt.Fprintln(os.Stderr, text.Bold(text.Red(smallArrow)), err)
			continue
		case os.IsNotExist(err), cmdArgs.ExistsArg("f", "force"):
			if err = os.RemoveAll(filepath.Join(path, name)); err != nil {
				fmt.Fprintln(os.Stderr, text.Bold(text.Red(smallArrow)), err)
				continue
			}
		default:
			fmt.Printf("%s %s %s\n", text.Yellow(smallArrow), text.Cyan(name), "already downloaded -- use -f to overwrite")
			continue
		}

		names[name] = url
	}

	if len(missing) != 0 {
		fmt.Println(text.Yellow(text.Bold(smallArrow)), "Missing ABS packages: ", text.Cyan(strings.Join(missing, "  ")))
	}

	download := func(pkg string, url string) {
		defer wg.Done()
		if err := downloadAndUnpack(tarBin, url, cacheDir); err != nil {
			errs.Add(fmt.Errorf("%s Failed to get pkgbuild: %s: %s", text.Bold(text.Red(arrow)), text.Bold(text.Cyan(pkg)), text.Bold(text.Red(err.Error()))))
			return
		}

		_, stderr, err := exec.CaptureBin("mv", filepath.Join(cacheDir, "packages", pkg, "trunk"), filepath.Join(path, pkg))
		mux.Lock()
		downloaded++
		if err != nil {
			errs.Add(fmt.Errorf("%s Failed to move %s: %s", text.Bold(text.Red(arrow)), text.Bold(text.Cyan(pkg)), text.Bold(text.Red(stderr))))
		} else {
			fmt.Printf(text.Bold(text.Cyan("::"))+" Downloaded PKGBUILD from ABS (%d/%d): %s\n", downloaded, len(names), text.Cyan(pkg))
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
	errs.Add(os.RemoveAll(filepath.Join(cacheDir, "packages")))
	return len(missing) != 0, errs.Return()
}

func GetPkgbuilds(config *runtime.Configuration, cmdArgs *types.Arguments, alpmHandle *alpm.Handle, pkgs []string) error {
	missing := false
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	pkgs = query.RemoveInvalidTargets(config.Mode, pkgs)
	aur, repo, err := query.PackageSlices(alpmHandle, config.Mode, pkgs)

	if err != nil {
		return err
	}

	for n := range aur {
		_, pkg := query.SplitDBFromName(aur[n])
		aur[n] = pkg
	}

	info, err := query.AURInfoPrint(config, aur)
	if err != nil {
		return err
	}

	if len(repo) > 0 {
		missing, err = getPkgbuildsfromABS(alpmHandle, cmdArgs, config.TarBin, config.BuildDir, repo, wd)
		if err != nil {
			return err
		}
	}

	if len(aur) > 0 {
		allBases := types.GetBases(info)
		bases := make([]types.Base, 0)

		for _, base := range allBases {
			name := base.Pkgbase()
			_, err = os.Stat(filepath.Join(wd, name))
			switch {
			case err != nil && !os.IsNotExist(err):
				fmt.Fprintln(os.Stderr, text.Bold(text.Red(smallArrow)), err)
				continue
			case os.IsNotExist(err), cmdArgs.ExistsArg("f", "force"), exec.ShouldUseGit(filepath.Join(wd, name), config.GitClone):
				if err = os.RemoveAll(filepath.Join(wd, name)); err != nil {
					fmt.Fprintln(os.Stderr, text.Bold(text.Red(smallArrow)), err)
					continue
				}
			default:
				fmt.Printf("%s %s %s\n", text.Yellow(smallArrow), text.Cyan(name), "already downloaded -- use -f to overwrite")
				continue
			}

			bases = append(bases, base)
		}

		if _, err = Pkgbuilds(config, bases, nil, wd); err != nil {
			return err
		}

		missing = missing || len(aur) != len(info)
	}

	if missing {
		err = fmt.Errorf("")
	}

	return err
}
