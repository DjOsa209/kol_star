package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var contentPageTitlePattern = regexp.MustCompile(`(?is)<title\b[^>]*>(.*?)</title>`)
var contentURLPattern = regexp.MustCompile(`https?://[^\s,，;；]+`)
var projectContentTitleRefreshes sync.Map

type projectContentTitleCandidate struct {
	ID              int
	Platform        string
	CooperationType string
	CurrentTitle    string
	ContentURL      string
}

func (a *app) scheduleProjectContentTitleRefresh(projectID int) {
	if projectID <= 0 {
		return
	}
	if _, alreadyRunning := projectContentTitleRefreshes.LoadOrStore(projectID, true); alreadyRunning {
		return
	}
	go func() {
		defer projectContentTitleRefreshes.Delete(projectID)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		a.refreshProjectContentTitles(ctx, projectID)
	}()
}

func (a *app) refreshProjectContentTitles(ctx context.Context, projectID int) {
	rows, err := a.DB().QueryContext(ctx,
		`select c.id,
		        coalesce(nullif(c.content_platform, ''), nullif(r.platform, ''), 'Website'),
		        c.cooperation_type, c.creative_name,
		        coalesce(nullif(c.final_link, ''), nullif(c.deliverable_links, ''), '')
		   from biz_cooperations c
		   left join biz_resources r on r.id = c.resource_id
		  where c.project_id = ?
		    and coalesce(nullif(c.final_link, ''), nullif(c.deliverable_links, ''), '') <> ''`,
		projectID,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	candidates := make([]projectContentTitleCandidate, 0)
	for rows.Next() {
		var candidate projectContentTitleCandidate
		if err := rows.Scan(
			&candidate.ID,
			&candidate.Platform,
			&candidate.CooperationType,
			&candidate.CurrentTitle,
			&candidate.ContentURL,
		); err != nil {
			return
		}
		if contentTitleNeedsRefresh(candidate.CurrentTitle, candidate.CooperationType) {
			candidates = append(candidates, candidate)
			if len(candidates) >= 12 {
				break
			}
		}
	}
	if len(candidates) == 0 {
		return
	}

	limiter := make(chan struct{}, 4)
	var group sync.WaitGroup
	for _, candidate := range candidates {
		candidate := candidate
		group.Add(1)
		go func() {
			defer group.Done()
			select {
			case limiter <- struct{}{}:
				defer func() { <-limiter }()
			case <-ctx.Done():
				return
			}
			contentURL := firstContentMetadataURL(candidate.ContentURL)
			if contentURL == "" {
				return
			}
			titleCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
			defer cancel()
			title := fetchContentMetadataTitle(titleCtx, candidate.Platform, contentURL)
			if title == "" {
				title = contentTitleFromURL(contentURL)
			}
			if title == "" {
				return
			}
			_, _ = a.DB().ExecContext(ctx,
				`update biz_cooperations
				    set creative_name = ?
				  where id = ? and project_id = ?`,
				title, candidate.ID, projectID,
			)
		}()
	}
	group.Wait()
}

func contentTitleNeedsRefresh(title, cooperationType string) bool {
	title = strings.TrimSpace(title)
	return title == "" ||
		title == strings.TrimSpace(cooperationType) ||
		title == "已导入发布内容" ||
		title == "已同步内容"
}

func firstContentMetadataURL(value string) string {
	return strings.TrimSpace(contentURLPattern.FindString(value))
}

func fetchContentMetadataTitle(ctx context.Context, platform, contentURL string) string {
	if err := validatePublicWebsiteURL(ctx, contentURL); err != nil {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(platform), "TikTok") {
		if title := fetchTikTokOEmbedTitle(ctx, contentURL); title != "" {
			return title
		}
	}
	return fetchHTMLContentTitle(ctx, contentURL)
}

func fetchTikTokOEmbedTitle(ctx context.Context, contentURL string) string {
	endpoint := "https://www.tiktok.com/oembed?url=" + url.QueryEscape(contentURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 6 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ""
	}
	var payload struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return ""
	}
	return sanitizeContentTitle(payload.Title)
}

func fetchHTMLContentTitle(ctx context.Context, contentURL string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, contentURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; KOLAdminContentMetadata/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	client := &http.Client{
		Timeout: 6 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("重定向次数过多")
			}
			return validatePublicWebsiteURL(req.Context(), req.URL.String())
		},
	}
	response, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return ""
	}
	document := string(body)
	for _, tag := range websiteTagPattern.FindAllString(document, -1) {
		attributes := websiteAttributes(tag)
		property := strings.ToLower(firstNonEmpty(attributes["property"], attributes["name"]))
		if property != "og:title" && property != "twitter:title" {
			continue
		}
		if title := sanitizeContentTitle(attributes["content"]); title != "" {
			return title
		}
	}
	if match := contentPageTitlePattern.FindStringSubmatch(document); len(match) > 1 {
		return sanitizeContentTitle(match[1])
	}
	return ""
}

func sanitizeContentTitle(value string) string {
	value = html.UnescapeString(value)
	value = strings.Join(strings.Fields(value), " ")
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= 240 {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:240]))
}

func contentTitleFromURL(contentURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(contentURL))
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	path, _ := url.PathUnescape(strings.Trim(parsed.Path, "/"))
	if path == "" {
		return strings.TrimPrefix(parsed.Hostname(), "www.")
	}
	parts := strings.Split(path, "/")
	last := strings.TrimSpace(parts[len(parts)-1])
	if last == "" || regexp.MustCompile(`^\d+$`).MatchString(last) {
		return strings.TrimPrefix(parsed.Hostname(), "www.") + "/" + path
	}
	last = strings.NewReplacer("-", " ", "_", " ").Replace(last)
	return sanitizeContentTitle(last)
}
