package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

const resourceRecentMetricsQueryPattern = `(?s)select p.resource_id as resourceId,.*p.published_at >= bounds.windowStart and p.published_at <= bounds.windowEnd.*cross join \(select \? as windowStart, \? as windowEnd\) bounds.*where p.resource_id in`

func resourceMetricsTestRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"resourceId", "collectedPostCount", "sampleCount", "sampleViews", "sampleInteractions", "latestCollectedAt"})
}

func TestResourceMetricsListAndPlatformAccountsShareCollectedSamples(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	rows := []map[string]any{
		{"id": 1, "name": "Creator", "platform": "YouTube", "avgViews": 999999, "engagementRate": 0.9, "followers": 150, "videoCount": 900},
		{"id": 2, "name": "Creator", "platform": "Instagram", "avgViews": 777777, "engagementRate": 0.8, "followers": 50, "videoCount": 700},
	}
	mock.ExpectQuery(`(?s)select r.id, lower\(trim\(r.name\)\).*from biz_resources r.*where lower\(trim\(r.name\)\) in`).WithArgs("creator").
		WillReturnRows(sqlmock.NewRows([]string{"id", "resourceName", "platform", "avgViews", "engagementRate", "followers", "platformAudience", "videoCount"}).
			AddRow(1, "creator", "YouTube", 999999, 0.9, 150, 150, 900).
			AddRow(2, "creator", "Instagram", 777777, 0.8, 50, 50, 700))
	mock.ExpectQuery(resourceRecentMetricsQueryPattern).WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), 1, 2).
		WillReturnRows(resourceMetricsTestRows().AddRow(1, 7, 2, 1000, 105, nil).AddRow(2, 6, 0, 0, 0, nil))
	for range 2 {
		mock.ExpectQuery(`(?s)from biz_resource_metric_snapshots.*resource_id in`).WithArgs(1, 2).
			WillReturnRows(sqlmock.NewRows([]string{"resourceId", "audienceSize", "averageViews", "averageInteractions", "snapshotWeek"}).
				AddRow(1, 100, 100000, 8000, "2026-01-01").AddRow(2, 50, 10000, 500, "2026-01-01"))
	}
	if err := a.attachResourcePlatformAccounts(context.Background(), rows); err != nil {
		t.Fatal(err)
	}
	if err := a.attachResourceWeeklyDeltas(context.Background(), rows); err != nil {
		t.Fatal(err)
	}
	accounts := rows[0]["platformAccounts"].([]map[string]any)
	for i, row := range rows {
		for _, key := range []string{"avgViews", "avgInteractions", "engagementRate", "contentCount", "collectedPostCount", "recentMetrics", "weeklyDeltas"} {
			if !reflect.DeepEqual(row[key], accounts[i][key]) {
				t.Fatalf("list and profile disagree on %s: list=%#v profile=%#v", key, row[key], accounts[i][key])
			}
		}
		deltas := row["weeklyDeltas"].(map[string]any)
		if deltas["views"] != nil || deltas["interactions"] != nil || deltas["interactionRate"] != nil {
			t.Fatalf("incomparable historical content averages leaked into deltas: %#v", deltas)
		}
	}
	if rows[0]["avgViews"] != float64(500) || rows[0]["avgInteractions"] != 52.5 || rows[0]["engagementRate"] != 0.105 {
		t.Fatalf("sample averages must use all collected counters: %#v", rows[0])
	}
	if rows[0]["contentCount"] != int64(2) || rows[0]["collectedPostCount"] != int64(7) || rows[0]["videoCount"] != 900 {
		t.Fatalf("window sample, stored posts and platform totals were conflated: %#v", rows[0])
	}
	if rows[1]["avgViews"] != nil || rows[1]["avgInteractions"] != nil || rows[1]["engagementRate"] != nil {
		t.Fatalf("old stored averages must not fill missing recent samples: %#v", rows[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResourceMetricsKeepsMeasuredZerosAndExplicitWindow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	now := time.Date(2026, 10, 22, 8, 0, 0, 0, time.UTC)
	start := now.AddDate(0, 0, -30)
	rows := []map[string]any{{"id": 1, "avgViews": 100000, "engagementRate": 0.1}, {"id": 2, "avgViews": 300000, "engagementRate": 0.2}}
	mock.ExpectQuery(resourceRecentMetricsQueryPattern).WithArgs(start, now, 1, 2).
		WillReturnRows(resourceMetricsTestRows().AddRow(1, 5, 2, 0, 10, now.Add(-time.Hour).UnixMilli()))
	if err := a.attachResourceRecentMetricsAt(context.Background(), rows, now); err != nil {
		t.Fatal(err)
	}
	metrics := rows[0]["recentMetrics"].(resourceRecentMetrics)
	if metrics.Source != "collected_posts" || metrics.Coverage != "sample" || metrics.WindowDays != 30 || metrics.SampleCount != 2 || metrics.WindowStart != start.Format(time.RFC3339) || metrics.WindowEnd != now.Format(time.RFC3339) {
		t.Fatalf("sample scope is incomplete: %#v", metrics)
	}
	if metrics.AvgViews == nil || *metrics.AvgViews != 0 || metrics.AvgInteractions == nil || *metrics.AvgInteractions != 5 || metrics.EngagementRate != nil {
		t.Fatalf("measured zero and unavailable ratio must stay distinct: %#v", metrics)
	}
	if metrics.LatestCollectedAt == nil || *metrics.LatestCollectedAt != now.Add(-time.Hour).Format(time.RFC3339) {
		t.Fatalf("missing collected timestamp: %#v", metrics)
	}
	if rows[1]["recentMetrics"].(resourceRecentMetrics).Source != "unavailable" || rows[1]["avgViews"] != nil {
		t.Fatalf("resource with no posts has fabricated metrics: %#v", rows[1])
	}
	legacy := rows[1]["legacyMetrics"].(map[string]any)
	if legacy["avgViews"] != 300000 || legacy["source"] != "stored_unknown" || legacy["windowDays"] != nil {
		t.Fatalf("legacy values lost their distinct provenance: %#v", legacy)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResourcePostsSeparatesAllCollectedStatsFromRecentSample(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectQuery(`select count\(\*\)`).WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectQuery(`(?s)select p.id.*coalesce\(nullif\(r.avatar_url, ''\), r.avatar_remote_url, ''\) as resourceAvatarUrl`).WithArgs(1, 12, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "viewCount"}).AddRow(1, 0).AddRow(2, 100))
	mock.ExpectQuery(`(?s)select id, name.*coalesce\(nullif\(avatar_url, ''\), avatar_remote_url, ''\) as avatarUrl`).WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "avgViews", "engagementRate", "videoCount"}).AddRow(1, "Creator", 999999, 0.9, 900))
	mock.ExpectQuery(resourceRecentMetricsQueryPattern).WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), 1).
		WillReturnRows(resourceMetricsTestRows().AddRow(1, 3, 2, 100, 10, nil))
	mock.ExpectQuery(`(?s)select count\(\*\) as postCount,.*avg\(view_count\) as avgViews,`).WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"postCount", "totalViews", "avgViews"}).AddRow(3, 100, 100.0/3))
	w := httptest.NewRecorder()
	a.businessResourcePosts(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"resourceId":1}`)))
	var response struct {
		Code int `json:"code"`
		Data struct {
			Resource map[string]any `json:"resource"`
			Stats    map[string]any `json:"stats"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Code != 0 {
		t.Fatalf("resource posts failed: %s (%v)", w.Body.String(), err)
	}
	if response.Data.Resource["avgViews"] != float64(50) || response.Data.Resource["contentCount"] != float64(2) {
		t.Fatalf("posts resource uses a different recent average: %#v", response.Data.Resource)
	}
	if response.Data.Stats["source"] != "collected_posts" || response.Data.Stats["coverage"] != "sample" || response.Data.Stats["windowDays"] != nil || response.Data.Stats["recentMetrics"] == nil {
		t.Fatalf("all-collected stats must be distinct from recent samples: %#v", response.Data.Stats)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
