package main

import (
	"testing"
	"time"
)

func TestResourceSyncScheduleStateWeekly(t *testing.T) {
	schedule := resourceSyncSchedule{
		Frequency: "weekly", Weekday: 1, RunTime: "02:00", Timezone: "Asia/Shanghai",
	}
	now := time.Date(2026, time.September, 21, 3, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	key, due, next, err := resourceSyncScheduleState(schedule, now)
	if err != nil {
		t.Fatal(err)
	}
	if key != "2026-W39" || !due {
		t.Fatalf("expected due 2026-W39 schedule, got key=%q due=%v", key, due)
	}
	if got := next.In(now.Location()).Format("2006-01-02 15:04"); got != "2026-09-28 02:00" {
		t.Fatalf("unexpected next run: %s", got)
	}
}

func TestResourceSyncScheduleStateBeforeDailyRun(t *testing.T) {
	schedule := resourceSyncSchedule{
		Frequency: "daily", Weekday: 1, RunTime: "09:30", Timezone: "Asia/Shanghai",
	}
	now := time.Date(2026, time.September, 21, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	key, due, next, err := resourceSyncScheduleState(schedule, now)
	if err != nil {
		t.Fatal(err)
	}
	if key != "2026-09-21" || due {
		t.Fatalf("expected pending daily schedule, got key=%q due=%v", key, due)
	}
	if got := next.In(now.Location()).Format("15:04"); got != "09:30" {
		t.Fatalf("unexpected next run time: %s", got)
	}
}
