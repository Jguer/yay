package mock

import (
	"time"

	"github.com/Jguer/yay/v11/pkg/db"

	"github.com/Jguer/go-alpm/v2"
)

type (
	IPackage = alpm.IPackage
	Depend   = alpm.Depend
	Upgrade  = db.Upgrade
)

type DBExecutor struct {
	db.Executor
	IsCorrectVersionInstalledFn func(string, string) bool
	SyncPackageFn               func(string) IPackage
	PackagesFromGroupFn         func(string) []IPackage
	LocalSatisfierExistsFn      func(string) bool
	SyncSatisfierFn             func(string) IPackage
}

func (t *DBExecutor) AlpmArchitectures() ([]string, error) {
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
	return nil
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

func (t DBExecutor) RefreshHandle() error {
	panic("implement me")
}

func (t DBExecutor) RepoUpgrades(b bool) ([]Upgrade, error) {
	panic("implement me")
}

func (t DBExecutor) Repos() []string {
	panic("implement me")
}

func (t DBExecutor) SatisfierFromDB(s, s2 string) IPackage {
	panic("implement me")
}

func (t DBExecutor) SyncPackage(s string) IPackage {
	if t.SyncPackageFn != nil {
		return t.SyncPackageFn(s)
	}
	panic("implement me")
}

func (t DBExecutor) SyncPackages(s ...string) []IPackage {
	panic("implement me")
}

func (t DBExecutor) SyncSatisfier(s string) IPackage {
	if t.SyncSatisfierFn != nil {
		return t.SyncSatisfierFn(s)
	}
	panic("implement me")
}

func (t DBExecutor) SyncSatisfierExists(s string) bool {
	panic("implement me")
}
