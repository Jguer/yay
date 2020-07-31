package db

import (
	"time"

	alpm "github.com/Jguer/go-alpm"
)

type RepoPackage interface {
	Base() string
	BuildDate() time.Time
	DB() *alpm.DB
	Description() string
	ISize() int64
	Name() string
	ShouldIgnore() bool
	Size() int64
	Version() string
}
