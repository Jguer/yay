package text

import "time"

// Formats a unix timestamp to ISO 8601 date (yyyy-mm-dd)
func FormatTime(i int) string {
	t := time.Unix(int64(i), 0)
	return t.Format("2006-01-02")
}

// Formats a unix timestamp to ISO 8601 date (Mon 02 Jan 2006 03:04:05 PM MST)
func FormatTimeQuery(i int) string {
	t := time.Unix(int64(i), 0)
	return t.Format("Mon 02 Jan 2006 03:04:05 PM MST")
}
