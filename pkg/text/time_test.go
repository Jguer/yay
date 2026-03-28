//go:build !integration
// +build !integration

package text

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFormatTime(t *testing.T) {
	t.Parallel()

	require.Equal(t, "1970-01-01", FormatTime(0))
	now := time.Unix(0, 0)
	require.Equal(t, now.Format("2006-01-02"), FormatTime(0))
}

func TestFormatTimeQuery(t *testing.T) {
	t.Parallel()

	now := time.Unix(0, 0)
	require.Equal(t, now.Format("Mon 02 Jan 2006 03:04:05 PM MST"), FormatTimeQuery(0))
}
