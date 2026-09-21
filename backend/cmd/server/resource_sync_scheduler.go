package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

type resourceSyncSchedule struct {
	Enabled       bool
	Frequency     string
	Weekday       int
	RunTime       string
	Timezone      string
	LastRunKey    string
	LastStartedAt sql.NullTime
}

func (a *app) ensureResourceSyncSchedule(ctx context.Context) error {
	_, err := a.DB().ExecContext(ctx,
		`insert ignore into biz_resource_sync_schedule
		  (id, enabled, frequency, weekday, run_time, timezone)
		 values (1, 0, 'weekly', 1, '02:00', 'Asia/Shanghai')`)
	return err
}

func (a *app) resourceSyncSchedule(ctx context.Context) (resourceSyncSchedule, error) {
	if err := a.ensureResourceSyncSchedule(ctx); err != nil {
		return resourceSyncSchedule{}, err
	}
	var schedule resourceSyncSchedule
	var enabled int
	err := a.DB().QueryRowContext(ctx,
		`select enabled, frequency, weekday, run_time, timezone, last_run_key, last_started_at
		   from biz_resource_sync_schedule where id = 1`,
	).Scan(&enabled, &schedule.Frequency, &schedule.Weekday, &schedule.RunTime,
		&schedule.Timezone, &schedule.LastRunKey, &schedule.LastStartedAt)
	schedule.Enabled = enabled == 1
	return schedule, err
}

func normalizeResourceSyncSchedule(raw map[string]any) (resourceSyncSchedule, error) {
	frequency := strings.ToLower(strings.TrimSpace(str(raw, "frequency")))
	if frequency != "daily" && frequency != "weekly" {
		return resourceSyncSchedule{}, fmt.Errorf("刷新周期仅支持 daily 或 weekly")
	}
	weekday := clampInt(intField(raw, "weekday"), 1, 7)
	runTime := strings.TrimSpace(str(raw, "runTime"))
	if _, err := time.Parse("15:04", runTime); err != nil {
		return resourceSyncSchedule{}, fmt.Errorf("执行时间格式应为 HH:mm")
	}
	timezone := strings.TrimSpace(str(raw, "timezone"))
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return resourceSyncSchedule{}, fmt.Errorf("无效时区：%s", timezone)
	}
	return resourceSyncSchedule{
		Enabled: boolInt(raw, "enabled") == 1, Frequency: frequency,
		Weekday: weekday, RunTime: runTime, Timezone: timezone,
	}, nil
}

func resourceSyncScheduleState(schedule resourceSyncSchedule, now time.Time) (string, bool, time.Time, error) {
	location, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		return "", false, time.Time{}, err
	}
	clock, err := time.Parse("15:04", schedule.RunTime)
	if err != nil {
		return "", false, time.Time{}, err
	}
	localNow := now.In(location)
	if schedule.Frequency == "daily" {
		today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), clock.Hour(), clock.Minute(), 0, 0, location)
		if localNow.Before(today) {
			return today.Format("2006-01-02"), false, today, nil
		}
		return today.Format("2006-01-02"), true, today.AddDate(0, 0, 1), nil
	}
	isoWeekday := int(localNow.Weekday())
	if isoWeekday == 0 {
		isoWeekday = 7
	}
	startOfDay := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	weekStart := startOfDay.AddDate(0, 0, -(isoWeekday - 1))
	scheduled := weekStart.AddDate(0, 0, schedule.Weekday-1).Add(time.Duration(clock.Hour())*time.Hour + time.Duration(clock.Minute())*time.Minute)
	year, week := scheduled.ISOWeek()
	key := fmt.Sprintf("%04d-W%02d", year, week)
	if localNow.Before(scheduled) {
		return key, false, scheduled, nil
	}
	return key, true, scheduled.AddDate(0, 0, 7), nil
}

func (a *app) runResourceSyncScheduler(ctx context.Context) {
	check := func() {
		if err := a.tryStartScheduledResourceSync(ctx, time.Now()); err != nil {
			log.Printf("scheduled resource sync check failed: %v", err)
		}
	}
	check()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}

func (a *app) tryStartScheduledResourceSync(ctx context.Context, now time.Time) error {
	schedule, err := a.resourceSyncSchedule(ctx)
	if err != nil || !schedule.Enabled {
		return err
	}
	key, due, _, err := resourceSyncScheduleState(schedule, now)
	if err != nil || !due || schedule.LastRunKey == key {
		return err
	}
	tx, err := a.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var enabled int
	var lastRunKey string
	if err := tx.QueryRowContext(ctx,
		`select enabled, last_run_key from biz_resource_sync_schedule where id = 1 for update`,
	).Scan(&enabled, &lastRunKey); err != nil {
		return err
	}
	if enabled != 1 || lastRunKey == key {
		return nil
	}
	var running int
	if err := tx.QueryRowContext(ctx,
		`select count(*) from biz_platform_sync_jobs
		  where status = '运行中' and job_type in ('resource_sync_all', 'resource_sync_one')`,
	).Scan(&running); err != nil {
		return err
	}
	if running > 0 {
		return nil
	}
	result, err := tx.ExecContext(ctx,
		`insert into biz_platform_sync_jobs
		  (job_type, status, started_at, message)
		 values ('resource_sync_all', '运行中', now(), '自动同步任务已启动')`)
	if err != nil {
		return err
	}
	jobID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`update biz_resource_sync_schedule
		    set last_run_key = ?, last_started_at = now()
		  where id = 1`, key); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	go a.runBusinessResourcesSyncAll(int(jobID), nil)
	return nil
}
