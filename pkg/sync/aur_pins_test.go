//go:build !integration

package sync

import (
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v13/pkg/dep"
)

func TestInstalledAURTargets(t *testing.T) {
	t.Parallel()

	targets := []map[string]*dep.InstallInfo{
		{"repo-pkg": {Source: dep.Sync}, "ok": {Source: dep.AUR}},
		{"broken": {Source: dep.AUR}, "dep": {Source: dep.AUR}},
	}

	got := installedAURTargets(targets, map[string]error{"broken": errors.New("boom")})
	slices.Sort(got)
	assert.Equal(t, []string{"dep", "ok"}, got)
}
