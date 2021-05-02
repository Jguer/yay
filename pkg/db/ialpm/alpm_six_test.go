// +build six

package ialpm

import (
	alpm "github.com/Jguer/go-alpm/v2"
)

func alpmTestGetArch(h *alpm.Handle) ([]string, error) {
	architectures, err := h.GetArchitectures()

	return architectures.Slice(), err
}
