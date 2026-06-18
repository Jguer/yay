//go:build !integration

package main

import (
	"testing"

	"github.com/Jguer/aur"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/dep"
)

func TestInfoToInstallInfo(t *testing.T) {
	info := infoToInstallInfo([]aur.Pkg{
		{Name: "foo", PackageBase: "foo-base"},
		{Name: "bar", PackageBase: "bar-base"},
	})

	require.Len(t, info, 1)
	require.Len(t, info[0], 2)
	require.Equal(t, &dep.InstallInfo{AURBase: "foo-base", Source: dep.AUR}, info[0]["foo"])
	require.Equal(t, &dep.InstallInfo{AURBase: "bar-base", Source: dep.AUR}, info[0]["bar"])
}
