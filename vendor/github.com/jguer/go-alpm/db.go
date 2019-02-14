// db.go - Functions for database handling.
//
// Copyright (c) 2013 The go-alpm Authors
//
// MIT Licensed. See LICENSE for details.

package alpm

/*
#include <alpm.h>
#include <alpm_list.h>
*/
import "C"

import (
	"fmt"
	"io"
	"unsafe"
)

// DB structure representing a alpm database.
type DB struct {
	ptr    *C.alpm_db_t
	handle Handle
}

// DBList structure representing a alpm database list.
type DBList struct {
	*list
	handle Handle
}

// ForEach executes an action on each DB.
func (l DBList) ForEach(f func(DB) error) error {
	return l.forEach(func(p unsafe.Pointer) error {
		return f(DB{(*C.alpm_db_t)(p), l.handle})
	})
}

// Slice converst DB list to DB slice.
func (l DBList) Slice() []DB {
	slice := []DB{}
	l.ForEach(func(db DB) error {
		slice = append(slice, db)
		return nil
	})
	return slice
}

// SyncDBByName finds a registered database by name.
func (h *Handle) SyncDBByName(name string) (db *DB, err error) {
	dblist, err := h.SyncDBs()
	if err != nil {
		return nil, err
	}
	dblist.ForEach(func(b DB) error {
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

// RegisterSyncDB Loads a sync database with given name and signature check level.
func (h *Handle) RegisterSyncDB(dbname string, siglevel SigLevel) (*DB, error) {
	cName := C.CString(dbname)
	defer C.free(unsafe.Pointer(cName))

	db := C.alpm_register_syncdb(h.ptr, cName, C.int(siglevel))
	if db == nil {
		return nil, h.LastError()
	}
	return &DB{db, *h}, nil
}

func (db *DB) Unregister() error {
	ok := C.alpm_db_unregister(db.ptr)
	if ok != 0 {
		return db.handle.LastError()
	}

	return nil
}

func (h *Handle) UnregisterAllSyncDBs() error {
	ok := C.alpm_unregister_all_syncdbs(h.ptr)
	if ok != 0 {
		return h.LastError()
	}

	return nil
}

// Name returns name of the db
func (db *DB) Name() string {
	return C.GoString(C.alpm_db_get_name(db.ptr))
}

// Servers returns host server URL.
func (db *DB) Servers() []string {
	ptr := unsafe.Pointer(C.alpm_db_get_servers(db.ptr))
	return StringList{(*list)(ptr)}.Slice()
}

// SetServers sets server list to use.
func (db *DB) SetServers(servers []string) {
	C.alpm_db_set_servers(db.ptr, nil)
	for _, srv := range servers {
		Csrv := C.CString(srv)
		defer C.free(unsafe.Pointer(Csrv))
		C.alpm_db_add_server(db.ptr, Csrv)
	}
}

// AddServers adds a string to the server list.
func (db *DB) AddServer(server string) {
	Csrv := C.CString(server)
	defer C.free(unsafe.Pointer(Csrv))
	C.alpm_db_add_server(db.ptr, Csrv)
}

// SetUsage sets the Usage of the database
func (db *DB) SetUsage(usage Usage) {
	C.alpm_db_set_usage(db.ptr, C.int(usage))
}

// Name searches a package in db.
func (db *DB) Pkg(name string) *Package {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	ptr := C.alpm_db_get_pkg(db.ptr, cName)
	return &Package{ptr, db.handle}
}

// PkgCachebyGroup returns a PackageList of packages belonging to a group
func (l DBList) FindGroupPkgs(name string) PackageList {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	pkglist := (*C.struct___alpm_list_t)(unsafe.Pointer(l.list))
	pkgcache := (*list)(unsafe.Pointer(C.alpm_find_group_pkgs(pkglist, cName)))
	return PackageList{pkgcache, l.handle}
}

// PkgCache returns the list of packages of the database
func (db *DB) PkgCache() PackageList {
	pkgcache := (*list)(unsafe.Pointer(C.alpm_db_get_pkgcache(db.ptr)))
	return PackageList{pkgcache, db.handle}
}

func (db *DB) Search(targets []string) PackageList {
	var needles *C.alpm_list_t

	for _, str := range targets {
		needles = C.alpm_list_add(needles, unsafe.Pointer(C.CString(str)))
	}

	pkglist := (*list)(unsafe.Pointer(C.alpm_db_search(db.ptr, needles)))
	C.alpm_list_free(needles)
	return PackageList{pkglist, db.handle}
}
