package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type creatorDiscoveryEvidence struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Query   string `json:"query"`
	Summary string `json:"summary"`
}

type creatorDiscoverySnapshot struct {
	ChannelID   string                     `json:"channelId"`
	Name        string                     `json:"name"`
	Handle      string                     `json:"handle"`
	ProfileURL  string                     `json:"profileUrl"`
	AvatarURL   string                     `json:"avatarUrl"`
	Description string                     `json:"description"`
	Country     string                     `json:"country"`
	Language    string                     `json:"language"`
	Followers   *int64                     `json:"followers"`
	TotalViews  int64                      `json:"totalViews"`
	VideoCount  int64                      `json:"videoCount"`
	RecentPosts []platformPost             `json:"recentPosts"`
	Evidence    []creatorDiscoveryEvidence `json:"evidence"`
	CollectedAt string                     `json:"collectedAt"`
	Warning     string                     `json:"warning"`
}

// Discovery only reads public YouTube data. Callers obtain cfg through
// effectivePlatformAPIConfig and decide how to persist unverified candidates.
func discoverYouTubeChannels(ctx context.Context, cfg PlatformAPIConfig, queries []string, maxCandidates int, onCandidate func(creatorDiscoverySnapshot) error) error {
	if strings.TrimSpace(cfg.YouTubeAPIKey) == "" {
		return fmt.Errorf("未配置 YouTube API Key，请在抓取控制中配置")
	}
	client, err := youtubeHTTPClient(cfg)
	if err != nil {
		return fmt.Errorf("YouTube 代理配置无效，请检查抓取控制中的代理地址")
	}
	return discoverYouTubeChannelsWithClient(ctx, client, "https://www.googleapis.com/youtube/v3", cfg.YouTubeAPIKey, queries, maxCandidates, onCandidate)
}

func discoverYouTubeChannelsWithClient(ctx context.Context, client *http.Client, baseURL, apiKey string, queries []string, maxCandidates int, onCandidate func(creatorDiscoverySnapshot) error) error {
	if onCandidate == nil {
		return fmt.Errorf("YouTube 发现缺少候选处理函数")
	}
	cleanQueries := make([]string, 0, 3)
	seenQueries := map[string]bool{}
	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query != "" && !seenQueries[query] {
			cleanQueries = append(cleanQueries, query)
			seenQueries[query] = true
		}
		if len(cleanQueries) == 3 {
			break
		}
	}
	if len(cleanQueries) == 0 {
		return fmt.Errorf("请输入至少一条 YouTube 发现关键词")
	}
	maxCandidates = clampInt(maxCandidates, 1, 30)
	provider := creatorDiscoveryYouTubeClient{client: client, baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey}
	seeds := make([]creatorDiscoverySnapshot, 0, maxCandidates)
	byChannel := map[string]int{}
	searchFailures := []string{}
	for index, query := range cleanQueries {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("YouTube 发现已中止：%w", err)
		}
		var payload struct {
			Items []struct {
				ID struct {
					VideoID string `json:"videoId"`
				} `json:"id"`
				Snippet struct {
					ChannelID    string `json:"channelId"`
					ChannelTitle string `json:"channelTitle"`
					Title        string `json:"title"`
					Description  string `json:"description"`
				} `json:"snippet"`
			} `json:"items"`
		}
		params := url.Values{"part": {"snippet"}, "type": {"video"}, "maxResults": {"20"}, "q": {query}}
		if err := provider.get(ctx, "search", params, &payload); err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("YouTube 发现已中止：%w", ctx.Err())
			}
			searchFailures = append(searchFailures, fmt.Sprintf("第 %d 条关键词：%s", index+1, err.Error()))
			continue
		}
		for _, item := range payload.Items {
			channelID := strings.TrimSpace(item.Snippet.ChannelID)
			if channelID == "" || item.ID.VideoID == "" {
				continue
			}
			position, exists := byChannel[channelID]
			if !exists {
				if len(seeds) >= maxCandidates {
					continue
				}
				position = len(seeds)
				byChannel[channelID] = position
				seeds = append(seeds, creatorDiscoverySnapshot{
					ChannelID: channelID, Name: item.Snippet.ChannelTitle,
					ProfileURL:  "https://www.youtube.com/channel/" + url.PathEscape(channelID),
					RecentPosts: []platformPost{}, Evidence: []creatorDiscoveryEvidence{},
				})
			}
			seed := &seeds[position]
			if len(seed.Evidence) < 12 {
				seed.Evidence = append(seed.Evidence, creatorDiscoveryEvidence{
					Title: item.Snippet.Title, URL: "https://www.youtube.com/watch?v=" + url.QueryEscape(item.ID.VideoID),
					Query: query, Summary: item.Snippet.Description,
				})
			}
		}
	}
	for _, seed := range seeds {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("YouTube 采集已中止：%w", err)
		}
		candidate := provider.collect(ctx, seed)
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("YouTube 采集已中止：%w", err)
		}
		if err := onCandidate(candidate); err != nil {
			return fmt.Errorf("保存 YouTube 发现候选失败：%w", err)
		}
	}
	if len(searchFailures) > 0 {
		return fmt.Errorf("YouTube 部分搜索未完成（%d/%d），已采集 %d 个频道；%s", len(searchFailures), len(cleanQueries), len(seeds), strings.Join(searchFailures, "；"))
	}
	return nil
}

type creatorDiscoveryYouTubeClient struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

// Never echo upstream response bodies or transport errors: both may contain
// request URLs with the API key, proxy credentials, or other sensitive values.
func (p creatorDiscoveryYouTubeClient) get(ctx context.Context, path string, params url.Values, target any) error {
	params.Set("key", p.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/"+path+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("YouTube 请求无法创建")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("YouTube 请求已中止：%w", ctx.Err())
		}
		return fmt.Errorf("YouTube 网络请求失败，请检查网络和代理配置")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var payload struct {
			Error struct {
				Errors []struct {
					Reason string `json:"reason"`
				} `json:"errors"`
			} `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&payload)
		message := "服务请求失败"
		for _, item := range payload.Error.Errors {
			switch item.Reason {
			case "quotaExceeded", "dailyLimitExceeded":
				message = "API 配额已耗尽，请稍后重试或检查配额"
			case "keyInvalid", "accessNotConfigured", "ipRefererBlocked", "forbidden":
				message = "API 凭证或访问权限不可用，请检查抓取控制配置"
			case "channelNotFound", "playlistNotFound", "videoNotFound":
				message = "请求的公开频道或作品已不可用"
			}
		}
		return fmt.Errorf("YouTube %s（HTTP %d）", message, resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4*1024*1024)).Decode(target); err != nil {
		return fmt.Errorf("YouTube 返回的数据格式无效")
	}
	return nil
}

func (p creatorDiscoveryYouTubeClient) collect(ctx context.Context, seed creatorDiscoverySnapshot) creatorDiscoverySnapshot {
	seed.CollectedAt = time.Now().UTC().Format(time.RFC3339)
	var payload struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title           string `json:"title"`
				CustomURL       string `json:"customUrl"`
				Description     string `json:"description"`
				Country         string `json:"country"`
				DefaultLanguage string `json:"defaultLanguage"`
				Thumbnails      map[string]struct {
					URL string `json:"url"`
				} `json:"thumbnails"`
			} `json:"snippet"`
			Statistics struct {
				SubscriberCount       *string `json:"subscriberCount"`
				HiddenSubscriberCount bool    `json:"hiddenSubscriberCount"`
				ViewCount             string  `json:"viewCount"`
				VideoCount            string  `json:"videoCount"`
			} `json:"statistics"`
			ContentDetails struct {
				RelatedPlaylists struct {
					Uploads string `json:"uploads"`
				} `json:"relatedPlaylists"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	params := url.Values{"part": {"snippet,statistics,contentDetails"}, "id": {seed.ChannelID}}
	if err := p.get(ctx, "channels", params, &payload); err != nil {
		seed.Warning = "频道详情采集失败：" + err.Error()
		return seed
	}
	if len(payload.Items) == 0 || payload.Items[0].ID != seed.ChannelID {
		seed.Warning = "搜索命中的频道目前无法获取公开详情，需重新核实"
		return seed
	}
	item := payload.Items[0]
	seed.Name = item.Snippet.Title
	seed.Handle = item.Snippet.CustomURL
	seed.Description = item.Snippet.Description
	// These are channel-reported fields. Search language and query geography
	// are retrieval hints and must never become verified country/language facts.
	seed.Country = item.Snippet.Country
	seed.Language = item.Snippet.DefaultLanguage
	seed.AvatarURL = bestThumbnailURL(item.Snippet.Thumbnails)
	seed.TotalViews = parseCount(item.Statistics.ViewCount)
	seed.VideoCount = parseCount(item.Statistics.VideoCount)
	if item.Statistics.SubscriberCount != nil && !item.Statistics.HiddenSubscriberCount {
		if followers, err := strconv.ParseInt(*item.Statistics.SubscriberCount, 10, 64); err == nil && followers >= 0 {
			seed.Followers = &followers
		}
	}
	seed.Evidence = append(seed.Evidence, creatorDiscoveryEvidence{
		Title: "YouTube 频道公开资料", URL: seed.ProfileURL,
		Summary: "频道声明国家：" + firstNonEmpty(seed.Country, "未提供") + "；频道简介：" + seed.Description,
	})
	posts, err := p.recentPosts(ctx, item.ContentDetails.RelatedPlaylists.Uploads, seed.ChannelID)
	if err != nil {
		seed.Warning = "频道资料已采集，最近作品采集失败：" + err.Error()
	} else {
		seed.RecentPosts = posts
	}
	return seed
}

func (p creatorDiscoveryYouTubeClient) recentPosts(ctx context.Context, playlistID, channelID string) ([]platformPost, error) {
	posts := []platformPost{}
	if playlistID == "" {
		return posts, nil
	}
	var playlist struct {
		Items []struct {
			ContentDetails struct {
				VideoID string `json:"videoId"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	if err := p.get(ctx, "playlistItems", url.Values{"part": {"contentDetails"}, "playlistId": {playlistID}, "maxResults": {"6"}}, &playlist); err != nil {
		return posts, err
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, item := range playlist.Items {
		id := item.ContentDetails.VideoID
		if id != "" && !seen[id] && len(ids) < 6 {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	if len(ids) == 0 {
		return posts, nil
	}
	var payload struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				PublishedAt string `json:"publishedAt"`
				Title       string `json:"title"`
				Description string `json:"description"`
				ChannelID   string `json:"channelId"`
				Thumbnails  map[string]struct {
					URL string `json:"url"`
				} `json:"thumbnails"`
			} `json:"snippet"`
			Statistics struct {
				ViewCount    string `json:"viewCount"`
				LikeCount    string `json:"likeCount"`
				CommentCount string `json:"commentCount"`
			} `json:"statistics"`
			ContentDetails struct {
				Duration string `json:"duration"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	if err := p.get(ctx, "videos", url.Values{"part": {"snippet,statistics,contentDetails"}, "id": {strings.Join(ids, ",")}}, &payload); err != nil {
		return posts, err
	}
	byID := map[string]platformPost{}
	for _, item := range payload.Items {
		if !seen[item.ID] || item.Snippet.ChannelID != channelID {
			continue
		}
		cover := bestThumbnailURL(item.Snippet.Thumbnails)
		byID[item.ID] = platformPost{
			PlatformPostID: item.ID, Title: item.Snippet.Title, Description: item.Snippet.Description,
			PostURL: "https://www.youtube.com/watch?v=" + url.QueryEscape(item.ID), CoverURL: cover, CoverRemoteURL: cover,
			MediaType: "VIDEO", PublishedAt: parsePlatformTime(item.Snippet.PublishedAt), Duration: parseYouTubeDurationSeconds(item.ContentDetails.Duration),
			ViewCount: parseCount(item.Statistics.ViewCount), LikeCount: parseCount(item.Statistics.LikeCount), CommentCount: parseCount(item.Statistics.CommentCount), Raw: item,
		}
	}
	for _, id := range ids {
		if post, ok := byID[id]; ok {
			posts = append(posts, post)
		}
	}
	return posts, nil
}
