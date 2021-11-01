package mock

import (
	"time"

	alpm "github.com/Jguer/go-alpm/v2"
)

type Package struct {
	PBase         string
	PBuildDate    time.Time
	PDB           *DB
	PDescription  string
	PISize        int64
	PName         string
	PShouldIgnore bool
	PSize         int64
	PVersion      string
	PReason       alpm.PkgReason
}

func (p *Package) Base() string {
	return p.PBase
}

func (p *Package) BuildDate() time.Time {
	return p.PBuildDate
}

func (p *Package) DB() alpm.IDB {
	return p.PDB
}

func (p *Package) Description() string {
	return p.PDescription
}

func (p *Package) ISize() int64 {
	return p.PISize
}

func (p *Package) Name() string {
	return p.PName
}

func (p *Package) ShouldIgnore() bool {
	return p.PShouldIgnore
}

func (p *Package) Size() int64 {
	return p.PSize
}

func (p *Package) Version() string {
	return p.PVersion
}

func (p *Package) Reason() alpm.PkgReason {
	return p.PReason
}

func (p *Package) FileName() string {
	panic("not implemented") // TODO: Implement
}

func (p *Package) Base64Signature() string {
	panic("not implemented") // TODO: Implement
}

func (p *Package) Validation() alpm.Validation {
	panic("not implemented") // TODO: Implement
}

// Architecture returns the package target Architecture.
func (p *Package) Architecture() string {
	panic("not implemented") // TODO: Implement
}

// Backup returns a list of package backups.
func (p *Package) Backup() alpm.BackupList {
	panic("not implemented") // TODO: Implement
}

// Conflicts returns the conflicts of the package as a DependList.
func (p *Package) Conflicts() alpm.DependList {
	panic("not implemented") // TODO: Implement
}

// Depends returns the package's dependency list.
func (p *Package) Depends() alpm.DependList {
	panic("not implemented") // TODO: Implement
}

// Depends returns the package's optional dependency list.
func (p *Package) OptionalDepends() alpm.DependList {
	panic("not implemented") // TODO: Implement
}

// Depends returns the package's check dependency list.
func (p *Package) CheckDepends() alpm.DependList {
	panic("not implemented") // TODO: Implement
}

// Depends returns the package's make dependency list.
func (p *Package) MakeDepends() alpm.DependList {
	panic("not implemented") // TODO: Implement
}

// Files returns the file list of the package.
func (p *Package) Files() []alpm.File {
	panic("not implemented") // TODO: Implement
}

// ContainsFile checks if the path is in the package filelist.
func (p *Package) ContainsFile(path string) (alpm.File, error) {
	panic("not implemented") // TODO: Implement
}

// Groups returns the groups the package belongs to.
func (p *Package) Groups() alpm.StringList {
	panic("not implemented") // TODO: Implement
}

// InstallDate returns the package install date.
func (p *Package) InstallDate() time.Time {
	panic("not implemented") // TODO: Implement
}

// Licenses returns the package license list.
func (p *Package) Licenses() alpm.StringList {
	panic("not implemented") // TODO: Implement
}

// SHA256Sum returns package SHA256Sum.
func (p *Package) SHA256Sum() string {
	panic("not implemented") // TODO: Implement
}

// MD5Sum returns package MD5Sum.
func (p *Package) MD5Sum() string {
	panic("not implemented") // TODO: Implement
}

// Packager returns package packager name.
func (p *Package) Packager() string {
	panic("not implemented") // TODO: Implement
}

// Provides returns DependList of packages provides by package.
func (p *Package) Provides() alpm.DependList {
	panic("not implemented") // TODO: Implement
}

// Origin returns package origin.
func (p *Package) Origin() alpm.PkgFrom {
	panic("not implemented") // TODO: Implement
}

// Replaces returns a DependList with the packages this package replaces.
func (p *Package) Replaces() alpm.DependList {
	panic("not implemented") // TODO: Implement
}

// URL returns the upstream URL of the package.
func (p *Package) URL() string {
	panic("not implemented") // TODO: Implement
}

// ComputeRequiredBy returns the names of reverse dependencies of a package.
func (p *Package) ComputeRequiredBy() []string {
	panic("not implemented") // TODO: Implement
}

// ComputeOptionalFor returns the names of packages that optionally
// require the given package.
func (p *Package) ComputeOptionalFor() []string {
	panic("not implemented") // TODO: Implement
}

// SyncNewVersion checks if there is a new version of the
// package in a given DBlist.
func (p *Package) SyncNewVersion(l alpm.IDBList) alpm.IPackage {
	panic("not implemented") // TODO: Implement
}

func (p *Package) Type() string {
	panic("not implemented") // TODO: Implement
}

type DB struct {
	alpm.IDB
	name string
}

func NewDB(name string) *DB {
	return &DB{name: name}
}

func (d *DB) Name() string {
	return d.name
}
