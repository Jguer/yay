//go:build !integration

package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/settings/parser"
)

func TestMinAgeApplies_onlySysupgrade(t *testing.T) {
	t.Parallel()

	install := parser.MakeArguments()
	_ = install.AddArg("S")
	install.AddTarget("foo")
	require.False(t, minAgeApplies(install))

	sysupgrade := parser.MakeArguments()
	_ = sysupgrade.AddArg("S")
	_ = sysupgrade.AddArg("u")
	require.True(t, minAgeApplies(sysupgrade))

	fullUpgrade := parser.MakeArguments()
	_ = fullUpgrade.AddArg("S")
	_ = fullUpgrade.AddArg("y")
	_ = fullUpgrade.AddArg("u")
	require.True(t, minAgeApplies(fullUpgrade))
}
