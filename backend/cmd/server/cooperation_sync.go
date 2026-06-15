package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type cooperationPostLink struct {
	Platform string
	PostID   string
	URL      string
}

type cooperationPostSyncResult struct {
	Synced   bool   `json:"synced"`
	Source   string `json:"source,omitempty"`
	Platform string `json:"platform,omitempty"`
	PostID   string `json:"postId,omitempty"`
	Message  string `json:"message,omitempty"`
}

func (a *app) syncCooperationPost(ctx context.Context, cooperationID int, allowAPI bool) (cooperationPostSyncResult, error) {
	var resourceID int
	var deliverableLinks string
	if err := a.DB().QueryRowContext(ctx,
		`select resource_id, coalesce(deliverable_links, '')
		   from biz_cooperations where id = ? limit 1`,
		cooperationID,
	).Scan(&resourceID, &deliverableLinks); err != nil {
		return cooperationPostSyncResult{}, err
	}
	if strings.TrimSpace(deliverableLinks) == "" {
		return cooperationPostSyncResult{}, nil
	}
	link, err := parseCooperationPostLink(deliverableLinks)
	if err != nil {
		return cooperationPostSyncResult{Message: err.Error()}, nil
	}

	post, found, err := a.findStoredPlatformPost(ctx, resourceID, link)
	if err != nil {
		return cooperationPostSyncResult{}, err
	}
	source := "作品库"
	if !found && allowAPI {
		post, err = a.fetchCooperationPlatformPost(ctx, resourceID, link)
		if err != nil {
			return cooperationPostSyncResult{
				Platform: link.Platform,
				PostID:   link.PostID,
				Message:  "API 获取作品数据失败：" + err.Error(),
			}, nil
		}
		found = true
		source = "API"
	}
	if !found {
		return cooperationPostSyncResult{
			Platform: link.Platform,
			PostID:   link.PostID,
			Message:  "作品库中未找到匹配作品",
		}, nil
	}
	if err := a.applyPlatformPostToCooperation(ctx, cooperationID, post); err != nil {
		return cooperationPostSyncResult{}, err
	}
	return cooperationPostSyncResult{
		Synced:   true,
		Source:   source,
		Platform: link.Platform,
		PostID:   post.PlatformPostID,
		Message:  fmt.Sprintf("已通过%s同步合作作品数据", source),
	}, nil
}

func (a *app) syncCooperationsFromStoredPostsForResource(ctx context.Context, resourceID int) error {
	rows, err := a.DB().QueryContext(ctx,
		`select id from biz_cooperations
		  where resource_id = ? and coalesce(deliverable_links, '') <> ''`,
		resourceID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	var cooperationIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return err
		}
		cooperationIDs = append(cooperationIDs, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range cooperationIDs {
		if _, err := a.syncCooperationPost(ctx, id, false); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) syncImportedCooperations(ctx context.Context, batchID string) (int, []string) {
	rows, err := a.DB().QueryContext(ctx,
		`select id from biz_cooperations where import_batch_id = ? order by id`,
		batchID,
	)
	if err != nil {
		return 0, []string{err.Error()}
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return 0, []string{err.Error()}
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, []string{err.Error()}
	}
	synced := 0
	var warnings []string
	for _, id := range ids {
		result, err := a.syncCooperationPost(ctx, id, true)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("合作记录 %d：%v", id, err))
			continue
		}
		if result.Synced {
			synced++
		} else if result.Message != "" {
			warnings = append(warnings, fmt.Sprintf("合作记录 %d：%s", id, result.Message))
		}
	}
	return synced, warnings
}

func parseCooperationPostLink(value string) (cooperationPostLink, error) {
	for _, field := range strings.Fields(value) {
		candidate := strings.Trim(field, "，,;；")
		if index := strings.Index(candidate, "http"); index >= 0 {
			candidate = candidate[index:]
		}
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Host == "" {
			continue
		}
		host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
		segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		switch {
		case host == "youtu.be" && len(segments) > 0:
			return cooperationPostLink{Platform: "YouTube", PostID: segments[0], URL: candidate}, nil
		case strings.HasSuffix(host, "youtube.com"):
			id := parsed.Query().Get("v")
			if id == "" && len(segments) >= 2 && (segments[0] == "shorts" || segments[0] == "embed") {
				id = segments[1]
			}
			if id != "" {
				return cooperationPostLink{Platform: "YouTube", PostID: id, URL: candidate}, nil
			}
		case strings.HasSuffix(host, "tiktok.com"):
			for index, segment := range segments {
				if segment == "video" && index+1 < len(segments) {
					return cooperationPostLink{Platform: "TikTok", PostID: segments[index+1], URL: candidate}, nil
				}
			}
		case strings.HasSuffix(host, "instagram.com"):
			if len(segments) >= 2 && (segments[0] == "p" || segments[0] == "reel" || segments[0] == "reels" || segments[0] == "tv") {
				return cooperationPostLink{Platform: "Instagram", PostID: segments[1], URL: candidate}, nil
			}
		}
	}
	return cooperationPostLink{}, fmt.Errorf("发布链接不是可识别的 YouTube、TikTok 或 Instagram 作品链接")
}

func (a *app) findStoredPlatformPost(ctx context.Context, resourceID int, link cooperationPostLink) (platformPost, bool, error) {
	rows, err := a.DB().QueryContext(ctx,
		`select platform_post_id, title, description, post_url, cover_url, media_type,
		        published_at, duration_seconds, view_count, like_count, comment_count, share_count
		   from biz_resource_platform_posts
		  where resource_id = ? and platform = ?
		  order by synced_at desc`,
		resourceID, link.Platform,
	)
	if err != nil {
		return platformPost{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		post, err := scanPlatformPost(rows)
		if err != nil {
			return platformPost{}, false, err
		}
		if platformPostMatchesLink(post, link) {
			return post, true, nil
		}
	}
	return platformPost{}, false, rows.Err()
}

func scanPlatformPost(scanner interface{ Scan(...any) error }) (platformPost, error) {
	var post platformPost
	var publishedAt sql.NullTime
	err := scanner.Scan(
		&post.PlatformPostID, &post.Title, &post.Description, &post.PostURL, &post.CoverURL,
		&post.MediaType, &publishedAt, &post.Duration, &post.ViewCount, &post.LikeCount,
		&post.CommentCount, &post.ShareCount,
	)
	if publishedAt.Valid {
		post.PublishedAt = &publishedAt.Time
	}
	return post, err
}

func platformPostMatchesLink(post platformPost, link cooperationPostLink) bool {
	if link.PostID != "" && strings.EqualFold(strings.TrimSpace(post.PlatformPostID), link.PostID) {
		return true
	}
	postLink, err := parseCooperationPostLink(post.PostURL)
	return err == nil && postLink.Platform == link.Platform && postLink.PostID == link.PostID
}

func (a *app) applyPlatformPostToCooperation(ctx context.Context, cooperationID int, post platformPost) error {
	var releaseDate any
	if post.PublishedAt != nil {
		releaseDate = post.PublishedAt.Format("2006-01-02")
	}
	_, err := a.DB().ExecContext(ctx,
		`update biz_cooperations set
		  views = ?, engagement_count = ?, comments_count = ?,
		  release_date = coalesce(?, release_date)
		 where id = ?`,
		post.ViewCount, post.LikeCount+post.ShareCount, post.CommentCount, releaseDate, cooperationID,
	)
	return err
}

func (a *app) fetchCooperationPlatformPost(ctx context.Context, resourceID int, link cooperationPostLink) (platformPost, error) {
	switch link.Platform {
	case "YouTube":
		return a.fetchYouTubePostByID(ctx, resourceID, link.PostID)
	case "TikTok":
		return a.fetchTikTokPostByID(ctx, resourceID, link.PostID)
	case "Instagram":
		return a.fetchInstagramPostByURL(ctx, resourceID, link.URL)
	default:
		return platformPost{}, fmt.Errorf("暂不支持平台 %s", link.Platform)
	}
}

func (a *app) fetchTikTokPostByID(ctx context.Context, resourceID int, postID string) (platformPost, error) {
	apiKey := strings.TrimSpace(tikHubAPIKey(a.effectivePlatformAPIConfig(ctx)))
	if apiKey == "" {
		return platformPost{}, fmt.Errorf("未配置 TikHub API Key")
	}
	data, err := tikhubGET(ctx, &http.Client{Timeout: 20 * time.Second}, apiKey,
		"/tiktok/app/v3/fetch_one_video_v2", url.Values{"aweme_id": []string{postID}})
	if err != nil {
		return platformPost{}, err
	}
	item := findSinglePlatformItem(data)
	posts := normalizeTikHubTikTokPosts(map[string]any{"items": []any{item}}, "")
	if len(posts) == 0 {
		return platformPost{}, fmt.Errorf("TikHub 未返回 TikTok 作品数据")
	}
	post := posts[0]
	if err := a.upsertResourcePlatformPosts(ctx, resourceID, "TikTok", []platformPost{post}); err != nil {
		return platformPost{}, err
	}
	return post, nil
}

func (a *app) fetchInstagramPostByURL(ctx context.Context, resourceID int, postURL string) (platformPost, error) {
	apiKey := strings.TrimSpace(tikHubAPIKey(a.effectivePlatformAPIConfig(ctx)))
	if apiKey == "" {
		return platformPost{}, fmt.Errorf("未配置 TikHub API Key")
	}
	data, err := tikhubGET(ctx, &http.Client{Timeout: 20 * time.Second}, apiKey,
		"/instagram/v1/fetch_post_by_url", url.Values{"post_url": []string{postURL}})
	if err != nil {
		return platformPost{}, err
	}
	item := findSinglePlatformItem(data)
	posts := normalizeTikHubInstagramPosts(map[string]any{"items": []any{item}}, "")
	if len(posts) == 0 {
		return platformPost{}, fmt.Errorf("TikHub 未返回 Instagram 作品数据")
	}
	post := posts[0]
	if err := a.upsertResourcePlatformPosts(ctx, resourceID, "Instagram", []platformPost{post}); err != nil {
		return platformPost{}, err
	}
	return post, nil
}

func findSinglePlatformItem(data map[string]any) map[string]any {
	return findSinglePlatformItemValue(data, 0)
}

func findSinglePlatformItemValue(value any, depth int) map[string]any {
	if depth > 8 {
		return map[string]any{}
	}
	data, ok := value.(map[string]any)
	if !ok {
		if list, ok := value.([]any); ok {
			for _, item := range list {
				if found := findSinglePlatformItemValue(item, depth+1); len(found) > 0 {
					return found
				}
			}
		}
		return map[string]any{}
	}
	if firstNonEmpty(
		anyString(data["id"]), anyString(data["pk"]), anyString(data["aweme_id"]),
		anyString(data["shortcode"]), anyString(data["code"]),
	) != "" {
		return data
	}
	for _, key := range []string{"aweme_detail", "aweme", "item", "item_info", "item_struct", "media", "post", "data"} {
		if nested, exists := data[key]; exists {
			if found := findSinglePlatformItemValue(nested, depth+1); len(found) > 0 {
				return found
			}
		}
	}
	for _, key := range []string{"items", "value", "list", "aweme_list", "medias"} {
		if nested, exists := data[key]; exists {
			if found := findSinglePlatformItemValue(nested, depth+1); len(found) > 0 {
				return found
			}
		}
	}
	return map[string]any{}
}
