package query

import "github.com/Jguer/go-alpm"

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
