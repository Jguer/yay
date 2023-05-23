package dep

import (
	"strings"

	"github.com/Jguer/yay/v12/pkg/db"
	aur "github.com/Jguer/yay/v12/pkg/query"
)

func splitDep(dep string) (pkg, mod, ver string) {
	split := strings.FieldsFunc(dep, func(c rune) bool {
		match := c == '>' || c == '<' || c == '='

		if match {
			mod += string(c)
		}

		return match
	})

	if len(split) == 0 {
		return "", "", ""
	}

	if len(split) == 1 {
		return split[0], "", ""
	}

	return split[0], mod, split[1]
}

func pkgSatisfies(name, version, dep string) bool {
	depName, depMod, depVersion := splitDep(dep)

	if depName != name {
		return false
	}

	return verSatisfies(version, depMod, depVersion)
}

func provideSatisfies(provide, dep, pkgVersion string) bool {
	depName, depMod, depVersion := splitDep(dep)
	provideName, provideMod, provideVersion := splitDep(provide)

	if provideName != depName {
		return false
	}

	// Unversioned provides can not satisfy a versioned dep
	if provideMod == "" && depMod != "" {
		provideVersion = pkgVersion // Example package: pagure
	}

	return verSatisfies(provideVersion, depMod, depVersion)
}

func verSatisfies(ver1, mod, ver2 string) bool {
	switch mod {
	case "=":
		return db.VerCmp(ver1, ver2) == 0
	case "<":
		return db.VerCmp(ver1, ver2) < 0
	case "<=":
		return db.VerCmp(ver1, ver2) <= 0
	case ">":
		return db.VerCmp(ver1, ver2) > 0
	case ">=":
		return db.VerCmp(ver1, ver2) >= 0
	}

	return true
}

func satisfiesAur(dep string, pkg *aur.Pkg) bool {
	if pkgSatisfies(pkg.Name, pkg.Version, dep) {
		return true
	}

	for _, provide := range pkg.Provides {
		if provideSatisfies(provide, dep, pkg.Version) {
			return true
		}
	}

	return false
}
