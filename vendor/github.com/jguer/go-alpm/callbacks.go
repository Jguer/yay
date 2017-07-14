// callbacks.go - Handles libalpm callbacks.
//
// Copyright (c) 2013 The go-alpm Authors
//
// MIT Licensed. See LICENSE for details.

package alpm

/*
#include <stdint.h>
#include <alpm.h>
void logCallback(uint16_t level, char *cstring);
void go_alpm_log_cb(alpm_loglevel_t level, const char *fmt, va_list arg);
void go_alpm_set_logging(alpm_handle_t *handle);
*/
import "C"

var DefaultLogLevel = LogWarning

func DefaultLogCallback(lvl uint16, s string) {
	if lvl <= DefaultLogLevel {
		print("go-alpm: ", s)
	}
}

var log_callback = DefaultLogCallback

//export logCallback
func logCallback(level uint16, cstring *C.char) {
	log_callback(level, C.GoString(cstring))
}

func (h *Handle) SetLogCallback(cb func(uint16, string)) {
	log_callback = cb
	C.go_alpm_set_logging(h.ptr)
}
