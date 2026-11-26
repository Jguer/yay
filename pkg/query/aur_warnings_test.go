//go:build !integration

package query

import (
	"testing"

	"github.com/Jguer/aur"
	alpm "github.com/Jguer/dyalpm"
	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v13/pkg/db/mock"
)

func TestCalculateMissing(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		installed   []*mock.Package
		aurData     map[string]*aur.Pkg
		wantMissing []string
	}{
		{
			name:      "debug package of a split package",
			installed: []*mock.Package{{PName: "immich-cli"}, {PName: "immich-debug"}},
			aurData:   map[string]*aur.Pkg{"immich-cli": {Name: "immich-cli", PackageBase: "immich"}},
		},
		{
			name:      "debug package matching a package name",
			installed: []*mock.Package{{PName: "foo"}, {PName: "foo-debug"}},
			aurData:   map[string]*aur.Pkg{"foo": {Name: "foo", PackageBase: "foo"}},
		},
		{
			name:        "debug package without its pkgbase",
			installed:   []*mock.Package{{PName: "foo"}, {PName: "bar-debug"}},
			aurData:     map[string]*aur.Pkg{"foo": {Name: "foo", PackageBase: "foo"}},
			wantMissing: []string{"bar-debug"},
		},
		{
			name:        "non-debug package named after a pkgbase",
			installed:   []*mock.Package{{PName: "immich"}, {PName: "immich-cli"}},
			aurData:     map[string]*aur.Pkg{"immich-cli": {Name: "immich-cli", PackageBase: "immich"}},
			wantMissing: []string{"immich"},
		},
		{
			name:      "ignored package",
			installed: []*mock.Package{{PName: "baz", PShouldIgnore: true}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			names := make([]string, 0, len(tc.installed))
			remote := make(map[string]alpm.Package, len(tc.installed))
			for _, pkg := range tc.installed {
				names = append(names, pkg.PName)
				remote[pkg.PName] = pkg
			}

			warnings := &AURWarnings{}
			warnings.CalculateMissing(names, remote, tc.aurData)

			assert.Equal(t, tc.wantMissing, warnings.Missing)
		})
	}
}
