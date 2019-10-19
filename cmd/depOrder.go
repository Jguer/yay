package main

import (
	alpm "github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v9/pkg/stringset"
	rpc "github.com/mikkeloscar/aur"
)

// Base is an AUR base package
type Base []*rpc.Pkg

// Pkgbase returns the first base package.
func (b Base) Pkgbase() string {
	return b[0].PackageBase
}

// Version returns the first base package version.
func (b Base) Version() string {
	return b[0].Version
}

// URLPath returns the first base package URL.
func (b Base) URLPath() string {
	return b[0].URLPath
}

type depOrder struct {
	Aur     []Base
	Repo    []*alpm.Package
	Runtime stringset.StringSet
}

func makeDepOrder() *depOrder {
	return &depOrder{
		make([]Base, 0),
		make([]*alpm.Package, 0),
		make(stringset.StringSet),
	}
}

func getDepOrder(dp *depPool) *depOrder {
	do := makeDepOrder()

	for _, target := range dp.Targets {
		dep := target.DepString()
		aurPkg := dp.Aur[dep]
		if aurPkg != nil && pkgSatisfies(aurPkg.Name, aurPkg.Version, dep) {
			do.orderPkgAur(aurPkg, dp, true)
		}

		aurPkg = dp.findSatisfierAur(dep)
		if aurPkg != nil {
			do.orderPkgAur(aurPkg, dp, true)
		}

		repoPkg := dp.findSatisfierRepo(dep)
		if repoPkg != nil {
			do.orderPkgRepo(repoPkg, dp, true)
		}
	}

	return do
}

func (do *depOrder) orderPkgAur(pkg *rpc.Pkg, dp *depPool, runtime bool) {
	if runtime {
		do.Runtime.Set(pkg.Name)
	}
	delete(dp.Aur, pkg.Name)

	for i, deps := range [3][]string{pkg.Depends, pkg.MakeDepends, pkg.CheckDepends} {
		for _, dep := range deps {
			aurPkg := dp.findSatisfierAur(dep)
			if aurPkg != nil {
				do.orderPkgAur(aurPkg, dp, runtime && i == 0)
			}

			repoPkg := dp.findSatisfierRepo(dep)
			if repoPkg != nil {
				do.orderPkgRepo(repoPkg, dp, runtime && i == 0)
			}
		}
	}

	for i, base := range do.Aur {
		if base.Pkgbase() == pkg.PackageBase {
			do.Aur[i] = append(base, pkg)
			return
		}
	}

	do.Aur = append(do.Aur, Base{pkg})
}

func (do *depOrder) orderPkgRepo(pkg *alpm.Package, dp *depPool, runtime bool) {
	if runtime {
		do.Runtime.Set(pkg.Name())
	}
	delete(dp.Repo, pkg.Name())

	_ = pkg.Depends().ForEach(func(dep alpm.Depend) (err error) {
		repoPkg := dp.findSatisfierRepo(dep.String())
		if repoPkg != nil {
			do.orderPkgRepo(repoPkg, dp, runtime)
		}

		return nil
	})

	do.Repo = append(do.Repo, pkg)
}

func (do *depOrder) HasMake() bool {
	lenAur := 0
	for _, base := range do.Aur {
		lenAur += len(base)
	}

	return len(do.Runtime) != lenAur+len(do.Repo)
}

func (do *depOrder) getMake() []string {
	makeOnly := []string{}

	for _, base := range do.Aur {
		for _, pkg := range base {
			if !do.Runtime.Get(pkg.Name) {
				makeOnly = append(makeOnly, pkg.Name)
			}
		}
	}

	for _, pkg := range do.Repo {
		if !do.Runtime.Get(pkg.Name()) {
			makeOnly = append(makeOnly, pkg.Name())
		}
	}

	return makeOnly
}
