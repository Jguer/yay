//go:build !integration

package settings

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// GIVEN a default config
// WHEN handleOption processes the --noage flag
// THEN NoAge is set to true
func TestHandleOptionNoAge(t *testing.T) {
	cfg := DefaultConfig("test")
	require.False(t, cfg.NoAge)

	require.True(t, cfg.handleOption("noage", ""))
	require.True(t, cfg.NoAge)
}

// GIVEN a default config
// WHEN handleOption receives the value of another flag
// THEN NoAge keeps its default value
func TestHandleOptionNoAgeDefault(t *testing.T) {
	cfg := DefaultConfig("test")

	require.True(t, cfg.handleOption("separatesources", ""))
	require.False(t, cfg.NoAge)
}
