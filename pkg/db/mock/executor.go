package mock

import (
	"time"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/text"

	"github.com/Jguer/go-alpm/v2"
)

type (
	IPackage = alpm.IPackage
	Depend   = alpm.Depend
	Upgrade  = db.Upgrade
)

type DBExecutor struct {
	db.Executor
	AlpmArchitecturesFn           func() ([]string, error)
	InstalledRemotePackageNamesFn func() []string
	InstalledRemotePackagesFn     func() map[string]IPackage
	IsCorrectVersionInstalledFn   func(string, string) bool
	LocalPackageFn                func(string) IPackage
	LocalPackagesFn               func() []IPackage
	LocalSatisfierExistsFn        func(string) bool
	PackageDependsFn              func(IPackage) []Depend
	PackageOptionalDependsFn      func(alpm.IPackage) []alpm.Depend
	PackageProvidesFn             func(IPackage) []Depend
	PackagesFromGroupFn           func(string) []IPackage
	PackagesFromGroupAndDBFn      func(string, string) ([]IPackage, error)
	RefreshHandleFn               func() error
	ReposFn                       func() []string
	SyncPackageFn                 func(string) IPackage
	SyncPackagesFn                func(...string) []IPackage
	SyncSatisfierFn               func(string) IPackage
	SatisfierFromDBFn             func(string, string) (IPackage, error)
	SyncUpgradesFn                func(bool) (map[string]db.SyncUpgrade, error)
	SetLoggerFn                   func(*text.Logger)
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
	if t.LocalPackagesFn != nil {
		return t.LocalPackagesFn()
	}

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
	if t.PackageDependsFn != nil {
		return t.PackageDependsFn(iPackage)
	}

	panic("implement me")
}

func (t *DBExecutor) PackageGroups(iPackage IPackage) []string {
	return []string{}
}

func (t *DBExecutor) PackageOptionalDepends(iPackage IPackage) []Depend {
	if t.PackageOptionalDependsFn != nil {
		return t.PackageOptionalDependsFn(iPackage)
	}

	panic("implement me")
}

func (t *DBExecutor) PackageProvides(iPackage IPackage) []Depend {
	if t.PackageProvidesFn != nil {
		return t.PackageProvidesFn(iPackage)
	}

	panic("implement me")
}

func (t *DBExecutor) PackagesFromGroup(s string) []IPackage {
	if t.PackagesFromGroupFn != nil {
		return t.PackagesFromGroupFn(s)
	}

	panic("implement me")
}

func (t *DBExecutor) PackagesFromGroupAndDB(s, s2 string) ([]IPackage, error) {
	if t.PackagesFromGroupAndDBFn != nil {
		return t.PackagesFromGroupAndDBFn(s, s2)
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

func (t *DBExecutor) SatisfierFromDB(s, s2 string) (IPackage, error) {
	if t.SatisfierFromDBFn != nil {
		return t.SatisfierFromDBFn(s, s2)
	}
	panic("implement me")
}

func (t *DBExecutor) SyncPackage(s string) IPackage {
	if t.SyncPackageFn != nil {
		return t.SyncPackageFn(s)
	}
	panic("implement me")
}

func (t *DBExecutor) SyncPackages(s ...string) []IPackage {
	if t.SyncPackagesFn != nil {
		return t.SyncPackagesFn(s...)
	}
	panic("implement me")
}

func (t *DBExecutor) SyncSatisfier(s string) IPackage {
	if t.SyncSatisfierFn != nil {
		return t.SyncSatisfierFn(s)
	}
	panic("implement me")
}

func (t *DBExecutor) SyncSatisfierExists(s string) bool {
	if t.SyncSatisfierFn != nil {
		return t.SyncSatisfierFn(s) != nil
	}
	panic("implement me")
}

func (t *DBExecutor) SetLogger(logger *text.Logger) {
	if t.SetLoggerFn != nil {
		t.SetLoggerFn(logger)
		return
	}
	panic("implement me")
}
