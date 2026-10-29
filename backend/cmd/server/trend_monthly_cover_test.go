package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMonthlyRefreshMissingCovers(t *testing.T) {
	t.Setenv("TREND_DATA_DIR", t.TempDir())
	id := "7680176677351148821"
	var jpegBody bytes.Buffer
	jpeg.Encode(&jpegBody, image.NewRGBA(image.Rect(0, 0, 10, 10)), nil)
	calls := 0
	state := map[string]any{"source": map[string]any{"data": map[string]any{"/embed/v2/" + id: map[string]any{"videoData": map[string]any{"itemInfos": map[string]any{"covers": []string{"https://p16-common-sign.tiktokcdn.com/cover.jpg"}}}}}}}
	payload, _ := json.Marshal(state)
	client := &http.Client{Transport: monthlyTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("credentials forwarded")
		}
		calls++
		body := jpegBody.Bytes()
		if r.URL.Host == "www.tiktok.com" {
			body = []byte(`<script id="__FRONTITY_CONNECT_STATE__" type="application/json">` + string(payload) + `</script>`)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}
	archive := monthlyArchive{Records: []monthlyTrend{{Month: "2026-09", VideoID: id}, {Month: "2026-10", VideoID: "7691077405422750990"}}}
	got := syncMonthlyCovers(context.Background(), client, archive, "2026-09")
	if got.Added != 1 || got.Ready != 1 || got.Pending != 0 {
		t.Fatalf("missing September cover: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(trendDataDir(), "media", "monthly_cover_"+id+".jpg")); err != nil {
		t.Fatal(err)
	}
	before := calls
	got = syncMonthlyCovers(context.Background(), client, archive, "2026-09")
	if calls != before || got.Added != 0 {
		t.Fatal("downloaded cached cover")
	}
}
func TestMonthlyCoverBoundary(t *testing.T) {
	for _, u := range []string{"http://p16.tiktokcdn.com/a", "https://p16.tiktokcdn.com.evil.test/a", "https://127.0.0.1/a", "https://p16.tiktokcdn.com:443/a", "https://user@p16.tiktokcdn.com/a"} {
		if validMonthlyCoverURL(u) {
			t.Fatal("unsafe image host")
		}
	}
	if _, e := monthlyCoverReferences([]byte(`<script id="__FRONTITY_CONNECT_STATE__">{}</script>`), "123"); e == nil {
		t.Fatal("accepted missing metadata")
	}
	t.Setenv("TREND_DATA_DIR", t.TempDir())
	client := &http.Client{Transport: monthlyTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader("blocked"))}, nil
	})}
	got := syncMonthlyCovers(context.Background(), client, monthlyArchive{Records: []monthlyTrend{{Month: "2026-09", VideoID: "7680176677351148821"}}}, "2026-09")
	if got.Pending != 1 || got.Ready != 0 {
		t.Fatal("failure not reported")
	}
}
