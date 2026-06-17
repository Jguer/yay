//go:build !integration

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

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Duration
		want string
	}{
		{name: "sub-minute discarded", in: 59*time.Second + 999*time.Millisecond, want: "0m"},
		{name: "pure minutes", in: 45 * time.Minute, want: "45m"},
		{name: "does not round up to next day", in: 23*time.Hour + 59*time.Minute + 31*time.Second, want: "23h59m"},
		{name: "exact day", in: 24 * time.Hour, want: "1d"},
		{name: "day and hour", in: 6*24*time.Hour + 23*time.Hour + 59*time.Minute, want: "6d23h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, FormatDuration(tt.in))
		})
	}
}
