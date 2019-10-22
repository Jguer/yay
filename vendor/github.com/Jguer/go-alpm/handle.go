// handle.go - libalpm handle type and methods.
//
// Copyright (c) 2013 The go-alpm Authors
//
// MIT Licensed. See LICENSE for details.

// Package alpm implements Go bindings to the libalpm library used by Pacman,
// the Arch Linux package manager. Libalpm allows the creation of custom front
// ends to the Arch Linux package ecosystem.
//
// Libalpm does not include support for the Arch User Repository (AUR).
package alpm

// #include <alpm.h>
// #include <stdio.h> //C.free
// #include <fnmatch.h> //C.FNM_NOMATCH
import "C"

import (
	"unsafe"
)

// Handle contains the pointer to the alpm handle
type Handle struct {
	ptr *C.alpm_handle_t
}

//
//alpm options getters and setters
//

//helper functions for wrapping list_t getters and setters
func (h *Handle) optionGetList(f func(*C.alpm_handle_t) *C.alpm_list_t) (StringList, error) {
	alpmList := f(h.ptr)
	goList := StringList{(*list)(unsafe.Pointer(alpmList))}

	if alpmList == nil {
		return goList, h.LastError()
	}
	return goList, nil
}

func (h *Handle) optionSetList(hookDirs []string, f func(*C.alpm_handle_t, *C.alpm_list_t) C.int) error {
	var list *C.alpm_list_t

	for _, dir := range hookDirs {
		cDir := C.CString(dir)
		list = C.alpm_list_add(list, unsafe.Pointer(cDir))
		defer C.free(unsafe.Pointer(cDir))
	}

	ok := f(h.ptr, list)
	if ok < 0 {
		return h.LastError()
	}
	return nil
}

func (h *Handle) optionAddList(hookDir string, f func(*C.alpm_handle_t, *C.char) C.int) error {
	cHookDir := C.CString(hookDir)
	defer C.free(unsafe.Pointer(cHookDir))
	ok := f(h.ptr, cHookDir)
	if ok < 0 {
		return h.LastError()
	}
	return nil
}

func (h *Handle) optionRemoveList(dir string, f func(*C.alpm_handle_t, *C.char) C.int) (bool, error) {
	cDir := C.CString(dir)
	ok := f(h.ptr, cDir)
	defer C.free(unsafe.Pointer(cDir))
	if ok < 0 {
		return ok == 1, h.LastError()
	}
	return ok == 1, nil
}

func (h *Handle) optionMatchList(dir string, f func(*C.alpm_handle_t, *C.char) C.int) (bool, error) {
	cDir := C.CString(dir)
	ok := f(h.ptr, cDir)
	defer C.free(unsafe.Pointer(cDir))
	if ok == 0 {
		return true, nil
	} else if ok == C.FNM_NOMATCH {
		return false, h.LastError()
	}
	return false, nil
}

//helper functions for *char based getters and setters
func (h *Handle) optionGetStr(f func(*C.alpm_handle_t) *C.char) (string, error) {
	cStr := f(h.ptr)
	str := C.GoString(cStr)
	if cStr == nil {
		return str, h.LastError()
	}

	return str, nil
}

func (h *Handle) optionSetStr(str string, f func(*C.alpm_handle_t, *C.char) C.int) error {
	cStr := C.CString(str)
	defer C.free(unsafe.Pointer(cStr))
	ok := f(h.ptr, cStr)

	if ok < 0 {
		h.LastError()
	}
	return nil
}

//
//end of helpers
//

func (h *Handle) Root() (string, error) {
	return h.optionGetStr(func(handle *C.alpm_handle_t) *C.char {
		return C.alpm_option_get_root(handle)
	})
}

func (h *Handle) DBPath() (string, error) {
	return h.optionGetStr(func(handle *C.alpm_handle_t) *C.char {
		return C.alpm_option_get_dbpath(handle)
	})
}

func (h *Handle) Lockfile() (string, error) {
	return h.optionGetStr(func(handle *C.alpm_handle_t) *C.char {
		return C.alpm_option_get_lockfile(handle)
	})
}

func (h *Handle) CacheDirs() (StringList, error) {
	return h.optionGetList(func(handle *C.alpm_handle_t) *C.alpm_list_t {
		return C.alpm_option_get_cachedirs(handle)
	})
}

func (h *Handle) AddCacheDir(hookDir string) error {
	return h.optionAddList(hookDir, func(handle *C.alpm_handle_t, str *C.char) C.int {
		return C.alpm_option_add_cachedir(handle, str)
	})
}

func (h *Handle) SetCacheDirs(hookDirs []string) error {
	return h.optionSetList(hookDirs, func(handle *C.alpm_handle_t, l *C.alpm_list_t) C.int {
		return C.alpm_option_set_cachedirs(handle, l)
	})
}

func (h *Handle) RemoveCacheDir(dir string) (bool, error) {
	return h.optionRemoveList(dir, func(handle *C.alpm_handle_t, str *C.char) C.int {
		return C.alpm_option_remove_cachedir(handle, str)
	})
}

func (h *Handle) HookDirs() (StringList, error) {
	return h.optionGetList(func(handle *C.alpm_handle_t) *C.alpm_list_t {
		return C.alpm_option_get_hookdirs(handle)
	})
}

func (h *Handle) AddHookDir(hookDir string) error {
	return h.optionAddList(hookDir, func(handle *C.alpm_handle_t, str *C.char) C.int {
		return C.alpm_option_add_hookdir(handle, str)
	})
}

func (h *Handle) SetHookDirs(hookDirs []string) error {
	return h.optionSetList(hookDirs, func(handle *C.alpm_handle_t, l *C.alpm_list_t) C.int {
		return C.alpm_option_set_hookdirs(handle, l)
	})
}

func (h *Handle) RemoveHookDir(dir string) (bool, error) {
	return h.optionRemoveList(dir, func(handle *C.alpm_handle_t, str *C.char) C.int {
		return C.alpm_option_remove_hookdir(handle, str)
	})
}

func (h *Handle) LogFile() (string, error) {
	return h.optionGetStr(func(handle *C.alpm_handle_t) *C.char {
		return C.alpm_option_get_logfile(handle)
	})
}

func (h *Handle) SetLogFile(str string) error {
	return h.optionSetStr(str, func(handle *C.alpm_handle_t, c_str *C.char) C.int {
		return C.alpm_option_set_logfile(handle, c_str)
	})
}

func (h *Handle) GPGDir() (string, error) {
	return h.optionGetStr(func(handle *C.alpm_handle_t) *C.char {
		return C.alpm_option_get_gpgdir(handle)
	})
}

func (h *Handle) SetGPGDir(str string) error {
	return h.optionSetStr(str, func(handle *C.alpm_handle_t, c_str *C.char) C.int {
		return C.alpm_option_set_gpgdir(handle, c_str)
	})
}

func (h *Handle) UseSyslog() (bool, error) {
	ok := C.alpm_option_get_usesyslog(h.ptr)
	b := false

	if ok > 0 {
		b = true
	}
	if ok < 0 {
		return b, h.LastError()
	}
	return b, nil
}

func (h *Handle) SetUseSyslog(value bool) error {
	var intValue C.int
	if value {
		intValue = 1
	}

	ok := C.alpm_option_set_usesyslog(h.ptr, intValue)
	if ok < 0 {
		return h.LastError()
	}
	return nil
}

func (h *Handle) NoUpgrades() (StringList, error) {
	return h.optionGetList(func(handle *C.alpm_handle_t) *C.alpm_list_t {
		return C.alpm_option_get_noupgrades(handle)
	})
}

func (h *Handle) AddNoUpgrade(hookDir string) error {
	return h.optionAddList(hookDir, func(handle *C.alpm_handle_t, str *C.char) C.int {
		return C.alpm_option_add_noupgrade(handle, str)
	})
}

func (h *Handle) SetNoUpgrades(hookDirs []string) error {
	return h.optionSetList(hookDirs, func(handle *C.alpm_handle_t, l *C.alpm_list_t) C.int {
		return C.alpm_option_set_noupgrades(handle, l)
	})
}

func (h *Handle) RemoveNoUpgrade(dir string) (bool, error) {
	return h.optionRemoveList(dir, func(handle *C.alpm_handle_t, str *C.char) C.int {
		return C.alpm_option_remove_noupgrade(handle, str)
	})
}

func (h *Handle) MatchNoUpgrade(dir string) (bool, error) {
	return h.optionMatchList(dir, func(handle *C.alpm_handle_t, str *C.char) C.int {
		return C.alpm_option_match_noupgrade(handle, str)
	})
}

func (h *Handle) NoExtracts() (StringList, error) {
	return h.optionGetList(func(handle *C.alpm_handle_t) *C.alpm_list_t {
		return C.alpm_option_get_noextracts(handle)
	})
}

func (h *Handle) AddNoExtract(hookDir string) error {
	return h.optionAddList(hookDir, func(handle *C.alpm_handle_t, str *C.char) C.int {
		return C.alpm_option_add_noextract(handle, str)
	})
}

func (h *Handle) SetNoExtracts(hookDirs []string) error {
	return h.optionSetList(hookDirs, func(handle *C.alpm_handle_t, l *C.alpm_list_t) C.int {
		return C.alpm_option_set_noextracts(handle, l)
	})
}

func (h *Handle) RemoveNoExtract(dir string) (bool, error) {
	return h.optionRemoveList(dir, func(handle *C.alpm_handle_t, str *C.char) C.int {
		return C.alpm_option_remove_noextract(handle, str)
	})
}

func (h *Handle) MatchNoExtract(dir string) (bool, error) {
	return h.optionMatchList(dir, func(handle *C.alpm_handle_t, str *C.char) C.int {
		return C.alpm_option_match_noextract(handle, str)
	})
}

func (h *Handle) IgnorePkgs() (StringList, error) {
	return h.optionGetList(func(handle *C.alpm_handle_t) *C.alpm_list_t {
		return C.alpm_option_get_ignorepkgs(handle)
	})
}

func (h *Handle) AddIgnorePkg(hookDir string) error {
	return h.optionAddList(hookDir, func(handle *C.alpm_handle_t, str *C.char) C.int {
		return C.alpm_option_add_ignorepkg(handle, str)
	})
}

func (h *Handle) SetIgnorePkgs(hookDirs []string) error {
	return h.optionSetList(hookDirs, func(handle *C.alpm_handle_t, l *C.alpm_list_t) C.int {
		return C.alpm_option_set_ignorepkgs(handle, l)
	})
}

func (h *Handle) RemoveIgnorePkg(dir string) (bool, error) {
	return h.optionRemoveList(dir, func(handle *C.alpm_handle_t, str *C.char) C.int {
		return C.alpm_option_remove_ignorepkg(handle, str)
	})
}

func (h *Handle) IgnoreGroups() (StringList, error) {
	return h.optionGetList(func(handle *C.alpm_handle_t) *C.alpm_list_t {
		return C.alpm_option_get_ignoregroups(handle)
	})
}

func (h *Handle) AddIgnoreGroup(hookDir string) error {
	return h.optionAddList(hookDir, func(handle *C.alpm_handle_t, str *C.char) C.int {
		return C.alpm_option_add_ignoregroup(handle, str)
	})
}

func (h *Handle) SetIgnoreGroups(hookDirs []string) error {
	return h.optionSetList(hookDirs, func(handle *C.alpm_handle_t, l *C.alpm_list_t) C.int {
		return C.alpm_option_set_ignoregroups(handle, l)
	})
}

func (h *Handle) RemoveIgnoreGroup(dir string) (bool, error) {
	return h.optionRemoveList(dir, func(handle *C.alpm_handle_t, str *C.char) C.int {
		return C.alpm_option_remove_ignoregroup(handle, str)
	})
}

/*func (h *Handle) optionGetList(f func(*C.alpm_handle_t) *C.alpm_list_t) (StringList, error){
	alpmList := f(h.ptr)
	goList := StringList{(*list)(unsafe.Pointer(alpmList))}

	if alpmList == nil {
		return goList, h.LastError()
	}
	return goList, nil
}*/

//use alpm_depend_t
func (h *Handle) AssumeInstalled() (DependList, error) {
	alpmList := C.alpm_option_get_assumeinstalled(h.ptr)
	depList := DependList{(*list)(unsafe.Pointer(alpmList))}

	if alpmList == nil {
		return depList, h.LastError()
	}
	return depList, nil
}

func (h *Handle) AddAssumeInstalled(dep Depend) error {
	cDep := convertCDepend(dep)
	defer freeCDepend(cDep)

	ok := C.alpm_option_add_assumeinstalled(h.ptr, cDep)
	if ok < 0 {
		return h.LastError()
	}
	return nil
}

// TODO: Fix
// func (h *Handle) SetAssumeInstalled(deps []Depend) error {
// 	//calling this function the first time causes alpm to set the
// 	//assumeinstalled list to a list containing go allocated alpm_depend_t's
// 	//this is bad because alpm might at some point tree to free them
// 	//i believe this is whats causing this function to misbhave
// 	//although i am not 100% sure
// 	//maybe using C.malloc to make the struct could fix the problem
// 	//pacamn does not use alpm_option_set_assumeinstalled in its source
// 	//code so anybody using this should beable to do file without it
// 	//although for the sake of completeness it would be nice to have this
// 	//working
// 	panic("This function (SetAssumeInstalled) does not work properly, please do not use. See source code for more details")
// 	var list *C.alpm_list_t

// 	for _, dep := range deps {
// 		cDep := convertCDepend(dep)
// 		defer freeCDepend(cDep)
// 		list = C.alpm_list_add(list, unsafe.Pointer(cDep))
// 	}

// 	ok := C.alpm_option_set_assumeinstalled(h.ptr, list)
// 	if ok < 0 {
// 		return h.LastError()
// 	}
// 	return nil

// }

// TODO: Fix
//  func (h *Handle) RemoveAssumeInstalled(dep Depend) (bool, error) {
//internally alpm uses alpm_list_remove to remove a alpm_depend_t from
//the list
//i believe this function considers items equal if they are the same
//item in memeory, not just the same data
//every time we convert a go Depend to a alpm_depend_c we create a new
//instance of a alpm_depend_c
//this means that if you add a Depend using AddAssumeInstalled then try
//to remove it using the same Depend c will consider them different
//items and not remove them
//pacamn does not use alpm_option_set_assumeinstalled in its source
//code so anybody using this should beable to do file without it
//although for the sake of completeness it would be nice to have this
//working
// 	panic("This function (RemoveAssumeInstalled) does not work properly, please do not use. See source code for more details")
// 	cDep := convertCDepend(dep)
// 	defer freeCDepend(cDep)

// 	ok := C.alpm_option_remove_assumeinstalled(h.ptr, cDep)
// 	if ok < 0 {
// 		return ok == 1, h.LastError()
// 	}
// 	return ok == 1, nil
// }

func (h *Handle) Arch() (string, error) {
	return h.optionGetStr(func(handle *C.alpm_handle_t) *C.char {
		return C.alpm_option_get_arch(handle)
	})
}

func (h *Handle) SetArch(str string) error {
	return h.optionSetStr(str, func(handle *C.alpm_handle_t, cStr *C.char) C.int {
		return C.alpm_option_set_arch(handle, cStr)
	})
}

// LocalDB returns the local database relative to the given handle.
func (h *Handle) LocalDB() (*DB, error) {
	db := C.alpm_get_localdb(h.ptr)
	if db == nil {
		return nil, h.LastError()
	}
	return &DB{db, *h}, nil
}

// SyncDBs returns list of Synced DBs.
func (h *Handle) SyncDBs() (DBList, error) {
	dblist := C.alpm_get_syncdbs(h.ptr)
	if dblist == nil {
		return DBList{nil, *h}, h.LastError()
	}
	dblistPtr := unsafe.Pointer(dblist)
	return DBList{(*list)(dblistPtr), *h}, nil
}

func (h *Handle) CheckSpace() (bool, error) {
	ok := C.alpm_option_get_checkspace(h.ptr)
	b := false

	if ok > 0 {
		b = true
	}
	if ok < 0 {
		return b, h.LastError()
	}
	return b, nil
}

func (h *Handle) SetCheckSpace(value bool) error {
	var cValue C.int
	if value {
		cValue = 1
	}

	ok := C.alpm_option_set_checkspace(h.ptr, cValue)
	if ok < 0 {
		return h.LastError()
	}
	return nil
}

func (h *Handle) DBExt() (string, error) {
	return h.optionGetStr(func(handle *C.alpm_handle_t) *C.char {
		return C.alpm_option_get_dbext(handle)
	})
}

func (h *Handle) SetDBExt(str string) error {
	return h.optionSetStr(str, func(handle *C.alpm_handle_t, cStr *C.char) C.int {
		return C.alpm_option_set_dbext(handle, cStr)
	})
}

func (h *Handle) GetDefaultSigLevel() (SigLevel, error) {
	sigLevel := C.alpm_option_get_default_siglevel(h.ptr)

	if sigLevel < 0 {
		return SigLevel(sigLevel), h.LastError()
	}
	return SigLevel(sigLevel), nil
}

func (h *Handle) SetDefaultSigLevel(siglevel SigLevel) error {
	ok := C.alpm_option_set_default_siglevel(h.ptr, C.int(siglevel))

	if ok < 0 {
		return h.LastError()
	}
	return nil
}

func (h *Handle) GetLocalFileSigLevel() (SigLevel, error) {
	sigLevel := C.alpm_option_get_local_file_siglevel(h.ptr)

	if sigLevel < 0 {
		return SigLevel(sigLevel), h.LastError()
	}
	return SigLevel(sigLevel), nil
}

func (h *Handle) SetLocalFileSigLevel(siglevel SigLevel) error {
	ok := C.alpm_option_set_local_file_siglevel(h.ptr, C.int(siglevel))

	if ok < 0 {
		return h.LastError()
	}
	return nil
}

func (h *Handle) GetRemoteFileSigLevel() (SigLevel, error) {
	sigLevel := C.alpm_option_get_remote_file_siglevel(h.ptr)

	if sigLevel < 0 {
		return SigLevel(sigLevel), h.LastError()
	}
	return SigLevel(sigLevel), nil
}

func (h *Handle) SetRemoteFileSigLevel(siglevel SigLevel) error {
	ok := C.alpm_option_set_remote_file_siglevel(h.ptr, C.int(siglevel))

	if ok < 0 {
		return h.LastError()
	}
	return nil
}
