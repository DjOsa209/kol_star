package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func localFixtureDataset() trendDataset {
	d := trendDataset{CapturedAt: time.Now().UTC().Format(time.RFC3339), CurveEnd: "2026-10-15", Details: map[string]map[string]any{}}
	for i := 1; i <= 100; i++ {
		id := fmt.Sprint(i)
		d.Rows = append(d.Rows, map[string]any{"hashtagID": id, "hashtagName": "fixture"})
		d.Details[id] = map[string]any{"hashtagID": id, "hashtagName": "fixture", "token": "fixture-secret"}
	}
	return d
}
func localFixtureApp(t *testing.T) *app {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := &app{trendCacheOverride: &synchronizedTrendCache{}}
	a.config.Store(Config{TrendLocal: TrendLocalConfig{Enabled: true, StorageRoot: root}})
	return a
}
func createLocalFixture(t *testing.T, a *app) (string, string) {
	t.Helper()
	r := requestForTrendOwner("/business/trends/local-jobs?tab=tag&region=US&period=30", 1)
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	a.trendLocalCreate(w, r)
	if w.Code != 200 {
		t.Fatal("fixture task creation failed")
	}
	var response struct {
		Data struct {
			ID    string `json:"id"`
			Token string `json:"token"`
		} `json:"data"`
	}
	if json.Unmarshal(w.Body.Bytes(), &response) != nil {
		t.Fatal("invalid fixture")
	}
	return response.Data.ID, response.Data.Token
}
func uploadLocalFixture(a *app, id, token string, raw []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/business/trends/local-upload?job="+url.QueryEscape(id), bytes.NewReader(raw))
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	a.trendLocalUpload(w, r)
	return w
}
func TestTrendLocalUploadAtomicAndIdempotent(t *testing.T) {
	a := localFixtureApp(t)
	id, token := createLocalFixture(t, a)
	d := localFixtureDataset()
	raw, _ := json.Marshal(d)
	if w := uploadLocalFixture(a, id, token, raw); w.Code != 200 {
		t.Fatal("upload rejected")
	}
	j := a.localJob(id)
	deadline := time.Now().Add(3 * time.Second)
	for {
		j.mu.Lock()
		state := j.State
		j.mu.Unlock()
		if state == "complete" {
			break
		}
		if state == "failed" || time.Now().After(deadline) {
			t.Fatal("publication failed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if w := uploadLocalFixture(a, id, token, raw); w.Code != 200 {
		t.Fatal("same payload retry failed")
	}
	changed := localFixtureDataset()
	changed.Rows[0]["hashtagName"] = "changed"
	other, _ := json.Marshal(changed)
	if w := uploadLocalFixture(a, id, token, other); w.Code != 409 {
		t.Fatal("grant accepted different payload")
	}
	cache := a.trendCacheOverride.(*synchronizedTrendCache)
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.writes != 1 {
		t.Fatal("dataset published more than once")
	}
	for _, stored := range cache.values {
		if bytes.Contains(stored, []byte("fixture-secret")) {
			t.Fatal("credential-like field reached cache")
		}
	}
	files, _ := filepath.Glob(filepath.Join(a.trendLocalConfig().StorageRoot, "user-1", "dataset-*.json"))
	if len(files) != 1 {
		t.Fatal("durable dataset missing")
	}
}
func TestTrendLocalUploadRejectsInvalidAndExpired(t *testing.T) {
	for _, mode := range []string{"wrong-token", "expired", "partial", "duplicate", "wrong-detail", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			a := localFixtureApp(t)
			id, token := createLocalFixture(t, a)
			d := localFixtureDataset()
			want := 400
			switch mode {
			case "wrong-token":
				token = strings.Repeat("0", 64)
				want = 401
			case "expired":
				a.localJob(id).ExpiresAt = time.Now().Add(-time.Second)
				want = 401
			case "partial":
				d.Rows = d.Rows[:3]
			case "duplicate":
				d.Rows[1]["hashtagID"] = "1"
			case "wrong-detail":
				d.Details["1"]["hashtagID"] = "2"
			case "cancelled":
				a.localJob(id).State = "cancelled"
				want = 409
			}
			raw, _ := json.Marshal(d)
			if w := uploadLocalFixture(a, id, token, raw); w.Code != want {
				t.Fatalf("unexpected rejection status %d", w.Code)
			}
			if a.trendCacheOverride.(*synchronizedTrendCache).writes != 0 {
				t.Fatal("invalid upload wrote cache")
			}
		})
	}
}
func TestTrendLocalOwnerAndLoggingBoundaries(t *testing.T) {
	a := localFixtureApp(t)
	id, _ := createLocalFixture(t, a)
	r := requestForTrendOwner("/business/trends/local-jobs/"+id, 2)
	r.SetPathValue("job", id)
	w := httptest.NewRecorder()
	a.trendLocalStatus(w, r)
	if w.Code != 404 {
		t.Fatal("cross-owner task access")
	}
	r = httptest.NewRequest("POST", "/api/business/trends/local-upload", strings.NewReader(`{"token":"fixture-secret"}`))
	r.Header.Set("Content-Type", "application/json")
	if strings.Contains(captureRequestBody(r), "fixture-secret") {
		t.Fatal("upload body logged")
	}
	for _, raw := range []string{"https://admin.example.test/api", "http://127.0.0.1:8080/api"} {
		if !validLocalServer(raw) {
			t.Fatal("valid upload server rejected")
		}
	}
	for _, raw := range []string{"http://admin.example.test/api", "https://user:password@example.test", "file:///tmp/upload", "https://example.test?token=fixture"} {
		if validLocalServer(raw) {
			t.Fatal("unsafe upload server accepted")
		}
	}
}

func TestTrendLocalLimitsAndRevocation(t *testing.T) {
	a := localFixtureApp(t)
	id, token := createLocalFixture(t, a)
	d := localFixtureDataset()
	raw, _ := json.Marshal(d)
	a.trendLocalSlots <- struct{}{}
	a.trendLocalSlots <- struct{}{}
	if w := uploadLocalFixture(a, id, token, raw); w.Code != 429 {
		t.Fatal("processing concurrency limit missing")
	}
	<-a.trendLocalSlots
	<-a.trendLocalSlots
	if w := uploadLocalFixture(a, id, token, make([]byte, trendCacheMaxBytes+1)); w.Code != 413 {
		t.Fatal("oversized body accepted")
	}
	r := httptest.NewRequest("GET", "/business/trends/local-upload?job="+id, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	r.RemoteAddr = "192.0.2.1:1234"
	w := httptest.NewRecorder()
	a.trendLocalUploadStatus(w, r)
	if w.Code != 403 {
		t.Fatal("plaintext remote upload authorization accepted")
	}
	createLocalFixture(t, a)
	if w := uploadLocalFixture(a, id, token, raw); w.Code != 409 {
		t.Fatal("superseded grant remained usable")
	}
}
func TestTrendLocalImageSSRFGuards(t *testing.T) {
	for _, raw := range []string{"https://a.tiktokcdn.com/img", "https://a.tiktokcdn-us.com/img"} {
		if !validLocalImageURL(raw) {
			t.Fatal("expected CDN rejected")
		}
	}
	for _, raw := range []string{"http://a.tiktokcdn.com/img", "https://tiktokcdn.com.evil.test/img", "https://127.0.0.1/img", "https://a.tiktokcdn.com:8080/img", "https://user:pass@a.tiktokcdn.com/img", "https://a.tiktokcdn.com/img#fragment"} {
		if validLocalImageURL(raw) {
			t.Fatal("unsafe media URL accepted")
		}
	}
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.100.100.200", "0.0.0.0", "::1", "::ffff:127.0.0.1", "fc00::1", "fe80::1", "224.0.0.1"} {
		if publicTrendIP(net.ParseIP(raw)) {
			t.Fatal("nonpublic address accepted")
		}
	}
	if !publicTrendIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public address rejected")
	}
	if localImageContent([]byte("<svg xmlns='http://www.w3.org/2000/svg'></svg>")) {
		t.Fatal("active media content accepted")
	}
}

type trendMediaFixtureTransport struct {
	body  []byte
	calls int
}

func (f *trendMediaFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	f.calls++
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(f.body)), Request: r}, nil
}
func TestTrendLocalImageStorageAndOwnerIsolation(t *testing.T) {
	a := localFixtureApp(t)
	root, err := localTrendOwnerRoot(a.trendLocalConfig().StorageRoot, 1)
	if err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	transport := &trendMediaFixtureTransport{body: encoded.Bytes()}
	d := localFixtureDataset()
	remote := "https://fixture.tiktokcdn.com/image.png"
	d.Rows[0]["coverURL"] = remote
	d.Details["1"]["coverURL"] = remote
	saved, failed := saveLocalTrendImagesUsing(context.Background(), root, &d, &http.Client{Transport: transport})
	if saved != 1 || failed != 0 || transport.calls != 1 {
		t.Fatal("image download not deduplicated")
	}
	path := d.Rows[0]["coverURL"].(string)
	if !strings.HasPrefix(path, "/business/trends/local-media/") {
		t.Fatal("image URL not replaced")
	}
	name := filepath.Base(path)
	if _, err = os.Stat(filepath.Join(root, name)); err != nil {
		t.Fatal("image not saved")
	}
	for _, owner := range []int{1, 2} {
		r := requestForTrendOwner(path, owner)
		r.SetPathValue("name", name)
		w := httptest.NewRecorder()
		a.trendLocalMedia(w, r)
		if owner == 1 && w.Code != 200 || owner == 2 && w.Code != 404 {
			t.Fatal("media owner boundary failed")
		}
	}
}
