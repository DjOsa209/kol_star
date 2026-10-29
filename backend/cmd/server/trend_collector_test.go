package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestTrendBrowserInputAndEndpointBoundaries(t *testing.T) {
	for _, endpoint := range []string{"http://ads.tiktok.com/CreativeOne/KnowledgeAPI/GetHashtagList", "https://ads.tiktok.com.evil.test/CreativeOne/KnowledgeAPI/GetHashtagList", "https://ads.tiktok.com:443/CreativeOne/KnowledgeAPI/GetHashtagList", "https://user:pass@ads.tiktok.com/CreativeOne/KnowledgeAPI/GetHashtagList", "https://127.0.0.1/CreativeOne/KnowledgeAPI/GetHashtagList", "https://ads.tiktok.com/anything"} {
		if allowedTrendEndpoint(endpoint) {
			t.Fatal("accepted forbidden endpoint")
		}
	}
	if !allowedTrendEndpoint("https://ads.tiktok.com/CreativeOne/KnowledgeAPI/GetHashtagList") {
		t.Fatal("rejected business endpoint")
	}
	for _, u := range []string{"file:///private/test", "http://accounts.google.com", "https://accounts.google.com.evil.test", "https://ads.tiktok.com:1234", "https://user:pass@ads.tiktok.com"} {
		if trendLoginOrigin(u) != "" {
			t.Fatal("exposed forbidden login page")
		}
	}
	for _, v := range []trendInput{{Target: "a", Kind: "text", Text: strings.Repeat("a", 513)}, {Target: "a", Kind: "text", Text: "x\ny"}, {Target: "a", Kind: "key", Key: "F12"}, {Target: "a", Kind: "navigate"}, {Target: "a", Kind: "click", X: 1280}, {Target: "a", Kind: "wheel", Delta: 801}, {Kind: "text", Text: "fixture"}} {
		if validTrendInput(v) {
			t.Fatal("accepted invalid browser control")
		}
	}
	if !validTrendInput(trendInput{Target: "a", Kind: "text", Text: "fixture@example.test"}) {
		t.Fatal("rejected normal input")
	}
}
func TestTrendLoginInputNeverCapturedInLogs(t *testing.T) {
	for _, prefix := range []string{"", "/api"} {
		payload := `{"target":"fixture","kind":"text","text":"fixture-sensitive-input"}`
		r := httptest.NewRequest("POST", prefix+"/business/trends/collect/fixture/input", strings.NewReader(payload))
		r.Header.Set("Content-Type", "application/json")
		if captureRequestBody(r) != "[remote browser input omitted]" {
			t.Fatal("remote login input would be logged")
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != payload {
			t.Fatal("redaction consumed live input")
		}
	}
}
func TestTrendDatasetScopeAndSanitization(t *testing.T) {
	q, _ := parseTrendQuery(nil, false)
	key := trendDatasetKey(1, q, "node", "prefix:")
	q.Page, q.Size, q.ID = 4, 10, "42"
	if key != trendDatasetKey(1, q, "node", "prefix:") {
		t.Fatal("pages did not share atomic dataset")
	}
	if key == trendDatasetKey(2, q, "node", "prefix:") {
		t.Fatal("cross-user cache collision")
	}
	q.Region = "JP"
	if key == trendDatasetKey(1, q, "node", "prefix:") {
		t.Fatal("cross-region cache collision")
	}
	clean := sanitizeTrendBusiness(map[string]any{"cookie": "fixture", "hashtagID": "123", "metrics": map[string]any{"token": "fixture", "sixSecondsVTR": 0.4}, "videoList": []any{map[string]any{"itemID": "456", "password": "fixture"}}})
	data, _ := json.Marshal(clean)
	if strings.Contains(string(data), "fixture") || !strings.Contains(string(data), "sixSecondsVTR") {
		t.Fatal("unsafe or incomplete business projection")
	}
}
func fixtureTrendRequester(mode string) trendRequester {
	return func(ctx context.Context, endpoint string, body map[string]any) (map[string]any, error) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if strings.Contains(endpoint, "GetHashtagDetail") {
			if mode == "detail-failure" {
				return nil, errors.New("fixture failure")
			}
			id := fmt.Sprint(body["hashtagID"])
			if mode == "wrong-detail" {
				id = "999"
			}
			return map[string]any{"hashtagID": id, "hashtagName": "fixture", "cookie": "fixture-secret"}, nil
		}
		n := body["page"].(int)
		size := 20
		if mode == "anonymous" {
			size = 3
		}
		if mode == "duplicate" {
			n = 1
		}
		rows := []any{}
		for i := 0; i < size; i++ {
			rows = append(rows, map[string]any{"hashtagID": fmt.Sprint((n-1)*20 + i + 1), "hashtagName": "fixture", "secret": "fixture-secret"})
		}
		return map[string]any{"items": rows, "pagination": map[string]any{"page": body["page"], "limit": 20, "totalCount": 100, "hasMore": n < 5}}, nil
	}
}
func TestTrendCollectionRejectsPartialDuplicateAndWrongDetails(t *testing.T) {
	q, _ := parseTrendQuery(nil, false)
	for _, mode := range []string{"anonymous", "duplicate", "detail-failure", "wrong-detail"} {
		ctx := context.WithValue(context.Background(), trendRequesterKey{}, fixtureTrendRequester(mode))
		if _, err := collectTrendDataset(ctx, q, func(int) {}); err == nil {
			t.Fatal("accepted incomplete dataset:", mode)
		}
	}
}

type synchronizedTrendCache struct {
	mu     sync.Mutex
	values map[string][]byte
	writes int
	fail   bool
}

func (c *synchronizedTrendCache) Get(_ context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.values[key], nil
}
func (c *synchronizedTrendCache) Set(_ context.Context, key string, data []byte, _ time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail {
		return errors.New("unavailable")
	}
	if c.values == nil {
		c.values = map[string][]byte{}
	}
	c.values[key] = data
	c.writes++
	return nil
}

type fixtureBrowser struct {
	request     trendRequester
	openErr     error
	closed      bool
	mu          sync.Mutex
	screenCalls int
}

func (f *fixtureBrowser) Open(context.Context, trendQuery) error { return f.openErr }
func (f *fixtureBrowser) Request(ctx context.Context, u string, b map[string]any) (map[string]any, error) {
	return f.request(ctx, u, b)
}
func (f *fixtureBrowser) Screen(context.Context, string) (trendScreen, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.screenCalls++
	return trendScreen{}, nil
}
func (f *fixtureBrowser) Input(context.Context, trendInput) error { return nil }
func (f *fixtureBrowser) Close()                                  { f.closed = true }
func requestForTrendOwner(path string, owner int) *http.Request {
	r := httptest.NewRequest("GET", path, nil)
	return r.WithContext(context.WithValue(r.Context(), trendOwnerKey{}, owner))
}
func TestTrendCollectorLoginRecoveryAndAtomicRedisPublication(t *testing.T) {
	cache := &synchronizedTrendCache{}
	a := &app{trendCacheOverride: cache}
	q, _ := parseTrendQuery(nil, false)
	q.Live = true
	var lock sync.Mutex
	loggedIn := false
	driver := &fixtureBrowser{request: func(ctx context.Context, u string, b map[string]any) (map[string]any, error) {
		lock.Lock()
		ready := loggedIn
		lock.Unlock()
		mode := "anonymous"
		if ready {
			mode = "complete"
		}
		return fixtureTrendRequester(mode)(ctx, u, b)
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	job := trendJob{ID: "fixture-job", State: "starting", query: q, cancel: cancel, Total: 100}
	s := &trendBrowserSession{job: &job, browser: driver, lastUsed: time.Now()}
	done := make(chan struct{})
	go func() { a.runTrendCollection(ctx, 1, s, job, TrendBrowserConfig{}); close(done) }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		s.mu.Lock()
		state := s.job.State
		s.mu.Unlock()
		if state == "needs_login" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("did not request login")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cache.mu.Lock()
	writes := cache.writes
	cache.mu.Unlock()
	if writes != 0 {
		t.Fatal("published anonymous results")
	}
	lock.Lock()
	loggedIn = true
	lock.Unlock()
	select {
	case <-done:
	case <-time.After(40 * time.Second):
		t.Fatal("did not resume after login")
	}
	s.mu.Lock()
	state, cacheState := s.job.State, s.job.Cache
	s.mu.Unlock()
	if state != "complete" || cacheState != "UPDATED" {
		t.Fatal("collection failed after login")
	}
	cache.mu.Lock()
	writes = cache.writes
	cache.mu.Unlock()
	if writes != 1 {
		t.Fatal("dataset not published atomically")
	}
	w := httptest.NewRecorder()
	a.trendBrowserData(w, requestForTrendOwner("/business/trends/list", 1), q, false)
	if w.Code != 200 || w.Header().Get("X-Trend-Cache") != "HIT" || !strings.Contains(w.Body.String(), `"total":100`) {
		t.Fatal("could not read published list")
	}
	q.ID = "100"
	w = httptest.NewRecorder()
	a.trendBrowserData(w, requestForTrendOwner("/business/trends/detail", 1), q, true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"hashtagID":"100"`) {
		t.Fatal("detail missing after publication")
	}
	w = httptest.NewRecorder()
	a.trendBrowserData(w, requestForTrendOwner("/business/trends/detail", 2), q, true)
	if w.Code != 409 {
		t.Fatal("another user could read dataset")
	}
}
func TestTrendCollectorJobOwnershipAndCancellation(t *testing.T) {
	a := &app{}
	m := a.trendManager()
	driver := &fixtureBrowser{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.sessions[1] = &trendBrowserSession{job: &trendJob{ID: "job-a", State: "needs_login", cancel: cancel}, browser: driver, lastUsed: time.Now()}
	r := requestForTrendOwner("/business/trends/collect/job-a/screen", 2)
	r.SetPathValue("job", "job-a")
	w := httptest.NewRecorder()
	a.trendLoginScreen(w, r)
	if w.Code != 404 || driver.screenCalls != 0 {
		t.Fatal("cross-user screen access")
	}
	r = requestForTrendOwner("/business/trends/collect/job-a/cancel", 1)
	r.SetPathValue("job", "job-a")
	w = httptest.NewRecorder()
	a.trendCollectCancel(w, r)
	if ctx.Err() == nil {
		t.Fatal("job not cancelled")
	}
	w = httptest.NewRecorder()
	a.trendLoginScreen(w, r)
	if w.Code != 409 || driver.screenCalls != 0 {
		t.Fatal("cancelled job exposed browser")
	}
}
func TestTrendCollectorCapacityAndExpiredCache(t *testing.T) {
	m := &trendBrowserManager{sessions: map[int]*trendBrowserSession{}}
	if _, err := m.session(1, 1, true); err != nil {
		t.Fatal(err)
	}
	if _, err := m.session(2, 1, true); err == nil {
		t.Fatal("unbounded browser allocation")
	}
	q, _ := parseTrendQuery(url.Values{"live": {"1"}}, false)
	cfg := RedisConfig{}.withDefaults()
	bucket, _, _ := trendCacheNode(time.Now(), cfg)
	body, _ := json.Marshal(trendDataset{ExpiresAt: time.Now().Add(-time.Second)})
	cache := &synchronizedTrendCache{values: map[string][]byte{trendDatasetKey(1, q, bucket, cfg.KeyPrefix): body}}
	a := &app{trendCacheOverride: cache}
	w := httptest.NewRecorder()
	a.trendBrowserData(w, requestForTrendOwner("/", 1), q, false)
	if w.Code != 409 {
		t.Fatal("served expired result")
	}
}
func TestTrendChromeSmoke(t *testing.T) {
	executable := os.Getenv("TREND_BROWSER_TEST_EXECUTABLE")
	if executable == "" {
		t.Skip("explicit browser executable required")
	}
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	driver, err := newTrendChrome(context.Background(), TrendBrowserConfig{Executable: executable, ProfileRoot: root}, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	chrome := driver.(*trendChrome)
	// An isolated synthetic page, no account or login credentials are accessed.
	ctx, cancel := context.WithTimeout(chrome.ctx, 10*time.Second)
	defer cancel()
	var screenshot []byte
	if err = chromedp.Run(ctx, chromedp.Navigate("data:text/html,<title>Collector fixture</title><input id='test'>"), chromedp.CaptureScreenshot(&screenshot)); err != nil {
		t.Fatal(err)
	}
	if len(screenshot) == 0 {
		t.Fatal("browser did not render")
	}
	if _, err = driver.Screen(ctx, ""); err == nil {
		t.Fatal("remote login window exposed non-allowlisted page")
	}
	info, err := os.Stat(filepath.Join(root, "user-1"))
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("unsafe persistent profile")
	}
}

func TestTrendRemoteControlRequiresSecureTransport(t *testing.T) {
	r := httptest.NewRequest("POST", "/business/trends/collect", nil)
	r.RemoteAddr = "203.0.113.1:12345"
	r.Header.Set("X-Forwarded-Proto", "https")
	if trendControlTransport(r) {
		t.Fatal("trusted spoofed proxy header")
	}
	r.RemoteAddr = "127.0.0.1:12345"
	if !trendControlTransport(r) {
		t.Fatal("rejected local proxy")
	}
	r.RemoteAddr = "[::1]:12345"
	if !trendControlTransport(r) {
		t.Fatal("rejected IPv6 loopback")
	}
}
func TestTrendMalformedControlRejected(t *testing.T) {
	for _, payload := range []string{`{"kind":"navigate","url":"http://localhost"}`, `{} {}`, strings.Repeat("x", 4097)} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(payload))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		var v trendInput
		if decodeTrendControl(w, r, &v) {
			t.Fatal("accepted malformed control body")
		}
	}
}
func TestTrendCancelledAndFailedJobsPreserveRedis(t *testing.T) {
	for _, state := range []string{"cancelled", "detail-failure"} {
		cache := &synchronizedTrendCache{values: map[string][]byte{"previous": []byte("previous-result")}}
		a := &app{trendCacheOverride: cache}
		q, _ := parseTrendQuery(nil, false)
		ctx, cancel := context.WithCancel(context.Background())
		job := trendJob{ID: "fixture", State: "starting", query: q, cancel: cancel}
		if state == "cancelled" {
			job.State = "cancelled"
			cancel()
		}
		s := &trendBrowserSession{job: &job, browser: &fixtureBrowser{request: fixtureTrendRequester(state)}}
		a.runTrendCollection(ctx, 1, s, job, TrendBrowserConfig{})
		if cache.writes != 0 || string(cache.values["previous"]) != "previous-result" {
			t.Fatal("failed run replaced previous cache")
		}
	}
}

func TestTrendAttachEndpointAndOwnerBoundaries(t *testing.T) {
	for _, raw := range []string{"ws://127.0.0.1:9222/devtools/browser", "ws://[::1]:9222/devtools/browser/id"} {
		if !validTrendAttachEndpoint(raw) {
			t.Fatal("rejected local endpoint")
		}
	}
	for _, raw := range []string{"ws://example.com:9222/devtools/browser", "ws://127.0.0.1.evil.test:9222/devtools/browser", "ws://127.0.0.1:9222/devtools/page/id", "ws://127.0.0.1:9222/devtools/browser?token=x", "ws://user:pass@127.0.0.1:9222/devtools/browser", "ws://127.0.0.1:9222/devtools/browser/../page"} {
		if validTrendAttachEndpoint(raw) {
			t.Fatal("accepted unsafe attach endpoint")
		}
	}
	if _, err := attachTrendChrome(context.Background(), TrendBrowserConfig{Mode: "attach", AttachedUserID: 1, Endpoint: "ws://127.0.0.1:9222/devtools/browser"}, 2); err == nil {
		t.Fatal("accepted wrong browser owner")
	}
}

func TestTrendAttachRetainsConnectionAfterTask(t *testing.T) {
	for _, mode := range []string{"complete", "detail-failure", "open-failure"} {
		t.Run(mode, func(t *testing.T) {
			a := &app{trendCacheOverride: &synchronizedTrendCache{}}
			driver := &fixtureBrowser{request: fixtureTrendRequester(mode)}
			if mode == "open-failure" {
				driver.openErr = errors.New("fixture navigation timeout")
			}
			q, _ := parseTrendQuery(nil, false)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			job := trendJob{ID: "persistent", State: "starting", query: q, cancel: cancel}
			s := &trendBrowserSession{job: &job, browser: driver}
			a.runTrendCollection(ctx, 1, s, job, TrendBrowserConfig{Mode: "attach"})
			if driver.closed || s.browser != driver {
				t.Fatal("task disconnected user's existing Chrome")
			}
		})
	}
}
func TestTrendExternalResumeIsOwnerScoped(t *testing.T) {
	a := &app{}
	m := a.trendManager()
	resume := make(chan struct{}, 1)
	m.sessions[1] = &trendBrowserSession{lastUsed: time.Now(), job: &trendJob{ID: "external", State: "needs_login", LoginMode: "external", resume: resume}}
	r := requestForTrendOwner("/business/trends/collect/external/resume", 2)
	r.SetPathValue("job", "external")
	w := httptest.NewRecorder()
	a.trendCollectResume(w, r)
	if w.Code != 404 {
		t.Fatal("cross-user resume")
	}
	r = requestForTrendOwner("/business/trends/collect/external/resume", 1)
	r.SetPathValue("job", "external")
	w = httptest.NewRecorder()
	a.trendCollectResume(w, r)
	if w.Code != 200 || len(resume) != 1 {
		t.Fatal("owner could not resume")
	}
	w = httptest.NewRecorder()
	a.trendCollectResume(w, r)
	if w.Code != 409 {
		t.Fatal("duplicate resume accepted")
	}
}

func TestTrendExternalLoginDisconnectsThenResumes(t *testing.T) {
	for _, failReconnect := range []bool{false, true} {
		a := &app{trendCacheOverride: &synchronizedTrendCache{}}
		before := &fixtureBrowser{request: fixtureTrendRequester("anonymous")}
		after := &fixtureBrowser{request: fixtureTrendRequester("complete")}
		a.trendManager().factory = func(context.Context, TrendBrowserConfig, int) (trendBrowserDriver, error) {
			if !before.closed {
				t.Error("reconnected before detaching for manual login")
			}
			if failReconnect {
				return nil, errors.New("fixture unavailable")
			}
			return after, nil
		}
		q, _ := parseTrendQuery(nil, false)
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		resume := make(chan struct{}, 1)
		resume <- struct{}{}
		job := trendJob{ID: "resume-fixture", State: "starting", query: q, cancel: cancel, resume: resume, LoginMode: "external"}
		s := &trendBrowserSession{job: &job, browser: before}
		a.runTrendCollection(ctx, 1, s, job, TrendBrowserConfig{Mode: "attach"})
		if !before.closed {
			t.Fatal("manual login retained old debugging connection")
		}
		cache := a.trendCacheOverride.(*synchronizedTrendCache)
		if failReconnect {
			if s.job.State != "failed" || cache.writes != 0 {
				t.Fatal("failed reconnect published data")
			}
		} else if s.job.State != "complete" || cache.writes != 1 || after.closed {
			t.Fatal("manual login did not resume and preserve successful session")
		}
	}
}

func TestTrendNavigationErrorsAreRedacted(t *testing.T) {
	if strings.Contains(trendNavigationMessage(errors.New("private fixture URL")), "private") {
		t.Fatal("raw navigation error leaked")
	}
	if !strings.Contains(trendNavigationMessage(context.DeadlineExceeded), "超时") {
		t.Fatal("timeout not distinguished")
	}
	if !strings.Contains(trendNavigationMessage(errors.New("net::ERR_CONNECTION_CLOSED private")), "ERR_CONNECTION_CLOSED") {
		t.Fatal("safe network category missing")
	}
}
func TestTrendAttachChromeLifecycle(t *testing.T) {
	executable := os.Getenv("TREND_BROWSER_TEST_EXECUTABLE")
	if executable == "" {
		t.Skip("explicit isolated browser test required")
	}
	root, _ := filepath.EvalSymlinks(t.TempDir())
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	launched, err := newTrendChrome(context.Background(), TrendBrowserConfig{Executable: executable, ProfileRoot: root}, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer launched.Close()
	original := launched.(*trendChrome)
	// Only the synthetic test profile's debugger address is read, never personal profile data.
	address, err := os.ReadFile(filepath.Join(root, "user-1", "DevToolsActivePort"))
	if err != nil {
		t.Fatal("test browser did not advertise debugging endpoint")
	}
	lines := strings.Fields(string(address))
	if len(lines) != 2 {
		t.Fatal("invalid test debugger address")
	}
	endpoint := "ws://127.0.0.1:" + lines[0] + lines[1]
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/delayed-image" {
			select {
			case <-time.After(4 * time.Second):
			case <-r.Context().Done():
			}
			return
		}
		fmt.Fprint(w, "<title>Original fixture tab</title><body>Fixture")
		if r.URL.Path == "/ready" {
			fmt.Fprint(w, `<img src="/delayed-image">`)
		}
	}))
	defer server.Close()
	if err = chromedp.Run(original.ctx, chromedp.Navigate(server.URL), chromedp.Evaluate(`localStorage.setItem('public-fixture','shared-business-state')`, nil)); err != nil {
		t.Fatal(err)
	}
	attached, err := attachTrendChrome(context.Background(), TrendBrowserConfig{Mode: "attach", Endpoint: endpoint, AttachedUserID: 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	remote := attached.(*trendChrome)
	var value string
	if err = chromedp.Run(remote.ctx, chromedp.Navigate(server.URL), chromedp.Evaluate(`localStorage.getItem('public-fixture')`, &value)); err != nil {
		t.Fatal(err)
	}
	if value != "shared-business-state" {
		t.Fatal("attached tab did not reuse original browser context")
	}
	if _, err = remote.Screen(context.Background(), ""); err == nil {
		t.Fatal("attach mode exposed unrelated browser screenshot")
	}
	collectorID := chromedp.FromContext(remote.ctx).Target.TargetID
	attached.Close()
	remaining, err := chromedp.Targets(original.ctx)
	if err != nil {
		t.Fatal("could not inspect synthetic browser")
	}
	found := false
	for _, page := range remaining {
		if page.TargetID == collectorID {
			found = true
		}
	}
	if !found {
		t.Fatal("disconnect closed the Ads tab needed for manual login")
	}
	reconnected, err := attachTrendChrome(context.Background(), TrendBrowserConfig{Mode: "attach", Endpoint: endpoint, AttachedUserID: 1, attachTarget: string(collectorID)}, 1)
	if err != nil {
		t.Fatal("could not reconnect preserved tab")
	}
	if chromedpTargetID(reconnected.(*trendChrome)) != collectorID {
		t.Fatal("resume created a replacement login tab")
	}
	reconnected.Close()
	var title string
	if err = chromedp.Run(original.ctx, chromedp.Title(&title)); err != nil || title != "Original fixture tab" {
		t.Fatal("disconnect closed original browser or user's tab")
	}
	readyCtx, cancelReady := context.WithTimeout(original.ctx, 3*time.Second)
	defer cancelReady()
	if err = navigateTrendPage(readyCtx, server.URL+"/ready"); err != nil {
		t.Fatal("navigation waited for unrelated image resource")
	}
}
