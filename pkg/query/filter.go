package query

import (
	alpm "github.com/Jguer/go-alpm"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/text"
)

// FilterPackages filters packages based on source and type from local repository.
func FilterPackages(alpmHandle *alpm.Handle) (
	local, remote []alpm.Package,
	localNames, remoteNames []string,
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
				local = append(local, k)
				localNames = append(localNames, k.Name())
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
	return local, remote, localNames, remoteNames, err
}

func RemoveInvalidTargets(targets []string, mode string) []string {
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
