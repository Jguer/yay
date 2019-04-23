// alpm.go - Implements exported libalpm functions.
//
// Copyright (c) 2013 The go-alpm Authors
//
// MIT Licensed. See LICENSE for details.

package alpm

// #cgo LDFLAGS: -lalpm
// #include <alpm.h>
import "C"

import "unsafe"

// Init initializes alpm handle
func Initialize(root, dbpath string) (*Handle, error) {
	cRoot := C.CString(root)
	cDBPath := C.CString(dbpath)
	var cErr C.alpm_errno_t
	h := C.alpm_initialize(cRoot, cDBPath, &cErr)

	defer C.free(unsafe.Pointer(cRoot))
	defer C.free(unsafe.Pointer(cDBPath))

	if cErr != 0 {
		return nil, Error(cErr)
	}

	return &Handle{h}, nil
}

// Release releases the alpm handle
func (h *Handle) Release() error {
	if er := C.alpm_release(h.ptr); er != 0 {
		return Error(er)
	}
	h.ptr = nil
	return nil
}

// LastError gets the last pm_error
func (h *Handle) LastError() error {
	if h.ptr != nil {
		cErr := C.alpm_errno(h.ptr)
		if cErr != 0 {
			return Error(cErr)
		}
	}
	return nil
}

// Version returns libalpm version string.
func Version() string {
	return C.GoString(C.alpm_version())
}
