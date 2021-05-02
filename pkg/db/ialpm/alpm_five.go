// +build !six

package ialpm

import (
	alpm "github.com/Jguer/go-alpm/v2"
)

func alpmSetArchitecture(alpmHandle *alpm.Handle, arch []string) error {
	return alpmHandle.SetArch(arch[0])
}

func (ae *AlpmExecutor) AlpmArchitectures() ([]string, error) {
	arch, err := ae.handle.Arch()

	return []string{arch}, err
}
