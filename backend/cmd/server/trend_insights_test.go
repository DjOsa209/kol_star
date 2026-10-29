package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestTrendQueryValidationAndSnapshotIsolation(t *testing.T) {
	for _, query := range []string{"tab=other", "region=localhost", "region=US%2f..", "tab=video&period=90", "period=0", "pageSize=1000", "currentPage=-1", "orderByMetric=0", "tab=video&region=GB"} {
		values, _ := url.ParseQuery(query)
		if _, err := parseTrendQuery(values, false); err == nil {
			t.Errorf("accepted invalid query %q", query)
		}
	}
	for _, id := range []string{"../snapshot.json", "1/2", "javascript:alert(1)", ""} {
		if _, err := parseTrendQuery(url.Values{"id": {id}}, true); err == nil {
			t.Errorf("accepted invalid id %q", id)
		}
	}
	for _, q := range []trendQuery{{Tab: "tag", Region: "JP", Period: 30}, {Tab: "tag", Region: "US", Period: 7}, {Tab: "video", Region: "US", Period: 30, Order: 2}, {Tab: "tag", Region: "US", Period: 30, Live: true}} {
		if q.archived() {
			t.Fatalf("wrong snapshot selected: %+v", q)
		}
	}
}

func TestTrendSnapshotPaginationAndDetail(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TREND_DATA_DIR", dir)
	snapshot := trendSnapshot{CapturedAt: "2026-10-29T00:00:00Z", TagCurveEnd: "2026-10-15", TagDetails: map[string]map[string]any{}}
	for i := 1; i <= 100; i++ {
		id := strconv.Itoa(i)
		snapshot.Tags = append(snapshot.Tags, map[string]any{"hashtagID": id})
		snapshot.Videos = append(snapshot.Videos, map[string]any{"itemID": id})
		snapshot.TagDetails[id] = map[string]any{"hashtagID": id, "ageProfile": []any{}}
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "snapshot.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	trendStore.Once, trendStore.snapshot, trendStore.err = sync.Once{}, nil, nil
	t.Cleanup(func() { trendStore.Once, trendStore.snapshot, trendStore.err = sync.Once{}, nil, nil })
	a := &app{}
	ids := map[string]bool{}
	for page := 1; page <= 5; page++ {
		r := httptest.NewRequest(http.MethodGet, "/business/trends/list?tab=tag&currentPage="+string(rune('0'+page)), nil)
		w := httptest.NewRecorder()
		a.trendList(w, r)
		var result struct {
			Code int
			Data struct {
				List    []struct{ HashtagID string }
				Total   int
				HasMore bool
				Meta    struct {
					Source   string
					CurveEnd string
				}
			}
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Code != 0 || len(result.Data.List) != 20 || result.Data.Total != 100 || result.Data.Meta.Source != "archive" || result.Data.Meta.CurveEnd != "2026-10-15" {
			t.Fatalf("bad page %d: %s", page, w.Body.String())
		}
		if result.Data.HasMore != (page < 5) {
			t.Fatal("bad hasMore")
		}
		for _, row := range result.Data.List {
			if ids[row.HashtagID] {
				t.Fatal("duplicate ID")
			}
			ids[row.HashtagID] = true
		}
	}
	if len(ids) != 100 {
		t.Fatal("not all 100 tags")
	}
	w := httptest.NewRecorder()
	a.trendDetail(w, httptest.NewRequest(http.MethodGet, "/business/trends/detail?id=1", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"ageProfile"`) || strings.Contains(w.Body.String(), "relatedInterests") {
		t.Fatal("missing real detail or invented interests")
	}
	w = httptest.NewRecorder()
	a.trendList(w, httptest.NewRequest(http.MethodGet, "/business/trends/list?tab=video&currentPage=5", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"hasMore":false`) {
		t.Fatal("video final page unavailable")
	}
}

func TestTrendProtectedEndpoints(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{}.withDefaults())
	cache := &fakeTrendHotCache{}
	a.trendCacheOverride = cache
	mux := http.NewServeMux()
	a.routes(mux)
	for _, path := range []string{"/business/trends/options", "/business/trends/list", "/business/trends/detail?id=1", "/business/trends/media/cover_1.jpg", "/business/trends/local-jobs/fixture", "/business/trends/local-client/darwin-arm64", "/business/trends/local-media/fixture"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != 401 {
			t.Fatalf("unprotected %s: %d", path, w.Code)
		}
	}
	for _, tc := range []struct{ method, path string }{
		{"POST", "/business/trends/local-jobs"}, {"POST", "/business/trends/local-jobs/fixture/cancel"},
		{"POST", "/business/trends/collect"}, {"GET", "/business/trends/collect/job"}, {"GET", "/business/trends/collect/job/screen"}, {"POST", "/business/trends/collect/job/input"}, {"POST", "/business/trends/collect/job/cancel"}, {"POST", "/business/trends/collect/job/login"}, {"POST", "/business/trends/collect/job/resume"}, {"POST", "/business/trends/collect/job/desktop"},
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != 401 {
			t.Fatal("unprotected collection endpoint")
		}
	}
	now := time.Now()
	session, err := newSessionID()
	if err != nil {
		t.Fatal(err)
	}
	token := newLoginTokenResponse(7, now, now, session)["accessToken"].(string)
	mock.ExpectQuery("select s.last_seen_unix").WillReturnRows(sqlmock.NewRows([]string{"last_seen_unix"}).AddRow(now.Unix()))
	mock.ExpectQuery("select r.code").WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("reader"))
	mock.ExpectQuery("select count\\(\\*\\)").WithArgs(7, trendMenu).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	r := httptest.NewRequest(http.MethodGet, "/business/trends/list", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("reader without menu got %d", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if cache.reads != 0 || cache.writes != 0 {
		t.Fatal("cache accessed before menu authorization")
	}
}

func TestTrendMediaRejectsTraversalAndSymlinks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TREND_DATA_DIR", dir)
	if err := os.Mkdir(filepath.Join(dir, "media"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "private"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "private"), filepath.Join(dir, "media", "cover_1.jpg")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../private", "snapshot.json", "cover_1.jpg", "cover_2.jpg"} {
		r := httptest.NewRequest(http.MethodGet, "/media", nil)
		r.SetPathValue("name", name)
		w := httptest.NewRecorder()
		(&app{}).trendMedia(w, r)
		if w.Code != 404 {
			t.Fatalf("exposed %s", name)
		}
	}
}

func TestTrendUpstreamRejectsFailureAndPreservesLargeIDs(t *testing.T) {
	for _, body := range []string{`{"BaseResp":{"StatusCode":1}}`, `not json`, `{"items":[]}`} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		_, err := trendRequest(context.Background(), s.URL, nil)
		s.Close()
		if err == nil {
			t.Errorf("accepted invalid response %q", body)
		}
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://127.0.0.1:1", http.StatusFound)
	}))
	defer s.Close()
	if _, err := trendRequest(context.Background(), s.URL, nil); err == nil {
		t.Fatal("followed redirect")
	}
	v := normalizeTrendVideo(map[string]any{"itemInfo": map[string]any{"itemID": json.Number("7690899465175043358")}})
	if v["itemID"] != "7690899465175043358" {
		t.Fatal("video ID lost precision")
	}
}
