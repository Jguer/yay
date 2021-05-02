// +build !six

package ialpm

import (
	alpm "github.com/Jguer/go-alpm/v2"
)

func alpmTestGetArch(h *alpm.Handle) ([]string, error) {
	arch, err := h.Arch()

	return []string{arch}, err
}
