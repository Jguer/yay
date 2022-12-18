package upgrade

import (
	"context"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/query"
	"github.com/Jguer/yay/v11/pkg/text"
	"github.com/Jguer/yay/v11/pkg/vcs"
)

func UpDevel(
	ctx context.Context,
	remote []db.IPackage, // should be a map
	aurdata map[string]*query.Pkg,
	localCache *vcs.InfoStore,
) UpSlice {
	toUpdate := make([]db.IPackage, 0, len(aurdata))
	toRemove := make([]string, 0)

	for _, pkgName := range localCache.ToUpgrade(ctx) {
		if _, ok := aurdata[pkgName]; ok {
			for _, pkg := range remote {
				if pkg.Name() == pkgName {
					toUpdate = append(toUpdate, pkg)
				}
			}
		} else {
			toRemove = append(toRemove, pkgName)
		}
	}

	toUpgrade := UpSlice{Up: make([]Upgrade, 0, len(toUpdate)), Repos: []string{"devel"}}

	for _, pkg := range toUpdate {
		if pkg.ShouldIgnore() {
			printIgnoringPackage(pkg, "latest-commit")
		} else {
			toUpgrade.Up = append(toUpgrade.Up,
				Upgrade{
					Name:          pkg.Name(),
					Base:          pkg.Base(),
					Repository:    "devel",
					LocalVersion:  pkg.Version(),
					RemoteVersion: "latest-commit",
					Reason:        pkg.Reason(),
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
						Base:          aurPkg.PackageBase,
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
