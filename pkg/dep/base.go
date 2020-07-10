package dep

import rpc "github.com/mikkeloscar/aur"

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

// Packages foo and bar from a pkgbase named base would print like so:
// base (foo bar)
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

func GetBases(pkgs []*rpc.Pkg) []Base {
	basesMap := make(map[string]Base)
	for _, pkg := range pkgs {
		basesMap[pkg.PackageBase] = append(basesMap[pkg.PackageBase], pkg)
	}

	bases := make([]Base, 0, len(basesMap))
	for _, base := range basesMap {
		bases = append(bases, base)
	}

	return bases
}
