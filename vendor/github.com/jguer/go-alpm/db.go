// db.go - Functions for database handling.
//
// Copyright (c) 2013 The go-alpm Authors
//
// MIT Licensed. See LICENSE for details.

package alpm

/*
#include <alpm.h>
*/
import "C"

import (
	"fmt"
	"io"
	"unsafe"
)

// Db structure representing a alpm database.
type Db struct {
	ptr    *C.alpm_db_t
	handle Handle
}

// DbList structure representing a alpm database list.
type DbList struct {
	*list
	handle Handle
}

// ForEach executes an action on each Db.
func (l DbList) ForEach(f func(Db) error) error {
	return l.forEach(func(p unsafe.Pointer) error {
		return f(Db{(*C.alpm_db_t)(p), l.handle})
	})
}

// Slice converst Db list to Db slice.
func (l DbList) Slice() []Db {
	slice := []Db{}
	l.ForEach(func(db Db) error {
		slice = append(slice, db)
		return nil
	})
	return slice
}

// LocalDb returns the local database relative to the given handle.
func (h Handle) LocalDb() (*Db, error) {
	db := C.alpm_get_localdb(h.ptr)
	if db == nil {
		return nil, h.LastError()
	}
	return &Db{db, h}, nil
}

// SyncDbs returns list of Synced DBs.
func (h Handle) SyncDbs() (DbList, error) {
	dblist := C.alpm_get_syncdbs(h.ptr)
	if dblist == nil {
		return DbList{nil, h}, h.LastError()
	}
	dblistPtr := unsafe.Pointer(dblist)
	return DbList{(*list)(dblistPtr), h}, nil
}

// SyncDbByName finds a registered database by name.
func (h Handle) SyncDbByName(name string) (db *Db, err error) {
	dblist, err := h.SyncDbs()
	if err != nil {
		return nil, err
	}
	dblist.ForEach(func(b Db) error {
		if b.Name() == name {
			db = &b
			return io.EOF
		}
		return nil
	})
	if db != nil {
		return db, nil
	}
	return nil, fmt.Errorf("database %s not found", name)
}

// RegisterSyncDb Loads a sync database with given name and signature check level.
func (h Handle) RegisterSyncDb(dbname string, siglevel SigLevel) (*Db, error) {
	cName := C.CString(dbname)
	defer C.free(unsafe.Pointer(cName))

	db := C.alpm_register_syncdb(h.ptr, cName, C.alpm_siglevel_t(siglevel))
	if db == nil {
		return nil, h.LastError()
	}
	return &Db{db, h}, nil
}

// Name returns name of the db
func (db Db) Name() string {
	return C.GoString(C.alpm_db_get_name(db.ptr))
}

// Servers returns host server URL.
func (db Db) Servers() []string {
	ptr := unsafe.Pointer(C.alpm_db_get_servers(db.ptr))
	return StringList{(*list)(ptr)}.Slice()
}

// SetServers sets server list to use.
func (db Db) SetServers(servers []string) {
	C.alpm_db_set_servers(db.ptr, nil)
	for _, srv := range servers {
		Csrv := C.CString(srv)
		defer C.free(unsafe.Pointer(Csrv))
		C.alpm_db_add_server(db.ptr, Csrv)
	}
}

// PkgByName searches a package in db.
func (db Db) PkgByName(name string) (*Package, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	ptr := C.alpm_db_get_pkg(db.ptr, cName)
	if ptr == nil {
		return nil,
			fmt.Errorf("Error when retrieving %s from database %s: %s",
				name, db.Name(), db.handle.LastError())
	}
	return &Package{ptr, db.handle}, nil
}

// PkgCachebyGroup returns a PackageList of packages belonging to a group
func (l DbList) PkgCachebyGroup(name string) (PackageList, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	pkglist := (*C.struct___alpm_list_t)(unsafe.Pointer(l.list))

	pkgcache := (*list)(unsafe.Pointer(C.alpm_find_group_pkgs(pkglist, cName)))
	if pkgcache == nil {
		return PackageList{pkgcache, l.handle},
			fmt.Errorf("Error when retrieving group %s from database list: %s",
				name, l.handle.LastError())
	}

	return PackageList{pkgcache, l.handle}, nil
}

// PkgCache returns the list of packages of the database
func (db Db) PkgCache() PackageList {
	pkgcache := (*list)(unsafe.Pointer(C.alpm_db_get_pkgcache(db.ptr)))
	return PackageList{pkgcache, db.handle}
}
