//go:build !integration
// +build !integration

package text

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitDBFromName(t *testing.T) {
	t.Parallel()

	db, pkg := SplitDBFromName("core/pkg")
	require.Equal(t, "core", db)
	require.Equal(t, "pkg", pkg)

	db, pkg = SplitDBFromName("pkg")
	require.Equal(t, "", db)
	require.Equal(t, "pkg", pkg)
}

func TestCreateOSC8Link(t *testing.T) {
	t.Parallel()

	got := CreateOSC8Link("https://example.com", "text")
	require.Equal(t, "\033]8;;https://example.com\033\\text\033]8;;\033\\", got)
}

func TestHumanReadable(t *testing.T) {
	t.Parallel()

	require.Equal(t, "10.0 B", Human(10))
	require.Equal(t, "1.0 KiB", Human(1024))
	require.Equal(t, "1.5 KiB", Human(1536))
}
