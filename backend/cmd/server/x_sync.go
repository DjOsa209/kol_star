package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type tikHubXUser struct {
	ID             string
	Handle         string
	DisplayName    string
	AvatarURL      string
	FollowerCount  int64
	FollowingCount int64
	PostCount      int64
}

func xHandleIdentifier(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "<nil>" {
			continue
		}
		if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
			host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
			if host != "x.com" && host != "twitter.com" && host != "mobile.twitter.com" {
				continue
			}
			segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
			if len(segments) == 0 || segments[0] == "" {
				continue
			}
			switch strings.ToLower(segments[0]) {
			case "home", "explore", "search", "i", "intent", "share":
				continue
			default:
				return sanitizeXHandle(segments[0])
			}
		}
		if !strings.ContainsAny(value, " /") {
			return sanitizeXHandle(value)
		}
	}
	return ""
}

func sanitizeXHandle(value string) string {
	value = strings.TrimPrefix(strings.Trim(strings.TrimSpace(value), "/"), "@")
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func (a *app) syncXResource(ctx context.Context, id int) (map[string]any, error) {
	apiKey := strings.TrimSpace(tikHubAPIKey(a.effectivePlatformAPIConfig(ctx)))
	if apiKey == "" {
		return nil, fmt.Errorf("未配置 TikHub API Key，请在系统管理/抓取控制中配置")
	}
	var resource struct {
		Name           string
		ResourceType   string
		PlatformURL    string
		PlatformUserID string
		PlatformHandle string
	}
	if err := a.DB().QueryRowContext(ctx,
		`select name, resource_type, platform_url, platform_user_id, platform_handle
		   from biz_resources where id = ? limit 1`,
		id,
	).Scan(&resource.Name, &resource.ResourceType, &resource.PlatformURL, &resource.PlatformUserID, &resource.PlatformHandle); err != nil {
		return nil, err
	}
	handle := xHandleIdentifier(resource.PlatformHandle, resource.PlatformURL, resource.Name)
	if handle == "" && strings.TrimSpace(resource.PlatformUserID) == "" {
		return nil, fmt.Errorf("请先填写 X 主页链接、@handle 或 rest_id")
	}
	params := url.Values{}
	if handle != "" {
		params.Set("screen_name", handle)
	} else {
		params.Set("rest_id", strings.TrimSpace(resource.PlatformUserID))
	}
	client := &http.Client{Timeout: 45 * time.Second}
	profileData, err := tikhubGET(ctx, client, apiKey, "/twitter/web/fetch_user_profile", params)
	if err != nil {
		return nil, err
	}
	user := normalizeTikHubXUser(profileData, handle, resource.PlatformUserID)
	if user.ID == "" && user.Handle == "" {
		return nil, fmt.Errorf("TikHub 未返回 X 账号数据，请检查 @handle 或 rest_id")
	}

	syncPosts, postLimit := a.platformPostSyncOptions(ctx, "X", 20)
	posts := []platformPost{}
	warnings := []string{}
	if syncPosts {
		postParams := url.Values{}
		if user.Handle != "" {
			postParams.Set("screen_name", user.Handle)
		} else {
			postParams.Set("rest_id", user.ID)
		}
		postData, postErr := tikhubGET(ctx, client, apiKey, "/twitter/web/fetch_user_post_tweet", postParams)
		if postErr != nil {
			warnings = append(warnings, "X 作品获取失败："+postErr.Error())
		} else {
			posts = limitPlatformPosts(normalizeTikHubXPosts(postData, user.Handle), postLimit)
			if err := a.upsertResourcePlatformPosts(ctx, id, "X", posts); err != nil {
				return nil, err
			}
		}
	}

	totalViews := sumPostViews(posts)
	avgViews := averageViewedPostViews(posts)
	engagementRate := platformPostEngagementRate(posts, user.FollowerCount)
	avatarURL := normalizedRemoteImageURL(user.AvatarURL)
	resourceName := syncedResourceName(resource.PlatformURL, firstNonEmpty(user.DisplayName, user.Handle))
	_, err = a.DB().ExecContext(ctx,
		`update biz_resources set
		  name = if(? <> '', ?, name),
		  followers = ?, total_views = if(? > 0, ?, total_views),
		  avg_views = if(? > 0, ?, avg_views), video_count = ?,
		  engagement_rate = if(? > 0, ?, engagement_rate),
		  platform = 'X', platform_user_id = ?, platform_handle = ?,
		  platform_url = if(? <> '', ?, platform_url),
		  avatar_url = if(? <> '', ?, avatar_url),
		  avatar_remote_url = if(? <> '', ?, avatar_remote_url),
		  audience_size_unit = if(resource_type = '媒体', 'UMV', 'Followers'),
		  last_sync_status = '成功', last_sync_error = '', last_sync_at = now()
		 where id = ?`,
		resourceName, resourceName, user.FollowerCount,
		totalViews, totalViews, avgViews, avgViews, user.PostCount,
		engagementRate, engagementRate, user.ID, user.Handle,
		user.Handle, "https://x.com/"+user.Handle,
		avatarURL, avatarURL, user.AvatarURL, user.AvatarURL, id,
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"platform":       "X",
		"platformUserId": user.ID,
		"platformHandle": user.Handle,
		"name":           resourceName,
		"followers":      user.FollowerCount,
		"totalViews":     totalViews,
		"avgViews":       avgViews,
		"videoCount":     user.PostCount,
		"engagementRate": engagementRate,
		"avatarUrl":      avatarURL,
		"syncedPosts":    len(posts),
		"posts":          posts,
		"warnings":       warnings,
		"syncedAt":       time.Now().Format(time.RFC3339),
	}, nil
}

func (a *app) fetchXPostByURL(ctx context.Context, resourceID int, postID, postURL string) (platformPost, error) {
	apiKey := strings.TrimSpace(tikHubAPIKey(a.effectivePlatformAPIConfig(ctx)))
	if apiKey == "" {
		return platformPost{}, fmt.Errorf("未配置 TikHub API Key")
	}
	postID = strings.TrimSpace(postID)
	if postID == "" {
		return platformPost{}, fmt.Errorf("X 作品链接中缺少推文 ID")
	}
	data, err := tikhubGET(ctx, &http.Client{Timeout: 45 * time.Second}, apiKey,
		"/twitter/web/fetch_tweet_detail", url.Values{"tweet_id": []string{postID}})
	if err != nil {
		return platformPost{}, err
	}
	posts := normalizeTikHubXPosts(data, xHandleIdentifier(postURL))
	if len(posts) == 0 {
		return platformPost{}, fmt.Errorf("TikHub 未返回 X 作品数据")
	}
	post := posts[0]
	if post.PlatformPostID != postID {
		for _, candidate := range posts {
			if candidate.PlatformPostID == postID {
				post = candidate
				break
			}
		}
	}
	post.PostURL = firstNonEmpty(strings.TrimSpace(postURL), post.PostURL)
	if err := a.upsertSingleContentPlatformPost(ctx, resourceID, "X", post); err != nil {
		return platformPost{}, err
	}
	return post, nil
}

func normalizeTikHubXUser(data map[string]any, fallbackHandle, fallbackID string) tikHubXUser {
	candidates := collectNestedMaps(data)
	for _, candidate := range candidates {
		legacy := mapAt(candidate, "legacy")
		source := candidate
		if len(legacy) > 0 {
			source = legacy
		}
		handle := sanitizeXHandle(firstNonEmpty(
			anyString(source["screen_name"]),
			anyString(source["username"]),
			anyString(candidate["screen_name"]),
			anyString(source["profile"]),
			anyString(candidate["profile"]),
		))
		id := firstNonEmpty(anyString(candidate["rest_id"]), anyString(candidate["id_str"]), anyString(candidate["id"]))
		if handle == "" && id == "" {
			continue
		}
		return tikHubXUser{
			ID:          firstNonEmpty(id, fallbackID),
			Handle:      firstNonEmpty(handle, fallbackHandle),
			DisplayName: firstNonEmpty(anyString(source["name"]), anyString(candidate["name"]), handle, fallbackHandle),
			AvatarURL: firstNonEmpty(
				imageURL(source["profile_image_url_https"]),
				imageURL(source["profile_image_url"]),
				imageURL(candidate["profile_image_url_https"]),
				imageURL(candidate["avatar"]),
			),
			FollowerCount: firstNonZeroInt64(
				source["followers_count"], candidate["followers_count"], candidate["followers"],
				source["sub_count"], candidate["sub_count"],
			),
			FollowingCount: firstNonZeroInt64(
				source["friends_count"], source["following_count"], candidate["following_count"],
				source["friends"], candidate["friends"],
			),
			PostCount: firstNonZeroInt64(
				source["statuses_count"], candidate["statuses_count"], candidate["tweet_count"],
			),
		}
	}
	return tikHubXUser{}
}

func normalizeTikHubXPosts(data map[string]any, handle string) []platformPost {
	seen := map[string]bool{}
	posts := make([]platformPost, 0)
	for _, candidate := range collectNestedMaps(data) {
		legacy := mapAt(candidate, "legacy")
		source := candidate
		if len(legacy) > 0 {
			source = legacy
		}
		id := firstNonEmpty(
			anyString(candidate["tweet_id"]),
			anyString(candidate["rest_id"]),
			anyString(source["id_str"]),
			anyString(candidate["id_str"]),
			anyString(candidate["id"]),
		)
		text := firstNonEmpty(anyString(source["full_text"]), anyString(source["text"]), anyString(candidate["full_text"]))
		if id == "" || text == "" || seen[id] {
			continue
		}
		seen[id] = true
		viewCount := firstNonZeroInt64(
			source["view_count"],
			candidate["view_count"],
			source["views"],
			candidate["views"],
			mapAt(candidate, "views")["count"],
		)
		coverURL, mediaType := xPostMedia(source)
		if coverURL == "" {
			coverURL, mediaType = xPostMedia(candidate)
		}
		posts = append(posts, platformPost{
			PlatformPostID: id,
			Title:          truncateText(text, 120),
			Description:    text,
			PostURL:        "https://x.com/" + firstNonEmpty(handle, "i") + "/status/" + id,
			CoverURL:       coverURL,
			MediaType:      firstNonEmpty(mediaType, "POST"),
			PublishedAt:    parseXTime(firstNonEmpty(anyString(source["created_at"]), anyString(candidate["created_at"]))),
			ViewCount:      viewCount,
			LikeCount: firstNonZeroInt64(
				source["favorite_count"], source["like_count"], candidate["favorite_count"],
				source["favorites"], candidate["favorites"], source["likes"], candidate["likes"],
			),
			CommentCount: firstNonZeroInt64(
				source["reply_count"], candidate["reply_count"], source["replies"], candidate["replies"],
			),
			ShareCount: firstNonZeroInt64(
				source["retweet_count"], candidate["retweet_count"], source["retweets"], candidate["retweets"],
			) + firstNonZeroInt64(
				source["quote_count"], candidate["quote_count"], source["quotes"], candidate["quotes"],
			),
			SaveCount: firstNonZeroInt64(source["bookmarks"], candidate["bookmarks"]),
			Raw:       candidate,
		})
	}
	return posts
}

func collectNestedMaps(value any) []map[string]any {
	result := make([]map[string]any, 0)
	var visit func(any)
	visit = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			result = append(result, typed)
			for _, child := range typed {
				visit(child)
			}
		case []any:
			for _, child := range typed {
				visit(child)
			}
		}
	}
	visit(value)
	return result
}

func xPostMedia(row map[string]any) (string, string) {
	media := mapAt(row, "media")
	for _, mediaType := range []string{"video", "photo", "animated_gif"} {
		for _, item := range firstListAt(media, mediaType) {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			cover := firstNonEmpty(
				imageURL(entry["media_url_https"]),
				imageURL(entry["media_url"]),
				imageURL(entry["url"]),
			)
			if cover != "" {
				return cover, strings.ToUpper(mediaType)
			}
		}
	}
	for _, containerKey := range []string{"extended_entities", "entities"} {
		container := mapAt(row, containerKey)
		for _, item := range firstListAt(container, "media") {
			media, ok := item.(map[string]any)
			if !ok {
				continue
			}
			mediaType := strings.ToUpper(anyString(media["type"]))
			cover := firstNonEmpty(
				imageURL(media["media_url_https"]),
				imageURL(media["media_url"]),
				imageURL(media["url"]),
			)
			if cover != "" {
				return cover, mediaType
			}
		}
	}
	return "", ""
}

func parseXTime(value string) *time.Time {
	if parsed := parsePlatformTime(value); parsed != nil {
		return parsed
	}
	for _, layout := range []string{time.RubyDate, "Mon Jan 02 15:04:05 -0700 2006"} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}
