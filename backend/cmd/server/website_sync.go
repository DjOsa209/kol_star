package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

type similarwebMonthlyMetrics struct {
	Month           string
	UniqueVisitors  int64
	Visits          int64
	PageViews       int64
	BounceRate      float64
	ProviderUpdated string
}

var websiteTagPattern = regexp.MustCompile(`(?is)<(?:link|meta)\b[^>]*>`)
var websiteAttrPattern = regexp.MustCompile(`(?is)([a-zA-Z_:.-]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)

func normalizeWebsiteDomain(values ...string) (string, string) {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "<nil>" {
			continue
		}
		candidate := value
		if !strings.Contains(candidate, "://") {
			candidate = "https://" + candidate
		}
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Hostname() == "" {
			continue
		}
		host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
		host = strings.TrimPrefix(host, "www.")
		if host == "" || strings.Contains(host, " ") {
			continue
		}
		return host, "https://" + host
	}
	return "", ""
}

func (a *app) syncWebsiteResource(ctx context.Context, id int) (map[string]any, error) {
	var resource struct {
		Name         string
		ResourceType string
		PlatformURL  string
		Website      string
	}
	if err := a.DB().QueryRowContext(ctx,
		`select name, resource_type, platform_url, website
		   from biz_resources where id = ? limit 1`,
		id,
	).Scan(&resource.Name, &resource.ResourceType, &resource.PlatformURL, &resource.Website); err != nil {
		return nil, err
	}
	if resource.ResourceType != "媒体" {
		return nil, fmt.Errorf("Website 平台仅用于媒体资源")
	}
	domain, homepage := normalizeWebsiteDomain(resource.PlatformURL, resource.Website, resource.Name)
	if domain == "" {
		return nil, fmt.Errorf("请填写有效的网站域名或主页链接")
	}

	avatarRemoteURL, avatarWarning := fetchWebsiteAvatar(ctx, homepage)
	avatarURL := localizeResourceImage(ctx, id, "avatar", avatarRemoteURL)
	if avatarRemoteURL != "" {
		_, _ = a.DB().ExecContext(ctx,
			`update biz_resources
			    set avatar_url = if(? <> '', ?, avatar_url),
			        avatar_remote_url = ?, platform = 'Website',
			        platform_url = ?, website = ?
			  where id = ?`,
			avatarURL, avatarURL, avatarRemoteURL, homepage, homepage, id,
		)
	}

	apiKey := strings.TrimSpace(a.effectivePlatformAPIConfig(ctx).SimilarwebAPIKey)
	if apiKey == "" {
		message := "媒体头像已抓取；未配置 Similarweb API Key，UMV 暂未更新"
		if avatarWarning != "" {
			message = avatarWarning + "；未配置 Similarweb API Key，UMV 暂未更新"
		}
		_, _ = a.DB().ExecContext(ctx,
			`update biz_resources
			    set platform = 'Website', platform_url = ?, website = ?,
			        audience_size_unit = 'UMV', reference_source = 'Similarweb',
			        last_sync_status = '待配置', last_sync_error = ?, last_sync_at = now()
			  where id = ?`,
			homepage, homepage, message, id,
		)
		return map[string]any{
			"platform":         "Website",
			"domain":           domain,
			"audienceSize":     0,
			"audienceSizeUnit": "UMV",
			"avatarUrl":        avatarURL,
			"avatarRemoteUrl":  avatarRemoteURL,
			"warnings":         []string{message},
			"syncedAt":         time.Now().Format(time.RFC3339),
		}, nil
	}

	metrics, err := fetchSimilarwebMonthlyMetrics(ctx, apiKey, domain, time.Now())
	if err != nil {
		return nil, err
	}
	if metrics.UniqueVisitors <= 0 {
		return nil, fmt.Errorf("Similarweb 暂无该域名的月独立访客估算")
	}
	_, err = a.DB().ExecContext(ctx,
		`update biz_resources set
		  platform = 'Website', platform_url = ?, website = ?,
		  avatar_url = if(? <> '', ?, avatar_url),
		  avatar_remote_url = if(? <> '', ?, avatar_remote_url),
		  audience_size = ?, audience_size_unit = 'UMV',
		  umv_month = ?, umv_country = 'WW', umv_web_source = 'total',
		  umv_cross_device_deduplicated = 0,
		  monthly_visits = ?, monthly_page_views = ?, website_bounce_rate = ?,
		  reference_source = 'Similarweb V5', provider_updated_at = now(),
		  last_sync_status = '成功', last_sync_error = '', last_sync_at = now()
		 where id = ?`,
		homepage, homepage, avatarURL, avatarURL, avatarRemoteURL, avatarRemoteURL,
		metrics.UniqueVisitors, metrics.Month, metrics.Visits, metrics.PageViews,
		metrics.BounceRate, id,
	)
	if err != nil {
		return nil, err
	}
	warnings := []string{}
	if avatarWarning != "" {
		warnings = append(warnings, avatarWarning)
	}
	return map[string]any{
		"platform":         "Website",
		"domain":           domain,
		"audienceSize":     metrics.UniqueVisitors,
		"audienceSizeUnit": "UMV",
		"umvMonth":         metrics.Month,
		"monthlyVisits":    metrics.Visits,
		"monthlyPageViews": metrics.PageViews,
		"bounceRate":       metrics.BounceRate,
		"avatarUrl":        avatarURL,
		"avatarRemoteUrl":  avatarRemoteURL,
		"warnings":         warnings,
		"syncedAt":         time.Now().Format(time.RFC3339),
	}, nil
}

func fetchSimilarwebMonthlyMetrics(ctx context.Context, apiKey, domain string, now time.Time) (similarwebMonthlyMetrics, error) {
	month := now.AddDate(0, -1, 0)
	startDate := fmt.Sprintf("%04d-%02d", month.Year(), month.Month())
	params := url.Values{}
	params.Set("domain", domain)
	params.Set("metrics", "unique_visitors,visits,page_views,bounce_rate")
	params.Set("granularity", "monthly")
	params.Set("start_date", startDate)
	params.Set("end_date", startDate)
	params.Set("web_source", "total")
	params.Set("country", "ww")
	params.Set("main_domain_only", "false")
	params.Set("format", "json")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.similarweb.com/v5/website-analysis/websites/traffic-and-engagement?"+params.Encode(), nil)
	if err != nil {
		return similarwebMonthlyMetrics{}, err
	}
	req.Header.Set("api-key", apiKey)
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return similarwebMonthlyMetrics{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return similarwebMonthlyMetrics{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if len(message) > 500 {
			message = message[:500]
		}
		return similarwebMonthlyMetrics{}, fmt.Errorf("Similarweb API 请求失败：%s", firstNonEmpty(message, resp.Status))
	}
	var payload struct {
		Meta map[string]any   `json:"meta"`
		Data []map[string]any `json:"data"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return similarwebMonthlyMetrics{}, fmt.Errorf("Similarweb API 响应解析失败：%w", err)
	}
	if len(payload.Data) == 0 {
		return similarwebMonthlyMetrics{Month: startDate}, nil
	}
	sort.SliceStable(payload.Data, func(i, j int) bool {
		return anyString(payload.Data[i]["date"]) < anyString(payload.Data[j]["date"])
	})
	row := payload.Data[len(payload.Data)-1]
	return similarwebMonthlyMetrics{
		Month:           firstNonEmpty(anyString(row["date"])[:minInt(len(anyString(row["date"])), 7)], startDate),
		UniqueVisitors:  anyInt64(row["unique_visitors"]),
		Visits:          anyInt64(row["visits"]),
		PageViews:       anyInt64(row["page_views"]),
		BounceRate:      anyFloat64(row["bounce_rate"]),
		ProviderUpdated: anyString(mapAt(payload.Meta, "request")["last_updated"]),
	}, nil
}

func fetchWebsiteAvatar(ctx context.Context, homepage string) (string, string) {
	if err := validatePublicWebsiteURL(ctx, homepage); err != nil {
		return "", "网站头像抓取失败：" + err.Error()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, homepage, nil)
	if err != nil {
		return "", "网站头像抓取失败：" + err.Error()
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; KOLAdminWebsiteProfile/1.0)")
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("重定向次数过多")
			}
			return validatePublicWebsiteURL(req.Context(), req.URL.String())
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "网站头像抓取失败：" + err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "网站头像抓取失败：" + resp.Status
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", "网站头像抓取失败：" + err.Error()
	}
	baseURL := resp.Request.URL
	type candidate struct {
		URL      string
		Priority int
	}
	candidates := make([]candidate, 0)
	for _, tag := range websiteTagPattern.FindAllString(string(body), -1) {
		attrs := websiteAttributes(tag)
		rel := strings.ToLower(attrs["rel"])
		property := strings.ToLower(firstNonEmpty(attrs["property"], attrs["name"]))
		rawURL := firstNonEmpty(attrs["href"], attrs["content"])
		priority := 0
		switch {
		case strings.Contains(rel, "apple-touch-icon"):
			priority = 40
		case strings.Contains(rel, "icon"):
			priority = 30
		case property == "og:image":
			priority = 20
		case property == "twitter:image":
			priority = 10
		}
		if priority == 0 || rawURL == "" {
			continue
		}
		if parsed, err := url.Parse(rawURL); err == nil {
			candidates = append(candidates, candidate{URL: baseURL.ResolveReference(parsed).String(), Priority: priority})
		}
	}
	candidates = append(candidates, candidate{URL: baseURL.ResolveReference(&url.URL{Path: "/favicon.ico"}).String(), Priority: 1})
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Priority > candidates[j].Priority })
	for _, item := range candidates {
		if normalized := normalizedRemoteImageURL(item.URL); normalized != "" {
			return normalized, ""
		}
	}
	return "", "网站未声明可用头像"
}

func websiteAttributes(tag string) map[string]string {
	attrs := map[string]string{}
	for _, match := range websiteAttrPattern.FindAllStringSubmatch(tag, -1) {
		value := firstNonEmpty(match[2], match[3], match[4])
		attrs[strings.ToLower(match[1])] = strings.TrimSpace(value)
	}
	return attrs
}

func validatePublicWebsiteURL(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("网站地址无效")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".local") {
		return fmt.Errorf("不允许访问本地地址")
	}
	addresses, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("域名无法解析")
	}
	for _, address := range addresses {
		if address.IsLoopback() || address.IsPrivate() || address.IsUnspecified() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() {
			return fmt.Errorf("不允许访问内网地址")
		}
	}
	return nil
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
