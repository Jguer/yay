package mock

import (
	"time"

	"github.com/Jguer/yay/v12/pkg/db"

	"github.com/Jguer/go-alpm/v2"
)

type (
	IPackage = alpm.IPackage
	Depend   = alpm.Depend
	Upgrade  = db.Upgrade
)

type DBExecutor struct {
	db.Executor
	LocalPackageFn                func(string) IPackage
	IsCorrectVersionInstalledFn   func(string, string) bool
	SyncPackageFn                 func(string) IPackage
	PackagesFromGroupFn           func(string) []IPackage
	LocalSatisfierExistsFn        func(string) bool
	SyncSatisfierFn               func(string) IPackage
	AlpmArchitecturesFn           func() ([]string, error)
	InstalledRemotePackageNamesFn func() []string
	InstalledRemotePackagesFn     func() map[string]IPackage
	SyncUpgradesFn                func(bool) (map[string]db.SyncUpgrade, error)
	RefreshHandleFn               func() error
	ReposFn                       func() []string
}

func (t *DBExecutor) InstalledRemotePackageNames() []string {
	if t.InstalledRemotePackageNamesFn != nil {
		return t.InstalledRemotePackageNamesFn()
	}
	panic("implement me")
}

func (t *DBExecutor) InstalledRemotePackages() map[string]IPackage {
	if t.InstalledRemotePackagesFn != nil {
		return t.InstalledRemotePackagesFn()
	}
	panic("implement me")
}

func (t *DBExecutor) AlpmArchitectures() ([]string, error) {
	if t.AlpmArchitecturesFn != nil {
		return t.AlpmArchitecturesFn()
	}
	panic("implement me")
}

func (t *DBExecutor) BiggestPackages() []IPackage {
	panic("implement me")
}

func (t *DBExecutor) Cleanup() {
	panic("implement me")
}

func (t *DBExecutor) IsCorrectVersionInstalled(s, s2 string) bool {
	if t.IsCorrectVersionInstalledFn != nil {
		return t.IsCorrectVersionInstalledFn(s, s2)
	}
	panic("implement me")
}

func (t *DBExecutor) LastBuildTime() time.Time {
	panic("implement me")
}

func (t *DBExecutor) LocalPackage(s string) IPackage {
	if t.LocalPackageFn != nil {
		return t.LocalPackageFn(s)
	}

	panic("implement me")
}

func (t *DBExecutor) LocalPackages() []IPackage {
	panic("implement me")
}

func (t *DBExecutor) LocalSatisfierExists(s string) bool {
	if t.LocalSatisfierExistsFn != nil {
		return t.LocalSatisfierExistsFn(s)
	}
	panic("implement me")
}

func (t *DBExecutor) PackageConflicts(iPackage IPackage) []Depend {
	panic("implement me")
}

func (t *DBExecutor) PackageDepends(iPackage IPackage) []Depend {
	panic("implement me")
}

func (t *DBExecutor) PackageGroups(iPackage IPackage) []string {
	return []string{}
}

func (t *DBExecutor) PackageOptionalDepends(iPackage IPackage) []Depend {
	panic("implement me")
}

func (t *DBExecutor) PackageProvides(iPackage IPackage) []Depend {
	panic("implement me")
}

func (t *DBExecutor) PackagesFromGroup(s string) []IPackage {
	if t.PackagesFromGroupFn != nil {
		return t.PackagesFromGroupFn(s)
	}

	panic("implement me")
}

func (t *DBExecutor) RefreshHandle() error {
	if t.RefreshHandleFn != nil {
		return t.RefreshHandleFn()
	}
	panic("implement me")
}

func (t *DBExecutor) SyncUpgrades(b bool) (map[string]db.SyncUpgrade, error) {
	if t.SyncUpgradesFn != nil {
		return t.SyncUpgradesFn(b)
	}
	panic("implement me")
}

func (t *DBExecutor) Repos() []string {
	if t.ReposFn != nil {
		return t.ReposFn()
	}
	panic("implement me")
}

func (t *DBExecutor) SatisfierFromDB(s, s2 string) IPackage {
	panic("implement me")
}

func (t *DBExecutor) SyncPackage(s string) IPackage {
	if t.SyncPackageFn != nil {
		return t.SyncPackageFn(s)
	}
	panic("implement me")
}

func (t *DBExecutor) SyncPackages(s ...string) []IPackage {
	panic("implement me")
}

func (t *DBExecutor) SyncSatisfier(s string) IPackage {
	if t.SyncSatisfierFn != nil {
		return t.SyncSatisfierFn(s)
	}
	panic("implement me")
}

func (t *DBExecutor) SyncSatisfierExists(s string) bool {
	panic("implement me")
}
