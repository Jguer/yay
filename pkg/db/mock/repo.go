package mock

import (
	"io"
	"time"

	alpm "github.com/Jguer/dyalpm"
)

// DependList is a lightweight helper for test fixtures.
type DependList struct {
	Depends []alpm.Depend
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
	PDepends      DependList
	PProvides     DependList
	PArchitecture string
}

var _ alpm.Package = (*Package)(nil)

func (p *Package) Base() string {
	return p.PBase
}

func (p *Package) BuildDate() time.Time {
	return p.PBuildDate
}

func (p *Package) DB() alpm.Database {
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
	return p.PArchitecture
}

// Backup returns a list of package backups.
func (p *Package) Backup() []alpm.Backup {
	panic("not implemented")
}

// Conflicts returns the conflicts of the package as a DependList.
func (p *Package) Conflicts() []alpm.Depend {
	panic("not implemented")
}

// Depends returns the package's dependency list.
func (p *Package) Depends() []alpm.Depend {
	return p.PDepends.Depends
}

// Depends returns the package's optional dependency list.
func (p *Package) OptionalDepends() []alpm.Depend {
	panic("not implemented")
}

// Depends returns the package's check dependency list.
func (p *Package) CheckDepends() []alpm.Depend {
	panic("not implemented")
}

// Depends returns the package's make dependency list.
func (p *Package) MakeDepends() []alpm.Depend {
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
func (p *Package) Groups() []string {
	panic("not implemented")
}

// InstallDate returns the package install date.
func (p *Package) InstallDate() time.Time {
	panic("not implemented")
}

// Licenses returns the package license list.
func (p *Package) Licenses() []string {
	panic("not implemented")
}

// SHA256Sum returns package SHA256Sum.
func (p *Package) SHA256Sum() string {
	panic("not implemented")
}

// Packager returns package packager name.
func (p *Package) Packager() string {
	panic("not implemented")
}

// Provides returns DependList of packages provides by package.
func (p *Package) Provides() []alpm.Depend {
	return p.PProvides.Depends
}

// Origin returns package origin.
func (p *Package) Origin() alpm.PkgFrom {
	panic("not implemented")
}

// Replaces returns a DependList with the packages this package replaces.
func (p *Package) Replaces() []alpm.Depend {
	panic("not implemented")
}

// URL returns the upstream URL of the package.
func (p *Package) URL() string {
	panic("not implemented")
}

// ComputeRequiredBy returns the names of reverse dependencies of a package.
func (p *Package) ComputeRequiredBy() ([]string, error) {
	panic("not implemented")
}

// ComputeOptionalFor returns the names of packages that optionally
// require the given package.
func (p *Package) ComputeOptionalFor() ([]string, error) {
	panic("not implemented")
}

// SyncNewVersion checks if there is a new version of the
// package in a given DBlist.
func (p *Package) SyncNewVersion(dbs []alpm.Database) alpm.Package {
	panic("not implemented")
}

func (p *Package) Type() string {
	panic("not implemented")
}

func (p *Package) CheckMD5Sum() error {
	panic("not implemented")
}

func (p *Package) CheckPGPSignature() (alpm.SigList, error) {
	panic("not implemented")
}

func (p *Package) Contains(path string) bool {
	panic("not implemented")
}

func (p *Package) Free() error {
	return nil
}

// New methods required by dyalpm refactoring
func (p *Package) HasScriptlet() bool {
	return false
}

func (p *Package) DownloadSize() int64 {
	return 0
}

func (p *Package) NativeHandle() alpm.Handle {
	return nil
}

func (p *Package) Sig() string {
	return ""
}

func (p *Package) PkgValidation() alpm.PkgValidation {
	return alpm.PkgValidationUnknown
}

func (p *Package) XData() string {
	return ""
}

func (p *Package) Changelog() (io.ReadCloser, error) {
	return nil, nil
}

func (p *Package) SyncGetNewVersion(dbsSync []alpm.Database) alpm.Package {
	return nil
}

type DB struct {
	alpm.Database
	name string
}

func NewDB(name string) *DB {
	return &DB{name: name}
}

func (d *DB) Name() string {
	return d.name
}

func (d *DB) Pkg(name string) alpm.Package {
	return nil
}

func (d *DB) PkgCache() alpm.PackageIterator {
	return alpm.PackageIterator{}
}

func (d *DB) Search(needles []string) alpm.PackageIterator {
	return alpm.PackageIterator{}
}

func (d *DB) SetServers(servers []string) error {
	return nil
}

func (d *DB) SetUsage(usage int) error {
	return nil
}
