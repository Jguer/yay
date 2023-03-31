package mock

import (
	"time"

	alpm "github.com/Jguer/go-alpm/v2"
)

type DependList struct {
	Depends []Depend
}

func (d DependList) Slice() []alpm.Depend {
	return d.Depends
}

func (d DependList) ForEach(f func(*alpm.Depend) error) error {
	for i := range d.Depends {
		dep := &d.Depends[i]
		err := f(dep)
		if err != nil {
			return err
		}
	}

	return nil
}

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
	PDepends      alpm.IDependList
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
	panic("not implemented")
}

func (p *Package) Base64Signature() string {
	panic("not implemented")
}

func (p *Package) Validation() alpm.Validation {
	panic("not implemented")
}

// Architecture returns the package target Architecture.
func (p *Package) Architecture() string {
	panic("not implemented")
}

// Backup returns a list of package backups.
func (p *Package) Backup() alpm.BackupList {
	panic("not implemented")
}

// Conflicts returns the conflicts of the package as a DependList.
func (p *Package) Conflicts() alpm.IDependList {
	panic("not implemented")
}

// Depends returns the package's dependency list.
func (p *Package) Depends() alpm.IDependList {
	if p.PDepends != nil {
		return p.PDepends
	}
	return alpm.DependList{}
}

// Depends returns the package's optional dependency list.
func (p *Package) OptionalDepends() alpm.IDependList {
	panic("not implemented")
}

// Depends returns the package's check dependency list.
func (p *Package) CheckDepends() alpm.IDependList {
	panic("not implemented")
}

// Depends returns the package's make dependency list.
func (p *Package) MakeDepends() alpm.IDependList {
	panic("not implemented")
}

// Files returns the file list of the package.
func (p *Package) Files() []alpm.File {
	panic("not implemented")
}

// ContainsFile checks if the path is in the package filelist.
func (p *Package) ContainsFile(path string) (alpm.File, error) {
	panic("not implemented")
}

// Groups returns the groups the package belongs to.
func (p *Package) Groups() alpm.StringList {
	panic("not implemented")
}

// InstallDate returns the package install date.
func (p *Package) InstallDate() time.Time {
	panic("not implemented")
}

// Licenses returns the package license list.
func (p *Package) Licenses() alpm.StringList {
	panic("not implemented")
}

// SHA256Sum returns package SHA256Sum.
func (p *Package) SHA256Sum() string {
	panic("not implemented")
}

// MD5Sum returns package MD5Sum.
func (p *Package) MD5Sum() string {
	panic("not implemented")
}

// Packager returns package packager name.
func (p *Package) Packager() string {
	panic("not implemented")
}

// Provides returns DependList of packages provides by package.
func (p *Package) Provides() alpm.IDependList {
	return alpm.DependList{}
}

// Origin returns package origin.
func (p *Package) Origin() alpm.PkgFrom {
	panic("not implemented")
}

// Replaces returns a DependList with the packages this package replaces.
func (p *Package) Replaces() alpm.IDependList {
	panic("not implemented")
}

// URL returns the upstream URL of the package.
func (p *Package) URL() string {
	panic("not implemented")
}

// ComputeRequiredBy returns the names of reverse dependencies of a package.
func (p *Package) ComputeRequiredBy() []string {
	panic("not implemented")
}

// ComputeOptionalFor returns the names of packages that optionally
// require the given package.
func (p *Package) ComputeOptionalFor() []string {
	panic("not implemented")
}

// SyncNewVersion checks if there is a new version of the
// package in a given DBlist.
func (p *Package) SyncNewVersion(l alpm.IDBList) alpm.IPackage {
	panic("not implemented")
}

func (p *Package) Type() string {
	panic("not implemented")
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
