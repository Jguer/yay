package dep

import "strings"

// splitDep splits a dependency string into name, operator and version.
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

// internMod returns the operator string without allocating for common cases.
func internMod(b []byte, modLen int, dep string) string {
	if modLen > len(b) {
		var sb strings.Builder
		for i := 0; i < len(dep); i++ {
			if c := dep[i]; c == '<' || c == '>' || c == '=' {
				sb.WriteByte(c)
			}
		}

		return sb.String()
	}

	switch string(b) {
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
