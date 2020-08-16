package query

import (
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/text"
)

// GetPackageNamesBySource returns package names with and without correspondence in SyncDBS respectively
func GetPackageNamesBySource(dbExecutor db.Executor) (local, remote []string, err error) {
outer:
	for _, localpkg := range dbExecutor.LocalPackages() {
		for _, syncpkg := range dbExecutor.SyncPackages() {
			if localpkg.Name() == syncpkg.Name() {
				local = append(local, localpkg.Name())
				continue outer
			}
		}
		remote = append(remote, localpkg.Name())
	}
	return local, remote, err
}

// GetRemotePackages returns packages with no correspondence in SyncDBS.
func GetRemotePackages(dbExecutor db.Executor) (
	remote []db.RepoPackage,
	remoteNames []string) {
outer:
	for _, localpkg := range dbExecutor.LocalPackages() {
		for _, syncpkg := range dbExecutor.SyncPackages() {
			if localpkg.Name() == syncpkg.Name() {
				continue outer
			}
		}
		remote = append(remote, localpkg)
		remoteNames = append(remoteNames, localpkg.Name())
	}
	return remote, remoteNames
}

func RemoveInvalidTargets(targets []string, mode settings.TargetMode) []string {
	filteredTargets := make([]string, 0)

	for _, target := range targets {
		dbName, _ := text.SplitDBFromName(target)

		if dbName == "aur" && mode == settings.ModeRepo {
			text.Warnln(gotext.Get("%s: can't use target with option --repo -- skipping", text.Cyan(target)))
			continue
		}

		if dbName != "aur" && dbName != "" && mode == settings.ModeAUR {
			text.Warnln(gotext.Get("%s: can't use target with option --aur -- skipping", text.Cyan(target)))
			continue
		}

		filteredTargets = append(filteredTargets, target)
	}

	return filteredTargets
}
