package query

import (
	alpm "github.com/Jguer/go-alpm"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/text"
)

// GetPackageNamesBySource returns package names with and without correspondence in SyncDBS respectively
func GetPackageNamesBySource(alpmHandle *alpm.Handle) (local, remote []string, err error) {
	localDB, err := alpmHandle.LocalDB()
	if err != nil {
		return nil, nil, err
	}
	dbList, err := alpmHandle.SyncDBs()
	if err != nil {
		return nil, nil, err
	}

	err = localDB.PkgCache().ForEach(func(k alpm.Package) error {
		found := false
		// For each DB search for our secret package.
		_ = dbList.ForEach(func(d alpm.DB) error {
			if found {
				return nil
			}

			if d.Pkg(k.Name()) != nil {
				found = true
				local = append(local, k.Name())
			}
			return nil
		})
		if !found {
			remote = append(remote, k.Name())
		}
		return nil
	})
	return local, remote, err
}

// GetRemotePackages returns packages with no correspondence in SyncDBS.
func GetRemotePackages(alpmHandle *alpm.Handle) (
	remote []alpm.Package,
	remoteNames []string,
	err error) {
	localDB, err := alpmHandle.LocalDB()
	if err != nil {
		return
	}
	dbList, err := alpmHandle.SyncDBs()
	if err != nil {
		return
	}

	f := func(k alpm.Package) error {
		found := false
		// For each DB search for our secret package.
		_ = dbList.ForEach(func(d alpm.DB) error {
			if found {
				return nil
			}

			if d.Pkg(k.Name()) != nil {
				found = true
			}
			return nil
		})

		if !found {
			remote = append(remote, k)
			remoteNames = append(remoteNames, k.Name())
		}
		return nil
	}

	err = localDB.PkgCache().ForEach(f)
	return remote, remoteNames, err
}

func RemoveInvalidTargets(targets []string, mode settings.TargetMode) []string {
	filteredTargets := make([]string, 0)

	for _, target := range targets {
		db, _ := text.SplitDBFromName(target)

		if db == "aur" && mode == settings.ModeRepo {
			text.Warnln(gotext.Get("%s: can't use target with option --repo -- skipping", text.Cyan(target)))
			continue
		}

		if db != "aur" && db != "" && mode == settings.ModeAUR {
			text.Warnln(gotext.Get("%s: can't use target with option --aur -- skipping", text.Cyan(target)))
			continue
		}

		filteredTargets = append(filteredTargets, target)
	}

	return filteredTargets
}
