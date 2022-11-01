package db

import (
	"time"

	alpm "github.com/Jguer/go-alpm/v2"
)

type (
	IPackage = alpm.IPackage
	Depend   = alpm.Depend
)

// VerCmp performs version comparison according to Pacman conventions. Return
// value is <0 if and only if v1 is older than v2.
func VerCmp(v1, v2 string) int {
	return alpm.VerCmp(v1, v2)
}

type Upgrade struct {
	Name          string
	Base          string
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
	PackageGroups(IPackage) []string
	PackageOptionalDepends(IPackage) []Depend
	PackageProvides(IPackage) []Depend
	PackagesFromGroup(string) []IPackage
	RefreshHandle() error
	RepoUpgrades(bool) ([]Upgrade, error)
	Repos() []string
	SatisfierFromDB(string, string) IPackage
	SyncPackage(string) IPackage
	SyncPackages(...string) []IPackage
	SyncSatisfier(string) IPackage
	SyncSatisfierExists(string) bool
}
