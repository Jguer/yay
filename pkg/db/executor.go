package db

import (
	"time"

	alpm "github.com/Jguer/go-alpm"

	"github.com/Jguer/yay/v10/pkg/upgrade"
)

type RepoPackage interface {
	Base() string
	BuildDate() time.Time
	DB() *alpm.DB
	Description() string
	ISize() int64
	Name() string
	ShouldIgnore() bool
	Size() int64
	Version() string
	Reason() alpm.PkgReason
}

type Executor interface {
	AlpmArch() (string, error)
	BiggestPackages() []RepoPackage
	Cleanup()
	IsCorrectVersionInstalled(string, string) bool
	LastBuildTime() time.Time
	LocalPackage(string) RepoPackage
	LocalPackages() []RepoPackage
	LocalSatisfierExists(string) bool
	PackageConflicts(RepoPackage) []alpm.Depend
	PackageDepends(RepoPackage) []alpm.Depend
	PackageFromDB(string, string) RepoPackage
	PackageGroups(RepoPackage) []string
	PackageOptionalDepends(RepoPackage) []alpm.Depend
	PackageProvides(RepoPackage) []alpm.Depend
	PackagesFromGroup(string) []RepoPackage
	RefreshHandle() error
	RepoUpgrades(bool) (upgrade.UpSlice, error)
	SyncPackages(...string) []RepoPackage
	SyncSatisfier(string) RepoPackage
	SyncSatisfierExists(string) bool
}
