package main

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (a *app) tikTokCollectorCallback(w http.ResponseWriter, r *http.Request) {
	if !a.validCollectorToken(r) {
		writeError(w, http.StatusUnauthorized, 401, "采集插件未授权")
		return
	}
	body := readBody(r)
	result, err := a.persistTikTokCollectorPayload(r.Context(), body)
	if err != nil {
		writeError(w, http.StatusOK, 10001, err.Error())
		return
	}
	writeOK(w, result)
}

func (a *app) validCollectorToken(r *http.Request) bool {
	expected := strings.TrimSpace(a.Config().Collector.AgentToken)
	if expected == "" {
		return false
	}
	actual := strings.TrimSpace(r.Header.Get("X-Collector-Token"))
	if actual == "" {
		token := strings.TrimSpace(r.Header.Get("Authorization"))
		actual = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

func (a *app) persistTikTokCollectorPayload(ctx context.Context, body map[string]any) (map[string]any, error) {
	creator, ok := body["creator"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("creator 不能为空")
	}
	username := sanitizeTikTokHandle(firstNonEmpty(anyString(creator["username"]), anyString(creator["uniqueId"]), anyString(creator["handle"])))
	secUID := firstNonEmpty(anyString(creator["secUid"]), anyString(creator["sec_uid"]), anyString(creator["platformUserId"]))
	displayName := firstNonEmpty(anyString(creator["name"]), anyString(creator["displayName"]), username)
	profileURL := firstNonEmpty(anyString(creator["profileUrl"]), tikTokProfileURL(username))
	if username == "" && secUID == "" {
		return nil, fmt.Errorf("creator.username 或 creator.secUid 至少需要一个")
	}

	resourceID, created, err := a.upsertTikTokCollectorResource(ctx, int64Field(body, "resourceId"), collectorCreator{
		Name:          displayName,
		Username:      username,
		SecUID:        secUID,
		AvatarURL:     anyString(creator["avatarUrl"]),
		ProfileURL:    profileURL,
		Followers:     anyInt64(creator["followerCount"]),
		Following:     anyInt64(creator["followingCount"]),
		Likes:         anyInt64(creator["likesCount"]),
		VideoCount:    anyInt64(creator["videoCount"]),
		EngagementRaw: anyFloat64(creator["engagementRate"]),
	})
	if err != nil {
		return nil, err
	}

	posts := collectorPosts(body["posts"])
	if err := a.upsertResourcePlatformPosts(ctx, int(resourceID), "TikTok", posts); err != nil {
		return nil, err
	}
	avgViews := averagePostViews(posts)
	totalViews := sumPostViews(posts)
	if avgViews > 0 || totalViews > 0 {
		_, err = a.DB().ExecContext(ctx,
			`update biz_resources set
			   avg_views = if(? > 0, ?, avg_views),
			   total_views = if(? > 0, ?, total_views)
			 where id = ?`,
			avgViews, avgViews, totalViews, totalViews, resourceID,
		)
		if err != nil {
			return nil, err
		}
	}

	return map[string]any{
		"resourceId":  resourceID,
		"created":     created,
		"syncedPosts": len(posts),
		"platform":    "TikTok",
		"username":    username,
		"secUid":      secUID,
		"receivedAt":  time.Now().Format(time.RFC3339),
	}, nil
}

type collectorCreator struct {
	Name          string
	Username      string
	SecUID        string
	AvatarURL     string
	ProfileURL    string
	Followers     int64
	Following     int64
	Likes         int64
	VideoCount    int64
	EngagementRaw float64
}

func (a *app) upsertTikTokCollectorResource(ctx context.Context, requestedID int64, creator collectorCreator) (int64, bool, error) {
	if requestedID > 0 {
		if _, err := a.DB().ExecContext(ctx,
			`update biz_resources set
			   name = if(? <> '', ?, name),
			   platform = 'TikTok',
			   followers = if(? > 0, ?, followers),
			   video_count = if(? > 0, ?, video_count),
			   engagement_rate = if(? > 0, ?, engagement_rate),
			   platform_user_id = if(? <> '', ?, platform_user_id),
			   platform_handle = if(? <> '', ?, platform_handle),
			   platform_url = if(? <> '', ?, platform_url),
			   avatar_url = if(? <> '', ?, avatar_url),
			   last_sync_status = '成功',
			   last_sync_error = '',
			   last_sync_at = now()
			 where id = ?`,
			creator.Name, creator.Name, creator.Followers, creator.Followers,
			creator.VideoCount, creator.VideoCount, creator.EngagementRaw, creator.EngagementRaw,
			creator.SecUID, creator.SecUID, creator.Username, creator.Username,
			creator.ProfileURL, creator.ProfileURL, creator.AvatarURL, creator.AvatarURL, requestedID,
		); err != nil {
			return 0, false, err
		}
		return requestedID, false, nil
	}

	id, err := a.findTikTokCollectorResource(ctx, creator)
	if err != nil {
		return 0, false, err
	}
	if id > 0 {
		_, err = a.DB().ExecContext(ctx,
			`update biz_resources set
			   name = if(? <> '', ?, name),
			   followers = if(? > 0, ?, followers),
			   video_count = if(? > 0, ?, video_count),
			   engagement_rate = if(? > 0, ?, engagement_rate),
			   platform_user_id = if(? <> '', ?, platform_user_id),
			   platform_handle = if(? <> '', ?, platform_handle),
			   platform_url = if(? <> '', ?, platform_url),
			   avatar_url = if(? <> '', ?, avatar_url),
			   last_sync_status = '成功',
			   last_sync_error = '',
			   last_sync_at = now()
			 where id = ?`,
			creator.Name, creator.Name, creator.Followers, creator.Followers,
			creator.VideoCount, creator.VideoCount, creator.EngagementRaw, creator.EngagementRaw,
			creator.SecUID, creator.SecUID, creator.Username, creator.Username,
			creator.ProfileURL, creator.ProfileURL, creator.AvatarURL, creator.AvatarURL, id,
		)
		return id, false, err
	}

	result, err := a.DB().ExecContext(ctx,
		`insert into biz_resources
		  (name, resource_type, platform, status, followers, engagement_rate, video_count,
		   platform_user_id, platform_handle, platform_url, avatar_url, score, level,
		   risk_level, last_sync_status, last_sync_error, last_sync_at)
		 values (?, 'KOL', 'TikTok', '可合作', ?, ?, ?, ?, ?, ?, ?, 60, 'B',
		         '低', '成功', '', now())`,
		firstNonEmpty(creator.Name, creator.Username, creator.SecUID), creator.Followers,
		creator.EngagementRaw, creator.VideoCount, creator.SecUID, creator.Username,
		creator.ProfileURL, creator.AvatarURL,
	)
	if err != nil {
		return 0, false, err
	}
	newID, err := result.LastInsertId()
	return newID, true, err
}

func (a *app) findTikTokCollectorResource(ctx context.Context, creator collectorCreator) (int64, error) {
	var id int64
	if creator.SecUID != "" {
		err := a.DB().QueryRowContext(ctx,
			`select id from biz_resources
			  where platform = 'TikTok' and platform_user_id = ?
			  limit 1`,
			creator.SecUID,
		).Scan(&id)
		if err == nil || !isNoRows(err) {
			return id, err
		}
	}
	if creator.Username != "" {
		err := a.DB().QueryRowContext(ctx,
			`select id from biz_resources
			  where platform = 'TikTok' and platform_handle = ?
			  limit 1`,
			creator.Username,
		).Scan(&id)
		if err == nil || !isNoRows(err) {
			return id, err
		}
	}
	return 0, nil
}

func collectorPosts(raw any) []platformPost {
	values, ok := raw.([]any)
	if !ok {
		return nil
	}
	posts := make([]platformPost, 0, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		id := firstNonEmpty(anyString(item["id"]), anyString(item["platformPostId"]), anyString(item["awemeId"]))
		if id == "" {
			continue
		}
		publishedAt := collectorPostTime(item)
		posts = append(posts, platformPost{
			PlatformPostID: id,
			Title:          firstNonEmpty(anyString(item["title"]), truncateText(anyString(item["description"]), 120)),
			Description:    firstNonEmpty(anyString(item["description"]), anyString(item["desc"])),
			PostURL:        firstNonEmpty(anyString(item["url"]), anyString(item["postUrl"]), tikTokVideoURL(anyString(item["username"]), id)),
			CoverURL:       firstNonEmpty(anyString(item["coverUrl"]), anyString(item["cover"])),
			MediaType:      firstNonEmpty(anyString(item["mediaType"]), "VIDEO"),
			PublishedAt:    publishedAt,
			Duration:       int(anyInt64(item["durationSeconds"])),
			ViewCount:      anyInt64(firstPresent(item, "viewCount", "playCount")),
			LikeCount:      anyInt64(item["likeCount"]),
			CommentCount:   anyInt64(item["commentCount"]),
			ShareCount:     anyInt64(item["shareCount"]),
			Raw:            item,
		})
	}
	return posts
}

func collectorPostTime(item map[string]any) *time.Time {
	if value := anyString(firstPresent(item, "publishedAt", "createTime", "createdAt")); value != "" {
		if parsed := parsePlatformTime(value); parsed != nil {
			return parsed
		}
		if ts := anyInt64(value); ts > 0 {
			if ts > 9999999999 {
				ts = ts / 1000
			}
			utc := time.Unix(ts, 0).UTC()
			return &utc
		}
	}
	return nil
}

func firstPresent(item map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			return value
		}
	}
	return nil
}

func anyString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func int64Field(body map[string]any, key string) int64 {
	return anyInt64(body[key])
}

func anyInt64(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	case string:
		value := strings.TrimSpace(v)
		if value == "" {
			return 0
		}
		var n int64
		_, _ = fmt.Sscan(value, &n)
		return n
	default:
		return 0
	}
}

func anyFloat64(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		value := strings.TrimSpace(v)
		if value == "" {
			return 0
		}
		var n float64
		_, _ = fmt.Sscan(value, &n)
		return n
	default:
		return 0
	}
}

func sanitizeTikTokHandle(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	value = strings.TrimPrefix(value, "@")
	if strings.Contains(value, "tiktok.com") {
		parts := strings.Split(value, "/@")
		if len(parts) > 1 {
			value = strings.Split(parts[1], "/")[0]
		}
	}
	return strings.TrimPrefix(value, "@")
}

func tikTokProfileURL(username string) string {
	username = sanitizeTikTokHandle(username)
	if username == "" {
		return ""
	}
	return "https://www.tiktok.com/@" + username
}

func tikTokVideoURL(username, postID string) string {
	username = sanitizeTikTokHandle(username)
	postID = strings.TrimSpace(postID)
	if username == "" || postID == "" {
		return ""
	}
	return "https://www.tiktok.com/@" + username + "/video/" + postID
}

func isNoRows(err error) bool {
	return err == sql.ErrNoRows
}
