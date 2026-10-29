package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

type trendBrowserDriver interface {
	Open(context.Context, trendQuery) error
	Request(context.Context, string, map[string]any) (map[string]any, error)
	Screen(context.Context, string) (trendScreen, error)
	Input(context.Context, trendInput) error
	Close()
}
type trendScreenTab struct {
	ID     string `json:"id"`
	Origin string `json:"origin"`
}
type trendScreen struct {
	Image  string           `json:"image"`
	Target string           `json:"target"`
	Origin string           `json:"origin"`
	Tabs   []trendScreenTab `json:"tabs"`
	Width  int              `json:"width"`
	Height int              `json:"height"`
}
type trendInput struct {
	Target string  `json:"target"`
	Kind   string  `json:"kind"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Delta  float64 `json:"delta"`
	Text   string  `json:"text"`
	Key    string  `json:"key"`
}
type trendChrome struct {
	external   bool
	mu         sync.Mutex
	ctx        context.Context
	close      context.CancelFunc
	tabs       map[target.ID]context.Context
	tabCancels []context.CancelFunc
	lastScreen time.Time
	lastInput  time.Time
}

func (c *trendChrome) connected() bool {
	b := chromedp.FromContext(c.ctx).Browser
	if b == nil || c.ctx.Err() != nil {
		return false
	}
	select {
	case <-b.LostConnection:
		return false
	default:
		return true
	}
}

func chromedpTargetID(c *trendChrome) target.ID {
	if tab := chromedp.FromContext(c.ctx).Target; tab != nil {
		return tab.TargetID
	}
	return ""
}

// Only fixed categories reach the UI; never expose upstream URLs or raw errors.
func trendNavigationMessage(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "打开 Ads 页面超时，请确认普通 Chrome 能正常访问 Ads 后重试"
	}
	if errors.Is(err, context.Canceled) {
		return "Chrome 连接已中断，请检查 Chrome 中的连接授权后重试"
	}
	for _, code := range []string{"ERR_CONNECTION_CLOSED", "ERR_CONNECTION_RESET", "ERR_PROXY_CONNECTION_FAILED", "ERR_TUNNEL_CONNECTION_FAILED", "ERR_NAME_NOT_RESOLVED", "ERR_INTERNET_DISCONNECTED"} {
		if strings.Contains(err.Error(), code) {
			return "无法访问 Ads（" + code + "），请检查 Chrome 网络后重试"
		}
	}
	return "无法打开 Ads，请确认普通 Chrome 能正常访问 Ads 后重试"
}

func newTrendChrome(request context.Context, cfg TrendBrowserConfig, owner int) (trendBrowserDriver, error) {
	if cfg.Mode == "attach" {
		return attachTrendChrome(request, cfg, owner)
	}
	if cfg.Mode != "" && cfg.Mode != "launch" {
		return nil, errors.New("invalid collector mode")
	}
	if owner <= 0 || !filepath.IsAbs(cfg.Executable) || !filepath.IsAbs(cfg.ProfileRoot) {
		return nil, errors.New("browser paths must be configured")
	}
	if runtime.GOOS == "linux" && (os.Geteuid() == 0 || os.Getenv("DISPLAY") == "") {
		return nil, errors.New("browser requires unprivileged user and display")
	}
	info, err := os.Stat(cfg.Executable)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return nil, errors.New("browser executable unavailable")
	}
	// Administrators provision the private root outside the repository. Never copy a personal profile.
	root, err := filepath.EvalSymlinks(cfg.ProfileRoot)
	if err != nil || root != filepath.Clean(cfg.ProfileRoot) {
		return nil, errors.New("profile root unavailable")
	}
	info, err = os.Stat(root)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("profile root must be private")
	}
	profile := filepath.Join(root, "user-"+strconv.Itoa(owner))
	if err = os.Mkdir(profile, 0700); err != nil && !os.IsExist(err) {
		return nil, errors.New("profile unavailable")
	}
	info, err = os.Lstat(profile)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("unsafe profile directory")
	}
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts, chromedp.ExecPath(cfg.Executable), chromedp.UserDataDir(profile), chromedp.Flag("headless", false), chromedp.WindowSize(1280, 800), chromedp.Flag("force-device-scale-factor", "1"), chromedp.Flag("disable-dev-shm-usage", true))
	// Keep Chromium's sandbox and TLS checks enabled. No stealth flags, copied cookies,
	// password automation, or application authorization headers are sent to Ads.
	alloc, closeAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	tab, closeTab := chromedp.NewContext(alloc, chromedp.WithErrorf(func(string, ...any) { log.Print("trend browser protocol event unavailable") }))
	stopBoot := context.AfterFunc(request, func() { closeTab(); closeAlloc() })
	defer stopBoot()
	bootTimer := time.AfterFunc(35*time.Second, func() { closeTab(); closeAlloc() })
	defer bootTimer.Stop()
	if err = chromedp.Run(tab, chromedp.EmulateViewport(1280, 800), browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorDeny)); err != nil {
		closeTab()
		closeAlloc()
		return nil, errors.New("browser launch failed")
	}
	c := &trendChrome{ctx: tab, tabs: map[target.ID]context.Context{}}
	c.close = func() { closeTab(); closeAlloc() }
	return c, nil
}
func (c *trendChrome) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, cancel := range c.tabCancels {
		cancel()
	}
	c.close()
}
func trendBrowserContext(parent, request context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	stop := context.AfterFunc(request, cancel)
	return ctx, func() { stop(); cancel() }
}
func (c *trendChrome) Open(request context.Context, q trendQuery) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	ctx, cancel := trendBrowserContext(c.ctx, request, 35*time.Second)
	defer cancel()
	// Only managed browsers own all tabs. An attached browser belongs to the user.
	if !c.external {
		targets, err := chromedp.Targets(ctx)
		if err != nil {
			return errors.New("browser unavailable")
		}
		main := chromedp.FromContext(c.ctx).Target.TargetID
		for _, t := range targets {
			if t.Type == "page" && t.TargetID != main {
				if err = chromedp.Run(ctx, target.CloseTarget(t.TargetID)); err != nil {
					return errors.New("previous login tab unavailable")
				}
			}
		}
	}
	q.ID = ""
	return navigateTrendPage(ctx, trendSourceURL(q))
}

func navigateTrendPage(ctx context.Context, destination string) error {
	encoded, _ := json.Marshal(destination)
	ready := `(() => { const expected = new URL(` + string(encoded) + `); return location.origin === expected.origin && location.pathname === expected.pathname && document.readyState !== 'loading' && !!document.body; })()`
	var alreadyReady bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(ready, &alreadyReady)); err == nil && alreadyReady {
		return nil
	}
	return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		_, _, failure, err := page.Navigate(destination).Do(ctx)
		if err != nil {
			return err
		}
		if failure != "" {
			return errors.New(failure)
		}
		return nil
	}), chromedp.Poll(ready, nil, chromedp.WithPollingInterval(200*time.Millisecond)))
}
func allowedTrendEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Port() != "" {
		return false
	}
	if u.Host != "ads.tiktok.com" && u.Host != "ads.us.tiktok.com" {
		return false
	}
	switch u.Path {
	case "/CreativeOne/KnowledgeAPI/GetHashtagList", "/CreativeOne/KnowledgeAPI/GetHashtagDetail", "/CreativeOne/Report/GetTopContentsOverview", "/CreativeOne/Report/CreativeCenterGetTopContentsList", "/CreativeOne/Report/CreativeCenterGetTopContentsItemDetail":
		return true
	}
	return false
}
func (c *trendChrome) Request(request context.Context, endpoint string, body map[string]any) (map[string]any, error) {
	if !allowedTrendEndpoint(endpoint) {
		return nil, errors.New("unsupported upstream request")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ctx, cancel := trendBrowserContext(c.ctx, request, 22*time.Second)
	defer cancel()
	args, _ := json.Marshal(map[string]any{"url": endpoint, "body": body})
	// Only this fixed program is executed. JSON-encoded data is not executable code.
	// Browser cookies remain inside Chromium; only a bounded business response returns.
	expression := `(async (args) => {
 const controller = new AbortController(); const timer = setTimeout(() => controller.abort(), 18000);
 try {
  const response = await fetch(args.url, { method: args.body === null ? 'GET' : 'POST', credentials: 'include', redirect: 'error', signal: controller.signal, headers: args.body === null ? {} : {'Content-Type':'application/json'}, body: args.body === null ? undefined : JSON.stringify(args.body) });
  if (response.status !== 200) return {status:response.status, body:''};
  if (!response.headers.get('content-type')?.includes('json')) return {status:401,body:''};
  const reader=response.body.getReader();const decoder=new TextDecoder();let size=0,text='';
  while(true) {const {done,value}=await reader.read();if(done)break;size+=value.byteLength;if(size>8388608){await reader.cancel();return {status:502,body:''};}text+=decoder.decode(value,{stream:true});}
  text+=decoder.decode();return {status:200,body:text};
 } catch { return {status:502,body:''}; } finally { clearTimeout(timer); }
})(` + string(args) + `)`
	var result struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	}
	err := chromedp.Run(ctx, chromedp.Evaluate(expression, &result, func(p *cdpruntime.EvaluateParams) *cdpruntime.EvaluateParams { return p.WithAwaitPromise(true) }))
	if err != nil {
		return nil, errors.New("browser request unavailable")
	}
	if result.Status == 401 || result.Status == 403 {
		return nil, errTrendLogin
	}
	if result.Status != 200 || len(result.Body) > trendCacheMaxBytes {
		return nil, errors.New("upstream unavailable")
	}
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewBufferString(result.Body))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return nil, errors.New("invalid business response")
	}
	base, _ := value["BaseResp"].(map[string]any)
	if fmt.Sprint(base["StatusCode"]) != "0" {
		// Unknown business errors are not assumed to be expired authentication.
		return nil, errors.New("upstream business rejected")
	}
	return value, nil
}
func trendLoginOrigin(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return ""
	}
	switch u.Host {
	case "ads.tiktok.com", "ads.us.tiktok.com", "accounts.google.com", "business-sso.tiktok.com", "business.tiktok.com":
		return "https://" + u.Host
	}
	return ""
}
func (c *trendChrome) loginTarget(ctx context.Context, id string) (context.Context, []trendScreenTab, string, error) {
	targets, err := chromedp.Targets(ctx)
	if err != nil {
		return nil, nil, "", errors.New("browser unavailable")
	}
	var tabs []trendScreenTab
	selected := target.ID(id)
	origin := ""
	for _, t := range targets {
		if t.Type != "page" {
			continue
		}
		o := trendLoginOrigin(t.URL)
		if o == "" {
			continue
		}
		tabs = append(tabs, trendScreenTab{ID: string(t.TargetID), Origin: o})
		if id == "" && (selected == "" || t.OpenerID != "") {
			selected = t.TargetID
			origin = o
		}
		if id != "" && t.TargetID == selected {
			origin = o
		}
	}
	if origin == "" {
		return nil, nil, "", errors.New("login target unavailable")
	}
	main := chromedp.FromContext(c.ctx).Target.TargetID
	tab := c.ctx
	if selected != main {
		tab = c.tabs[selected]
		if tab == nil {
			var cancel context.CancelFunc
			tab, cancel = chromedp.NewContext(c.ctx, chromedp.WithTargetID(selected))
			c.tabs[selected] = tab
			c.tabCancels = append(c.tabCancels, cancel)
		}
	}
	return tab, tabs, origin, nil
}
func (c *trendChrome) Screen(request context.Context, id string) (trendScreen, error) {
	if c.external {
		return trendScreen{}, errors.New("sign in directly in Chrome")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var result trendScreen
	if time.Since(c.lastScreen) < 500*time.Millisecond {
		return result, errors.New("screen rate limit")
	}
	c.lastScreen = time.Now()
	ctx, cancel := trendBrowserContext(c.ctx, request, 8*time.Second)
	defer cancel()
	tab, tabs, origin, err := c.loginTarget(ctx, id)
	if err != nil {
		return result, err
	}
	frameCtx, frameCancel := trendBrowserContext(tab, ctx, 7*time.Second)
	defer frameCancel()
	var jpeg []byte
	err = chromedp.Run(frameCtx, chromedp.EmulateViewport(1280, 800), chromedp.ActionFunc(func(ctx context.Context) error {
		var e error
		jpeg, e = page.CaptureScreenshot().WithFormat(page.CaptureScreenshotFormatJpeg).WithQuality(65).Do(ctx)
		return e
	}))
	if err != nil || len(jpeg) > 2<<20 {
		return result, errors.New("screen unavailable")
	}
	result = trendScreen{Image: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpeg), Target: string(chromedp.FromContext(tab).Target.TargetID), Origin: origin, Tabs: tabs, Width: 1280, Height: 800}
	return result, nil
}
func validTrendInput(v trendInput) bool {
	if len(v.Target) == 0 || len(v.Target) > 128 || math.IsNaN(v.X) || math.IsNaN(v.Y) || math.IsNaN(v.Delta) {
		return false
	}
	switch v.Kind {
	case "click":
		return v.X >= 0 && v.X < 1280 && v.Y >= 0 && v.Y < 800 && v.Text == "" && v.Key == ""
	case "wheel":
		return v.X >= 0 && v.X < 1280 && v.Y >= 0 && v.Y < 800 && math.Abs(v.Delta) <= 800 && v.Text == "" && v.Key == ""
	case "text":
		if len(v.Text) == 0 || len(v.Text) > 512 || v.Key != "" {
			return false
		}
		for _, r := range v.Text {
			if unicode.IsControl(r) {
				return false
			}
		}
		return true
	case "key":
		return v.Text == "" && contains([]string{"Enter", "Tab", "Backspace", "Delete", "Escape", "ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Home", "End"}, v.Key)
	}
	return false
}
func (c *trendChrome) Input(request context.Context, v trendInput) error {
	if c.external {
		return errors.New("sign in directly in Chrome")
	}
	if !validTrendInput(v) {
		return errors.New("unsupported input")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.lastInput) < 15*time.Millisecond {
		return errors.New("input rate limit")
	}
	c.lastInput = time.Now()
	ctx, cancel := trendBrowserContext(c.ctx, request, 8*time.Second)
	defer cancel()
	tab, _, _, err := c.loginTarget(ctx, v.Target)
	if err != nil {
		return err
	}
	inputCtx, inputCancel := trendBrowserContext(tab, ctx, 7*time.Second)
	defer inputCancel()
	var actions []chromedp.Action
	switch v.Kind {
	case "click":
		actions = []chromedp.Action{input.DispatchMouseEvent(input.MousePressed, v.X, v.Y).WithButton(input.Left).WithClickCount(1), input.DispatchMouseEvent(input.MouseReleased, v.X, v.Y).WithButton(input.Left).WithClickCount(1)}
	case "wheel":
		actions = []chromedp.Action{input.DispatchMouseEvent(input.MouseWheel, v.X, v.Y).WithDeltaY(v.Delta)}
	case "text":
		actions = []chromedp.Action{input.InsertText(v.Text)}
	case "key":
		codes := map[string]int64{"Enter": 13, "Tab": 9, "Backspace": 8, "Delete": 46, "Escape": 27, "ArrowLeft": 37, "ArrowUp": 38, "ArrowRight": 39, "ArrowDown": 40, "Home": 36, "End": 35}
		actions = []chromedp.Action{input.DispatchKeyEvent(input.KeyDown).WithKey(v.Key).WithWindowsVirtualKeyCode(codes[v.Key]), input.DispatchKeyEvent(input.KeyUp).WithKey(v.Key).WithWindowsVirtualKeyCode(codes[v.Key])}
	}
	if err = chromedp.Run(inputCtx, actions...); err != nil {
		return errors.New("input unavailable")
	}
	return nil
}
func (a *app) trendLoginScreen(w http.ResponseWriter, r *http.Request) {
	s, job := a.ownedTrendJob(w, r)
	if job == nil {
		return
	}
	if job.State != "needs_login" || job.LoginMode == "external" {
		writeError(w, 409, 40912, "当前任务无需登录")
		return
	}
	s.mu.Lock()
	driver := s.browser
	s.mu.Unlock()
	if driver == nil {
		writeError(w, 503, 50310, "采集浏览器不可用")
		return
	}
	screen, err := driver.Screen(r.Context(), r.URL.Query().Get("target"))
	if err != nil {
		writeError(w, 503, 50310, "登录画面暂时不可用，请重试")
		return
	}
	w.Header().Set("Pragma", "no-cache")
	writeOK(w, screen)
}
func (a *app) trendLoginInput(w http.ResponseWriter, r *http.Request) {
	s, job := a.ownedTrendJob(w, r)
	if job == nil {
		return
	}
	if job.State != "needs_login" || job.LoginMode == "external" {
		writeError(w, 409, 40912, "当前任务无需登录")
		return
	}
	var v trendInput
	if !decodeTrendControl(w, r, &v) {
		return
	}
	if !validTrendInput(v) {
		writeError(w, 400, 10001, "不支持该操作")
		return
	}
	s.mu.Lock()
	driver := s.browser
	s.mu.Unlock()
	if driver == nil {
		writeError(w, 503, 50310, "采集浏览器不可用")
		return
	}
	// Input and screenshots must never be logged, persisted, or included in audit payloads.
	if err := driver.Input(r.Context(), v); err != nil {
		writeError(w, 503, 50310, "登录操作未完成，请重试")
		return
	}
	writeOK(w, map[string]bool{"accepted": true})
}

func (a *app) trendLoginRestart(w http.ResponseWriter, r *http.Request) {
	s, job := a.ownedTrendJob(w, r)
	if job == nil {
		return
	}
	if job.State != "needs_login" || job.LoginMode == "external" {
		writeError(w, 409, 40912, "当前任务无需登录")
		return
	}
	s.mu.Lock()
	driver := s.browser
	s.mu.Unlock()
	if driver == nil {
		writeError(w, 503, 50310, "采集浏览器不可用")
		return
	}
	if err := driver.Open(r.Context(), job.query); err != nil {
		writeError(w, 503, 50310, "无法重新打开登录页，请检查网络")
		return
	}
	writeOK(w, map[string]bool{"opened": true})
}

// Chrome 144+ supports a user-approved remote connection to a running browser.
// Never launch/close that browser or attach to an unrelated user's tab. A fresh
// collection tab shares the existing default browser context and its Ads session.
func validTrendAttachEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "ws" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Port() == "" {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return false
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1024 || port > 65535 {
		return false
	}
	return u.Path == "/devtools/browser" || (strings.HasPrefix(u.Path, "/devtools/browser/") && len(strings.TrimPrefix(u.Path, "/devtools/browser/")) > 0 && !strings.Contains(strings.TrimPrefix(u.Path, "/devtools/browser/"), "/"))
}
func attachTrendChrome(request context.Context, cfg TrendBrowserConfig, owner int) (trendBrowserDriver, error) {
	if owner <= 0 || owner != cfg.AttachedUserID || !validTrendAttachEndpoint(cfg.Endpoint) {
		return nil, errors.New("existing Chrome connection not authorized")
	}
	// Own the transport separately: RemoteAllocator's normal cancellation closes
	// its target. Manual login requires preserving that tab, so disconnect the
	// transport before cancelling chromedp's tab context.
	connection, disconnect := context.WithCancel(context.Background())
	stopBoot := context.AfterFunc(request, disconnect)
	defer stopBoot()
	b, err := chromedp.NewBrowser(connection, cfg.Endpoint, chromedp.WithDialTimeout(45*time.Second), chromedp.WithBrowserErrorf(func(string, ...any) { log.Print("trend attached browser protocol event unavailable") }))
	if err != nil {
		disconnect()
		return nil, errors.New("existing Chrome unavailable")
	}
	alloc, closeAlloc := chromedp.NewRemoteAllocator(context.Background(), cfg.Endpoint, chromedp.NoModifyURL)
	chromedp.FromContext(alloc).Browser = b
	opts := []chromedp.ContextOption{}
	if cfg.attachTarget != "" {
		check, cancel := context.WithTimeout(connection, 3*time.Second)
		info, lookupErr := target.GetTargetInfo().WithTargetID(target.ID(cfg.attachTarget)).Do(cdp.WithExecutor(check, b))
		cancel()
		if lookupErr == nil && info.Type == "page" {
			opts = append(opts, chromedp.WithTargetID(info.TargetID))
		}
	}
	tab, closeTab := chromedp.NewContext(alloc, opts...)
	var once sync.Once
	closeConnection := func() {
		once.Do(func() {
			disconnect()
			<-b.LostConnection
			closeTab()
			closeAlloc()
		})
	}
	// Initialize on the durable context: cancelling a temporary first-Run context
	// would stop chromedp's target event loop immediately after attachment.
	stopInit := context.AfterFunc(request, closeConnection)
	defer stopInit()
	initTimer := time.AfterFunc(35*time.Second, closeConnection)
	defer initTimer.Stop()
	if err := chromedp.Run(tab); err != nil {
		closeConnection()
		return nil, errors.New("existing Chrome unavailable")
	}
	return &trendChrome{ctx: tab, external: true, tabs: map[target.ID]context.Context{}, close: closeConnection}, nil
}
