package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"golang.org/x/net/html"
)

type trafficCVMonthlyMetrics struct {
	Month           string
	Visits          int64
	PageViews       int64
	PagesPerVisit   float64
	BounceRate      float64
	AverageDuration string
}

type websiteResourceInfo struct {
	Name         string
	ResourceType string
	PlatformURL  string
	Website      string
}

var websiteTagPattern = regexp.MustCompile(`(?is)<(?:link|meta)\b[^>]*>`)
var websiteAttrPattern = regexp.MustCompile(`(?is)([a-zA-Z_:.-]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
var trafficCVTotalVisitsPattern = regexp.MustCompile(`(?i)\bTotal\s+Visits\b\s*([0-9][0-9,.]*\s*[KMBT]?)`)
var trafficCVPagesPerVisitPattern = regexp.MustCompile(`(?i)\bPages\s+per\s+Visit\b\s*([0-9][0-9,.]*)`)
var trafficCVBounceRatePattern = regexp.MustCompile(`(?i)\bBounce\s+Rate\b\s*([0-9][0-9,.]*)\s*%`)
var trafficCVDurationPattern = regexp.MustCompile(`(?i)\bAvg\.?\s+Duration\b\s*([0-9]{1,2}:[0-9]{2}:[0-9]{2})`)

func normalizeWebsiteDomain(values ...string) (string, string) {
	for _, value := range values {
		value = strings.Trim(strings.TrimSpace(value), "，,;；。")
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
	resource, err := a.websiteResourceByID(ctx, id)
	if err != nil {
		return nil, err
	}
	domain, homepage := normalizeWebsiteDomain(resource.PlatformURL, resource.Website, resource.Name)
	if domain == "" {
		return nil, fmt.Errorf("请填写有效的网站域名或主页链接")
	}

	avatarRemoteURL, avatarWarning := fetchWebsiteAvatar(ctx, homepage)
	avatarURL := normalizedRemoteImageURL(avatarRemoteURL)
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

	metrics, err := fetchTrafficCVMonthlyMetrics(ctx, domain, time.Now())
	if err != nil {
		return nil, err
	}
	return a.applyWebsiteTrafficMetrics(ctx, id, domain, homepage, metrics, avatarURL, avatarRemoteURL, avatarWarning)
}

func (a *app) websiteResourceByID(ctx context.Context, id int) (websiteResourceInfo, error) {
	var resource websiteResourceInfo
	if err := a.DB().QueryRowContext(ctx,
		`select name, resource_type, platform_url, website
		   from biz_resources where id = ? limit 1`,
		id,
	).Scan(&resource.Name, &resource.ResourceType, &resource.PlatformURL, &resource.Website); err != nil {
		return websiteResourceInfo{}, err
	}
	if resource.ResourceType != "媒体" {
		return websiteResourceInfo{}, fmt.Errorf("Website 平台仅用于媒体资源")
	}
	return resource, nil
}

func (a *app) applyWebsiteTrafficMetrics(
	ctx context.Context,
	id int,
	domain string,
	homepage string,
	metrics trafficCVMonthlyMetrics,
	avatarURL string,
	avatarRemoteURL string,
	avatarWarning string,
) (map[string]any, error) {
	if metrics.Visits <= 0 {
		return nil, fmt.Errorf("Traffic.cv 暂无该域名的月访问量估算")
	}
	_, err := a.DB().ExecContext(ctx,
		`update biz_resources set
		  platform = 'Website', platform_url = ?, website = ?,
		  avatar_url = if(? <> '', ?, avatar_url),
		  avatar_remote_url = if(? <> '', ?, avatar_remote_url),
		  followers = 0, audience_size = ?, audience_size_unit = 'Monthly Visits',
		  umv_month = '', umv_country = '', umv_web_source = '',
		  umv_cross_device_deduplicated = 0,
		  monthly_visits = ?, monthly_page_views = ?, website_bounce_rate = ?,
		  reference_source = 'Traffic.cv', provider_updated_at = now(),
		  last_sync_status = '成功', last_sync_error = '', last_sync_at = now()
		 where id = ?`,
		homepage, homepage, avatarURL, avatarURL, avatarRemoteURL, avatarRemoteURL,
		metrics.Visits, metrics.Visits, metrics.PageViews,
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
		"audienceSize":     metrics.Visits,
		"audienceSizeUnit": "Monthly Visits",
		"trafficMonth":     metrics.Month,
		"monthlyVisits":    metrics.Visits,
		"monthlyPageViews": metrics.PageViews,
		"pagesPerVisit":    metrics.PagesPerVisit,
		"bounceRate":       metrics.BounceRate,
		"averageDuration":  metrics.AverageDuration,
		"avatarUrl":        avatarURL,
		"avatarRemoteUrl":  avatarRemoteURL,
		"warnings":         warnings,
		"syncedAt":         time.Now().Format(time.RFC3339),
	}, nil
}

func (a *app) importTrafficCVHTML(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	body := readBody(r)
	id := intField(body, "id")
	if id == 0 {
		writeError(w, http.StatusOK, 10001, "资源 id 不能为空")
		return
	}
	rawHTML := stringField(body, "html")
	if rawHTML == "" || rawHTML == "<nil>" {
		writeError(w, http.StatusOK, 10002, "请粘贴已通过验证的 Traffic.cv 页面 HTML")
		return
	}
	resource, err := a.websiteResourceByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusOK, 10003, err.Error())
		return
	}
	domain, homepage := normalizeWebsiteDomain(resource.PlatformURL, resource.Website, resource.Name)
	if domain == "" {
		writeError(w, http.StatusOK, 10004, "请先填写有效的网站域名或主页链接")
		return
	}
	metrics, err := parseTrafficCVHTML([]byte(rawHTML), time.Now())
	if err != nil {
		writeError(w, http.StatusOK, 10005, err.Error())
		return
	}
	result, err := a.applyWebsiteTrafficMetrics(r.Context(), id, domain, homepage, metrics, "", "", "")
	if err != nil {
		writeDBError(w, err)
		return
	}
	result["source"] = "Traffic.cv HTML 导入"
	writeOK(w, result)
}

func fetchTrafficCVMonthlyMetrics(ctx context.Context, domain string, now time.Time) (trafficCVMonthlyMetrics, error) {
	reportURL := "https://traffic.cv/" + url.PathEscape(domain)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reportURL, nil)
	if err != nil {
		return trafficCVMonthlyMetrics{}, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; KOLAdminWebsiteTraffic/1.0; +https://traffic.cv/)")
	client, err := trafficCVHTTPClient()
	if err != nil {
		return trafficCVMonthlyMetrics{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return trafficCVMonthlyMetrics{}, fmt.Errorf("Traffic.cv 请求失败：%w", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	_ = resp.Body.Close()
	if readErr != nil {
		return trafficCVMonthlyMetrics{}, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || isTrafficCVChallengeHTML(body) {
		browserBody, browserErr := fetchTrafficCVHTMLWithBrowser(ctx, reportURL)
		if browserErr != nil {
			return trafficCVMonthlyMetrics{}, fmt.Errorf("Traffic.cv 页面访问失败：%s；浏览器回退失败：%v", resp.Status, browserErr)
		}
		body = browserBody
	}
	metrics, err := parseTrafficCVHTML(body, now)
	if err != nil {
		return trafficCVMonthlyMetrics{}, err
	}
	return metrics, nil
}

func trafficCVHTTPClient() (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if configured := strings.TrimSpace(os.Getenv("TRAFFIC_CV_PROXY_URL")); configured != "" {
		proxyURL, err := url.Parse(configured)
		if err != nil || proxyURL.Scheme == "" || proxyURL.Host == "" {
			return nil, fmt.Errorf("TRAFFIC_CV_PROXY_URL 配置无效")
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &http.Client{Timeout: 20 * time.Second, Transport: transport}, nil
}

func fetchTrafficCVHTMLWithBrowser(ctx context.Context, reportURL string) ([]byte, error) {
	browserPath := websiteScreenshotBrowserPath()
	if browserPath == "" {
		return nil, fmt.Errorf("服务器未安装 Chrome 或 Chromium")
	}
	options := append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(browserPath),
		chromedp.DisableGPU,
		chromedp.Flag("disable-dev-shm-usage", true),
	)
	if os.Geteuid() == 0 {
		options = append(options, chromedp.NoSandbox)
	}
	if proxyURL := strings.TrimSpace(os.Getenv("TRAFFIC_CV_PROXY_URL")); proxyURL != "" {
		options = append(options, chromedp.ProxyServer(proxyURL))
	}
	browserCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(browserCtx, options...)
	defer cancelAllocator()
	tabCtx, cancelTab := chromedp.NewContext(allocatorCtx)
	defer cancelTab()
	var renderedHTML string
	if err := chromedp.Run(
		tabCtx,
		chromedp.Navigate(reportURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(8*time.Second),
		chromedp.OuterHTML("html", &renderedHTML, chromedp.ByQuery),
	); err != nil {
		return nil, err
	}
	if isTrafficCVChallengeHTML([]byte(renderedHTML)) {
		return nil, fmt.Errorf("Cloudflare 验证页未完成")
	}
	return []byte(renderedHTML), nil
}

func parseTrafficCVHTML(body []byte, now time.Time) (trafficCVMonthlyMetrics, error) {
	if isTrafficCVChallengeHTML(body) {
		return trafficCVMonthlyMetrics{}, fmt.Errorf("Traffic.cv 返回了 Cloudflare 验证页")
	}
	document, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return trafficCVMonthlyMetrics{}, fmt.Errorf("Traffic.cv HTML 解析失败：%w", err)
	}
	pageText := trafficCVVisibleText(document)
	visits, err := parseTrafficCVCount(trafficCVPatternValue(trafficCVTotalVisitsPattern, pageText))
	if err != nil || visits <= 0 {
		return trafficCVMonthlyMetrics{}, fmt.Errorf("Traffic.cv HTML 中未找到 Total Visits")
	}
	pagesPerVisit, _ := strconv.ParseFloat(strings.ReplaceAll(trafficCVPatternValue(trafficCVPagesPerVisitPattern, pageText), ",", ""), 64)
	bouncePercent, _ := strconv.ParseFloat(strings.ReplaceAll(trafficCVPatternValue(trafficCVBounceRatePattern, pageText), ",", ""), 64)
	pageViews := int64(0)
	if pagesPerVisit > 0 {
		pageViews = int64(math.Round(float64(visits) * pagesPerVisit))
	}
	return trafficCVMonthlyMetrics{
		Month:           trafficCVSurveyMonth(now),
		Visits:          visits,
		PageViews:       pageViews,
		PagesPerVisit:   pagesPerVisit,
		BounceRate:      bouncePercent / 100,
		AverageDuration: trafficCVPatternValue(trafficCVDurationPattern, pageText),
	}, nil
}

func trafficCVVisibleText(document *html.Node) string {
	parts := make([]string, 0, 256)
	var visit func(*html.Node, bool)
	visit = func(node *html.Node, hidden bool) {
		if node.Type == html.ElementNode {
			switch strings.ToLower(node.Data) {
			case "script", "style", "noscript", "svg":
				hidden = true
			}
		}
		if node.Type == html.TextNode && !hidden {
			if value := strings.Join(strings.Fields(node.Data), " "); value != "" {
				parts = append(parts, value)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child, hidden)
		}
	}
	visit(document, false)
	return strings.Join(parts, " ")
}

func trafficCVPatternValue(pattern *regexp.Regexp, pageText string) string {
	match := pattern.FindStringSubmatch(pageText)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func parseTrafficCVCount(value string) (int64, error) {
	value = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), ",", ""))
	multiplier := float64(1)
	if len(value) > 0 {
		switch value[len(value)-1] {
		case 'K':
			multiplier = 1_000
			value = value[:len(value)-1]
		case 'M':
			multiplier = 1_000_000
			value = value[:len(value)-1]
		case 'B':
			multiplier = 1_000_000_000
			value = value[:len(value)-1]
		case 'T':
			multiplier = 1_000_000_000_000
			value = value[:len(value)-1]
		}
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || number < 0 {
		return 0, fmt.Errorf("访问量格式无效")
	}
	return int64(math.Round(number * multiplier)), nil
}

func trafficCVSurveyMonth(now time.Time) string {
	monthOffset := -1
	if now.Day() < 10 {
		monthOffset = -2
	}
	return now.AddDate(0, monthOffset, 0).Format("2006-01")
}

func isTrafficCVChallengeHTML(body []byte) bool {
	content := strings.ToLower(string(body))
	return strings.Contains(content, "cf-mitigated") ||
		strings.Contains(content, "challenge-platform") ||
		strings.Contains(content, "just a moment...")
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
