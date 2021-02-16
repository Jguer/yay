package db

import (
	"time"

	alpm "github.com/Jguer/go-alpm/v2"

	"github.com/Jguer/yay/v10/pkg/upgrade"
)

type IPackage = alpm.IPackage
type Depend = alpm.Depend

type Executor interface {
	AlpmArch() (string, error)
	BiggestPackages() []IPackage
	Cleanup()
	IsCorrectVersionInstalled(string, string) bool
	LastBuildTime() time.Time
	LocalPackage(string) IPackage
	LocalPackages() []IPackage
	LocalSatisfierExists(string) bool
	PackageConflicts(IPackage) []Depend
	PackageDepends(IPackage) []Depend
	SatisfierFromDB(string, string) IPackage
	PackageGroups(IPackage) []string
	PackageOptionalDepends(IPackage) []Depend
	PackageProvides(IPackage) []Depend
	PackagesFromGroup(string) []IPackage
	RefreshHandle() error
	RepoUpgrades(bool) (upgrade.UpSlice, error)
	SyncPackage(string) IPackage
	SyncPackages(...string) []IPackage
	SyncSatisfier(string) IPackage
	SyncSatisfierExists(string) bool
}
