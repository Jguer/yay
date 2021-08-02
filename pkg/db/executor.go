package db

import (
	"time"

	alpm "github.com/Jguer/go-alpm/v2"
)

type (
	IPackage = alpm.IPackage
	Depend   = alpm.Depend
)

func VerCmp(a, b string) int {
	return alpm.VerCmp(a, b)
}

type Upgrade struct {
	Name          string
	Repository    string
	LocalVersion  string
	RemoteVersion string
	Reason        alpm.PkgReason
}

type Executor interface {
	AlpmArchitectures() ([]string, error)
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
	RepoUpgrades(bool) ([]Upgrade, error)
	SyncPackage(string) IPackage
	SyncPackages(...string) []IPackage
	SyncSatisfier(string) IPackage
	SyncSatisfierExists(string) bool
	Repos() []string
}
