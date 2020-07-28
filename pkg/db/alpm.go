package db

import (
	alpm "github.com/Jguer/go-alpm"
)

type AlpmExecutor struct {
	Handle  *alpm.Handle
	LocalDB *alpm.DB
	SyncDB  alpm.DBList
}

func NewExecutor(handle *alpm.Handle) (*AlpmExecutor, error) {
	localDB, err := handle.LocalDB()
	if err != nil {
		return nil, err
	}
	syncDB, err := handle.SyncDBs()
	if err != nil {
		return nil, err
	}

	return &AlpmExecutor{Handle: handle, LocalDB: localDB, SyncDB: syncDB}, nil
}

func (ae *AlpmExecutor) LocalSatisfierExists(pkgName string) bool {
	if _, err := ae.LocalDB.PkgCache().FindSatisfier(pkgName); err != nil {
		return false
	}
	return true
}

func (ae *AlpmExecutor) IsCorrectVersionInstalled(pkgName, versionRequired string) bool {
	alpmPackage := ae.LocalDB.Pkg(pkgName)
	if alpmPackage == nil {
		return false
	}

	return alpmPackage.Version() == versionRequired
}

func (ae *AlpmExecutor) SyncSatisfier(pkgName string) RepoPackage {
	foundPkg, err := ae.SyncDB.FindSatisfier(pkgName)
	if err != nil {
		return nil
	}
	return foundPkg
}

func (ae *AlpmExecutor) PackagesFromGroup(groupName string) []RepoPackage {
	groupPackages := []RepoPackage{}
	_ = ae.SyncDB.FindGroupPkgs(groupName).ForEach(func(pkg alpm.Package) error {
		groupPackages = append(groupPackages, &pkg)
		return nil
	})
	return groupPackages
}

func (ae *AlpmExecutor) LocalPackages() []RepoPackage {
	localPackages := []RepoPackage{}
	_ = ae.LocalDB.PkgCache().ForEach(func(pkg alpm.Package) error {
		localPackages = append(localPackages, RepoPackage(&pkg))
		return nil
	})
	return localPackages
}

func (ae *AlpmExecutor) PackageFromDB(pkgName, dbName string) RepoPackage {
	singleDB, err := ae.Handle.SyncDBByName(dbName)
	if err != nil {
		return nil
	}
	foundPkg, err := singleDB.PkgCache().FindSatisfier(pkgName)
	if err != nil {
		return nil
	}
	return foundPkg
}

func (ae *AlpmExecutor) PackageDepends(pkg RepoPackage) []alpm.Depend {
	alpmPackage := pkg.(*alpm.Package)
	return alpmPackage.Depends().Slice()
}

func (ae *AlpmExecutor) PackageProvides(pkg RepoPackage) []alpm.Depend {
	alpmPackage := pkg.(*alpm.Package)
	return alpmPackage.Provides().Slice()
}

func (ae *AlpmExecutor) PackageConflicts(pkg RepoPackage) []alpm.Depend {
	alpmPackage := pkg.(*alpm.Package)
	return alpmPackage.Conflicts().Slice()
}
