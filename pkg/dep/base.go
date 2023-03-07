package dep

import (
	aur "github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/stringset"
)

// Base is an AUR base package.
type Base []*aur.Pkg

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

func (b Base) AnyIsInSet(set stringset.StringSet) bool {
	for _, pkg := range b {
		if set.Get(pkg.Name) {
			return true
		}
	}

	return false
}

// Packages foo and bar from a pkgbase named base would print like so:
// base (foo bar).
func (b Base) String() string {
	pkg := b[0]
	str := pkg.PackageBase

	if len(b) > 1 || pkg.PackageBase != pkg.Name {
		str2 := " ("
		for _, split := range b {
			str2 += split.Name + " "
		}

		str2 = str2[:len(str2)-1] + ")"

		str += str2
	}

	return str
}

func GetBases(pkgs []aur.Pkg) []Base {
	basesMap := make(map[string]Base)
	for i := range pkgs {
		pkg := &pkgs[i]
		basesMap[pkg.PackageBase] = append(basesMap[pkg.PackageBase], pkg)
	}

	bases := make([]Base, 0, len(basesMap))
	for _, base := range basesMap {
		bases = append(bases, base)
	}

	return bases
}
