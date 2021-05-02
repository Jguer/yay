// +build six

package ialpm

import (
	alpm "github.com/Jguer/go-alpm/v2"
)

func alpmSetArchitecture(alpmHandle *alpm.Handle, arch []string) error {
	return alpmHandle.SetArchitectures(arch)
}

func (ae *AlpmExecutor) AlpmArchitectures() ([]string, error) {
	architectures, err := ae.handle.GetArchitectures()

	return architectures.Slice(), err
}
