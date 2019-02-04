package alpm

// #include <alpm.h>
import "C"

import "unsafe"

// VerCmp performs version comparison according to Pacman conventions. Return
// value is <0 if and only if v1 is older than v2.
func VerCmp(v1, v2 string) int {
	c1 := C.CString(v1)
	c2 := C.CString(v2)
	defer C.free(unsafe.Pointer(c1))
	defer C.free(unsafe.Pointer(c2))
	result := C.alpm_pkg_vercmp(c1, c2)
	return int(result)
}
