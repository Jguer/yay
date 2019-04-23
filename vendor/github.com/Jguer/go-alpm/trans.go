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
	"unsafe"
)

func (h *Handle) TransInit(flags TransFlag) error {
	ret := C.alpm_trans_init(h.ptr, C.int(flags))
	if ret != 0 {
		return h.LastError()
	}

	return nil
}

func (h *Handle) TransRelease() error {
	ret := C.alpm_trans_release(h.ptr)
	if ret != 0 {
		return h.LastError()
	}

	return nil
}

func (h *Handle) TransGetAdd() PackageList {
	pkgs := C.alpm_trans_get_add(h.ptr)
	return PackageList{(*list)(unsafe.Pointer(pkgs)), *h}
}

func (h *Handle) TransGetRemove() PackageList {
	pkgs := C.alpm_trans_get_remove(h.ptr)
	return PackageList{(*list)(unsafe.Pointer(pkgs)), *h}
}

func (h *Handle) TransGetFlags() (TransFlag, error) {
	flags := C.alpm_trans_get_flags(h.ptr)

	if flags == -1 {
		return -1, h.LastError()
	}

	return TransFlag(flags), nil
}
