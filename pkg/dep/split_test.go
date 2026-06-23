//go:build !integration

package dep

import (
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
		{name: "empty", input: "", wantName: "", wantMod: "", wantDepVer: ""},
		{name: "plain", input: "base", wantName: "base"},
		{name: "greater", input: "base>=1.0", wantName: "base", wantMod: ">=", wantDepVer: "1.0"},
		{name: "less", input: "base<2.0", wantName: "base", wantMod: "<", wantDepVer: "2.0"},
		{name: "equal", input: "base=1", wantName: "base", wantMod: "=", wantDepVer: "1"},
		{name: "equal-less", input: "base=<1.0", wantName: "base", wantMod: "=<", wantDepVer: "1.0"},
		{name: "equal-greater", input: "base=>1.0", wantName: "base", wantMod: "=>", wantDepVer: "1.0"},
		{name: "overflow", input: "a>=>=>=>=>=>b", wantName: "a", wantMod: ">=>=>=>=>=>", wantDepVer: "b"},
		{name: "default-operator", input: "a<>b", wantName: "a", wantMod: "<>", wantDepVer: "b"},
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
