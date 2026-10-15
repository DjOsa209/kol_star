package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Each entry attests to a full year, not just a list of currently known holidays.
// The next year's dates must be verified against its official annual notice.
type workCalendarYear struct {
	Year     int      `json:"year" mapstructure:"year"`
	Source   string   `json:"source" mapstructure:"source"`
	Holidays []string `json:"holidays" mapstructure:"holidays"`
	Workdays []string `json:"workdays" mapstructure:"workdays"`
}
type workCalendar struct {
	years      map[int]bool
	exceptions map[string]bool
}

//go:embed data/cn_work_calendar.json
var cnWorkCalendarJSON []byte

var ipTimeZone = time.FixedZone("Asia/Shanghai", 8*60*60)

func ipLocalDate(t time.Time) time.Time {
	t = t.In(ipTimeZone)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, ipTimeZone)
}

func newWorkCalendar(overrides []workCalendarYear) (workCalendar, error) {
	var years []workCalendarYear
	if err := json.Unmarshal(cnWorkCalendarJSON, &years); err != nil {
		return workCalendar{}, err
	}
	byYear := map[int]workCalendarYear{}
	for _, y := range years {
		byYear[y.Year] = y
	}
	seen := map[int]bool{}
	for _, y := range overrides {
		if seen[y.Year] {
			return workCalendar{}, fmt.Errorf("工作日日历年份重复：%d", y.Year)
		}
		seen[y.Year] = true
		byYear[y.Year] = y
	}
	c := workCalendar{years: map[int]bool{}, exceptions: map[string]bool{}}
	for _, y := range byYear {
		source, err := url.Parse(y.Source)
		if y.Year < 2000 || y.Year > 2100 || err != nil || source.Scheme != "https" || source.Host == "" || len(y.Holidays) == 0 {
			return c, fmt.Errorf("%d 年工作日日历需要完整假期与 HTTPS 核验来源", y.Year)
		}
		parse := func(s string) (time.Time, error) {
			d, err := time.ParseInLocation("2006-01-02", s, ipTimeZone)
			if err != nil || d.Year() != y.Year {
				return time.Time{}, fmt.Errorf("%d 年工作日日历日期无效：%s", y.Year, s)
			}
			return d, nil
		}
		for _, period := range y.Holidays {
			dates := strings.Split(period, "/")
			if len(dates) != 2 {
				return c, fmt.Errorf("假期格式须为 YYYY-MM-DD/YYYY-MM-DD：%s", period)
			}
			start, err := parse(dates[0])
			if err != nil {
				return c, err
			}
			end, err := parse(dates[1])
			if err != nil {
				return c, err
			}
			if end.Before(start) {
				return c, fmt.Errorf("假期结束早于开始：%s", period)
			}
			for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
				key := d.Format("2006-01-02")
				if _, exists := c.exceptions[key]; exists {
					return c, fmt.Errorf("日历日期重复：%s", key)
				}
				c.exceptions[key] = false
			}
		}
		for _, date := range y.Workdays {
			if _, err := parse(date); err != nil {
				return c, err
			}
			if _, exists := c.exceptions[date]; exists {
				return c, fmt.Errorf("日历日期重复或放假与补班冲突：%s", date)
			}
			c.exceptions[date] = true
		}
		c.years[y.Year] = true
	}
	return c, nil
}

func (c workCalendar) isWorkday(d time.Time) (bool, error) {
	d = d.In(ipTimeZone)
	if !c.years[d.Year()] {
		return false, fmt.Errorf("%d 年法定节假日及调休日历尚未配置，截止时间待核定", d.Year())
	}
	if work, ok := c.exceptions[d.Format("2006-01-02")]; ok {
		return work, nil
	}
	return d.Weekday() != time.Saturday && d.Weekday() != time.Sunday, nil
}

// Excludes the start date; the due date includes its entire Shanghai civil day.
func (c workCalendar) deadline(start time.Time, days int) (time.Time, error) {
	d := ipLocalDate(start)
	for counted := 0; counted < days; {
		d = d.AddDate(0, 0, 1)
		work, err := c.isWorkday(d)
		if err != nil {
			return time.Time{}, err
		}
		if work {
			counted++
		}
	}
	return d.AddDate(0, 0, 1).Add(-time.Second), nil
}

func (c workCalendar) workdaysBetween(start, end time.Time) (int, error) {
	n := 0
	end = ipLocalDate(end)
	for d := ipLocalDate(start).AddDate(0, 0, 1); !d.After(end); d = d.AddDate(0, 0, 1) {
		work, err := c.isWorkday(d)
		if err != nil {
			return 0, err
		}
		if work {
			n++
		}
	}
	return n, nil
}

type ipStageDeadline struct {
	Label             string `json:"label"`
	WorkingDays       int    `json:"workingDays"`
	StartedAt         string `json:"startedAt,omitempty"`
	DueAt             string `json:"dueAt,omitempty"`
	CompletedAt       string `json:"completedAt,omitempty"`
	State             string `json:"state"`
	Late              bool   `json:"late"`
	RemainingWorkdays int    `json:"remainingWorkdays"`
	OverdueWorkdays   int    `json:"overdueWorkdays"`
	Warning           string `json:"warning,omitempty"`
}

func ipStageTime(v any) time.Time {
	s := fmt.Sprint(v)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if t, err := time.ParseInLocation(layout, s, ipTimeZone); err == nil {
			return t
		}
	}
	return time.Time{}
}

func (c workCalendar) stageDeadline(label string, days int, start, completed time.Time, active, finished bool, now time.Time) ipStageDeadline {
	d := ipStageDeadline{Label: label, WorkingDays: days, State: "waiting"}
	if !completed.IsZero() {
		d.CompletedAt = completed.In(ipTimeZone).Format(time.RFC3339)
	}
	if start.IsZero() {
		if active || finished {
			d.State = "unknown_start"
			if finished {
				d.State = "completed"
			}
			d.Warning = "历史需求缺少阶段开始时间，无法核定截止时间"
		}
		return d
	}
	d.StartedAt = start.In(ipTimeZone).Format(time.RFC3339)
	due, err := c.deadline(start, days)
	if err != nil {
		d.State = "calendar_pending"
		d.Warning = err.Error()
		return d
	}
	d.DueAt = due.Format(time.RFC3339)
	reference := now
	if finished {
		d.State = "completed"
		if completed.IsZero() {
			d.Warning = "历史需求缺少完成时间，无法判断是否按时"
			return d
		}
		reference = completed
	} else {
		d.State = "pending"
	}
	d.Late = !reference.Before(due.Add(time.Second))
	if d.Late {
		if !finished {
			d.State = "overdue"
		}
		d.OverdueWorkdays, err = c.workdaysBetween(due, reference)
	} else if !finished {
		if ipLocalDate(reference).Equal(ipLocalDate(due)) {
			d.State = "due_today"
		}
		d.RemainingWorkdays, err = c.workdaysBetween(reference, due)
	}
	if err != nil {
		d.Warning = err.Error()
	}
	return d
}

func (a *app) enrichIPRequestDeadlines(row map[string]any, now time.Time) {
	c, err := newWorkCalendar(a.Config().WorkCalendar)
	if err != nil {
		return
	} // Config decoding rejects invalid overrides.
	status := fmt.Sprint(row["status"])
	ipDone := status == "ip_reviewed" || status == "marketing_reviewed"
	row["deadlines"] = map[string]ipStageDeadline{
		"ip":        c.stageDeadline("IP意见", 10, ipStageTime(row["submittedAt"]), ipStageTime(row["ipReviewedAt"]), status == "submitted", ipDone, now),
		"marketing": c.stageDeadline("营销意见", 5, ipStageTime(row["ipReviewedAt"]), ipStageTime(row["marketingReviewedAt"]), status == "ip_reviewed", status == "marketing_reviewed", now),
	}
}
