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

func alpmSetLogCallback(alpmHandle *alpm.Handle, cb func(alpm.LogLevel, string)) {
	alpmHandle.SetLogCallback(func(ctx interface{}, lvl alpm.LogLevel, msg string) {
		cb := ctx.(func(alpm.LogLevel, string))
		cb(lvl, msg)
	}, cb)
}

func alpmSetQuestionCallback(alpmHandle *alpm.Handle, cb func(alpm.QuestionAny)) {
	alpmHandle.SetQuestionCallback(func(ctx interface{}, q alpm.QuestionAny) {
		cb := ctx.(func(alpm.QuestionAny))
		cb(q)
	}, cb)
}
