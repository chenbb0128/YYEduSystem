// Package businessdate contains date helpers for the organisation's local
// business day. Persisted timestamps remain UTC; only the calendar date used
// by daily workflows is resolved in Asia/Shanghai.
package businessdate

import "time"

const Layout = "2006-01-02"

var location = loadLocation()

func loadLocation() *time.Location {
	value, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return value
}

// Today returns the current Shanghai calendar day represented as a UTC
// midnight. This matches the way date-only values are stored and compared in
// the existing modules while avoiding a server-timezone dependency.
func Today(now time.Time) time.Time {
	local := now.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func TodayString(now time.Time) string {
	return Today(now).Format(Layout)
}
