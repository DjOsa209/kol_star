package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type fakeTrendHotCache struct {
	values  map[string][]byte
	writes  int
	reads   int
	expires time.Time
	fail    bool
}

func (f *fakeTrendHotCache) Get(_ context.Context, key string) ([]byte, error) {
	f.reads++
	if f.fail {
		return nil, errors.New("unavailable")
	}
	return f.values[key], nil
}
func (f *fakeTrendHotCache) Set(_ context.Context, key string, value []byte, expires time.Time) error {
	if f.fail {
		return errors.New("unavailable")
	}
	if f.values == nil {
		f.values = map[string][]byte{}
	}
	f.values[key] = value
	f.writes++
	f.expires = expires
	return nil
}

func TestTrendCacheExactTimeNodes(t *testing.T) {
	for _, tc := range []struct {
		now, want    string
		hour, minute int
	}{
		{"2026-10-29T15:59:59Z", "2026-10-30T00:00:00+08:00", 0, 0},
		{"2026-10-29T16:00:00Z", "2026-10-31T00:00:00+08:00", 0, 0},
		{"2026-10-29T21:29:59Z", "2026-10-30T05:30:00+08:00", 5, 30},
	} {
		now, _ := time.Parse(time.RFC3339, tc.now)
		_, expires, err := trendCacheNode(now, RedisConfig{NodeHour: tc.hour, NodeMinute: tc.minute})
		if err != nil || expires.Format(time.RFC3339) != tc.want {
			t.Fatalf("node %s: %v %v", tc.now, expires, err)
		}
	}
	if _, _, err := trendCacheNode(time.Now(), RedisConfig{NodeHour: 24}); err == nil {
		t.Fatal("accepted invalid reset hour")
	}
}

func TestTrendHotCacheHitsForceRefreshAndNoSlidingExpiry(t *testing.T) {
	cache := &fakeTrendHotCache{}
	a := &app{trendCacheOverride: cache}
	calls := 0
	h := a.withTrendHotCache("list", func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeOK(w, map[string]any{"itemID": "7690899465175043358", "call": calls})
	})
	query := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}
	first := query("/business/trends/list?tab=tag")
	expiry := cache.expires
	second := query("/business/trends/list?tab=tag")
	if calls != 1 || cache.writes != 1 || second.Header().Get("X-Trend-Cache") != "HIT" || strings.TrimSpace(first.Body.String()) != strings.TrimSpace(second.Body.String()) || !expiry.Equal(cache.expires) {
		t.Fatal("cache hit refreshed or changed data")
	}
	query("/business/trends/list?tab=tag&refresh=1")
	if calls != 2 || cache.writes != 2 {
		t.Fatal("explicit refresh did not overwrite cache")
	}
	query("/business/trends/list?tab=tag&region=JP")
	query("/business/trends/list?tab=video")
	query("/business/trends/list?tab=tag&period=7")
	query("/business/trends/list?tab=tag&currentPage=2")
	query("/business/trends/list?tab=tag&live=1")
	if calls != 7 {
		t.Fatalf("query scopes collided: %d", calls)
	}
}

func TestTrendHotCacheDoesNotCacheErrorsAndSurvivesRedisFailure(t *testing.T) {
	cache := &fakeTrendHotCache{}
	a := &app{trendCacheOverride: cache}
	h := a.withTrendHotCache("list", func(w http.ResponseWriter, r *http.Request) { writeError(w, 502, 50201, "暂时不可用") })
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodGet, "/business/trends/list", nil))
	if cache.writes != 0 || w.Code != 502 {
		t.Fatal("cached upstream error")
	}
	cache.fail = true
	h = a.withTrendHotCache("list", func(w http.ResponseWriter, r *http.Request) {
		writeOK(w, map[string]any{"list": []string{"real data"}})
	})
	w = httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodGet, "/business/trends/list", nil))
	if w.Code != 200 || w.Header().Get("X-Trend-Cache") != "UNAVAILABLE" || !strings.Contains(w.Body.String(), "real data") {
		t.Fatal("cache outage swallowed valid source result")
	}
}

func TestTrendHotCacheRejectsExpiredEntry(t *testing.T) {
	cache := &fakeTrendHotCache{values: map[string][]byte{}}
	a := &app{trendCacheOverride: cache}
	q, _ := parseTrendQuery(nil, false)
	cfg := RedisConfig{}.withDefaults()
	bucket, _, _ := trendCacheNode(time.Now(), cfg)
	key := trendHotKey("list", q, "zh", bucket, cfg.KeyPrefix)
	cache.values[key], _ = json.Marshal(trendCacheRecord{ExpiresAt: time.Now().Add(-time.Minute), Body: json.RawMessage(`{"code":0,"data":{"old":true}}`)})
	calls := 0
	h := a.withTrendHotCache("list", func(w http.ResponseWriter, r *http.Request) { calls++; writeOK(w, map[string]any{"fresh": true}) })
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodGet, "/business/trends/list", nil))
	if calls != 1 || strings.Contains(w.Body.String(), "old") {
		t.Fatal("served expired cache")
	}
}

func TestTrendRESPParserRejectsMalformedOversizeAndSecretErrors(t *testing.T) {
	for _, v := range []string{"$999999999\r\n", "$-2\r\n", "$3\r\nabcXX", "*2\r\n", "+oops\n", "-ERR secret password\r\n"} {
		_, err := readTrendRESP(bufio.NewReader(strings.NewReader(v)))
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("invalid redis response accepted or leaked: %q", v)
		}
	}
	v, err := readTrendRESP(bufio.NewReader(strings.NewReader("$5\r\nhello\r\n")))
	if err != nil || string(v) != "hello" {
		t.Fatal("bulk decoding failed")
	}
	if _, err := (redisTrendCache{RedisConfig{Addr: "example.com:6379"}}).Get(context.Background(), "test"); err == nil {
		t.Fatal("accepted plaintext remote redis")
	}
}

// Runs only against an explicitly provided isolated Redis instance; never flushes a DB.
func TestTrendRedisIntegration(t *testing.T) {
	addr := os.Getenv("TREND_REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("set TREND_REDIS_TEST_ADDR to isolated Redis")
	}
	cache := redisTrendCache{RedisConfig{Addr: addr, Database: 15}}
	wrongAuth := redisTrendCache{RedisConfig{Addr: addr, Password: "fixture-not-a-real-secret"}}
	if _, err := wrongAuth.Get(context.Background(), "test-auth-failure"); err == nil || strings.Contains(err.Error(), "fixture-not-a-real-secret") {
		t.Fatal("Redis authentication failure was not safely rejected")
	}
	key := "kol:trends:test:" + t.Name() + ":" + time.Now().Format("150405.000000000")
	ctx := context.Background()
	expires := time.Now().Add(200 * time.Millisecond)
	if err := cache.Set(ctx, key, []byte(`{"id":"7690899465175043358"}`), expires); err != nil {
		t.Fatal(err)
	}
	v, err := cache.Get(ctx, key)
	if err != nil || !strings.Contains(string(v), "7690899465175043358") {
		t.Fatal("Redis roundtrip failed")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		v, err = cache.Get(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		if len(v) == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("Redis PXAT expiration did not remove the key")
}
