//go:build !integration
// +build !integration

package text

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestColorHash(t *testing.T) {
	t.Parallel()

	original := UseColor
	defer func() { UseColor = original }()

	UseColor = true
	require.Equal(t, ColorHash("core"), ColorHash("core"))
	require.NotEqual(t, ColorHash("core"), ColorHash("extra"))

	UseColor = false
	require.Equal(t, "core", ColorHash("core"))
}
