//go:build !integration

package dep

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDepSplitDep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantName   string
		wantMod    string
		wantDepVer string
	}{
		{name: "plain", input: "base", wantName: "base"},
		{name: "greater", input: "base>=1.0", wantName: "base", wantMod: ">=", wantDepVer: "1.0"},
		{name: "less", input: "base<2.0", wantName: "base", wantMod: "<", wantDepVer: "2.0"},
		{name: "equal", input: "base=1", wantName: "base", wantMod: "=", wantDepVer: "1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			name, mod, depVer := splitDep(tc.input)
			require.Equal(t, tc.wantName, name)
			require.Equal(t, tc.wantMod, mod)
			require.Equal(t, tc.wantDepVer, depVer)
		})
	}
}

// referenceSplitDep is the original FieldsFunc-based implementation, kept here
// as an oracle to prove the optimized splitDep is byte-for-byte equivalent.
func referenceSplitDep(dep string) (pkg, mod, ver string) {
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

func TestSplitDepEquivalence(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"", "base", "base=1", "base>=1.0", "base<=2.0", "base>1", "base<2",
		"base=1.2.3-4", "linux>=1.5", "a=b=c", "a>b<c=d",
		">=1.0", "<2.0", "=", "<", ">", "<=", ">=", "===",
		"foo>", "foo<", "foo=", "foo>=", "name-with-dashes>=1:2.3-4",
		"py3>=3.10", "lib32-glibc", "ünïcödé>=1.0", "a>=>=b", "x====y",
		"pkg>=1.0<=2.0", "  spaced  ", "a>b>c>d>e", "==a==",
	}

	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			wantName, wantMod, wantVer := referenceSplitDep(in)
			gotName, gotMod, gotVer := splitDep(in)
			require.Equal(t, wantName, gotName, "name for %q", in)
			require.Equal(t, wantMod, gotMod, "mod for %q", in)
			require.Equal(t, wantVer, gotVer, "ver for %q", in)
		})
	}
}

func TestSplitDepNoAlloc(t *testing.T) {
	// The common dependency shapes must parse without heap allocations.
	for _, in := range []string{"base", "base=1", "base>=1.0", "base<=2.0", "base>1"} {
		allocs := testing.AllocsPerRun(100, func() {
			name, mod, ver := splitDep(in)
			_, _, _ = name, mod, ver
		})
		require.Zerof(t, allocs, "splitDep(%q) allocated %v times", in, allocs)
	}
}
