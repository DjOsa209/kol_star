package main

import (
	"context"
	"strings"
	"time"
)

// These are observed counters on collected posts published in the window,
// not complete platform activity or views accrued during the window.
type resourceRecentMetrics struct {
	Source            string   `json:"source"`
	Coverage          string   `json:"coverage"`
	WindowDays        int      `json:"windowDays"`
	SampleCount       int64    `json:"sampleCount"`
	AvgViews          *float64 `json:"avgViews"`
	AvgInteractions   *float64 `json:"avgInteractions"`
	EngagementRate    *float64 `json:"engagementRate"`
	WindowStart       string   `json:"windowStart"`
	WindowEnd         string   `json:"windowEnd"`
	CalculatedAt      string   `json:"calculatedAt"`
	LatestCollectedAt *string  `json:"latestCollectedAt"`
}

func (a *app) attachResourceRecentMetrics(ctx context.Context, rows []map[string]any) error {
	return a.attachResourceRecentMetricsAt(ctx, rows, time.Now().UTC())
}

func (a *app) attachResourceRecentMetricsAt(ctx context.Context, rows []map[string]any, now time.Time) error {
	now = now.UTC()
	start := now.AddDate(0, 0, -30)
	ids, placeholders := []any{}, []string{}
	seen := map[int64]bool{}
	for _, row := range rows {
		id := anyInt64(row["id"])
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
			placeholders = append(placeholders, "?")
		}
	}
	if len(ids) == 0 {
		return nil
	}
	args := append([]any{start, now}, ids...)
	stats, err := a.queryMaps(ctx, `select p.resource_id as resourceId,
	       count(*) as collectedPostCount,
	       sum(case when p.published_at >= bounds.windowStart and p.published_at <= bounds.windowEnd then 1 else 0 end) as sampleCount,
	       sum(case when p.published_at >= bounds.windowStart and p.published_at <= bounds.windowEnd then p.view_count else 0 end) as sampleViews,
	       sum(case when p.published_at >= bounds.windowStart and p.published_at <= bounds.windowEnd
	         then p.like_count + p.comment_count + p.share_count + p.save_count else 0 end) as sampleInteractions,
	       cast(unix_timestamp(max(case when p.published_at >= bounds.windowStart and p.published_at <= bounds.windowEnd
	         then p.synced_at end)) * 1000 as unsigned) as latestCollectedAt
	  from biz_resource_platform_posts p
	 cross join (select ? as windowStart, ? as windowEnd) bounds
	 where p.resource_id in (`+strings.Join(placeholders, ",")+`)
	 group by p.resource_id`, args...)
	if err != nil {
		return err
	}
	byID := make(map[int64]map[string]any, len(stats))
	for _, stat := range stats {
		byID[anyInt64(stat["resourceId"])] = stat
	}
	for _, row := range rows {
		stat := byID[anyInt64(row["id"])]
		metrics := resourceMetricsFromSample(stat, start, now)
		if _, exists := row["recentMetrics"]; !exists {
			// Older manual and provider averages have no dependable window metadata.
			row["legacyMetrics"] = map[string]any{
				"source": "stored_unknown", "windowDays": nil,
				"avgViews": row["avgViews"], "engagementRate": row["engagementRate"],
			}
		}
		row["recentMetrics"] = metrics
		row["avgViews"] = nullableResourceMetric(metrics.AvgViews)
		row["avgInteractions"] = nullableResourceMetric(metrics.AvgInteractions)
		row["engagementRate"] = nullableResourceMetric(metrics.EngagementRate)
		row["contentCount"] = metrics.SampleCount
		row["collectedPostCount"] = anyInt64(stat["collectedPostCount"])
	}
	return nil
}

func resourceMetricsFromSample(stat map[string]any, start, now time.Time) resourceRecentMetrics {
	metrics := resourceRecentMetrics{
		Source: "unavailable", Coverage: "sample", WindowDays: 30,
		SampleCount: anyInt64(stat["sampleCount"]),
		WindowStart: start.UTC().Format(time.RFC3339), WindowEnd: now.UTC().Format(time.RFC3339),
		CalculatedAt: now.UTC().Format(time.RFC3339),
	}
	if metrics.SampleCount <= 0 {
		return metrics
	}
	metrics.Source = "collected_posts"
	views, interactions := anyFloat64(stat["sampleViews"]), anyFloat64(stat["sampleInteractions"])
	avgViews, avgInteractions := views/float64(metrics.SampleCount), interactions/float64(metrics.SampleCount)
	metrics.AvgViews, metrics.AvgInteractions = &avgViews, &avgInteractions
	if views > 0 {
		rate := interactions / views
		metrics.EngagementRate = &rate
	}
	if timestamp := anyInt64(stat["latestCollectedAt"]); timestamp > 0 {
		latest := time.UnixMilli(timestamp).UTC().Format(time.RFC3339)
		metrics.LatestCollectedAt = &latest
	}
	return metrics
}

func nullableResourceMetric(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}
