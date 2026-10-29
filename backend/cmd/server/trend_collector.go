package main

// The collector runs in the backend process, so it has no public CDP port or
// separate machine credential. Every operation is scoped by the authenticated user.
import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TrendBrowserConfig struct {
	attachTarget   string
	Mode           string `mapstructure:"mode"`
	Endpoint       string `mapstructure:"endpoint"`
	AttachedUserID int    `mapstructure:"attached_user_id"`
	DesktopEnabled bool   `mapstructure:"desktop_enabled"`
	Enabled        bool   `mapstructure:"enabled"`
	Executable     string `mapstructure:"executable"`
	ProfileRoot    string `mapstructure:"profile_root"`
	MaxSessions    int    `mapstructure:"max_sessions"`
}

type trendOwnerKey struct{}

func trendOwner(r *http.Request) int { id, _ := r.Context().Value(trendOwnerKey{}).(int); return id }
func (a *app) requireTrend(next http.HandlerFunc) http.HandlerFunc {
	return a.requireMenu(trendMenu, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(canonicalAPIPath(r.URL.Path), "/business/trends/collect") && !trendControlTransport(r) {
			writeError(w, 403, 40310, "远程登录需要 HTTPS 或本机连接")
			return
		}
		// requireMenu has already validated this token against the session database.
		token, ok := accessTokenFromRequest(r)
		if !ok {
			writeError(w, 401, 401, "请重新登录后台")
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		next(w, r.WithContext(context.WithValue(r.Context(), trendOwnerKey{}, token.UserID)))
	})
}
func trendControlTransport(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(host)
	return err == nil && ip != nil && ip.IsLoopback()
}

func (a *app) trendBrowserConfig() TrendBrowserConfig {
	if cfg := a.config.Load(); cfg != nil {
		return cfg.(Config).TrendBrowser
	}
	return TrendBrowserConfig{}
}
func (a *app) trendRedis() (trendHotCache, RedisConfig) {
	cfg := RedisConfig{}.withDefaults()
	if c := a.config.Load(); c != nil {
		cfg = c.(Config).Redis.withDefaults()
	}
	if a.trendCacheOverride != nil {
		return a.trendCacheOverride, cfg
	}
	return redisTrendCache{cfg}, cfg
}
func trendDatasetKey(owner int, q trendQuery, bucket, prefix string) string {
	q.Live, q.Page, q.Size, q.ID = true, 1, 20, ""
	return trendHotKey("browser-dataset", q, "", bucket, prefix+"user:"+strconv.Itoa(owner)+":")
}

type trendDataset struct {
	Rows       []map[string]any          `json:"rows"`
	Details    map[string]map[string]any `json:"details"`
	CapturedAt string                    `json:"capturedAt"`
	CurveEnd   string                    `json:"curveEnd"`
	ExpiresAt  time.Time                 `json:"expiresAt"`
}

type trendJob struct {
	DesktopAvailable bool   `json:"desktopAvailable"`
	LoginMode        string `json:"loginMode"`
	resume           chan struct{}
	ID               string    `json:"id"`
	State            string    `json:"state"`
	Message          string    `json:"message"`
	Completed        int       `json:"completed"`
	Total            int       `json:"total"`
	Cache            string    `json:"cache"`
	ExpiresAt        time.Time `json:"expiresAt"`
	query            trendQuery
	cancel           context.CancelFunc
}

func trendTerminal(state string) bool {
	return state == "complete" || state == "failed" || state == "cancelled"
}

type trendBrowserSession struct {
	attachTarget string
	mu           sync.Mutex
	job          *trendJob
	browser      trendBrowserDriver
	lastUsed     time.Time
}
type trendBrowserManager struct {
	stopping bool
	mu       sync.Mutex
	sessions map[int]*trendBrowserSession
	factory  func(context.Context, TrendBrowserConfig, int) (trendBrowserDriver, error)
}

func (a *app) trendManager() *trendBrowserManager {
	a.trendBrowserOnce.Do(func() {
		a.trendBrowsers = &trendBrowserManager{sessions: map[int]*trendBrowserSession{}, factory: newTrendChrome}
	})
	return a.trendBrowsers
}
func (m *trendBrowserManager) session(owner, limit int, create bool) (*trendBrowserSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopping {
		return nil, errors.New("collector stopping")
	}
	if limit < 1 || limit > 8 {
		limit = 2
	}
	// Idle/finished browsers are closed before allocating another slot. No profile is deleted.
	for id, s := range m.sessions {
		s.mu.Lock()
		idle := time.Since(s.lastUsed) > 30*time.Minute && (s.job == nil || trendTerminal(s.job.State))
		if idle {
			if s.browser != nil {
				s.browser.Close()
			}
			delete(m.sessions, id)
		}
		s.mu.Unlock()
	}
	if s := m.sessions[owner]; s != nil {
		return s, nil
	}
	if !create {
		return nil, errors.New("missing session")
	}
	if len(m.sessions) >= limit {
		return nil, errors.New("collector capacity reached")
	}
	s := &trendBrowserSession{lastUsed: time.Now()}
	m.sessions[owner] = s
	time.AfterFunc(31*time.Minute, func() { m.expire(owner, s) })
	return s, nil
}
func (s *trendBrowserSession) update(id, state, message string, completed int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job == nil || s.job.ID != id || trendTerminal(s.job.State) {
		return
	}
	s.job.State, s.job.Message, s.job.Completed = state, message, completed
}
func (a *app) trendCollectStart(w http.ResponseWriter, r *http.Request) {
	cfg := a.trendBrowserConfig()
	if !cfg.Enabled {
		writeError(w, 503, 50310, "采集浏览器尚未启用，请联系管理员完成配置")
		return
	}
	q, err := parseTrendQuery(r.URL.Query(), false)
	if err != nil {
		writeError(w, 400, 10001, "筛选条件不正确")
		return
	}
	q.Live, q.Page, q.Size, q.ID = true, 1, 20, ""
	owner := trendOwner(r)
	if cfg.Mode == "attach" && (cfg.AttachedUserID <= 0 || owner != cfg.AttachedUserID) {
		writeError(w, 403, 40311, "当前用户未获授权连接此 Chrome 会话")
		return
	}
	if owner <= 0 {
		writeError(w, 401, 401, "请重新登录后台")
		return
	}
	m := a.trendManager()
	s, err := m.session(owner, cfg.MaxSessions, true)
	if err != nil {
		writeError(w, 429, 42910, "采集浏览器正在使用中，请稍后重试")
		return
	}
	s.mu.Lock()
	if s.job != nil && !trendTerminal(s.job.State) {
		if s.job.query != q {
			s.mu.Unlock()
			writeError(w, 409, 40910, "请先完成或取消当前采集")
			return
		}
		job := *s.job
		s.mu.Unlock()
		writeOK(w, job)
		return
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		s.mu.Unlock()
		writeError(w, 503, 50310, "无法创建采集任务")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	job := &trendJob{ID: hex.EncodeToString(nonce[:]), State: "starting", Message: "正在启动采集浏览器", Total: 100, ExpiresAt: time.Now().Add(30 * time.Minute), query: q, cancel: cancel}
	if cfg.Mode == "attach" {
		job.LoginMode = "external"
		job.DesktopAvailable = cfg.DesktopEnabled
		job.Message = "正在连接已打开的 Chrome，请在 Chrome 中允许连接"
		job.resume = make(chan struct{}, 1)
	}
	s.job, s.lastUsed = job, time.Now()
	response := *job
	s.mu.Unlock()
	go a.runTrendCollection(ctx, owner, s, response, cfg)
	writeOK(w, response)
}
func (a *app) ownedTrendJob(w http.ResponseWriter, r *http.Request) (*trendBrowserSession, *trendJob) {
	s, err := a.trendManager().session(trendOwner(r), a.trendBrowserConfig().MaxSessions, false)
	if err != nil {
		writeError(w, 404, 40410, "采集任务不存在")
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job == nil || s.job.ID != r.PathValue("job") {
		writeError(w, 404, 40410, "采集任务不存在")
		return nil, nil
	}
	s.lastUsed = time.Now()
	job := *s.job
	return s, &job
}
func (a *app) trendCollectStatus(w http.ResponseWriter, r *http.Request) {
	_, job := a.ownedTrendJob(w, r)
	if job != nil {
		writeOK(w, job)
	}
}
func (a *app) trendCollectCancel(w http.ResponseWriter, r *http.Request) {
	s, job := a.ownedTrendJob(w, r)
	if job == nil {
		return
	}
	s.mu.Lock()
	if s.job.ID == job.ID && !trendTerminal(s.job.State) {
		s.job.State, s.job.Message = "cancelled", "已取消采集"
		s.job.cancel()
	}
	s.mu.Unlock()
	writeOK(w, map[string]bool{"cancelled": true})
}

// A single Redis SET publishes a validated dataset, including its details. Partial
// anonymous pages and failed detail runs never overwrite a complete dataset.
func (a *app) runTrendCollection(ctx context.Context, owner int, s *trendBrowserSession, job trendJob, cfg TrendBrowserConfig) {
	defer job.cancel()
	defer func() {
		if ctx.Err() != nil {
			s.update(job.ID, "failed", "采集或登录等待已超时，请重试", 0)
		}
	}()
	s.mu.Lock()
	driver := s.browser
	if cfg.Mode == "attach" {
		cfg.attachTarget = s.attachTarget
	}
	s.mu.Unlock()
	if chrome, ok := driver.(*trendChrome); ok && chrome.external && !chrome.connected() {
		cfg.attachTarget = string(chromedpTargetID(chrome))
		chrome.Close()
		driver = nil
		s.mu.Lock()
		s.browser = nil
		s.mu.Unlock()
	}
	if driver == nil {
		var err error
		driver, err = a.trendManager().factory(ctx, cfg, owner)
		if err != nil {
			s.update(job.ID, "failed", "采集浏览器不可用，请检查配置；连接现有 Chrome 时，请先打开 Chrome 并允许远程调试连接", 0)
			return
		}
		s.mu.Lock()
		if ctx.Err() != nil || s.job.ID != job.ID || trendTerminal(s.job.State) {
			s.mu.Unlock()
			driver.Close()
			return
		}
		s.browser = driver
		s.mu.Unlock()
	}
	if err := driver.Open(ctx, job.query); err != nil {
		if cfg.Mode != "attach" {
			driver.Close()
			s.mu.Lock()
			s.browser = nil
			s.mu.Unlock()
		}
		s.update(job.ID, "failed", trendNavigationMessage(err), 0)
		return
	}
	ctx = context.WithValue(ctx, trendRequesterKey{}, trendRequester(driver.Request))
	for {
		s.update(job.ID, "collecting", "正在读取榜单与详情", 0)
		dataset, err := collectTrendDataset(ctx, job.query, func(n int) { s.update(job.ID, "collecting", "正在读取榜单与详情", n) })
		if errors.Is(err, errTrendLogin) {
			if cfg.Mode == "attach" {
				// End the debugging connection before asking the user to sign in normally.
				if chrome, ok := driver.(*trendChrome); ok {
					cfg.attachTarget = string(chromedpTargetID(chrome))
				}
				driver.Close()
				s.mu.Lock()
				s.browser = nil
				s.mu.Unlock()
				s.update(job.ID, "needs_login", "已断开采集连接。请在普通 Chrome 完成 Ads 登录，然后点击继续采集", 0)
				select {
				case <-ctx.Done():
					return
				case <-job.resume:
				}
				var reconnectErr error
				driver, reconnectErr = a.trendManager().factory(ctx, cfg, owner)
				if reconnectErr != nil {
					s.update(job.ID, "failed", "无法连接 Chrome，请在 Chrome 中允许连接后重试", 0)
					return
				}
				s.mu.Lock()
				if ctx.Err() != nil || s.job.ID != job.ID || trendTerminal(s.job.State) {
					s.mu.Unlock()
					driver.Close()
					return
				}
				s.browser = driver
				s.mu.Unlock()
				if reconnectErr = driver.Open(ctx, job.query); reconnectErr != nil {
					s.update(job.ID, "failed", trendNavigationMessage(reconnectErr), 0)
					return
				}
				ctx = context.WithValue(ctx, trendRequesterKey{}, trendRequester(driver.Request))
				continue
			}
			s.update(job.ID, "needs_login", "请在登录窗口完成 Ads 的 Google 登录；成功后将自动继续", 0)
			// Probe a fixed read-only business endpoint; never repeatedly navigate the login tab.
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(8 * time.Second):
				}
				probe := job.query
				probe.Page, probe.Size = 1, 20
				rows, _, _, probeErr := fetchTrendList(ctx, probe)
				if probeErr == nil && len(rows) > 4 {
					break
				}
				if probeErr != nil && !errors.Is(probeErr, errTrendLogin) {
					s.update(job.ID, "failed", "暂时无法检查登录状态，请稍后重试", 0)
					return
				}
			}
			continue
		}
		if err != nil {
			s.update(job.ID, "failed", "采集未完整完成，已保留上次结果；请稍后重试", 0)
			return
		}
		cache, redisCfg := a.trendRedis()
		bucket, expires, err := trendCacheNode(time.Now(), redisCfg)
		if err != nil {
			s.update(job.ID, "failed", "缓存过期节点配置不正确", 0)
			return
		}
		dataset.ExpiresAt = expires
		data, err := json.Marshal(dataset)
		if err != nil || len(data) > trendCacheMaxBytes {
			s.update(job.ID, "failed", "采集结果超过缓存容量，已保留上次结果", 0)
			return
		}
		// Serialize cancellation/publication: a cancelled or superseded job cannot publish.
		s.mu.Lock()
		if ctx.Err() != nil || s.job.ID != job.ID || trendTerminal(s.job.State) {
			s.mu.Unlock()
			return
		}
		err = cache.Set(ctx, trendDatasetKey(owner, job.query, bucket, redisCfg.KeyPrefix), data, expires)
		if err != nil {
			s.job.State, s.job.Message, s.job.Cache = "failed", "采集完成，但 Redis 写入失败；请恢复 Redis 后重试", "UNAVAILABLE"
		} else {
			s.job.State, s.job.Message, s.job.Cache, s.job.Completed = "complete", "榜单与详情已更新", "UPDATED", len(dataset.Rows)
		}
		s.mu.Unlock()
		return
	}
}

var errTrendLogin = errors.New("ads login required")

type trendRequester func(context.Context, string, map[string]any) (map[string]any, error)
type trendRequesterKey struct{}

func collectTrendDataset(ctx context.Context, q trendQuery, progress func(int)) (*trendDataset, error) {
	d := &trendDataset{Details: map[string]map[string]any{}, CapturedAt: time.Now().UTC().Format(time.RFC3339)}
	seen := map[string]bool{}
	q.Page, q.Size = 1, 20
	for len(d.Rows) < 100 {
		rows, pagination, end, err := fetchTrendList(ctx, q)
		if err != nil {
			return nil, err
		}
		if q.Page == 1 && len(rows) <= 4 {
			return nil, errTrendLogin
		}
		if trendInt(pagination["page"]) != q.Page || len(rows) == 0 || len(rows) > 20 {
			return nil, errors.New("invalid upstream pagination")
		}
		for _, row := range rows {
			key := "hashtagID"
			if q.Tab == "video" {
				key = "itemID"
			}
			id := fmt.Sprint(row[key])
			if seen[id] {
				return nil, errors.New("duplicate upstream page")
			}
			seen[id] = true
			d.Rows = append(d.Rows, sanitizeTrendBusiness(row))
		}
		d.CurveEnd = end
		if len(d.Rows) >= 100 {
			break
		}
		if pagination["hasMore"] != true {
			return nil, errors.New("incomplete upstream ranking")
		}
		q.Page++
	}
	if len(d.Rows) != 100 {
		return nil, errors.New("unexpected ranking size")
	}
	for i, row := range d.Rows {
		var detail map[string]any
		var err error
		if q.Tab == "tag" {
			id := fmt.Sprint(row["hashtagID"])
			detail, err = trendRequest(ctx, "https://ads.tiktok.com/CreativeOne/KnowledgeAPI/GetHashtagDetail", map[string]any{"hashtagID": id, "timeRange": q.Period, "countryCode": q.Region})
			if err == nil && fmt.Sprint(detail["hashtagID"]) != id {
				err = errors.New("detail ID mismatch")
			}
			if err == nil {
				d.Details[id] = sanitizeTrendBusiness(detail)
			}
		} else {
			id := fmt.Sprint(row["itemID"])
			var timestamp int64
			timestamp, err = trendVideoEnd(ctx, q.Region)
			if err == nil {
				params := url.Values{"itemID": {id}, "periodDimension": {trendVideoDimension(q.Period)}, "periodEndTimestamp": {strconv.FormatInt(timestamp, 10)}}
				detail, err = trendRequest(ctx, trendVideoHost(q.Region)+"/CreativeOne/Report/CreativeCenterGetTopContentsItemDetail?"+params.Encode(), nil)
				if entity, ok := detail["entityInfo"].(map[string]any); ok {
					detail = entity
				}
				if err == nil {
					detail = normalizeTrendVideo(detail)
					if fmt.Sprint(detail["itemID"]) != id {
						err = errors.New("detail ID mismatch")
					}
				}
				if err == nil {
					d.Details[id] = sanitizeTrendBusiness(detail)
				}
			}
		}
		if err != nil {
			return nil, err
		}
		progress(i + 1)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
	return d, nil
}

// Keep only documented business fields, recursively. Authentication material and
// unknown upstream fields never enter Redis or frontend responses.
func sanitizeTrendBusiness(value map[string]any) map[string]any {
	allowed := map[string]bool{}
	for _, k := range []string{"hashtagID", "hashtagName", "rankIndex", "publishCnt", "vv", "popularityCurve", "timestamp", "value", "topCreators", "ttUID", "nickname", "handleName", "avatarURL", "followedCnt", "ageProfile", "ageLevel", "vvPercent", "representativeCountryProfile", "countryCode", "countryTgiScore", "videoList", "itemID", "coverURL", "vid", "videoURL", "title", "createTime", "author", "name", "handle", "followers", "metrics", "tags", "contentLabelName", "videoViews", "likes", "comments", "shares", "engagementRate", "engagementRateLifeTime", "organicVideoViews", "organicVideoViewsLifeTime", "sixSecondsVTR", "sixSecondsVTRLifeTime", "videoViewsLifeTime", "sixSecondViewRate", "viewCount", "likeCount", "commentCount", "shareCount", "vvr6s", "vv6s", "finishRate"} {
		allowed[k] = true
	}
	var clean func(any) any
	clean = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			out := map[string]any{}
			for k, val := range x {
				if allowed[k] {
					out[k] = clean(val)
				}
			}
			return out
		case []any:
			out := make([]any, 0, len(x))
			for _, val := range x {
				out = append(out, clean(val))
			}
			return out
		default:
			return x
		}
	}
	return clean(value).(map[string]any)
}

func (a *app) trendBrowserData(w http.ResponseWriter, r *http.Request, q trendQuery, detail bool) {
	cache, cfg := a.trendRedis()
	bucket, _, err := trendCacheNode(time.Now(), cfg)
	if err != nil {
		writeError(w, 503, 50311, "缓存配置不正确")
		return
	}
	raw, err := cache.Get(r.Context(), trendDatasetKey(trendOwner(r), q, bucket, cfg.KeyPrefix))
	if err != nil {
		w.Header().Set("X-Trend-Cache", "UNAVAILABLE")
		writeError(w, 503, 50311, "Redis 暂时不可用")
		return
	}
	var d trendDataset
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if len(raw) == 0 || decoder.Decode(&d) != nil || !d.ExpiresAt.After(time.Now()) {
		writeError(w, 409, 40911, "请点击读取最新，完成采集后查看")
		return
	}
	meta := trendMetadata(q, "", d.CurveEnd)
	meta["capturedAt"] = d.CapturedAt
	meta["coverage"] = "complete"
	w.Header().Set("X-Trend-Cache", "HIT")
	w.Header().Set("X-Trend-Cache-Expires-At", d.ExpiresAt.Format(time.RFC3339))
	if detail {
		item, ok := d.Details[q.ID]
		if !ok {
			writeError(w, 404, 40411, "该内容不在本次采集结果中")
			return
		}
		writeOK(w, map[string]any{"detail": item, "meta": meta})
		return
	}
	start := min((q.Page-1)*q.Size, len(d.Rows))
	end := min(start+q.Size, len(d.Rows))
	writeOK(w, map[string]any{"list": d.Rows[start:end], "total": len(d.Rows), "currentPage": q.Page, "pageSize": q.Size, "hasMore": end < len(d.Rows), "meta": meta})
}

func decodeTrendControl(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		writeError(w, 415, 10001, "请使用 JSON 请求")
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	dec.DisallowUnknownFields()
	if dec.Decode(v) != nil || dec.Decode(&struct{}{}) != io.EOF {
		writeError(w, 400, 10001, "操作参数不正确")
		return false
	}
	return true
}

func (m *trendBrowserManager) expire(owner int, s *trendBrowserSession) {
	m.mu.Lock()
	if m.sessions[owner] != s {
		m.mu.Unlock()
		return
	}
	s.mu.Lock()
	idle := time.Since(s.lastUsed)
	if idle >= 30*time.Minute && (s.job == nil || trendTerminal(s.job.State)) {
		delete(m.sessions, owner)
		driver := s.browser
		s.mu.Unlock()
		m.mu.Unlock()
		if driver != nil {
			driver.Close()
		}
		return
	}
	s.mu.Unlock()
	m.mu.Unlock()
	time.AfterFunc(time.Minute, func() { m.expire(owner, s) })
}

func (a *app) closeTrendBrowsers() {
	m := a.trendManager()
	m.mu.Lock()
	m.stopping = true
	var drivers []trendBrowserDriver
	for owner, s := range m.sessions {
		s.mu.Lock()
		if s.job != nil && !trendTerminal(s.job.State) {
			s.job.State = "cancelled"
			s.job.cancel()
		}
		if s.browser != nil {
			drivers = append(drivers, s.browser)
		}
		s.mu.Unlock()
		delete(m.sessions, owner)
	}
	m.mu.Unlock()
	for _, driver := range drivers {
		driver.Close()
	}
}

func (a *app) trendCollectResume(w http.ResponseWriter, r *http.Request) {
	s, job := a.ownedTrendJob(w, r)
	if job == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job.State != "needs_login" || s.job.LoginMode != "external" || s.job.resume == nil {
		writeError(w, 409, 40912, "当前任务无需重新连接")
		return
	}
	select {
	case s.job.resume <- struct{}{}:
		s.job.State = "starting"
		s.job.Message = "正在重新连接 Chrome，请允许连接"
	default:
	}
	writeOK(w, *s.job)
}

// Desktop HTTP/WebSocket traffic is served by the authenticated HTTPS gateway,
// not by CDP. This endpoint only prepares an owner-scoped manual-login handoff.
func (a *app) trendDesktopOpen(w http.ResponseWriter, r *http.Request) {
	cfg := a.trendBrowserConfig()
	if !cfg.Enabled || cfg.Mode != "attach" || !cfg.DesktopEnabled {
		writeError(w, 503, 50312, "服务器浏览器窗口尚未配置")
		return
	}
	if trendOwner(r) <= 0 || trendOwner(r) != cfg.AttachedUserID {
		writeError(w, 403, 40311, "当前用户未获授权连接此 Chrome 会话")
		return
	}
	s, job := a.ownedTrendJob(w, r)
	if job == nil {
		return
	}
	s.mu.Lock()
	if s.job.ID != job.ID || (s.job.State != "needs_login" && !trendTerminal(s.job.State)) {
		s.mu.Unlock()
		writeError(w, 409, 40912, "请先完成或取消采集，再打开登录窗口")
		return
	}
	driver := s.browser
	if chrome, ok := driver.(*trendChrome); ok {
		s.attachTarget = string(chromedpTargetID(chrome))
	}
	s.browser = nil
	s.mu.Unlock()
	if driver != nil {
		driver.Close()
	}
	writeOK(w, map[string]string{"url": "/trend-desktop/vnc.html?autoconnect=true&resize=scale&path=trend-desktop/websockify"})
}
