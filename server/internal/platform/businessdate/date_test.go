package businessdate

import (
	"testing"
	"time"
)

func TestTodayUsesShanghaiCalendarDay(t *testing.T) {
	got := TodayString(time.Date(2026, time.September, 1, 16, 30, 0, 0, time.UTC))
	if got != "2026-09-02" {
		t.Fatalf("TodayString() = %q, want 2026-09-02", got)
	}
}

func TestTodayBeforeShanghaiMidnightKeepsPreviousDate(t *testing.T) {
	got := TodayString(time.Date(2026, time.September, 1, 15, 59, 59, 0, time.UTC))
	if got != "2026-09-01" {
		t.Fatalf("TodayString() = %q, want 2026-09-01", got)
	}
}
