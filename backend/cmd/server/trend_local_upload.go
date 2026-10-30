package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TrendLocalConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	StorageRoot string `mapstructure:"storage_root"`
	ClientDir   string `mapstructure:"client_dir"`
}
type trendLocalJob struct {
	mu           sync.Mutex
	ID           string    `json:"id"`
	State        string    `json:"state"`
	Message      string    `json:"message"`
	ExpiresAt    time.Time `json:"expiresAt"`
	ImagesSaved  int       `json:"imagesSaved"`
	ImagesFailed int       `json:"imagesFailed"`
	owner        int
	query        trendQuery
	secret       [32]byte
	payloadHash  [32]byte
	cancel       context.CancelFunc
	children     []*trendLocalJob
}

func (j *trendLocalJob) status() map[string]any {
	regions := j.regionStatus()
	return map[string]any{"id": j.ID, "state": j.State, "message": j.Message, "expiresAt": j.ExpiresAt, "imagesSaved": j.ImagesSaved, "imagesFailed": j.ImagesFailed, "regions": regions}
}
func (a *app) trendLocalConfig() TrendLocalConfig {
	if c := a.config.Load(); c != nil {
		return c.(Config).TrendLocal
	}
	return TrendLocalConfig{}
}
func (a *app) localJob(id string) *trendLocalJob {
	a.trendLocalMu.Lock()
	defer a.trendLocalMu.Unlock()
	return a.trendLocalJobs[id]
}
func (a *app) trendLocalCreate(w http.ResponseWriter, r *http.Request) {
	if !a.trendLocalConfig().Enabled {
		writeError(w, 503, 50320, "本地采集尚未启用")
		return
	}
	if !trendControlTransport(r) {
		writeError(w, 403, 40320, "本地采集授权需要 HTTPS")
		return
	}
	q, err := parseTrendQuery(r.URL.Query(), false)
	if err != nil {
		writeError(w, 400, 10001, "筛选条件不正确")
		return
	}
	q.Live, q.Page, q.Size, q.ID = true, 1, 20, ""
	queries, err := localTrendRegionQueries(q, r.URL.Query().Get("regions"))
	if err != nil {
		writeError(w, 400, 10001, "请至少选择一个当前榜单支持的地区，不能重复选择")
		return
	}
	owner := trendOwner(r)
	if owner <= 0 {
		writeError(w, 401, 401, "请重新登录后台")
		return
	}
	var id [16]byte
	var secret [32]byte
	if _, err = rand.Read(id[:]); err != nil {
		writeError(w, 503, 50320, "无法创建授权")
		return
	}
	if _, err = rand.Read(secret[:]); err != nil {
		writeError(w, 503, 50320, "无法创建授权")
		return
	}
	token := hex.EncodeToString(secret[:])
	j := &trendLocalJob{ID: hex.EncodeToString(id[:]), State: "waiting_upload", Message: "等待本地采集程序上传", ExpiresAt: time.Now().Add(30 * time.Minute), owner: owner, query: q, secret: sha256.Sum256([]byte(token))}
	if len(queries) > 1 {
		j.ExpiresAt = time.Now().Add(time.Duration(len(queries)) * 30 * time.Minute)
		for _, query := range queries {
			j.children = append(j.children, &trendLocalJob{ID: j.ID + "-" + query.Region, State: "waiting_upload", Message: "等待本地采集", ExpiresAt: j.ExpiresAt, owner: owner, query: query})
		}
	} else {
		j.query = queries[0]
	}
	a.trendLocalMu.Lock()
	if a.trendLocalJobs == nil {
		a.trendLocalJobs = map[string]*trendLocalJob{}
		a.trendLocalSlots = make(chan struct{}, 2)
	}
	for key, old := range a.trendLocalJobs {
		old.mu.Lock()
		old.status()
		if time.Now().After(old.ExpiresAt) {
			old.cancelRegions("授权已过期")
			delete(a.trendLocalJobs, key)
			old.mu.Unlock()
			continue
		}
		if old.owner == owner && old.State == "processing" {
			old.mu.Unlock()
			a.trendLocalMu.Unlock()
			writeError(w, 409, 40920, "服务器正在处理上次上传，请等待完成")
			return
		}
		if old.owner == owner && old.State == "waiting_upload" {
			old.cancelRegions("已被新的采集授权替代")
		}
		old.mu.Unlock()
	}
	if len(a.trendLocalJobs) >= 128 {
		a.trendLocalMu.Unlock()
		writeError(w, 429, 42920, "采集任务已满，请稍后重试")
		return
	}
	a.trendLocalJobs[j.ID] = j
	a.trendLocalMu.Unlock()
	result := j.status()
	result["token"] = token
	writeOK(w, result)
}
func (a *app) ownedLocalJob(w http.ResponseWriter, r *http.Request) *trendLocalJob {
	j := a.localJob(r.PathValue("job"))
	if j == nil || trendOwner(r) <= 0 || j.owner != trendOwner(r) {
		writeError(w, 404, 40420, "采集任务不存在")
		return nil
	}
	return j
}
func (a *app) trendLocalStatus(w http.ResponseWriter, r *http.Request) {
	j := a.ownedLocalJob(w, r)
	if j == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status()
	if time.Now().After(j.ExpiresAt) && j.State == "waiting_upload" {
		j.expireRegions()
		j.State = "failed"
		j.Message = "授权已过期，请重新创建"
	}
	writeOK(w, j.status())
}
func (a *app) trendLocalCancel(w http.ResponseWriter, r *http.Request) {
	j := a.ownedLocalJob(w, r)
	if j == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status()
	if j.State != "complete" {
		j.cancelRegions("已取消")
	}
	writeOK(w, j.status())
}
func (a *app) authorizedLocalUpload(w http.ResponseWriter, r *http.Request) *trendLocalJob {
	if !a.trendLocalConfig().Enabled || !trendControlTransport(r) {
		writeError(w, 403, 40320, "上传不可用或未使用 HTTPS")
		return nil
	}
	j := a.localJob(r.URL.Query().Get("job"))
	auth := r.Header.Get("Authorization")
	if j == nil || !strings.HasPrefix(auth, "Bearer ") || len(auth) != 71 {
		writeError(w, 401, 40120, "上传授权无效")
		return nil
	}
	hash := sha256.Sum256([]byte(strings.TrimPrefix(auth, "Bearer ")))
	if subtle.ConstantTimeCompare(j.secret[:], hash[:]) != 1 || time.Now().After(j.ExpiresAt) {
		writeError(w, 401, 40120, "上传授权无效或已过期")
		return nil
	}
	region := r.URL.Query().Get("region")
	if region == "" && (r.Method == http.MethodGet || len(j.children) == 0) {
		return j
	}
	if len(j.children) == 0 && region == j.query.Region {
		return j
	}
	for _, child := range j.children {
		if child.query.Region == region {
			return child
		}
	}
	writeError(w, 403, 40320, "此地区不在本次采集授权范围内")
	return nil
}
func (a *app) trendLocalUploadStatus(w http.ResponseWriter, r *http.Request) {
	j := a.authorizedLocalUpload(w, r)
	if j == nil {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	j.mu.Lock()
	defer j.mu.Unlock()
	writeOK(w, j.status())
}
func (a *app) trendLocalUpload(w http.ResponseWriter, r *http.Request) {
	j := a.authorizedLocalUpload(w, r)
	if j == nil {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Header.Get("Content-Type") != "application/json" {
		writeError(w, 415, 10001, "请上传 JSON 数据")
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, trendCacheMaxBytes))
	if err != nil {
		writeError(w, 413, 10001, "上传超过大小限制")
		return
	}
	digest := sha256.Sum256(raw)
	j.mu.Lock()
	if r.URL.Query().Get("failed") == "1" {
		defer j.mu.Unlock()
		if j.State != "waiting_upload" {
			writeError(w, 409, 40920, "此地区任务已提交或结束")
			return
		}
		j.State, j.Message = "failed", "本地采集未完成，旧缓存已保留"
		writeOK(w, j.status())
		return
	}
	if j.State != "waiting_upload" {
		if (j.State == "processing" || j.State == "complete") && j.payloadHash == digest {
			writeOK(w, j.status())
		} else {
			writeError(w, 409, 40920, "此授权已使用或取消，请重新创建")
		}
		j.mu.Unlock()
		return
	}
	var d trendDataset
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if decoder.Decode(&d) != nil || decoder.Decode(&struct{}{}) != io.EOF || validateLocalDataset(&d, j.query, time.Now()) != nil {
		j.mu.Unlock()
		writeError(w, 400, 10001, "榜单或详情不完整，未覆盖原数据")
		return
	}
	select {
	case a.trendLocalSlots <- struct{}{}:
	default:
		j.mu.Unlock()
		writeError(w, 429, 42920, "服务器正在处理其他上传，请稍后重试")
		return
	}
	j.payloadHash = digest
	j.State = "processing"
	j.Message = "已接收数据，服务器正在保存图片"
	ctx, cancel := context.WithDeadline(context.Background(), j.ExpiresAt)
	j.cancel = cancel
	receipt := j.status()
	j.mu.Unlock()
	go func() { defer func() { <-a.trendLocalSlots }(); a.publishLocalDataset(ctx, j, &d) }()
	writeOK(w, receipt)
}
func validateLocalDataset(d *trendDataset, q trendQuery, now time.Time) error {
	if len(d.Rows) != 100 || len(d.Details) != 100 {
		return errors.New("incomplete dataset")
	}
	captured, err := time.Parse(time.RFC3339, d.CapturedAt)
	if err != nil || captured.After(now.Add(5*time.Minute)) || captured.Before(now.Add(-time.Hour)) {
		return errors.New("invalid capture time")
	}
	seen := map[string]bool{}
	key := "hashtagID"
	if q.Tab == "video" {
		key = "itemID"
	}
	for i, row := range d.Rows {
		id := fmt.Sprint(row[key])
		detail, ok := d.Details[id]
		if !trendIDPattern.MatchString(id) || seen[id] || !ok || fmt.Sprint(detail[key]) != id {
			return errors.New("mismatched rows")
		}
		seen[id] = true
		d.Rows[i] = sanitizeTrendBusiness(row)
		d.Details[id] = sanitizeTrendBusiness(detail)
	}
	if d.CurveEnd != "" {
		if _, err = time.Parse("2006-01-02", d.CurveEnd); err != nil {
			return errors.New("invalid business date")
		}
	}
	return nil
}
func (a *app) publishLocalDataset(ctx context.Context, j *trendLocalJob, d *trendDataset) {
	defer j.cancel()
	fail := func(message string) {
		j.mu.Lock()
		defer j.mu.Unlock()
		if j.State != "cancelled" {
			j.State = "failed"
			j.Message = message
		}
	}
	root, err := localTrendOwnerRoot(a.trendLocalConfig().StorageRoot, j.owner)
	if err != nil {
		fail("服务器存储目录不可用，旧数据已保留")
		return
	}
	saved, failed := saveLocalTrendImages(ctx, root, d)
	if ctx.Err() != nil {
		fail("处理超时或已取消，旧数据已保留")
		return
	}
	cache, cfg := a.trendRedis()
	bucket, expires, err := trendCacheNode(time.Now(), cfg)
	if err != nil {
		fail("缓存过期配置无效")
		return
	}
	d.ExpiresAt = expires
	raw, err := json.Marshal(d)
	if err != nil || len(raw) > trendCacheMaxBytes {
		fail("结果超过缓存容量")
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.State == "cancelled" || ctx.Err() != nil {
		return
	}
	j.ImagesSaved, j.ImagesFailed = saved, failed
	if err = writeLocalTrendFile(root, "dataset-"+j.ID+".json", raw); err != nil {
		j.State = "failed"
		j.Message = "服务器保存结果失败，旧缓存已保留"
		return
	}
	if err = cache.Set(ctx, trendDatasetKey(j.owner, j.query, bucket, cfg.KeyPrefix), raw, expires); err != nil {
		j.State = "failed"
		j.Message = "结果已保存，但 Redis 更新失败，请重新创建上传任务"
		return
	}
	j.State = "complete"
	j.Message = "完整榜单及详情已更新"
	if failed > 0 {
		j.Message = fmt.Sprintf("榜单已更新，%d 张图片未保存，可重新采集重试", failed)
	}
}
func localTrendOwnerRoot(root string, owner int) (string, error) {
	if owner <= 0 || root == "" {
		return "", errors.New("invalid storage")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(absolute, 0700); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil || resolved != absolute {
		return "", errors.New("unsafe root")
	}
	own := filepath.Join(absolute, "user-"+strconv.Itoa(owner))
	if err = os.MkdirAll(own, 0700); err != nil {
		return "", err
	}
	st, err := os.Lstat(own)
	if err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || st.Mode().Perm()&0077 != 0 {
		return "", errors.New("unsafe owner directory")
	}
	return own, nil
}
func writeLocalTrendFile(root, name string, data []byte) error {
	f, err := os.CreateTemp(root, ".incoming-")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(root, name))
}

var localMediaName = regexp.MustCompile(`^[a-f0-9]{64}\.img$`)

func (a *app) trendLocalMedia(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !localMediaName.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	root, err := localTrendOwnerRoot(a.trendLocalConfig().StorageRoot, trendOwner(r))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	file := filepath.Join(root, name)
	st, err := os.Lstat(file)
	if err != nil || !st.Mode().IsRegular() || st.Size() > localImageLimit {
		http.NotFound(w, r)
		return
	}
	raw, err := os.ReadFile(file)
	if err != nil || !localImageContent(raw) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(raw))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, name, st.ModTime(), bytes.NewReader(raw))
}

var trendClientPlatforms = map[string]string{"darwin-arm64": "kol-trend-collector-darwin-arm64", "darwin-amd64": "kol-trend-collector-darwin-amd64", "linux-amd64": "kol-trend-collector-linux-amd64", "windows-amd64": "kol-trend-collector-windows-amd64.exe"}

func (a *app) trendLocalClientDownload(w http.ResponseWriter, r *http.Request) {
	name, ok := trendClientPlatforms[r.PathValue("platform")]
	if !ok || !a.trendLocalConfig().Enabled {
		http.NotFound(w, r)
		return
	}
	file := filepath.Join(a.trendLocalConfig().ClientDir, name)
	st, err := os.Lstat(file)
	if err != nil || !st.Mode().IsRegular() {
		writeError(w, 503, 50321, "采集程序尚未发布，请先运行构建脚本")
		return
	}
	f, err := os.Open(file)
	if err != nil {
		writeError(w, 503, 50321, "采集程序不可用")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, name, st.ModTime(), f)
}

func localJobQuery(q trendQuery) url.Values {
	return url.Values{"tab": {q.Tab}, "region": {q.Region}, "period": {strconv.Itoa(q.Period)}, "orderByMetric": {strconv.Itoa(q.Order)}, "live": {"1"}}
}
