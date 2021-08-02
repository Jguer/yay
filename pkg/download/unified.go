package download

import (
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/go-alpm/v2"

	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/multierror"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/settings/exe"
	"github.com/Jguer/yay/v10/pkg/text"
)

type DBSearcher interface {
	SyncPackage(string) db.IPackage
	SatisfierFromDB(string, string) db.IPackage
}

func downloadGitRepo(cmdRunner exe.Runner,
	cmdBuilder exe.GitCmdBuilder, pkgURL, pkgName, dest string, force bool, gitArgs ...string) error {
	finalDir := filepath.Join(dest, pkgName)

	if _, err := os.Stat(filepath.Join(finalDir, ".git")); os.IsNotExist(err) || (err == nil && force) {
		if _, errD := os.Stat(finalDir); force && errD == nil {
			if errR := os.RemoveAll(finalDir); errR != nil {
				return ErrGetPKGBUILDRepo{inner: errR, pkgName: pkgName, errOut: ""}
			}
		}

		gitArgs = append(gitArgs, pkgURL, pkgName)

		cloneArgs := make([]string, 0, len(gitArgs)+4)
		cloneArgs = append(cloneArgs, "clone", "--no-progress")
		cloneArgs = append(cloneArgs, gitArgs...)
		cmd := cmdBuilder.BuildGitCmd(dest, cloneArgs...)

		_, stderr, errCapture := cmdRunner.Capture(cmd, 0)
		if errCapture != nil {
			return ErrGetPKGBUILDRepo{inner: errCapture, pkgName: pkgName, errOut: stderr}
		}
	} else if err != nil {
		return ErrGetPKGBUILDRepo{
			inner:   err,
			pkgName: pkgName,
			errOut:  gotext.Get("error reading %s", filepath.Join(dest, pkgName, ".git")),
		}
	} else {
		cmd := cmdBuilder.BuildGitCmd(filepath.Join(dest, pkgName), "pull", "--ff-only")

		_, stderr, errCmd := cmdRunner.Capture(cmd, 0)
		if errCmd != nil {
			return ErrGetPKGBUILDRepo{inner: errCmd, pkgName: pkgName, errOut: stderr}
		}
	}

	return nil
}

func getURLName(pkg db.IPackage) string {
	name := pkg.Base()
	if name == "" {
		name = pkg.Name()
	}

	return name
}

func GetPkgbuilds(dbExecutor DBSearcher, httpClient *http.Client, targets []string, mode settings.TargetMode) (map[string][]byte, error) {
	pkgbuilds := make(map[string][]byte, len(targets))

	var (
		mux  sync.Mutex
		errs multierror.MultiError
		wg   sync.WaitGroup
	)

	sem := make(chan uint8, MaxConcurrentFetch)

	for _, target := range targets {
		aur := true

		dbName, name := text.SplitDBFromName(target)
		if dbName != "aur" && (mode == settings.ModeAny || mode == settings.ModeRepo) {
			if pkg := dbExecutor.SyncPackage(name); pkg != nil {
				aur = false
				name = getURLName(pkg)
				dbName = pkg.DB().Name()
			}
		}

		if aur && mode == settings.ModeRepo {
			// Mode does not allow AUR packages
			continue
		}

		sem <- 1

		wg.Add(1)

		go func(target, dbName, pkgName string, aur bool) {
			var (
				err      error
				pkgbuild []byte
			)

			if aur {
				pkgbuild, err = AURPKGBUILD(httpClient, pkgName)
			} else {
				pkgbuild, err = ABSPKGBUILD(httpClient, dbName, pkgName)
			}

			if err == nil {
				mux.Lock()
				pkgbuilds[target] = pkgbuild
				mux.Unlock()
			} else {
				errs.Add(err)
			}

			<-sem
			wg.Done()
		}(target, dbName, name, aur)
	}

	wg.Wait()

	return pkgbuilds, errs.Return()
}

func PKGBUILDRepos(dbExecutor DBSearcher,
	cmdRunner exe.Runner,
	cmdBuilder exe.GitCmdBuilder,
	targets []string, mode settings.TargetMode, aurURL, dest string, force bool) (map[string]bool, error) {
	cloned := make(map[string]bool, len(targets))

	var (
		mux  sync.Mutex
		errs multierror.MultiError
		wg   sync.WaitGroup
	)

	sem := make(chan uint8, MaxConcurrentFetch)

	for _, target := range targets {
		aur := true

		dbName, name := text.SplitDBFromName(target)
		if dbName != "aur" && (mode == settings.ModeAny || mode == settings.ModeRepo) {
			var pkg alpm.IPackage
			if dbName != "" {
				pkg = dbExecutor.SatisfierFromDB(name, dbName)
				if pkg == nil {
					// if the user precised a db but the package is not in the db
					// then it is missing
					continue
				}
			} else {
				pkg = dbExecutor.SyncPackage(name)
			}

			if pkg != nil {
				aur = false
				name = getURLName(pkg)
				dbName = pkg.DB().Name()
			}
		}

		if aur && mode == settings.ModeRepo {
			// Mode does not allow AUR packages
			continue
		}

		sem <- 1

		wg.Add(1)

		go func(target, dbName, pkgName string, aur bool) {
			var (
				err error
			)

			if aur {
				err = AURPKGBUILDRepo(cmdRunner, cmdBuilder, aurURL, pkgName, dest, force)
			} else {
				err = ABSPKGBUILDRepo(cmdRunner, cmdBuilder, dbName, pkgName, dest, force)
			}

			success := err == nil
			if success {
				mux.Lock()
				cloned[target] = success
				mux.Unlock()
			} else {
				errs.Add(err)
			}

			if aur {
				text.OperationInfoln(
					gotext.Get("(%d/%d) Downloaded PKGBUILD: %s",
						len(cloned), len(targets), text.Cyan(pkgName)))
			} else {
				text.OperationInfoln(
					gotext.Get("(%d/%d) Downloaded PKGBUILD from ABS: %s",
						len(cloned), len(targets), text.Cyan(pkgName)))
			}

			<-sem

			wg.Done()
		}(target, dbName, name, aur)
	}

	wg.Wait()

	return cloned, errs.Return()
}
