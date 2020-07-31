package db

import alpm "github.com/Jguer/go-alpm"

type RepoPackage interface {
	Base() string
	Name() string
	Version() string
	DB() *alpm.DB
	ISize() int64
	Size() int64
	Description() string
}
