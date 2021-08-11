package download

import (
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/multierror"
	"github.com/Jguer/yay/v10/pkg/settings/exe"
	"github.com/Jguer/yay/v10/pkg/settings/parser"
	"github.com/Jguer/yay/v10/pkg/text"
)

type DBSearcher interface {
	SyncPackage(string) db.IPackage
	SatisfierFromDB(string, string) db.IPackage
}

func downloadGitRepo(cmdBuilder exe.GitCmdBuilder,
	pkgURL, pkgName, dest string, force bool, gitArgs ...string) (bool, error) {
	finalDir := filepath.Join(dest, pkgName)
	newClone := true

	switch _, err := os.Stat(filepath.Join(finalDir, ".git")); {
	case os.IsNotExist(err) || (err == nil && force):
		if _, errD := os.Stat(finalDir); force && errD == nil {
			if errR := os.RemoveAll(finalDir); errR != nil {
				return false, ErrGetPKGBUILDRepo{inner: errR, pkgName: pkgName, errOut: ""}
			}
		}

		gitArgs = append(gitArgs, pkgURL, pkgName)

		cloneArgs := make([]string, 0, len(gitArgs)+4)
		cloneArgs = append(cloneArgs, "clone", "--no-progress")
		cloneArgs = append(cloneArgs, gitArgs...)
		cmd := cmdBuilder.BuildGitCmd(dest, cloneArgs...)

		_, stderr, errCapture := cmdBuilder.Capture(cmd, 0)
		if errCapture != nil {
			return false, ErrGetPKGBUILDRepo{inner: errCapture, pkgName: pkgName, errOut: stderr}
		}
	case err != nil:
		return false, ErrGetPKGBUILDRepo{
			inner:   err,
			pkgName: pkgName,
			errOut:  gotext.Get("error reading %s", filepath.Join(dest, pkgName, ".git")),
		}
	default:
		cmd := cmdBuilder.BuildGitCmd(filepath.Join(dest, pkgName), "pull", "--ff-only")

		_, stderr, errCmd := cmdBuilder.Capture(cmd, 0)
		if errCmd != nil {
			return false, ErrGetPKGBUILDRepo{inner: errCmd, pkgName: pkgName, errOut: stderr}
		}

		newClone = false
	}

	return newClone, nil
}

func getURLName(pkg db.IPackage) string {
	name := pkg.Base()
	if name == "" {
		name = pkg.Name()
	}

	return name
}

func PKGBUILDs(dbExecutor DBSearcher, httpClient *http.Client, targets []string,
	aurURL string, mode parser.TargetMode) (map[string][]byte, error) {
	pkgbuilds := make(map[string][]byte, len(targets))

	var (
		mux  sync.Mutex
		errs multierror.MultiError
		wg   sync.WaitGroup
	)

	sem := make(chan uint8, MaxConcurrentFetch)

	for _, target := range targets {
		// Probably replaceable by something in query.
		dbName, name, aur, toSkip := getPackageUsableName(dbExecutor, target, mode)
		if toSkip {
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
				pkgbuild, err = AURPKGBUILD(httpClient, pkgName, aurURL)
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
	cmdBuilder exe.GitCmdBuilder,
	targets []string, mode parser.TargetMode, aurURL, dest string, force bool) (map[string]bool, error) {
	cloned := make(map[string]bool, len(targets))

	var (
		mux  sync.Mutex
		errs multierror.MultiError
		wg   sync.WaitGroup
	)

	sem := make(chan uint8, MaxConcurrentFetch)

	for _, target := range targets {
		// Probably replaceable by something in query.
		dbName, name, aur, toSkip := getPackageUsableName(dbExecutor, target, mode)
		if toSkip {
			continue
		}

		sem <- 1

		wg.Add(1)

		go func(target, dbName, pkgName string, aur bool) {
			var (
				err      error
				newClone bool
			)

			if aur {
				newClone, err = AURPKGBUILDRepo(cmdBuilder, aurURL, pkgName, dest, force)
			} else {
				newClone, err = ABSPKGBUILDRepo(cmdBuilder, dbName, pkgName, dest, force)
			}

			progress := 0

			if err != nil {
				errs.Add(err)
			} else {
				mux.Lock()
				cloned[target] = newClone
				progress = len(cloned)
				mux.Unlock()
			}

			if aur {
				text.OperationInfoln(
					gotext.Get("(%d/%d) Downloaded PKGBUILD: %s",
						progress, len(targets), text.Cyan(pkgName)))
			} else {
				text.OperationInfoln(
					gotext.Get("(%d/%d) Downloaded PKGBUILD from ABS: %s",
						progress, len(targets), text.Cyan(pkgName)))
			}

			<-sem

			wg.Done()
		}(target, dbName, name, aur)
	}

	wg.Wait()

	return cloned, errs.Return()
}

// TODO: replace with dep.ResolveTargets.
func getPackageUsableName(dbExecutor DBSearcher, target string, mode parser.TargetMode) (dbname, pkgname string, aur, toSkip bool) {
	aur = true

	dbName, name := text.SplitDBFromName(target)
	if dbName != "aur" && mode.AtLeastRepo() {
		var pkg db.IPackage
		if dbName != "" {
			pkg = dbExecutor.SatisfierFromDB(name, dbName)
			if pkg == nil {
				// if the user precised a db but the package is not in the db
				// then it is missing
				// Mode does not allow AUR packages
				return dbName, name, aur, true
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

	if aur && mode == parser.ModeRepo {
		return dbName, name, aur, true
	}

	return dbName, name, aur, false
}
