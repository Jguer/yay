package main

import (
	alpm "github.com/jguer/go-alpm"
)

func questionCallback(question alpm.QuestionAny) {
	q, err := question.QuestionInstallIgnorepkg()
	if err == nil {
		q.SetInstall(true)
	}
}
