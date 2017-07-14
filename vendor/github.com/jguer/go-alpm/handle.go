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
import "C"

import (
	"unsafe"
)

type Handle struct {
	ptr *C.alpm_handle_t
}

// Initialize
func Init(root, dbpath string) (*Handle, error) {
	c_root := C.CString(root)
	defer C.free(unsafe.Pointer(c_root))
	c_dbpath := C.CString(dbpath)
	defer C.free(unsafe.Pointer(c_dbpath))
	var c_err C.alpm_errno_t
	h := C.alpm_initialize(c_root, c_dbpath, &c_err)

	if c_err != 0 {
		return nil, Error(c_err)
	}

	return &Handle{h}, nil
}

func (h *Handle) Release() error {
	if er := C.alpm_release(h.ptr); er != 0 {
		return Error(er)
	}
	h.ptr = nil
	return nil
}

func (h Handle) Root() string {
	return C.GoString(C.alpm_option_get_root(h.ptr))
}

func (h Handle) DbPath() string {
	return C.GoString(C.alpm_option_get_dbpath(h.ptr))
}

// LastError gets the last pm_error
func (h Handle) LastError() error {
	if h.ptr != nil {
		c_err := C.alpm_errno(h.ptr)
		if c_err != 0 {
			return Error(c_err)
		}
	}
	return nil
}

func (h Handle) UseSyslog() bool {
	value := C.alpm_option_get_usesyslog(h.ptr)
	return (value != 0)
}

func (h Handle) SetUseSyslog(value bool) error {
	var int_value C.int
	if value {
		int_value = 1
	} else {
		int_value = 0
	}
	ok := C.alpm_option_set_usesyslog(h.ptr, int_value)
	if ok < 0 {
		return h.LastError()
	}
	return nil
}
