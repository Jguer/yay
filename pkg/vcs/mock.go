package vcs

import (
	"context"

	gosrc "github.com/Morganamilo/go-srcinfo"
)

type Mock struct {
	OriginsByPackage map[string]OriginInfoByURL
	ToUpgradeReturn  []string
}

func (m *Mock) ToUpgrade(ctx context.Context) []string {
	return m.ToUpgradeReturn
}

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
