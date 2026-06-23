package dep

import (
	"strings"

	"github.com/Jguer/yay/v13/pkg/db"
	aur "github.com/Jguer/yay/v13/pkg/query"
)

// splitDep parses a dependency string into its package name, comparison
// operator and version. It is an allocation-free equivalent of the previous
// strings.FieldsFunc implementation:
//
//   - fields are maximal runs of non-operator bytes (operators: '<' '>' '=');
//   - mod is every operator byte concatenated, in order;
//   - 0 fields -> "","",""; 1 field -> field0,"",""; else field0, mod, field1.
//
// splitDep sits on the hottest path of dependency resolution and is called many
// times per package, so it scans once and reuses slices of dep for the name and
// version, and interns the operator string so the common path never allocates.
func splitDep(dep string) (pkg, mod, ver string) {
	var (
		f0s, f0e = -1, -1 // first field bounds
		f1s, f1e = -1, -1 // second field bounds
		fields   int
		modBuf   [8]byte // operator bytes in order; real deps use at most 2
		modLen   int
	)

	for i := 0; i < len(dep); {
		if c := dep[i]; c == '<' || c == '>' || c == '=' {
			if modLen < len(modBuf) {
				modBuf[modLen] = c
			}
			modLen++
			i++

			continue
		}

		start := i
		for i < len(dep) && dep[i] != '<' && dep[i] != '>' && dep[i] != '=' {
			i++
		}

		fields++
		switch fields {
		case 1:
			f0s, f0e = start, i
		case 2:
			f1s, f1e = start, i
		}
	}

	switch fields {
	case 0:
		return "", "", ""
	case 1:
		return dep[f0s:f0e], "", ""
	}

	n := min(modLen, len(modBuf))

	return dep[f0s:f0e], internMod(modBuf[:n], modLen, dep), dep[f1s:f1e]
}

// internMod returns the operator string for the collected operator bytes
// without allocating for the combinations that occur in practice. Switching on
// string(b) is special-cased by the compiler to avoid a heap allocation; only
// the unreachable overflow path (more operator bytes than modBuf can hold)
// falls back to rebuilding the exact string from dep.
func internMod(b []byte, modLen int, dep string) string {
	if modLen > len(b) {
		// Extremely rare malformed input: reconstruct every operator byte to
		// stay byte-for-byte compatible with the previous implementation.
		var sb strings.Builder
		for i := 0; i < len(dep); i++ {
			if c := dep[i]; c == '<' || c == '>' || c == '=' {
				sb.WriteByte(c)
			}
		}

		return sb.String()
	}

	switch string(b) {
	case "":
		return ""
	case "=":
		return "="
	case "<":
		return "<"
	case ">":
		return ">"
	case "<=":
		return "<="
	case ">=":
		return ">="
	case "=<":
		return "=<"
	case "=>":
		return "=>"
	default:
		return string(b)
	}
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
