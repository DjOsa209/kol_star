package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func monthlyFixture() []byte {
	raw := `{"nodeType":"document","content":[{"nodeType":"heading-3","content":[{"value":"Trend: Sample — September 04, 2025"}]},{"nodeType":"paragraph","content":[{"value":"Trend Recap: A short example. Another sentence."}]},{"nodeType":"paragraph","content":[{"value":"Audio: "},{"nodeType":"hyperlink","data":{"uri":"https://www.tiktok.com/music/sample-123"},"content":[{"value":"Original sound"}]}]},{"nodeType":"embedded-entry-block","data":{"target":{"sys":{"id":"video"}}}}]}`
	body := map[string]any{"result": map[string]any{"data": map[string]any{"contentfulArticle": map[string]any{"publicationDate": "2026-10-16T00:00", "articleSections": []any{map[string]any{"navigationTitle": "TikTok Trends: September 2026", "content": map[string]any{"raw": raw, "references": []any{map[string]any{"contentful_id": "video", "embedType": "TikTok", "url": "https://www.tiktok.com/@sample/video/7691077405422750990"}}}}}}}}}
	b, _ := json.Marshal(body)
	return b
}
func TestMonthlyParseReferencesAndDates(t *testing.T) {
	a, err := parseMonthlyPayload(monthlyFixture(), "2026-10-29")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Records) != 1 {
		t.Fatal("missing record")
	}
	r := a.Records[0]
	if r.Month != "2026-09" || r.PublishedAt != "2025-09-04" || !r.DateConflict || r.VideoID != "7691077405422750990" || r.AudioLabel != "Original sound" || r.Summary != "A short example." {
		t.Fatal("incorrect extraction")
	}
	again, _ := parseMonthlyPayload(monthlyFixture(), "later")
	if again.Records[0].ID != r.ID {
		t.Fatal("unstable identity")
	}
	for _, b := range [][]byte{[]byte(`{}`), []byte(`broken`), make([]byte, monthlyMaxBytes+1)} {
		if _, e := parseMonthlyPayload(b, "now"); e == nil {
			t.Fatal("accepted invalid source")
		}
	}
}
func TestMonthlyRejectsUnsafeMedia(t *testing.T) {
	for _, u := range []string{"http://www.tiktok.com/music/a", "https://www.tiktok.com.evil.test/music/a", "https://user:pass@www.tiktok.com/music/a", "https://127.0.0.1/music/a", "https://www.tiktok.com:443/music/a", "https://www.tiktok.com/music/../private", "javascript:alert(1)"} {
		if validMonthlyURL(u) {
			t.Fatal("accepted unsafe media")
		}
	}
	source := strings.ReplaceAll(string(monthlyFixture()), "https://www.tiktok.com/music/sample-123", "https://evil.test/audio")
	source = strings.ReplaceAll(source, "https://www.tiktok.com/@sample/video/7691077405422750990", "https://evil.test/video/7691077405422750990")
	a, e := parseMonthlyPayload([]byte(source), "now")
	if e != nil {
		t.Fatal(e)
	}
	if a.Records[0].AudioURL != "" || a.Records[0].VideoURL != "" {
		t.Fatal("unsafe references exposed")
	}
}
func TestMonthlyMonthIsolationAndArchiveMerge(t *testing.T) {
	t.Setenv("TREND_DATA_DIR", t.TempDir())
	a, _ := parseMonthlyPayload(monthlyFixture(), "now")
	a.Records = append(a.Records, monthlyTrend{ID: "older", Month: "2026-08", Title: "Archive"})
	if e := saveMonthlyArchive(a); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		query string
		code  int
		match string
	}{{"?month=2026-09", 200, "Sample"}, {"?month=2026-08", 200, "Archive"}, {"?month=2026-07", 200, "\"list\":[]"}, {"?month=../../private", 400, ""}} {
		w := httptest.NewRecorder()
		(&app{}).trendMonthly(w, httptest.NewRequest("GET", "/monthly"+tc.query, nil))
		if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.match) {
			t.Fatal("incorrect month filtering")
		}
	}
	merged := mergeMonthlyArchive(a, monthlyArchive{Records: []monthlyTrend{{ID: "new", Month: "2026-09"}}})
	if len(merged.Records) != 2 || merged.Records[0].ID != "new" || merged.Records[1].ID != "older" {
		t.Fatal("archive merge failed")
	}
}

type monthlyTransport func(*http.Request) (*http.Response, error)

func (f monthlyTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestMonthlyUpstreamBoundary(t *testing.T) {
	for _, code := range []int{200, 302, 500} {
		client := &http.Client{Transport: monthlyTransport(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != monthlyPayloadURL || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
				t.Fatal("unexpected upstream request")
			}
			return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(monthlyFixture())))}, nil
		})}
		_, err := fetchMonthlyPayload(context.Background(), client)
		if (err == nil) != (code == 200) {
			t.Fatal("status not enforced")
		}
	}
}
func TestMonthlyEndpointsRequireLoginAndMenu(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{}.withDefaults())
	mux := http.NewServeMux()
	a.routes(mux)
	for _, tc := range []struct{ method, path string }{{"GET", "/business/trends/monthly"}, {"POST", "/business/trends/monthly/refresh"}} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != 401 {
			t.Fatal("unprotected endpoint")
		}
		now := time.Now()
		session, e := newSessionID()
		if e != nil {
			t.Fatal(e)
		}
		token := newLoginTokenResponse(7, now, now, session)["accessToken"].(string)
		mock.ExpectQuery("select s.last_seen_unix").WillReturnRows(sqlmock.NewRows([]string{"last_seen_unix"}).AddRow(now.Unix()))
		mock.ExpectQuery("select r.code").WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("reader"))
		mock.ExpectQuery("select count\\(\\*\\)").WithArgs(7, trendMenu).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		r := httptest.NewRequest(tc.method, tc.path, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("menu boundary not enforced")
		}
	}
	if e := mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestMonthlyPublishedDateFormats(t *testing.T) {
	for _, date := range []string{"2026-10-16T00:00", "October 16, 2026", "2026-10-16"} {
		body := strings.ReplaceAll(string(monthlyFixture()), "2026-10-16T00:00", date)
		archive, err := parseMonthlyPayload([]byte(body), "now")
		if err != nil || archive.SourceUpdatedAt != "2026-10-16" {
			t.Fatal("source date normalization failed")
		}
	}
}

func TestMonthlyRefreshInitializesMissingDataDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TREND_DATA_DIR", filepath.Join(root, "first-deploy", "trends"))
	previousClient := trendHTTPClient
	trendHTTPClient = &http.Client{Transport: monthlyTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() == monthlyPayloadURL {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(monthlyFixture())))}, nil
		}
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader("unavailable"))}, nil
	})}
	t.Cleanup(func() { trendHTTPClient = previousClient })
	w := httptest.NewRecorder()
	(&app{}).trendMonthlyRefresh(w, httptest.NewRequest("POST", "/business/trends/monthly/refresh?month=2026-09", nil))
	if w.Code != 200 {
		t.Fatalf("first-deploy refresh failed: status=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	(&app{}).trendMonthly(w, httptest.NewRequest("GET", "/business/trends/monthly?month=2026-09", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Sample") {
		t.Fatalf("cache unavailable after initialization: status=%d", w.Code)
	}
}

func TestMonthlyArchiveInvalidDestinationPreservesExistingFiles(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(destination, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TREND_DATA_DIR", destination)
	archive, _ := parseMonthlyPayload(monthlyFixture(), "now")
	if err := saveMonthlyArchive(archive); err == nil {
		t.Fatal("accepted an invalid data directory")
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "keep" {
		t.Fatal("existing data changed")
	}
}

func TestMonthlyRefreshOnlyFetchesLater(t *testing.T) {
	t.Setenv("TREND_DATA_DIR", t.TempDir())
	previousClient := trendHTTPClient
	requests := 0
	trendHTTPClient = &http.Client{Transport: monthlyTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.String() != monthlyPayloadURL {
			t.Errorf("unexpected media request: %s", r.URL.Host)
			return nil, errors.New("media requests are forbidden")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(monthlyFixture())))}, nil
	})}
	t.Cleanup(func() { trendHTTPClient = previousClient })
	w := httptest.NewRecorder()
	(&app{}).trendMonthlyRefresh(w, httptest.NewRequest("POST", "/business/trends/monthly/refresh?month=2026-09", nil))
	if w.Code != 200 || requests != 1 || !strings.Contains(w.Body.String(), `"mediaMode":"tiktok-embed"`) {
		t.Fatalf("refresh must only fetch Later: status=%d requests=%d", w.Code, requests)
	}
	if _, err := os.Stat(filepath.Join(trendDataDir(), "media")); !os.IsNotExist(err) {
		t.Fatal("refresh unexpectedly created media cache")
	}
}
