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

func (h *Handle) SyncSysupgrade(enableDowngrade bool) error {
	intEnableDowngrade := C.int(0)

	if enableDowngrade {
		intEnableDowngrade = C.int(1)
	}

	ret := C.alpm_sync_sysupgrade(h.ptr, intEnableDowngrade)
	if ret != 0 {
		return h.LastError()
	}

	return nil
}
