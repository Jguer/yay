//go:build !integration

package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v13/pkg/dep"
	settingslua "github.com/Jguer/yay/v13/pkg/settings/lua"
)

func TestPostInstallEvent(t *testing.T) {
	t.Parallel()

	base := "aur-base"
	targets := []map[string]*dep.InstallInfo{
		{
			// Layer 0: two AUR packages.
			"pkgA": {
				Source:       dep.AUR,
				Reason:       dep.Explicit,
				Version:      "2.0-1",
				LocalVersion: "1.0-1",
				AURBase:      base,
			},
			"pkgB": {
				Source:       dep.AUR,
				Reason:       dep.Dep,
				Version:      "1.1-1",
				LocalVersion: "",
				AURBase:      base,
			},
		},
		{
			// Layer 1: one Sync package and a duplicate of pkgA (rollup case).
			"pkgC": {
				Source:  dep.Sync,
				Reason:  dep.MakeDep,
				Version: "3.0-1",
			},
			// pkgA appears in layer 1 too; layer merge last-wins → this version.
			"pkgA": {
				Source:       dep.AUR,
				Reason:       dep.Explicit,
				Version:      "2.0-2",
				LocalVersion: "1.0-1",
				AURBase:      base,
			},
		},
	}

	event := postInstallEvent(targets)

	// Must be sorted by name.
	want := &settingslua.PostInstallEvent{
		Packages: []settingslua.PostInstallPackage{
			{
				Name:         "pkgA",
				Version:      "2.0-2",
				LocalVersion: "1.0-1",
				Source:       "aur",
				Reason:       "explicit",
			},
			{
				Name:         "pkgB",
				Version:      "1.1-1",
				LocalVersion: "",
				Source:       "aur",
				Reason:       "dependency",
			},
			{
				Name:    "pkgC",
				Version: "3.0-1",
				Source:  "sync",
				Reason:  "make_dependency",
			},
		},
	}

	assert.Equal(t, want, event)
}

func TestPostInstallEventSourceAndReasonMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		source     dep.Source
		reason     dep.Reason
		wantSource string
		wantReason string
	}{
		{"aur explicit", dep.AUR, dep.Explicit, "aur", "explicit"},
		{"sync dep", dep.Sync, dep.Dep, "sync", "dependency"},
		{"local makedep", dep.Local, dep.MakeDep, "local", "make_dependency"},
		{"srcinfo checkdep", dep.SrcInfo, dep.CheckDep, "srcinfo", "check_dependency"},
		{"missing unknown", dep.Missing, dep.Reason(99), "missing", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantSource, luaSource(tt.source))
			assert.Equal(t, tt.wantReason, luaReason(tt.reason))
		})
	}
}
