package db

import (
	"time"

	alpm "github.com/Jguer/go-alpm/v2"

	"github.com/Jguer/yay/v10/pkg/upgrade"
)

type Executor interface {
	AlpmArch() (string, error)
	BiggestPackages() []alpm.IPackage
	Cleanup()
	IsCorrectVersionInstalled(string, string) bool
	LastBuildTime() time.Time
	LocalPackage(string) alpm.IPackage
	LocalPackages() []alpm.IPackage
	LocalSatisfierExists(string) bool
	PackageConflicts(alpm.IPackage) []alpm.Depend
	PackageDepends(alpm.IPackage) []alpm.Depend
	SatisfierFromDB(string, string) alpm.IPackage
	PackageGroups(alpm.IPackage) []string
	PackageOptionalDepends(alpm.IPackage) []alpm.Depend
	PackageProvides(alpm.IPackage) []alpm.Depend
	PackagesFromGroup(string) []alpm.IPackage
	RefreshHandle() error
	RepoUpgrades(bool) (upgrade.UpSlice, error)
	SyncPackage(string) alpm.IPackage
	SyncPackages(...string) []alpm.IPackage
	SyncSatisfier(string) alpm.IPackage
	SyncSatisfierExists(string) bool
}
