package main

import (
	"context"
	"encoding/json"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Only these fixed destinations are contacted. Result URLs are displayed as
// provenance and are never fetched by this service.
const (
	braveRecommendationSearchURL    = "https://api.search.brave.com/res/v1/web/search"
	wikidataRecommendationSearchURL = "https://www.wikidata.org/w/api.php"
)

const (
	recommendationEvidenceLimit       = 3
	recommendationEvidenceResponseMax = 1 << 20
)

var recommendationEvidenceHTML = regexp.MustCompile(`<[^>]*>`)
var recommendationWikidataID = regexp.MustCompile(`^Q[1-9][0-9]*$`)

type recommendationEvidenceCacheEntry struct {
	items     []recommendationEvidence
	status    string
	expiresAt time.Time
}

type recommendationEvidenceCacheStore struct {
	sync.Mutex
	entries map[string]recommendationEvidenceCacheEntry
}

var ipRecommendationEvidenceCache recommendationEvidenceCacheStore

func (cache *recommendationEvidenceCacheStore) get(key string, now time.Time) ([]recommendationEvidence, string, bool) {
	cache.Lock()
	defer cache.Unlock()
	entry, found := cache.entries[key]
	if !found {
		return nil, "", false
	}
	if !now.Before(entry.expiresAt) {
		delete(cache.entries, key)
		return nil, "", false
	}
	return append([]recommendationEvidence(nil), entry.items...), entry.status, true
}

func (cache *recommendationEvidenceCacheStore) put(key string, items []recommendationEvidence, status string, now time.Time, ttl time.Duration) {
	cache.Lock()
	defer cache.Unlock()
	if cache.entries == nil {
		cache.entries = make(map[string]recommendationEvidenceCacheEntry)
	}
	if _, exists := cache.entries[key]; !exists && len(cache.entries) >= 512 {
		// Evict the entry due to expire first. The map is intentionally small.
		var evict string
		var earliest time.Time
		for candidate, entry := range cache.entries {
			if evict == "" || entry.expiresAt.Before(earliest) {
				evict, earliest = candidate, entry.expiresAt
			}
		}
		delete(cache.entries, evict)
	}
	cache.entries[key] = recommendationEvidenceCacheEntry{
		items:     append([]recommendationEvidence(nil), items...),
		status:    status,
		expiresAt: now.Add(ttl),
	}
}

// fetchIPRecommendationEvidence searches for public information using only the
// IP's public name. It must not receive a project brief, private user query,
// budget, or any other internal data. Wikidata's structured entity data is CC0
// and can be persisted. Brave snippets may be persisted only when the deployment
// explicitly confirms its Brave plan includes storage rights.
func fetchIPRecommendationEvidence(ctx context.Context, ipName, ipType string) ([]recommendationEvidence, string) {
	apiKey := os.Getenv("BRAVE_SEARCH_API_KEY")
	storageAllowed := os.Getenv("BRAVE_SEARCH_STORAGE_ALLOWED") == "1"
	mode := "wikidata"
	if storageAllowed && strings.TrimSpace(apiKey) != "" {
		mode = "wikidata+brave"
	}
	// Cache only public catalog identity and provider mode. No user brief or
	// credential is retained in the cache key or value.
	key := cleanRecommendationEvidenceText(ipName, 120) + "\x00" + ipType + "\x00" + mode
	now := time.Now()
	if items, status, found := ipRecommendationEvidenceCache.get(key, now); found {
		return items, status
	}
	items, status := fetchIPRecommendationEvidenceWithTransport(ctx, ipName, ipType, apiKey, storageAllowed, http.DefaultTransport)
	if ctx.Err() == nil {
		ttl := 24 * time.Hour
		if status == "unavailable" {
			ttl = 2 * time.Minute
		}
		ipRecommendationEvidenceCache.put(key, items, status, now, ttl)
	}
	return items, status
}

func fetchIPRecommendationEvidenceWithTransport(ctx context.Context, ipName, ipType, apiKey string, braveStorageAllowed bool, transport http.RoundTripper) ([]recommendationEvidence, string) {
	name := cleanRecommendationEvidenceText(ipName, 120)
	name = strings.TrimSpace(strings.ReplaceAll(name, `"`, ""))
	if name == "" {
		return nil, "none"
	}
	wikidataEvidence, wikidataStatus := fetchWikidataRecommendationEvidence(ctx, name, ipType, transport)
	if !braveStorageAllowed || strings.TrimSpace(apiKey) == "" {
		return wikidataEvidence, wikidataStatus
	}
	braveEvidence, braveStatus := fetchBraveRecommendationEvidence(ctx, name, apiKey, transport)
	combined := append(wikidataEvidence, braveEvidence...)
	if len(combined) > recommendationEvidenceLimit {
		combined = combined[:recommendationEvidenceLimit]
	}
	if len(combined) > 0 {
		return combined, "available"
	}
	if wikidataStatus == "unavailable" && braveStatus == "unavailable" {
		return nil, "unavailable"
	}
	return nil, "none"
}

func fetchBraveRecommendationEvidence(ctx context.Context, name, apiKey string, transport http.RoundTripper) ([]recommendationEvidence, string) {
	endpoint, _ := url.Parse(braveRecommendationSearchURL)
	query := endpoint.Query()
	// The quoted name anchors an otherwise broad search in the actual IP.
	// Search terms are public and independent of the requester's brief.
	terms := recommendationIPSearchTerms(name)
	searchName := terms[len(terms)-1]
	query.Set("q", `"`+searchName+`" (licensing OR collaboration OR 联名 OR 授权)`)
	query.Set("count", "8")
	query.Set("safesearch", "moderate")
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, "unavailable"
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", strings.TrimSpace(apiKey))
	client := &http.Client{
		Timeout:   4 * time.Second,
		Transport: transport,
		// A redirect would change the fixed destination and could leak the key.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "unavailable"
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "unavailable"
	}
	var payload struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, recommendationEvidenceResponseMax)).Decode(&payload); err != nil {
		return nil, "unavailable"
	}
	evidence := make([]recommendationEvidence, 0, recommendationEvidenceLimit)
	seen := make(map[string]bool)
	for _, result := range payload.Web.Results {
		title := cleanRecommendationEvidenceText(result.Title, 160)
		summary := cleanRecommendationEvidenceText(result.Description, 360)
		link, source, ok := canonicalRecommendationEvidenceURL(result.URL)
		if !ok || title == "" || summary == "" || seen[link] || !recommendationEvidenceMentionsIP(title, summary, name) {
			continue
		}
		seen[link] = true
		evidence = append(evidence, recommendationEvidence{
			Title:   title,
			URL:     link,
			Source:  source,
			Summary: summary,
		})
		if len(evidence) == recommendationEvidenceLimit {
			break
		}
	}
	if len(evidence) == 0 {
		return nil, "none"
	}
	return evidence, "available"
}

func recommendationEvidenceMentionsIP(title, summary, ipName string) bool {
	haystack := recommendationNormalizeName(title + " " + summary)
	for _, term := range recommendationIPSearchTerms(ipName) {
		needle := recommendationNormalizeName(term)
		if len([]rune(needle)) >= 3 && strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

// Search labels and aliases, then accept only an exact name match. A search hit
// on a related but different IP must never become evidence for the candidate.
func fetchWikidataRecommendationEvidence(ctx context.Context, name, ipType string, transport http.RoundTripper) ([]recommendationEvidence, string) {
	terms := recommendationIPSearchTerms(name)
	for _, term := range terms {
		evidence, status := searchWikidataRecommendationTerm(ctx, term, ipType, transport)
		if status == "available" {
			return evidence, status
		}
		if status == "unavailable" {
			return nil, status
		}
	}
	return nil, "none"
}

func searchWikidataRecommendationTerm(ctx context.Context, term, ipType string, transport http.RoundTripper) ([]recommendationEvidence, string) {
	endpoint, _ := url.Parse(wikidataRecommendationSearchURL)
	query := endpoint.Query()
	query.Set("action", "wbsearchentities")
	query.Set("format", "json")
	query.Set("language", recommendationSearchLanguage(term))
	query.Set("limit", "5")
	query.Set("maxlag", "5")
	query.Set("search", term)
	query.Set("type", "item")
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, "unavailable"
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "kol-admin-ip-recommendations/1.0 (https://xmp.transsion.com)")
	client := &http.Client{
		Timeout:       4 * time.Second,
		Transport:     transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "unavailable"
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "unavailable"
	}
	var payload struct {
		Search []struct {
			ID          string `json:"id"`
			Label       string `json:"label"`
			Description string `json:"description"`
			Match       struct {
				Text string `json:"text"`
			} `json:"match"`
		} `json:"search"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, recommendationEvidenceResponseMax)).Decode(&payload); err != nil {
		return nil, "unavailable"
	}
	type entityMatch struct {
		evidence recommendationEvidence
		score    int
	}
	matches := make([]entityMatch, 0, len(payload.Search))
	for _, result := range payload.Search {
		if !recommendationWikidataID.MatchString(result.ID) ||
			(recommendationNormalizeName(result.Label) != recommendationNormalizeName(term) &&
				recommendationNormalizeName(result.Match.Text) != recommendationNormalizeName(term)) {
			continue
		}
		title := cleanRecommendationEvidenceText(result.Label, 160)
		summary := cleanRecommendationEvidenceText(result.Description, 360)
		if title == "" || summary == "" {
			continue
		}
		score := recommendationWikidataEntityScore(summary, ipType)
		if score == 0 && strings.TrimSpace(ipType) != "" && ipType != "其他" {
			continue
		}
		matches = append(matches, entityMatch{evidence: recommendationEvidence{
			Title:   title,
			URL:     "https://www.wikidata.org/wiki/" + result.ID,
			Source:  "Wikidata",
			Summary: summary,
		}, score: score})
	}
	if len(matches) == 0 {
		return nil, "none"
	}
	best, tied := matches[0], false
	for _, match := range matches[1:] {
		if match.score > best.score {
			best, tied = match, false
		} else if match.score == best.score {
			tied = true
		}
	}
	if tied {
		return nil, "none"
	}
	return []recommendationEvidence{best.evidence}, "available"
}

func recommendationWikidataEntityScore(description, ipType string) int {
	if strings.TrimSpace(ipType) == "" || ipType == "其他" {
		return 0
	}
	value := strings.ToLower(description)
	contains := func(words ...string) bool {
		for _, word := range words {
			if strings.Contains(value, word) {
				return true
			}
		}
		return false
	}
	score := 0
	switch ipType {
	case "游戏":
		if contains("video game", "computer game", "电子游戏", "游戏") {
			score += 3
		}
	case "动画":
		if contains("animat", "cartoon", "anime", "comic strip", "media franchise", "动画", "动漫", "漫画", "character", "mascot", "角色", "吉祥物") {
			score += 3
		}
	case "影视", "综艺":
		if contains("film", "movie", "television", "tv series", "cinema", "电影", "电视剧", "综艺") {
			score += 3
		}
	case "体育":
		if contains("sport", "athlet", "football", "basketball", "体育", "运动", "球队") {
			score += 3
		}
	case "潮牌", "品牌":
		if contains("brand", "trademark", "fashion label", "品牌", "商标") {
			score += 3
		}
	case "潮玩":
		if contains("toy", "collectible", "figure", "doll", "玩具", "潮玩", "手办") {
			score += 3
		}
	case "文化艺术":
		if contains("art", "museum", "exhibition", "文化", "艺术", "博物馆") {
			score += 3
		}
	case "线下活动":
		if contains("event", "festival", "conference", "活动", "节日", "展会") {
			score += 3
		}
	}
	// Licensing generally concerns a franchise, character, or brand rather
	// than an individual release with the same title. This only resolves exact
	// label collisions; a remaining tie yields no external evidence.
	if score > 0 && contains("franchise", "系列") {
		score += 6
	}
	if score > 0 && contains("character", "mascot", "角色", "吉祥物") {
		score += 4
	}
	return score
}

func recommendationIPSearchTerms(name string) []string {
	name = strings.TrimSpace(name)
	for _, prefix := range []string{"test-", "test_", "测试-", "测试_"} {
		if strings.HasPrefix(strings.ToLower(name), prefix) {
			name = strings.TrimSpace(name[len(prefix):])
			break
		}
	}
	terms := []string{name}
	// Imported lists sometimes concatenate Chinese and English names. Try one
	// public name variant when an exact full-name search has no entity match.
	latin := strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Han, r) {
			return ' '
		}
		return r
	}, name))
	if latin != name && len([]rune(latin)) >= 4 {
		terms = append(terms, latin)
	}
	return terms
}

func recommendationSearchLanguage(term string) string {
	for _, r := range term {
		if unicode.Is(unicode.Han, r) {
			return "zh"
		}
	}
	return "en"
}

func recommendationNormalizeName(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, value)
}

func cleanRecommendationEvidenceText(value string, maxRunes int) string {
	value = recommendationEvidenceHTML.ReplaceAllString(html.UnescapeString(value), " ")
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > maxRunes {
		return strings.TrimSpace(string(runes[:maxRunes]))
	}
	return value
}

func canonicalRecommendationEvidenceURL(raw string) (canonical, source string, ok bool) {
	if len(raw) == 0 || len(raw) > 2048 {
		return "", "", false
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Opaque != "" || parsed.User != nil {
		return "", "", false
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if !recommendationPublicDomain(host) || (parsed.Port() != "" && parsed.Port() != "443") {
		return "", "", false
	}
	params, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", "", false
	}
	for key := range params {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || lower == "fbclid" || lower == "gclid" || lower == "mc_cid" || lower == "mc_eid" {
			delete(params, key)
		}
	}
	parsed.Host = host
	parsed.Fragment = ""
	parsed.RawFragment = ""
	parsed.RawQuery = params.Encode()
	parsed.ForceQuery = false
	return parsed.String(), strings.TrimPrefix(host, "www."), true
}

func recommendationPublicDomain(host string) bool {
	if len(host) < 4 || len(host) > 253 || !strings.Contains(host, ".") || net.ParseIP(host) != nil {
		return false
	}
	for _, suffix := range []string{".local", ".localhost", ".internal", ".test", ".example", ".invalid"} {
		if strings.HasSuffix(host, suffix) {
			return false
		}
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || label[0] == '-' || label[len(label)-1] == '-' || len(label) > 63 {
			return false
		}
		for _, char := range label {
			if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-') {
				return false
			}
		}
	}
	return true
}
