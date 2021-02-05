package upgrade

import (
	"sync"

	alpm "github.com/Jguer/go-alpm/v2"
	"github.com/leonelquinteros/gotext"
	rpc "github.com/mikkeloscar/aur"

	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/vcs"
)

func UpDevel(
	remote []alpm.IPackage,
	aurdata map[string]*rpc.Pkg,
	localCache *vcs.InfoStore) UpSlice {
	toUpdate := make([]alpm.IPackage, 0, len(aurdata))
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

	toUpgrade := make(UpSlice, 0, len(toUpdate))
	for _, pkg := range toUpdate {
		if pkg.ShouldIgnore() {
			printIgnoringPackage(pkg, "latest-commit")
		} else {
			toUpgrade = append(toUpgrade,
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

func printIgnoringPackage(pkg alpm.IPackage, newPkgVersion string) {
	left, right := GetVersionDiff(pkg.Version(), newPkgVersion)

	text.Warnln(gotext.Get("%s: ignoring package upgrade (%s => %s)",
		text.Cyan(pkg.Name()),
		left, right,
	))
}

// UpAUR gathers foreign packages and checks if they have new versions.
// Output: Upgrade type package list.
func UpAUR(remote []alpm.IPackage, aurdata map[string]*rpc.Pkg, timeUpdate bool) UpSlice {
	toUpgrade := make(UpSlice, 0)

	for _, pkg := range remote {
		aurPkg, ok := aurdata[pkg.Name()]
		if !ok {
			continue
		}

		if (timeUpdate && (int64(aurPkg.LastModified) > pkg.BuildDate().Unix())) ||
			(alpm.VerCmp(pkg.Version(), aurPkg.Version) < 0) {
			if pkg.ShouldIgnore() {
				printIgnoringPackage(pkg, aurPkg.Version)
			} else {
				toUpgrade = append(toUpgrade,
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
