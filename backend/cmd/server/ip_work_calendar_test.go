package main

import (
	"github.com/spf13/viper"
	"testing"
	"time"
)

func TestIPWorkCalendarOfficialHolidaysAndMakeupDays(t *testing.T) {
	c, err := newWorkCalendar(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		date string
		work bool
	}{
		{"2025-01-01", false}, {"2025-01-26", true}, {"2025-02-08", true}, {"2025-04-27", true}, {"2025-10-08", false}, {"2025-10-11", true},
		{"2026-01-04", true}, {"2026-02-14", true}, {"2026-02-23", false}, {"2026-02-28", true}, {"2026-05-09", true}, {"2026-06-19", false}, {"2026-09-20", true}, {"2026-09-25", false}, {"2026-10-07", false}, {"2026-10-10", true}, {"2026-10-11", false},
	} {
		got, err := c.isWorkday(ipStageTime(tt.date + " 12:00:00"))
		if err != nil || got != tt.work {
			t.Errorf("%s work=%v err=%v want=%v", tt.date, got, err, tt.work)
		}
	}
}

func TestIPDeadlineNextDayAcrossHolidayYearAndTimezone(t *testing.T) {
	c, _ := newWorkCalendar(nil)
	for _, tt := range []struct {
		start string
		days  int
		want  string
	}{
		{"2026-09-30T15:00:00+08:00", 10, "2026-10-20T23:59:59+08:00"}, // Oct 10 Saturday counts
		{"2026-02-13T23:59:00+08:00", 5, "2026-02-27T23:59:59+08:00"},
		{"2026-02-14T01:00:00+08:00", 10, "2026-03-06T23:59:59+08:00"},
		{"2025-12-30T17:00:00+08:00", 5, "2026-01-07T23:59:59+08:00"},
		{"2026-10-15T16:30:00Z", 5, "2026-10-23T23:59:59+08:00"}, // Shanghai already Oct 16
		{"2026-10-17T10:00:00+08:00", 5, "2026-10-23T23:59:59+08:00"},
	} {
		got, err := c.deadline(ipStageTime(tt.start), tt.days)
		if err != nil || got.Format(time.RFC3339) != tt.want {
			t.Errorf("%s +%d: got %s err=%v want %s", tt.start, tt.days, got, err, tt.want)
		}
	}
	if _, err := c.deadline(ipStageTime("2026-12-30 12:00:00"), 5); err == nil {
		t.Fatal("unconfigured 2027 silently treated as ordinary weekdays")
	}
}

func TestIPDeadlineStatesAndExactEndOfDay(t *testing.T) {
	c, _ := newWorkCalendar(nil)
	start := ipStageTime("2026-10-15 15:00:00")
	for _, tt := range []struct {
		now, done, state   string
		late               bool
		remaining, overdue int
	}{
		{"2026-10-16 12:00:00", "", "pending", false, 4, 0},
		{"2026-10-22 23:59:59", "", "due_today", false, 0, 0},
		{"2026-10-23 00:00:00", "", "overdue", true, 0, 1},
		{"2026-10-24 12:00:00", "2026-10-22 23:59:59", "completed", false, 0, 0},
		{"2026-10-24 12:00:00", "2026-10-24 10:00:00", "completed", true, 0, 1},
	} {
		d := c.stageDeadline("营销意见", 5, start, ipStageTime(tt.done), true, tt.done != "", ipStageTime(tt.now))
		if d.State != tt.state || d.Late != tt.late || d.RemainingWorkdays != tt.remaining || d.OverdueWorkdays != tt.overdue {
			t.Errorf("%+v: %+v", tt, d)
		}
	}
	legacy := c.stageDeadline("IP意见", 10, time.Time{}, time.Time{}, true, false, time.Now())
	if legacy.State != "unknown_start" || legacy.DueAt != "" {
		t.Fatalf("invented legacy deadline: %+v", legacy)
	}
	future := c.stageDeadline("IP意见", 10, ipStageTime("2026-12-30 10:00:00"), time.Time{}, true, false, time.Now())
	if future.State != "calendar_pending" || future.DueAt != "" || future.Warning == "" {
		t.Fatalf("unknown calendar: %+v", future)
	}
}

func TestIPCalendarOverrideValidationAndConfigReload(t *testing.T) {
	valid := workCalendarYear{Year: 2027, Source: "https://example.gov.cn/official-notice", Holidays: []string{"2027-01-01/2027-01-03"}, Workdays: []string{"2027-01-09"}}
	// Synthetic future-year fixture exercises the update seam, not official 2027 dates.
	c, err := newWorkCalendar([]workCalendarYear{valid})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.deadline(ipStageTime("2026-12-30 10:00:00"), 5); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*workCalendarYear){
		func(y *workCalendarYear) { y.Workdays = []string{"2027-01-01"} },
		func(y *workCalendarYear) { y.Workdays = []string{"2026-01-09"} },
		func(y *workCalendarYear) { y.Holidays = []string{"2027-02-30/2027-03-01"} },
		func(y *workCalendarYear) { y.Source = "" },
		func(y *workCalendarYear) { y.Holidays = nil },
	} {
		bad := valid
		mutate(&bad)
		if _, err := newWorkCalendar([]workCalendarYear{bad}); err == nil {
			t.Errorf("accepted invalid year: %+v", bad)
		}
	}
	v := viper.New()
	setConfigDefaults(v)
	v.Set("work_calendar", []map[string]any{{"year": 2027, "source": valid.Source, "holidays": valid.Holidays, "workdays": valid.Workdays}})
	cfg, err := decodeConfig(v)
	if err != nil || len(cfg.WorkCalendar) != 1 {
		t.Fatalf("config override not decoded: %v", err)
	}
}
