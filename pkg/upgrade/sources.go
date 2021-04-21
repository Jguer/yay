package upgrade

import (
	"sync"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/vcs"
)

func UpDevel(
	remote []db.IPackage,
	aurdata map[string]*query.Pkg,
	localCache *vcs.InfoStore) UpSlice {
	toUpdate := make([]db.IPackage, 0, len(aurdata))
	toRemove := make([]string, 0)

	var mux1, mux2 sync.Mutex
	var wg sync.WaitGroup

	checkUpdate := func(pkgName string, e vcs.OriginInfoByURL) {
		defer wg.Done()

		if localCache.NeedsUpdate(e) {
			if _, ok := aurdata[pkgName]; ok {
				for _, pkg := range remote {
					if pkg.Name() == pkgName {
						mux1.Lock()
						toUpdate = append(toUpdate, pkg)
						mux1.Unlock()
						return
					}
				}
			}

			mux2.Lock()
			toRemove = append(toRemove, pkgName)
			mux2.Unlock()
		}
	}

	for pkgName, e := range localCache.OriginsByPackage {
		wg.Add(1)
		go checkUpdate(pkgName, e)
	}

	wg.Wait()

	toUpgrade := UpSlice{Up: make([]Upgrade, 0), Repos: []string{"devel"}}
	for _, pkg := range toUpdate {
		if pkg.ShouldIgnore() {
			printIgnoringPackage(pkg, "latest-commit")
		} else {
			toUpgrade.Up = append(toUpgrade.Up,
				Upgrade{
					Name:          pkg.Name(),
					Repository:    "devel",
					LocalVersion:  pkg.Version(),
					RemoteVersion: "latest-commit",
				})
		}
	}

	localCache.RemovePackage(toRemove)
	return toUpgrade
}

func printIgnoringPackage(pkg db.IPackage, newPkgVersion string) {
	left, right := GetVersionDiff(pkg.Version(), newPkgVersion)

	text.Warnln(gotext.Get("%s: ignoring package upgrade (%s => %s)",
		text.Cyan(pkg.Name()),
		left, right,
	))
}

// UpAUR gathers foreign packages and checks if they have new versions.
// Output: Upgrade type package list.
func UpAUR(remote []db.IPackage, aurdata map[string]*query.Pkg, timeUpdate bool) UpSlice {
	toUpgrade := UpSlice{Up: make([]Upgrade, 0), Repos: []string{"aur"}}

	for _, pkg := range remote {
		aurPkg, ok := aurdata[pkg.Name()]
		if !ok {
			continue
		}

		if (timeUpdate && (int64(aurPkg.LastModified) > pkg.BuildDate().Unix())) ||
			(db.VerCmp(pkg.Version(), aurPkg.Version) < 0) {
			if pkg.ShouldIgnore() {
				printIgnoringPackage(pkg, aurPkg.Version)
			} else {
				toUpgrade.Up = append(toUpgrade.Up,
					Upgrade{
						Name:          aurPkg.Name,
						Repository:    "aur",
						LocalVersion:  pkg.Version(),
						RemoteVersion: aurPkg.Version,
						Reason:        pkg.Reason(),
					})
			}
		}
	}

	return toUpgrade
}
