package main

import (
	"bytes"
	"context"
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

const trendMenu = "/business/trends"

var trendTagRegions = strings.Fields("US FR DE IT ES GB AR AU BR CA CO EG ID IL JP KR MY MX PH SA SG ZA TW TH TR AE VN")
var trendVideoRegions = strings.Fields("US JP VN TH ID")
var trendIDPattern = regexp.MustCompile(`^[0-9]{1,24}$`)
var trendMediaPattern = regexp.MustCompile(`^(cover|avatar|video_cover|video_avatar|monthly_cover)_[0-9]{1,24}\.jpg$`)

type trendQuery struct {
	Tab    string
	Region string
	Period int
	Page   int
	Size   int
	Order  int
	ID     string
	Live   bool
}

type trendSnapshot struct {
	CapturedAt        string                    `json:"capturedAt"`
	TagCurveEnd       string                    `json:"tagCurveEnd"`
	VideoEndTimestamp int64                     `json:"videoEndTimestamp"`
	Tags              []map[string]any          `json:"tags"`
	TagDetails        map[string]map[string]any `json:"tagDetails"`
	Videos            []map[string]any          `json:"videos"`
}

var trendStore struct {
	sync.Once
	snapshot *trendSnapshot
	err      error
}

func trendDataDir() string {
	if dir := os.Getenv("TREND_DATA_DIR"); dir != "" {
		return dir
	}
	return filepath.Join("data", "trends")
}

func loadTrendSnapshot() (*trendSnapshot, error) {
	trendStore.Do(func() {
		b, err := os.ReadFile(filepath.Join(trendDataDir(), "snapshot.json"))
		if err != nil {
			trendStore.err = err
			return
		}
		var snapshot trendSnapshot
		decoder := json.NewDecoder(bytes.NewReader(b))
		decoder.UseNumber()
		if err := decoder.Decode(&snapshot); err != nil {
			trendStore.err = err
			return
		}
		trendStore.snapshot = &snapshot
	})
	return trendStore.snapshot, trendStore.err
}

func parseTrendQuery(v url.Values, detail bool) (trendQuery, error) {
	q := trendQuery{Tab: "tag", Region: "US", Period: 30, Page: 1, Size: 20, Order: 1}
	if x := v.Get("tab"); x != "" {
		q.Tab = x
	}
	if x := v.Get("region"); x != "" {
		q.Region = x
	}
	if q.Tab != "tag" && q.Tab != "video" {
		return q, errors.New("invalid tab")
	}
	regions := trendTagRegions
	if q.Tab == "video" {
		regions = trendVideoRegions
	}
	if !contains(regions, q.Region) {
		return q, errors.New("invalid region")
	}
	for name, target := range map[string]*int{"period": &q.Period, "currentPage": &q.Page, "pageSize": &q.Size, "orderByMetric": &q.Order} {
		if x := v.Get(name); x != "" {
			n, err := strconv.Atoi(x)
			if err != nil {
				return q, errors.New("invalid number")
			}
			*target = n
		}
	}
	if q.Period != 7 && q.Period != 30 && (q.Period != 90 || q.Tab == "video") {
		return q, errors.New("invalid period")
	}
	if q.Page < 1 || q.Page > 100 || q.Size < 1 || q.Size > 50 || q.Order < 1 || q.Order > 3 {
		return q, errors.New("invalid pagination or order")
	}
	q.ID = v.Get("id")
	q.Live = v.Get("live") == "1"
	if detail && !trendIDPattern.MatchString(q.ID) {
		return q, errors.New("invalid id")
	}
	return q, nil
}

func (q trendQuery) archived() bool {
	return !q.Live && q.Region == "US" && q.Period == 30 && (q.Tab == "tag" || q.Order == 1)
}

func trendMetadata(q trendQuery, capturedAt, curveEnd string) map[string]any {
	source := "live"
	if capturedAt != "" {
		source = "archive"
	}
	return map[string]any{"source": source, "capturedAt": capturedAt, "curveEnd": curveEnd, "region": q.Region, "period": q.Period, "coverage": "limited", "sourceURL": trendSourceURL(q)}
}

func trendSourceURL(q trendQuery) string {
	tab := "hashtag"
	if q.Tab == "video" {
		tab = "video"
	}
	path := "https://ads.tiktok.com/creative/creativeCenter/trends/" + tab
	if q.Tab == "tag" && q.ID != "" {
		path += "/" + q.ID
	}
	return path + "?" + url.Values{"period": {strconv.Itoa(q.Period)}, "region": {q.Region}}.Encode()
}

func (a *app) trendOptions(w http.ResponseWriter, r *http.Request) {
	mode := "browser"
	if a.trendLocalConfig().Enabled {
		mode = "local"
	}
	writeOK(w, map[string]any{"collectionMode": mode, "tagRegions": trendTagRegions, "videoRegions": trendVideoRegions, "tagPeriods": []int{7, 30, 90}, "videoPeriods": []int{7, 30}})
}

func (a *app) trendList(w http.ResponseWriter, r *http.Request) {
	q, err := parseTrendQuery(r.URL.Query(), false)
	if err != nil {
		writeError(w, http.StatusBadRequest, 10001, "筛选条件不正确")
		return
	}
	if (a.trendBrowserConfig().Enabled || a.trendLocalConfig().Enabled) && !q.archived() {
		a.trendBrowserData(w, r, q, false)
		return
	}
	if q.archived() {
		if s, err := loadTrendSnapshot(); err == nil {
			rows := s.Tags
			curveEnd := s.TagCurveEnd
			if q.Tab == "video" {
				rows = s.Videos
				curveEnd = time.Unix(s.VideoEndTimestamp, 0).UTC().Format("2006-01-02")
			}
			start := min((q.Page-1)*q.Size, len(rows))
			end := min(start+q.Size, len(rows))
			meta := trendMetadata(q, s.CapturedAt, curveEnd)
			meta["coverage"] = "complete"
			writeOK(w, map[string]any{"list": rows[start:end], "total": len(rows), "currentPage": q.Page, "pageSize": q.Size, "hasMore": end < len(rows), "meta": meta})
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	rows, pagination, end, err := fetchTrendList(ctx, q)
	if err != nil {
		writeError(w, http.StatusBadGateway, 50201, "TikTok 暂时无法读取，请重试或打开原站查看")
		return
	}
	actualPage := trendInt(pagination["page"])
	if actualPage < 1 {
		actualPage = q.Page
	}
	actualSize := trendInt(pagination["limit"])
	if actualSize < 1 {
		actualSize = len(rows)
	}
	meta := trendMetadata(q, "", end)
	meta["pageMismatch"] = actualPage != q.Page
	writeOK(w, map[string]any{"list": rows, "total": trendInt(pagination["totalCount"]), "currentPage": actualPage, "pageSize": actualSize, "hasMore": pagination["hasMore"] == true && actualPage == q.Page, "meta": meta})
}

func (a *app) trendDetail(w http.ResponseWriter, r *http.Request) {
	q, err := parseTrendQuery(r.URL.Query(), true)
	if err != nil {
		writeError(w, http.StatusBadRequest, 10001, "筛选条件或内容编号不正确")
		return
	}
	if (a.trendBrowserConfig().Enabled || a.trendLocalConfig().Enabled) && !q.archived() {
		a.trendBrowserData(w, r, q, true)
		return
	}
	if q.archived() {
		if s, err := loadTrendSnapshot(); err == nil {
			if q.Tab == "tag" {
				if d, ok := s.TagDetails[q.ID]; ok {
					writeOK(w, map[string]any{"detail": d, "meta": trendMetadata(q, s.CapturedAt, s.TagCurveEnd)})
					return
				}
			} else {
				for _, v := range s.Videos {
					if v["itemID"] == q.ID {
						writeOK(w, map[string]any{"detail": v, "meta": trendMetadata(q, s.CapturedAt, time.Unix(s.VideoEndTimestamp, 0).UTC().Format("2006-01-02"))})
						return
					}
				}
			}
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	var d map[string]any
	if q.Tab == "tag" {
		d, err = trendRequest(ctx, "https://ads.tiktok.com/CreativeOne/KnowledgeAPI/GetHashtagDetail", map[string]any{"hashtagID": q.ID, "timeRange": q.Period, "countryCode": q.Region})
	} else {
		var timestamp int64
		timestamp, err = trendVideoEnd(ctx, q.Region)
		if err == nil {
			values := url.Values{"itemID": {q.ID}, "periodDimension": {trendVideoDimension(q.Period)}, "periodEndTimestamp": {strconv.FormatInt(timestamp, 10)}}
			d, err = trendRequest(ctx, trendVideoHost(q.Region)+"/CreativeOne/Report/CreativeCenterGetTopContentsItemDetail?"+values.Encode(), nil)
			if err == nil {
				if entity, ok := d["entityInfo"].(map[string]any); ok {
					d = entity
				}
				d = normalizeTrendVideo(d)
			}
		}
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, 50201, "TikTok 详情暂时无法读取，请重试或打开原站查看")
		return
	}
	delete(d, "BaseResp")
	if q.Tab == "tag" && fmt.Sprint(d["hashtagID"]) != q.ID || q.Tab == "video" && fmt.Sprint(d["itemID"]) != q.ID {
		writeError(w, http.StatusBadGateway, 50201, "TikTok 未返回对应内容的详情")
		return
	}
	writeOK(w, map[string]any{"detail": d, "meta": trendMetadata(q, "", trendCurveEnd(d))})
}

func (a *app) trendMedia(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !trendMediaPattern.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(trendDataDir(), "media", name)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || !stat.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, name, stat.ModTime(), f)
}

// Only fixed TikTok HTTPS endpoints are called. App credentials are never forwarded.
var trendHTTPClient = &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}

func trendRequest(ctx context.Context, endpoint string, body map[string]any) (map[string]any, error) {
	if requester, ok := ctx.Value(trendRequesterKey{}).(trendRequester); ok {
		return requester(ctx, endpoint, body)
	}
	method := http.MethodGet
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := trendHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream status %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, (8<<20)+1))
	if err != nil || len(data) > 8<<20 {
		return nil, errors.New("upstream body unavailable")
	}
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err = decoder.Decode(&value); err != nil {
		return nil, err
	}
	base, ok := value["BaseResp"].(map[string]any)
	if !ok || fmt.Sprint(base["StatusCode"]) != "0" {
		return nil, errors.New("upstream business error")
	}
	return value, nil
}

func trendInt(v any) int { n, _ := strconv.Atoi(fmt.Sprint(v)); return n }
func trendVideoHost(region string) string {
	if region == "US" {
		return "https://ads.us.tiktok.com"
	}
	return "https://ads.tiktok.com"
}
func trendVideoDimension(period int) string {
	if period == 7 {
		return "3"
	}
	return "5"
}
func trendVideoEnd(ctx context.Context, region string) (int64, error) {
	d, err := trendRequest(ctx, trendVideoHost(region)+"/CreativeOne/Report/GetTopContentsOverview", nil)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(fmt.Sprint(d["lastDailyEndTimestamp"]), 10, 64)
	if err != nil || n <= 0 {
		return 0, errors.New("missing period end")
	}
	return n, nil
}

func fetchTrendList(ctx context.Context, q trendQuery) ([]map[string]any, map[string]any, string, error) {
	var d map[string]any
	var err error
	key := "items"
	end := ""
	if q.Tab == "tag" {
		d, err = trendRequest(ctx, "https://ads.tiktok.com/CreativeOne/KnowledgeAPI/GetHashtagList", map[string]any{"timeRange": q.Period, "countryCode": q.Region, "page": q.Page, "limit": q.Size})
	} else {
		var timestamp int64
		timestamp, err = trendVideoEnd(ctx, q.Region)
		if err != nil {
			return nil, nil, "", err
		}
		end = time.Unix(timestamp, 0).UTC().Format("2006-01-02")
		key = "entityInfos"
		params := url.Values{"countryCode": {q.Region}, "periodDimension": {trendVideoDimension(q.Period)}, "periodEndTimestamp": {strconv.FormatInt(timestamp, 10)}, "orderByMetric": {strconv.Itoa(q.Order)}, "organicOnly": {"false"}, "page": {strconv.Itoa(q.Page)}, "limit": {strconv.Itoa(q.Size)}, "contentLabelIDs": {""}}
		d, err = trendRequest(ctx, trendVideoHost(q.Region)+"/CreativeOne/Report/CreativeCenterGetTopContentsList?"+params.Encode(), nil)
	}
	if err != nil {
		return nil, nil, "", err
	}
	rows := make([]map[string]any, 0)
	for _, raw := range trendArray(d[key]) {
		if row, ok := raw.(map[string]any); ok {
			if q.Tab == "video" {
				row = normalizeTrendVideo(row)
			} else {
				row["hashtagID"] = fmt.Sprint(row["hashtagID"])
			}
			id := row["hashtagID"]
			if q.Tab == "video" {
				id = row["itemID"]
			}
			if !trendIDPattern.MatchString(fmt.Sprint(id)) {
				continue
			}
			rows = append(rows, row)
		}
	}
	if q.Tab == "tag" && len(rows) > 0 {
		end = trendCurveEnd(rows[0])
	}
	pagination, _ := d["pagination"].(map[string]any)
	if pagination == nil {
		pagination = map[string]any{}
	}
	return rows, pagination, end, nil
}

func trendArray(v any) []any { a, _ := v.([]any); return a }
func trendCurveEnd(d map[string]any) string {
	a := trendArray(d["popularityCurve"])
	if len(a) == 0 {
		return ""
	}
	point, _ := a[len(a)-1].(map[string]any)
	if point == nil {
		return ""
	}
	n, err := strconv.ParseInt(fmt.Sprint(point["timestamp"]), 10, 64)
	if err != nil || n <= 0 {
		return ""
	}
	return time.Unix(n, 0).UTC().Format("2006-01-02")
}
func normalizeTrendVideo(e map[string]any) map[string]any {
	info, _ := e["itemInfo"].(map[string]any)
	author, _ := e["itemAuthorInfo"].(map[string]any)
	followers, _ := e["itemAuthorMetrics"].(map[string]any)
	return map[string]any{"itemID": fmt.Sprint(info["itemID"]), "title": info["title"], "createTime": info["createTime"], "coverURL": info["coverURL"], "videoURL": info["videoURL"], "metrics": e["itemMetrics"], "tags": e["contentTags"], "author": map[string]any{"name": author["nickName"], "handle": author["handlerName"], "avatarURL": author["avatarURI"], "followers": followers["followers"]}}
}
