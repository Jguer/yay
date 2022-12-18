package vcs

import (
	"context"

	gosrc "github.com/Morganamilo/go-srcinfo"
)

type Mock struct{}

func (m *Mock) Update(ctx context.Context, pkgName string, sources []gosrc.ArchString) {
}

func (m *Mock) Save() error {
	return nil
}

func (m *Mock) RemovePackage(pkgs []string) {
}

func (m *Mock) Load() error {
	return nil
}
