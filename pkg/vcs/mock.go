package vcs

import (
	"context"

	"github.com/Jguer/go-alpm/v2"
	gosrc "github.com/Morganamilo/go-srcinfo"
)

type Mock struct {
	OriginsByPackage map[string]OriginInfoByURL
	ToUpgradeReturn  []string
}

func (m *Mock) ToUpgrade(ctx context.Context, pkgName string) bool {
	for _, pkg := range m.ToUpgradeReturn {
		if pkg == pkgName {
			return true
		}
	}
	return false
}

func (m *Mock) Update(ctx context.Context, pkgName string, sources []gosrc.ArchString) {
}

func (m *Mock) Save() error {
	return nil
}

func (m *Mock) RemovePackages(pkgs []string) {
}

func (m *Mock) Load() error {
	return nil
}

func (m *Mock) CleanOrphans(pkgs map[string]alpm.IPackage) {
}
