package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNormalizeWebsiteDomain(t *testing.T) {
	domain, homepage := normalizeWebsiteDomain("https://www.example.com/news?id=1，")
	if domain != "example.com" || homepage != "https://example.com" {
		t.Fatalf("unexpected website: %q %q", domain, homepage)
	}
}

func TestFetchTrafficCVMonthlyMetricsLive(t *testing.T) {
	if os.Getenv("TRAFFIC_CV_LIVE_TEST") != "1" {
		t.Skip("set TRAFFIC_CV_LIVE_TEST=1 to run the live Traffic.cv check")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	metrics, err := fetchTrafficCVMonthlyMetrics(ctx, "infinixmobility.com", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if metrics.Visits <= 0 {
		t.Fatalf("expected live monthly visits, got %#v", metrics)
	}
}

func TestWebsiteAttributes(t *testing.T) {
	attrs := websiteAttributes(`<link rel="apple-touch-icon" href='/assets/icon.png'>`)
	if attrs["rel"] != "apple-touch-icon" || attrs["href"] != "/assets/icon.png" {
		t.Fatalf("unexpected attrs: %#v", attrs)
	}
}

func TestParseTrafficCVHTML(t *testing.T) {
	body := []byte(`<!doctype html><html><body>
		<section><span>Total Visits</span><strong>2.52M</strong><em>+4.08%</em></section>
		<section><span>Avg. Duration</span><strong>00:06:18</strong></section>
		<section><span>Pages per Visit</span><strong>2.10</strong></section>
		<section><span>Bounce Rate</span><strong>57.25%</strong></section>
	</body></html>`)
	metrics, err := parseTrafficCVHTML(body, time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if metrics.Visits != 2_520_000 || metrics.PageViews != 5_292_000 {
		t.Fatalf("unexpected traffic metrics: %#v", metrics)
	}
	if metrics.Month != "2026-06" || metrics.PagesPerVisit != 2.1 || metrics.BounceRate != 0.5725 {
		t.Fatalf("unexpected engagement metrics: %#v", metrics)
	}
	if metrics.AverageDuration != "00:06:18" {
		t.Fatalf("unexpected duration: %q", metrics.AverageDuration)
	}
}

func TestParseTrafficCVHTMLRejectsChallenge(t *testing.T) {
	_, err := parseTrafficCVHTML([]byte(`<html><title>Just a moment...</title><script src="/cdn-cgi/challenge-platform/x"></script></html>`), time.Now())
	if err == nil || !strings.Contains(err.Error(), "Cloudflare") {
		t.Fatalf("expected Cloudflare challenge error, got %v", err)
	}
}

func TestParseTrafficCVCount(t *testing.T) {
	tests := map[string]int64{"54.55K": 54_550, "504.04M": 504_040_000, "1.2B": 1_200_000_000, "12,345": 12_345}
	for input, want := range tests {
		got, err := parseTrafficCVCount(input)
		if err != nil || got != want {
			t.Fatalf("parseTrafficCVCount(%q) = %d, %v; want %d", input, got, err, want)
		}
	}
}

func TestTrafficCVHTTPClientRejectsInvalidProxy(t *testing.T) {
	t.Setenv("TRAFFIC_CV_PROXY_URL", "://invalid")
	if _, err := trafficCVHTTPClient(); err == nil {
		t.Fatal("expected invalid Traffic.cv proxy to be rejected")
	}
}

func TestImportTrafficCVHTMLUpdatesWebsiteMetrics(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("select name, resource_type, platform_url, website").
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{"name", "resource_type", "platform_url", "website"}).
			AddRow("Gadget Match", "媒体", "https://gadgetmatch.com", ""))
	mock.ExpectExec("update biz_resources set").
		WillReturnResult(sqlmock.NewResult(0, 1))

	requestBody, err := json.Marshal(map[string]any{
		"id": 42,
		"html": `<html><body>
			<span>Total Visits</span><strong>1.25M</strong>
			<span>Pages per Visit</span><strong>2.4</strong>
			<span>Bounce Rate</span><strong>40%</strong>
			<span>Avg. Duration</span><strong>00:02:15</strong>
		</body></html>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/business/resources/traffic-cv-html", bytes.NewReader(requestBody))
	recorder := httptest.NewRecorder()
	newApp(db, Config{}).importTrafficCVHTML(recorder, request)

	var response struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != 0 {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
	if got := int64(response.Data["monthlyVisits"].(float64)); got != 1_250_000 {
		t.Fatalf("monthly visits = %d, want 1250000", got)
	}
	if got := int64(response.Data["monthlyPageViews"].(float64)); got != 3_000_000 {
		t.Fatalf("monthly page views = %d, want 3000000", got)
	}
	if response.Data["source"] != "Traffic.cv HTML 导入" {
		t.Fatalf("unexpected source: %#v", response.Data["source"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
