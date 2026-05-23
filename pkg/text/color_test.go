//go:build !integration
// +build !integration

package text

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestColorHash is intentionally not parallel because it mutates the
// package-level UseColor variable. No other parallel test in this
// package reads UseColor, so sequential execution is sufficient.
func TestColorHash(t *testing.T) {
	original := UseColor
	defer func() { UseColor = original }()

	UseColor = true
	require.Equal(t, ColorHash("core"), ColorHash("core"))
	require.NotEqual(t, ColorHash("core"), ColorHash("extra"))

	UseColor = false
	require.Equal(t, "core", ColorHash("core"))
}
