//go:build !integration

package query

import (
	"testing"

	"github.com/Jguer/aur"
	"github.com/stretchr/testify/assert"
)

func TestMatchesSearch(t *testing.T) {
	t.Parallel()

	pkg := &aur.Pkg{
		Name:        "Linux",
		Description: "The Linux kernel and modules",
	}

	tests := []struct {
		name  string
		terms []string
		want  bool
	}{
		{
			name:  "all terms match name or description",
			terms: []string{"linux", "MODULES"},
			want:  true,
		},
		{
			name:  "missing term rejects package",
			terms: []string{"linux", "wayland"},
			want:  false,
		},
		{
			name:  "symbol term does not bypass text matching",
			terms: []string{"wayland", "✓"},
			want:  false,
		},
		{
			name:  "symbol term is ignored alongside matching text",
			terms: []string{"kernel", "✓"},
			want:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, matchesSearch(pkg, test.terms))
		})
	}
}
