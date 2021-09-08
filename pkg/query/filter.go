package query

import (
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/text"
)

// GetPackageNamesBySource returns package names with and without correspondence in SyncDBS respectively.
func GetPackageNamesBySource(dbExecutor db.Executor) (local, remote []string, err error) {
	for _, localpkg := range dbExecutor.LocalPackages() {
		pkgName := localpkg.Name()
		if dbExecutor.SyncPackage(pkgName) != nil {
			local = append(local, pkgName)
		} else {
			remote = append(remote, pkgName)
		}
	}

	return local, remote, err
}

// GetRemotePackages returns packages with no correspondence in SyncDBS.
func GetRemotePackages(dbExecutor db.Executor) (
	remote []db.IPackage,
	remoteNames []string) {
	for _, localpkg := range dbExecutor.LocalPackages() {
		pkgName := localpkg.Name()
		if dbExecutor.SyncPackage(pkgName) == nil {
			remote = append(remote, localpkg)
			remoteNames = append(remoteNames, pkgName)
		}
	}

	return remote, remoteNames
}

func RemoveInvalidTargets(targets []string, mode parser.TargetMode) []string {
	filteredTargets := make([]string, 0)

	for _, target := range targets {
		dbName, _ := text.SplitDBFromName(target)

		if dbName == "aur" && !mode.AtLeastAUR() {
			text.Warnln(gotext.Get("%s: can't use target with option --repo -- skipping", text.Cyan(target)))
			continue
		}

		if dbName != "aur" && dbName != "" && !mode.AtLeastRepo() {
			text.Warnln(gotext.Get("%s: can't use target with option --aur -- skipping", text.Cyan(target)))
			continue
		}

		filteredTargets = append(filteredTargets, target)
	}

	return filteredTargets
}
