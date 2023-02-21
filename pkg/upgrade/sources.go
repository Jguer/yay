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
	remote map[string]db.IPackage,
	aurdata map[string]*query.Pkg,
	localCache vcs.Store,
) UpSlice {
	toRemove := make([]string, 0)
	toUpgrade := UpSlice{Up: make([]Upgrade, 0), Repos: []string{"devel"}}

	for pkgName, pkg := range remote {
		if localCache.ToUpgrade(ctx, pkgName) {
			if _, ok := aurdata[pkgName]; !ok {
				text.Warnln(gotext.Get("ignoring package devel upgrade (no AUR info found):"), pkgName)
				continue
			}

			if pkg.ShouldIgnore() {
				printIgnoringPackage(pkg, "latest-commit")
				continue
			}

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

	localCache.RemovePackages(toRemove)

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
func UpAUR(remote map[string]db.IPackage, aurdata map[string]*query.Pkg, timeUpdate, enableDowngrade bool) UpSlice {
	toUpgrade := UpSlice{Up: make([]Upgrade, 0), Repos: []string{"aur"}}

	for name, pkg := range remote {
		aurPkg, ok := aurdata[name]
		if !ok {
			continue
		}

		if (timeUpdate && (int64(aurPkg.LastModified) > pkg.BuildDate().Unix())) ||
			(db.VerCmp(pkg.Version(), aurPkg.Version) < 0) || enableDowngrade {
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
