package download

import (
	"sync"

	"github.com/Jguer/go-alpm/v2"

	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/multierror"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/text"
)

func getURLName(pkg alpm.IPackage) string {
	name := pkg.Base()
	if name == "" {
		name = pkg.Name()
	}
	return name
}

func GetPkgbuilds(dbExecutor db.Executor, targets []string, mode settings.TargetMode) (map[string][]byte, error) {
	pkgbuilds := make(map[string][]byte, len(targets))
	var mux sync.Mutex
	var errs multierror.MultiError
	var wg sync.WaitGroup
	sem := make(chan uint8, MaxConcurrentFetch)

	for _, target := range targets {
		aur := true
		dbName, name := text.SplitDBFromName(target)
		if dbName != "aur" && (mode == settings.ModeAny || mode == settings.ModeRepo) {
			pkg := dbExecutor.SyncPackage(name)
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
			var err error
			var pkgbuild []byte

			if aur {
				pkgbuild, err = GetAURPkgbuild(pkgName)
			} else {
				pkgbuild, err = GetABSPkgbuild(dbName, pkgName)
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
