//go:build !integration

package settings

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandleOptionMinAge(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("test")

	require.True(t, cfg.handleOption("minage", "14"))
	require.Equal(t, 14, cfg.MinAge)

	require.True(t, cfg.handleOption("minage", "0"))
	require.Equal(t, 0, cfg.MinAge)

	cfg.MinAge = 7
	require.True(t, cfg.handleOption("minage", "not-a-number"))
	require.Equal(t, 7, cfg.MinAge)

	require.True(t, cfg.handleOption("minage", "-1"))
	require.Equal(t, 7, cfg.MinAge)
}
