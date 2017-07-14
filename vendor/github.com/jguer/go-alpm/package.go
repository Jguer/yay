// package.go - libalpm package type and methods.
//
// Copyright (c) 2013 The go-alpm Authors
//
// MIT Licensed. See LICENSE for details.

package alpm

/*
#include <alpm.h>

int pkg_cmp(const void *v1, const void *v2)
{
    alpm_pkg_t *p1 = (alpm_pkg_t *)v1;
    alpm_pkg_t *p2 = (alpm_pkg_t *)v2;
    unsigned long int s1 = alpm_pkg_get_isize(p1);
    unsigned long int s2 = alpm_pkg_get_isize(p2);
    return(s2 - s1);
}
*/
import "C"

import (
	"time"
	"unsafe"
)

// Package describes a single package and associated handle.
type Package struct {
	pmpkg  *C.alpm_pkg_t
	handle Handle
}

// PackageList describes a linked list of packages and associated handle.
type PackageList struct {
	*list
	handle Handle
}

// ForEach executes an action on each package of the PackageList.
func (l PackageList) ForEach(f func(Package) error) error {
	return l.forEach(func(p unsafe.Pointer) error {
		return f(Package{(*C.alpm_pkg_t)(p), l.handle})
	})
}

// Slice converts the PackageList to a Package Slice.
func (l PackageList) Slice() []Package {
	slice := []Package{}
	l.ForEach(func(p Package) error {
		slice = append(slice, p)
		return nil
	})
	return slice
}

// SortBySize returns a PackageList sorted by size.
func (l PackageList) SortBySize() PackageList {
	pkgList := (*C.struct___alpm_list_t)(unsafe.Pointer(l.list))

	pkgCache := (*list)(unsafe.Pointer(
		C.alpm_list_msort(pkgList,
			C.alpm_list_count(pkgList),
			C.alpm_list_fn_cmp(C.pkg_cmp))))

	return PackageList{pkgCache, l.handle}
}

// DependList describes a linkedlist of dependency type packages.
type DependList struct{ *list }

// ForEach executes an action on each package of the DependList.
func (l DependList) ForEach(f func(Depend) error) error {
	return l.forEach(func(p unsafe.Pointer) error {
		dep := convertDepend((*C.alpm_depend_t)(p))
		return f(dep)
	})
}

// Slice converts the DependList to a Depend Slice.
func (l DependList) Slice() []Depend {
	slice := []Depend{}
	l.ForEach(func(dep Depend) error {
		slice = append(slice, dep)
		return nil
	})
	return slice
}

// Architecture returns the package target Architecture.
func (pkg Package) Architecture() string {
	return C.GoString(C.alpm_pkg_get_arch(pkg.pmpkg))
}

// Backup returns a list of package backups.
func (pkg Package) Backup() BackupList {
	ptr := unsafe.Pointer(C.alpm_pkg_get_backup(pkg.pmpkg))
	return BackupList{(*list)(ptr)}
}

// BuildDate returns the BuildDate of the package.
func (pkg Package) BuildDate() time.Time {
	t := C.alpm_pkg_get_builddate(pkg.pmpkg)
	return time.Unix(int64(t), 0)
}

// Conflicts returns the conflicts of the package as a DependList.
func (pkg Package) Conflicts() DependList {
	ptr := unsafe.Pointer(C.alpm_pkg_get_conflicts(pkg.pmpkg))
	return DependList{(*list)(ptr)}
}

// DB returns the package's origin database.
func (pkg Package) DB() *Db {
	ptr := C.alpm_pkg_get_db(pkg.pmpkg)
	if ptr == nil {
		return nil
	}
	return &Db{ptr, pkg.handle}
}

// Depends returns the package's dependency list.
func (pkg Package) Depends() DependList {
	ptr := unsafe.Pointer(C.alpm_pkg_get_depends(pkg.pmpkg))
	return DependList{(*list)(ptr)}
}

// Description returns the package's description.
func (pkg Package) Description() string {
	return C.GoString(C.alpm_pkg_get_desc(pkg.pmpkg))
}

// Files returns the file list of the package.
func (pkg Package) Files() []File {
	cFiles := C.alpm_pkg_get_files(pkg.pmpkg)
	return convertFilelist(cFiles)
}

// Groups returns the groups the package belongs to.
func (pkg Package) Groups() StringList {
	ptr := unsafe.Pointer(C.alpm_pkg_get_groups(pkg.pmpkg))
	return StringList{(*list)(ptr)}
}

// ISize returns the package installed size.
func (pkg Package) ISize() int64 {
	t := C.alpm_pkg_get_isize(pkg.pmpkg)
	return int64(t)
}

// InstallDate returns the package install date.
func (pkg Package) InstallDate() time.Time {
	t := C.alpm_pkg_get_installdate(pkg.pmpkg)
	return time.Unix(int64(t), 0)
}

// Licenses returns the package license list.
func (pkg Package) Licenses() StringList {
	ptr := unsafe.Pointer(C.alpm_pkg_get_licenses(pkg.pmpkg))
	return StringList{(*list)(ptr)}
}

// SHA256Sum returns package SHA256Sum.
func (pkg Package) SHA256Sum() string {
	return C.GoString(C.alpm_pkg_get_sha256sum(pkg.pmpkg))
}

// MD5Sum returns package MD5Sum.
func (pkg Package) MD5Sum() string {
	return C.GoString(C.alpm_pkg_get_md5sum(pkg.pmpkg))
}

// Name returns package name.
func (pkg Package) Name() string {
	return C.GoString(C.alpm_pkg_get_name(pkg.pmpkg))
}

// Packager returns package packager name.
func (pkg Package) Packager() string {
	return C.GoString(C.alpm_pkg_get_packager(pkg.pmpkg))
}

// Provides returns DependList of packages provides by package.
func (pkg Package) Provides() DependList {
	ptr := unsafe.Pointer(C.alpm_pkg_get_provides(pkg.pmpkg))
	return DependList{(*list)(ptr)}
}

// Reason returns package install reason.
func (pkg Package) Reason() PkgReason {
	reason := C.alpm_pkg_get_reason(pkg.pmpkg)
	return PkgReason(reason)
}

// Origin returns package origin.
func (pkg Package) Origin() PkgFrom {
	origin := C.alpm_pkg_get_origin(pkg.pmpkg)
	return PkgFrom(origin)
}

// Replaces returns a DependList with the packages this package replaces.
func (pkg Package) Replaces() DependList {
	ptr := unsafe.Pointer(C.alpm_pkg_get_replaces(pkg.pmpkg))
	return DependList{(*list)(ptr)}
}

// Size returns the packed package size.
func (pkg Package) Size() int64 {
	t := C.alpm_pkg_get_size(pkg.pmpkg)
	return int64(t)
}

// URL returns the upstream URL of the package.
func (pkg Package) URL() string {
	return C.GoString(C.alpm_pkg_get_url(pkg.pmpkg))
}

// Version returns the package version.
func (pkg Package) Version() string {
	return C.GoString(C.alpm_pkg_get_version(pkg.pmpkg))
}

// ComputeRequiredBy returns the names of reverse dependencies of a package
func (pkg Package) ComputeRequiredBy() []string {
	result := C.alpm_pkg_compute_requiredby(pkg.pmpkg)
	requiredby := make([]string, 0)
	for i := (*list)(unsafe.Pointer(result)); i != nil; i = i.Next {
		defer C.free(unsafe.Pointer(i))
		if i.Data != nil {
			defer C.free(unsafe.Pointer(i.Data))
			name := C.GoString((*C.char)(unsafe.Pointer(i.Data)))
			requiredby = append(requiredby, name)
		}
	}
	return requiredby
}

// NewVersion checks if there is a new version of the package in the Synced DBs.
func (pkg Package) NewVersion(l DbList) *Package {
	ptr := C.alpm_sync_newversion(pkg.pmpkg,
		(*C.alpm_list_t)(unsafe.Pointer(l.list)))
	if ptr == nil {
		return nil
	}
	return &Package{ptr, l.handle}
}
