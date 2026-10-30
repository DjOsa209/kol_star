package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const monthlySource = "https://later.com/blog/tiktok-trends/"
const monthlyPayloadURL = "https://later.com/page-data/blog/tiktok-trends/page-data.json"
const monthlyMaxBytes = 4 << 20

var monthlyMonthPattern = regexp.MustCompile(`^20[0-9]{2}-(0[1-9]|1[0-2])$`)
var monthlyVideoPattern = regexp.MustCompile(`^/@[^/]+/video/([0-9]{10,24})/?$`)
var monthlyUpdateMu sync.Mutex

type monthlyTrend struct {
	ID           string `json:"id"`
	Month        string `json:"month"`
	Title        string `json:"title"`
	SourceDate   string `json:"sourceDate"`
	PublishedAt  string `json:"publishedAt"`
	DateConflict bool   `json:"dateConflict"`
	Summary      string `json:"summary"`
	AudioLabel   string `json:"audioLabel"`
	AudioURL     string `json:"audioURL"`
	VideoURL     string `json:"videoURL"`
	VideoID      string `json:"videoID"`
	CoverURL     string `json:"coverURL"`
}
type monthlyArchive struct {
	CapturedAt      string         `json:"capturedAt"`
	SourceUpdatedAt string         `json:"sourceUpdatedAt"`
	Records         []monthlyTrend `json:"records"`
}
type monthlyRichNode struct {
	NodeType string            `json:"nodeType"`
	Value    string            `json:"value"`
	Content  []monthlyRichNode `json:"content"`
	Data     struct {
		URI    string `json:"uri"`
		Target struct {
			Sys struct {
				ID string `json:"id"`
			} `json:"sys"`
		} `json:"target"`
	} `json:"data"`
}

func monthlyText(n monthlyRichNode) string {
	s := n.Value
	for _, c := range n.Content {
		s += monthlyText(c)
	}
	return s
}
func monthlyAudio(n monthlyRichNode) (string, string) {
	if n.NodeType == "hyperlink" && validMonthlyURL(n.Data.URI) {
		return n.Data.URI, strings.TrimSpace(monthlyText(n))
	}
	for _, c := range n.Content {
		if u, l := monthlyAudio(c); u != "" {
			return u, l
		}
	}
	return "", ""
}
func validMonthlyURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
		return false
	}
	if u.Host == "vm.tiktok.com" {
		return len(u.Path) > 1 && !strings.Contains(u.Path, "..")
	}
	return u.Host == "www.tiktok.com" && (monthlyVideoPattern.MatchString(u.Path) || strings.HasPrefix(u.Path, "/music/") || strings.HasPrefix(u.Path, "/t/")) && !strings.Contains(u.Path, "..")
}
func monthlyExcerpt(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	for _, sep := range []string{". ", "! ", "? "} {
		if n := strings.Index(s, sep); n > 0 {
			s = s[:n+1]
			break
		}
	}
	r := []rune(s)
	if len(r) > 180 {
		s = string(r[:177]) + "…"
	}
	return s
}
func parseMonthlyPayload(data []byte, capturedAt string) (monthlyArchive, error) {
	var p struct {
		Result struct {
			Data struct {
				Article struct {
					PublicationDate string `json:"publicationDate"`
					Sections        []struct {
						Month   string `json:"navigationTitle"`
						Content struct {
							Raw        string `json:"raw"`
							References []struct {
								ID        string `json:"contentful_id"`
								URL       string `json:"url"`
								EmbedType string `json:"embedType"`
							} `json:"references"`
						} `json:"content"`
					} `json:"articleSections"`
				} `json:"contentfulArticle"`
			} `json:"data"`
		} `json:"result"`
	}
	if len(data) > monthlyMaxBytes || !utf8.Valid(data) || json.Unmarshal(data, &p) != nil {
		return monthlyArchive{}, errors.New("invalid monthly payload")
	}
	out := monthlyArchive{CapturedAt: capturedAt, Records: []monthlyTrend{}}
	for _, layout := range []string{time.RFC3339, "2006-01-02", "January 2, 2006", "2006-01-02T15:04"} {
		if date, err := time.Parse(layout, p.Result.Data.Article.PublicationDate); err == nil {
			out.SourceUpdatedAt = date.Format("2006-01-02")
			break
		}
	}
	seen := map[string]bool{}
	for _, section := range p.Result.Data.Article.Sections {
		_, label, ok := strings.Cut(section.Month, ":")
		if !ok {
			continue
		}
		month, err := time.Parse("January 2006", strings.TrimSpace(label))
		if err != nil {
			continue
		}
		var doc monthlyRichNode
		if json.Unmarshal([]byte(section.Content.Raw), &doc) != nil || doc.NodeType != "document" {
			return monthlyArchive{}, errors.New("invalid monthly content")
		}
		refs := map[string]string{}
		for _, r := range section.Content.References {
			if r.EmbedType == "TikTok" && validMonthlyURL(r.URL) {
				refs[r.ID] = r.URL
			}
		}
		var current *monthlyTrend
		flush := func() {
			if current != nil && current.Title != "" && !seen[current.ID] {
				seen[current.ID] = true
				out.Records = append(out.Records, *current)
			}
		}
		for _, n := range doc.Content {
			s := strings.TrimSpace(monthlyText(n))
			if n.NodeType == "heading-3" && strings.HasPrefix(s, "Trend:") {
				flush()
				title, date, _ := strings.Cut(strings.TrimSpace(strings.TrimPrefix(s, "Trend:")), "—")
				title = strings.TrimSpace(title)
				date = strings.TrimSpace(date)
				if len([]rune(title)) > 200 {
					return monthlyArchive{}, errors.New("invalid trend title")
				}
				sum := sha256.Sum256([]byte(month.Format("2006-01") + "|" + strings.ToLower(title)))
				current = &monthlyTrend{ID: hex.EncodeToString(sum[:12]), Month: month.Format("2006-01"), Title: title, SourceDate: date}
				if d, e := time.Parse("January 02, 2006", date); e == nil {
					current.PublishedAt = d.Format("2006-01-02")
					current.DateConflict = d.Format("2006-01") != current.Month
				}
			} else if current != nil {
				switch {
				case strings.HasPrefix(s, "Trend Recap:"):
					current.Summary = monthlyExcerpt(strings.TrimPrefix(s, "Trend Recap:"))
				case strings.HasPrefix(s, "Audio:"):
					current.AudioURL, current.AudioLabel = monthlyAudio(n)
				case n.NodeType == "embedded-entry-block" && current.VideoURL == "":
					current.VideoURL = refs[n.Data.Target.Sys.ID]
					if u, e := url.Parse(current.VideoURL); e == nil {
						if m := monthlyVideoPattern.FindStringSubmatch(u.Path); len(m) == 2 {
							current.VideoID = m[1]
						}
					}
				}
			}
		}
		flush()
	}
	if len(out.Records) == 0 || len(out.Records) > 1000 || out.SourceUpdatedAt == "" {
		return monthlyArchive{}, errors.New("monthly structure unavailable")
	}
	return out, nil
}
func loadMonthlyArchive() (monthlyArchive, error) {
	f, err := os.Open(filepath.Join(trendDataDir(), "monthly.json"))
	if err != nil {
		return monthlyArchive{}, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, monthlyMaxBytes+1))
	if err != nil || len(b) > monthlyMaxBytes {
		return monthlyArchive{}, errors.New("monthly archive unavailable")
	}
	var archive monthlyArchive
	err = json.Unmarshal(b, &archive)
	return archive, err
}
func saveMonthlyArchive(archive monthlyArchive) error {
	b, err := json.Marshal(archive)
	if err != nil {
		return err
	}
	// Runtime caches are excluded from Git and may not exist on first deploy.
	// Create them as the service user; never broaden existing permissions.
	if err = os.MkdirAll(trendDataDir(), 0750); err != nil {
		return err
	}
	f, err := os.CreateTemp(trendDataDir(), "monthly-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(trendDataDir(), "monthly.json"))
}
func mergeMonthlyArchive(old, latest monthlyArchive) monthlyArchive {
	// Preserve months no longer present in the rolling source article. A present
	// month is replaced as a unit, so removed/corrected entries do not linger.
	months := map[string]bool{}
	for _, r := range latest.Records {
		months[r.Month] = true
	}
	for _, r := range old.Records {
		if !months[r.Month] {
			latest.Records = append(latest.Records, r)
		}
	}
	sort.SliceStable(latest.Records, func(i, j int) bool { return latest.Records[i].Month > latest.Records[j].Month })
	return latest
}
func (a *app) trendMonthly(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if month != "" && !monthlyMonthPattern.MatchString(month) {
		writeError(w, 400, 10001, "月份格式不正确")
		return
	}
	archive, err := loadMonthlyArchive()
	if err != nil {
		writeError(w, 503, 50301, "月度内容暂不可用，请更新内容后重试")
		return
	}
	counts := map[string]int{}
	for _, row := range archive.Records {
		counts[row.Month]++
	}
	keys := []string{}
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	if month == "" && len(keys) > 0 {
		month = keys[0]
	}
	type monthOption struct {
		Month string `json:"month"`
		Count int    `json:"count"`
	}
	options := []monthOption{}
	for _, k := range keys {
		options = append(options, monthOption{k, counts[k]})
	}
	rows := []monthlyTrend{}
	for _, row := range archive.Records {
		if row.Month != month {
			continue
		}
		// The snapshot stores public references, never raw HTML or playable promises.
		if !validMonthlyURL(row.AudioURL) {
			row.AudioURL = ""
			row.AudioLabel = ""
		}
		if !validMonthlyURL(row.VideoURL) {
			row.VideoURL = ""
			row.VideoID = ""
		}
		if !trendIDPattern.MatchString(row.VideoID) {
			row.VideoID = ""
		}
		row.CoverURL = ""
		name := "monthly_cover_" + row.VideoID + ".jpg"
		if row.VideoID != "" {
			if st, e := os.Lstat(filepath.Join(trendDataDir(), "media", name)); e == nil && st.Mode().IsRegular() {
				row.CoverURL = "/business/trends/media/" + name
			}
		}
		rows = append(rows, row)
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeOK(w, map[string]any{"months": options, "month": month, "list": rows, "total": len(rows), "sourceURL": monthlySource, "sourceUpdatedAt": archive.SourceUpdatedAt, "capturedAt": archive.CapturedAt})
}
func fetchMonthlyPayload(ctx context.Context, client *http.Client) (monthlyArchive, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, monthlyPayloadURL, nil)
	if err != nil {
		return monthlyArchive{}, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return monthlyArchive{}, errors.New("monthly upstream unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return monthlyArchive{}, errors.New("monthly upstream rejected")
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, monthlyMaxBytes+1))
	if err != nil {
		return monthlyArchive{}, err
	}
	return parseMonthlyPayload(bytes.TrimSpace(b), time.Now().Format(time.RFC3339))
}
func (a *app) trendMonthlyRefresh(w http.ResponseWriter, r *http.Request) {
	if !monthlyUpdateMu.TryLock() {
		writeError(w, 409, 40901, "内容正在更新，请稍后重试")
		return
	}
	defer monthlyUpdateMu.Unlock()
	month := r.URL.Query().Get("month")
	if month != "" && !monthlyMonthPattern.MatchString(month) {
		writeError(w, 400, 10001, "月份格式不正确")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	latest, err := fetchMonthlyPayload(ctx, trendHTTPClient)
	if err != nil {
		writeError(w, 502, 50201, "Later 内容更新失败，已保留上次采集结果")
		return
	}
	if previous, e := loadMonthlyArchive(); e == nil {
		latest = mergeMonthlyArchive(previous, latest)
	}
	if err = saveMonthlyArchive(latest); err != nil {
		writeError(w, 500, 50001, "月度内容保存失败，请检查服务器数据目录及写入权限")
		return
	}
	// Browsers render TikTok's official embeds directly. Refresh only the Later
	// archive, so server-side TikTok rate limits cannot block content updates.
	writeOK(w, map[string]any{"capturedAt": latest.CapturedAt, "sourceUpdatedAt": latest.SourceUpdatedAt, "mediaMode": "tiktok-embed"})
}
