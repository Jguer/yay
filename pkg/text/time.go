package text

import (
	"strconv"
	"time"
)

const (
	veryRecentPkgThreshold = 24 * time.Hour     // < 1 day → red
	recentPkgThreshold     = 7 * 24 * time.Hour // < 7 days → yellow
)

// FormatDuration formats a duration as at most two significant units ("2d", "3h", "38d8h", "1h30m", "45m").
// Sub-minute precision is discarded.
func FormatDuration(d time.Duration) string {
	d = d.Truncate(time.Minute)

	days := int(d / (24 * time.Hour))
	d -= time.Duration(days) * 24 * time.Hour
	hours := int(d / time.Hour)
	d -= time.Duration(hours) * time.Hour
	minutes := int(d / time.Minute)

	switch {
	case days > 0 && hours > 0:
		return strconv.Itoa(days) + "d" + strconv.Itoa(hours) + "h"
	case days > 0:
		return strconv.Itoa(days) + "d"
	case hours > 0 && minutes > 0:
		return strconv.Itoa(hours) + "h" + strconv.Itoa(minutes) + "m"
	case hours > 0:
		return strconv.Itoa(hours) + "h"
	default:
		return strconv.Itoa(minutes) + "m"
	}
}

// NowFunc returns the current time. Overridable in tests.
var NowFunc = time.Now

// FormatAgeTag returns a colored "[Xd]" age badge for an AUR package's LastModified timestamp.
// Returns "" when no AUR LastModified timestamp is available.
func FormatAgeTag(lastModified int64) string {
	if lastModified == 0 {
		return ""
	}

	age := NowFunc().Sub(time.Unix(lastModified, 0))
	tag := "[" + FormatDuration(age) + "]"

	switch {
	case age < veryRecentPkgThreshold:
		return Bold(Red(tag))
	case age < recentPkgThreshold:
		return Bold(Yellow(tag))
	default:
		return Cyan(tag)
	}
}

// Formats a unix timestamp to ISO 8601 date (yyyy-mm-dd).
func FormatTime(i int) string {
	t := time.Unix(int64(i), 0)
	return t.Format("2006-01-02")
}

// Formats a unix timestamp to ISO 8601 date (Mon 02 Jan 2006 03:04:05 PM MST).
func FormatTimeQuery(i int) string {
	t := time.Unix(int64(i), 0)
	return t.Format("Mon 02 Jan 2006 03:04:05 PM MST")
}
