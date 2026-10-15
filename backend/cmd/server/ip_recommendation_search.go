package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type ipExternalSearch struct {
	Query      string                   `json:"query"`
	Provider   string                   `json:"provider"`
	SearchedAt string                   `json:"searchedAt"`
	Results    []recommendationEvidence `json:"results"`
	Phase      string                   `json:"phase,omitempty"`
	Brief      *ipResearchBrief         `json:"brief,omitempty"`
	Candidates []ipExternalCandidate    `json:"candidates,omitempty"`
	Queries    []string                 `json:"queries,omitempty"`
	Rounds     int                      `json:"rounds,omitempty"`
	Gaps       []string                 `json:"gaps,omitempty"`
}

type ipSearchProvider struct{ name, endpoint, key string }

var ipExternalSearchCache recommendationEvidenceCacheStore

func configuredIPSearchProvider(configs ...WebSearchConfig) (ipSearchProvider, error) {
	var cfg WebSearchConfig
	if len(configs) > 0 {
		cfg = configs[0]
	}
	name := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(os.Getenv("IP_WEB_SEARCH_PROVIDER")))
	}
	tavilyKey := strings.TrimSpace(cfg.TavilyAPIKey)
	if tavilyKey == "" {
		tavilyKey = strings.TrimSpace(os.Getenv("TAVILY_API_KEY"))
	}
	if name == "" || name == "auto" {
		switch {
		case tavilyKey != "":
			name = "tavily"
		case strings.TrimSpace(os.Getenv("SEARXNG_BASE_URL")) != "":
			name = "searxng"
		case strings.TrimSpace(os.Getenv("BRAVE_SEARCH_API_KEY")) != "":
			name = "brave"
		}
	}
	p := ipSearchProvider{name: name}
	switch name {
	case "tavily":
		p.endpoint, p.key = "https://api.tavily.com/search", tavilyKey
	case "brave":
		if os.Getenv("BRAVE_SEARCH_STORAGE_ALLOWED") != "1" {
			return p, errors.New("Brave 搜索尚未配置结果存储权限，请联系管理员")
		}
		p.endpoint, p.key = braveRecommendationSearchURL, strings.TrimSpace(os.Getenv("BRAVE_SEARCH_API_KEY"))
	case "searxng":
		base, err := url.Parse(strings.TrimSpace(os.Getenv("SEARXNG_BASE_URL")))
		if err != nil || base == nil || base.Host == "" || base.User != nil || (base.Scheme != "https" && base.Scheme != "http") || base.RawQuery != "" || base.Fragment != "" {
			return p, errors.New("SearXNG 搜索地址未配置，请联系管理员")
		}
		p.endpoint = strings.TrimRight(base.String(), "/") + "/search"
		return p, nil
	default:
		return p, errors.New("联网搜索尚未配置，请联系管理员配置免费搜索服务")
	}
	if p.key == "" {
		return p, errors.New("联网搜索密钥未配置，请联系管理员")
	}
	return p, nil
}

// Only the explicitly entered public search keywords leave this service.
// Project criteria, attachments, budgets and conversation history are never sent.
func searchExternalIPResources(ctx context.Context, userID int, query string, configs ...WebSearchConfig) (*ipExternalSearch, error) {
	p, err := configuredIPSearchProvider(configs...)
	if err != nil {
		return nil, err
	}
	query = cleanRecommendationEvidenceText(query, 200)
	identity := sha256.Sum256([]byte(p.endpoint + "\x00" + p.key))
	cacheKey := fmt.Sprintf("sources-v1:%d:%s:%x:%s", userID, p.name, identity, query)
	now := time.Now()
	items, _, cached := ipExternalSearchCache.get(cacheKey, now)
	if !cached {
		items, err = searchExternalIPResourcesWithTransport(ctx, query, p, http.DefaultTransport)
		if err != nil {
			return nil, err
		}
		ipExternalSearchCache.put(cacheKey, items, "available", now, 15*time.Minute)
	}
	if items == nil {
		items = []recommendationEvidence{}
	}
	return &ipExternalSearch{Query: query, Provider: p.name, SearchedAt: now.Format(time.RFC3339), Results: items}, nil
}

func searchExternalIPResourcesWithTransport(ctx context.Context, query string, provider ipSearchProvider, transport http.RoundTripper) ([]recommendationEvidence, error) {
	items, err := searchIPResearchPages(ctx, query, provider, preferredIPResearchDomains(), 360, transport)
	return prioritizeIPResearchEvidence(filterIPResearchEvidence(items, researchTopic(query))), err
}

func searchIPResearchPages(ctx context.Context, query string, provider ipSearchProvider, domains []string, summaryLimit int, transport http.RoundTripper) ([]recommendationEvidence, error) {
	endpoint, err := url.Parse(provider.endpoint)
	if err != nil {
		return nil, errors.New("搜索服务配置无效")
	}
	method := http.MethodGet
	var body io.Reader
	switch provider.name {
	case "tavily":
		method = http.MethodPost
		encoded, _ := json.Marshal(map[string]any{"query": query, "search_depth": "basic", "max_results": 6, "auto_parameters": false, "include_answer": false, "include_raw_content": false, "include_images": false, "include_domains": domains, "include_domains_mode": "prefer", "include_published_date": true})
		body = bytes.NewReader(encoded)
	case "brave", "searxng":
		params := endpoint.Query()
		params.Set("q", query)
		if provider.name == "brave" {
			params.Set("count", "6")
			params.Set("safesearch", "moderate")
		} else {
			params.Set("format", "json")
			params.Set("categories", "general")
			params.Set("safesearch", "1")
		}
		endpoint.RawQuery = params.Encode()
	default:
		return nil, errors.New("搜索服务配置无效")
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, errors.New("搜索服务配置无效")
	}
	req.Header.Set("Accept", "application/json")
	if provider.name == "tavily" {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+provider.key)
	}
	if provider.name == "brave" {
		req.Header.Set("X-Subscription-Token", provider.key)
	}
	client := &http.Client{Timeout: 15 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("联网搜索暂时不可用，请稍后重试")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 || resp.StatusCode == 432 || resp.StatusCode == 433 || resp.StatusCode == 402 {
		return nil, errors.New("搜索额度已用完或请求过于频繁，请稍后重试")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("联网搜索服务返回异常，请联系管理员检查配置")
	}
	var payload struct {
		Results []struct {
			Title       string  `json:"title"`
			URL         string  `json:"url"`
			Content     string  `json:"content"`
			PublishedAt string  `json:"published_date"`
			Score       float64 `json:"score"`
		} `json:"results"`
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, recommendationEvidenceResponseMax)).Decode(&payload) != nil {
		return nil, errors.New("联网搜索结果无法读取，请重试")
	}
	items := []recommendationEvidence{}
	seen := map[string]bool{}
	appendResult := func(title, rawURL, summary, publishedAt string, score float64) {
		link, source, valid := canonicalRecommendationEvidenceURL(rawURL)
		title, summary = cleanRecommendationEvidenceText(title, 160), cleanRecommendationEvidenceText(summary, summaryLimit)
		if !valid || title == "" || summary == "" || seen[link] || len(items) >= 20 {
			return
		}
		seen[link] = true
		items = append(items, labelIPResearchEvidence(recommendationEvidence{Title: title, URL: link, Source: source, Summary: summary, Score: score, PublishedAt: cleanRecommendationEvidenceText(publishedAt, 80)}))
	}
	for _, result := range payload.Results {
		appendResult(result.Title, result.URL, result.Content, result.PublishedAt, result.Score)
	}
	for _, result := range payload.Web.Results {
		appendResult(result.Title, result.URL, result.Description, "", 0)
	}
	return items, nil
}

func externalIPSearchPresentation(search *ipExternalSearch) ipRecommendationPresentation {
	p := ipRecommendationPresentation{Source: "rule", Headline: "本次联网搜索没有找到可用线索", Explanation: "可换用具体 IP 名称、艺术家或公开合作关键词继续搜索。", Basis: []ipRecommendationBasis{}, Adjustments: []string{}, Questions: []string{}, SuggestedPrompts: []string{}, WebSearch: search}
	if len(search.Results) > 0 {
		p.Headline = fmt.Sprintf("找到 %d 条库外公开线索，可进一步核实合作适配", len(search.Results))
		p.Explanation = "右侧展示原始来源和公开摘要；这些线索尚未入库，也未确认授权、报价或合作档期。"
		p.Basis = []ipRecommendationBasis{{"检索关键词", search.Query}, {"核实方向", "先确认 IP 身份、版权方与合作渠道，再核对授权地域及费用"}}
	}
	return p
}
