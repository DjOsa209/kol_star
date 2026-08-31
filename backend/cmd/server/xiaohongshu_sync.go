package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func xiaohongshuUserIdentifier(values ...string) (userID, profileURL string) {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "<nil>" {
			continue
		}
		if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
			host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
			if host != "xiaohongshu.com" && !strings.HasSuffix(host, ".xiaohongshu.com") && host != "xhslink.com" && host != "xhslink.cn" {
				continue
			}
			segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
			for index, segment := range segments {
				if strings.EqualFold(segment, "profile") && index+1 < len(segments) {
					if id := sanitizeSocialHandle(segments[index+1]); id != "" {
						return id, "https://www.xiaohongshu.com/user/profile/" + id
					}
				}
			}
			return "", value
		}
		if !strings.ContainsAny(value, " /@") {
			if id := sanitizeSocialHandle(value); id != "" {
				return id, "https://www.xiaohongshu.com/user/profile/" + id
			}
		}
	}
	return "", ""
}

func xiaohongshuAPIParams(userID, shareText string) url.Values {
	params := url.Values{}
	if strings.TrimSpace(userID) != "" {
		params.Set("user_id", strings.TrimSpace(userID))
	} else if strings.TrimSpace(shareText) != "" {
		params.Set("share_text", strings.TrimSpace(shareText))
	}
	return params
}

func (a *app) syncXiaohongshuResource(ctx context.Context, id int) (map[string]any, error) {
	apiKey := strings.TrimSpace(tikHubAPIKey(a.effectivePlatformAPIConfig(ctx)))
	if apiKey == "" {
		return nil, fmt.Errorf("未配置 TikHub API Key，请在系统管理/抓取控制中配置")
	}
	resource, err := a.extendedSocialResource(ctx, id)
	if err != nil {
		return nil, err
	}
	userID, profileURL := xiaohongshuUserIdentifier(resource.PlatformUserID, resource.PlatformURL)
	if userID == "" && profileURL == "" {
		return nil, fmt.Errorf("请先填写有效的小红书用户 ID 或主页/分享链接")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	profileData, err := tikhubGETWithTransientRetry(ctx, client, apiKey,
		"/xiaohongshu/app_v2/get_user_info", xiaohongshuAPIParams(userID, profileURL))
	if err != nil {
		return nil, err
	}
	if serviceErr := xiaohongshuServiceError(profileData); serviceErr != "" {
		return nil, fmt.Errorf("小红书用户信息获取失败：%s", serviceErr)
	}
	user := normalizeTikHubXiaohongshuUser(profileData, userID, profileURL)
	if user.ID == "" && user.Handle == "" {
		return nil, fmt.Errorf("TikHub 未返回小红书账号数据，请检查用户 ID 或主页链接")
	}

	syncPosts, postLimit := a.platformPostSyncOptions(ctx, "小红书", 20)
	posts := []platformPost{}
	warnings := []string{}
	if syncPosts {
		params := xiaohongshuAPIParams(firstNonEmpty(user.ID, userID), profileURL)
		fetchedPosts, postErr := a.fetchXiaohongshuUserPosts(ctx, client, apiKey, params, postLimit)
		if postErr != nil {
			warnings = append(warnings, "小红书笔记获取失败："+postErr.Error())
		} else {
			posts = fetchedPosts
			if err := a.upsertResourcePlatformPosts(ctx, id, "小红书", posts); err != nil {
				return nil, err
			}
		}
	}
	if user.PostCount == 0 {
		user.PostCount = int64(len(posts))
	}
	return a.persistExtendedSocialUser(ctx, id, "小红书", user, posts, warnings)
}

func (a *app) fetchXiaohongshuUserPosts(
	ctx context.Context,
	client *http.Client,
	apiKey string,
	baseParams url.Values,
	postLimit int,
) ([]platformPost, error) {
	postLimit = clampInt(postLimit, 1, 50)
	posts := make([]platformPost, 0, postLimit)
	seenPosts := map[string]bool{}
	seenCursors := map[string]bool{}
	cursor := ""
	for page := 0; page < 10 && len(posts) < postLimit; page++ {
		params := url.Values{}
		for key, values := range baseParams {
			params[key] = append([]string(nil), values...)
		}
		params.Set("cursor", cursor)
		data, err := tikhubGETWithTransientRetry(ctx, client, apiKey,
			"/xiaohongshu/app_v2/get_user_posted_notes", params)
		if err != nil {
			return nil, err
		}
		if serviceErr := xiaohongshuServiceError(data); serviceErr != "" {
			return nil, fmt.Errorf("上游返回异常：%s", serviceErr)
		}
		pagePosts := normalizeTikHubXiaohongshuPosts(data)
		for _, post := range pagePosts {
			if seenPosts[post.PlatformPostID] {
				continue
			}
			seenPosts[post.PlatformPostID] = true
			posts = append(posts, post)
			if len(posts) == postLimit {
				break
			}
		}
		nextCursor := xiaohongshuNextCursor(data)
		if nextCursor == "" || nextCursor == cursor || seenCursors[nextCursor] || len(pagePosts) == 0 {
			break
		}
		seenCursors[nextCursor] = true
		cursor = nextCursor
	}
	return posts, nil
}

func xiaohongshuNextCursor(data map[string]any) string {
	cursor := ""
	for _, candidate := range collectNestedMaps(data) {
		if firstNonEmpty(
			anyString(candidate["note_id"]), anyString(candidate["noteId"]), anyString(candidate["item_id"]),
		) != "" {
			if value := firstNonEmpty(anyString(candidate["cursor"]), anyString(candidate["next_cursor"])); value != "" {
				cursor = value
			}
		}
	}
	if cursor != "" {
		return cursor
	}
	for _, candidate := range collectNestedMaps(data) {
		if value := firstNonEmpty(anyString(candidate["next_cursor"]), anyString(candidate["nextCursor"])); value != "" {
			return value
		}
	}
	return ""
}

func xiaohongshuServiceError(data map[string]any) string {
	for _, candidate := range collectNestedMaps(data) {
		for _, key := range []string{"error", "error_msg", "error_message", "err_msg", "message", "msg"} {
			message := strings.TrimSpace(anyString(candidate[key]))
			lower := strings.ToLower(message)
			if strings.Contains(message, "服务异常") || strings.Contains(message, "请求异常") ||
				strings.Contains(lower, "service error") || strings.Contains(lower, "invalid user") ||
				strings.Contains(lower, "invalid note") {
				return message
			}
		}
	}
	return ""
}

func normalizeTikHubXiaohongshuUser(data map[string]any, fallbackUserID, fallbackURL string) tikHubExtendedSocialUser {
	candidate := bestSocialProfileCandidate(data, []string{
		"user_id", "userid", "nickname", "red_id", "fans", "fans_count", "note_count", "avatar",
	})
	userID := firstNonEmpty(
		anyString(candidate["user_id"]), anyString(candidate["userId"]), anyString(candidate["userid"]),
		anyString(candidate["uid"]), fallbackUserID,
	)
	handle := firstNonEmpty(
		anyString(candidate["red_id"]), anyString(candidate["redId"]),
		anyString(candidate["xhs_id"]), anyString(candidate["xhsId"]),
	)
	profileURL := firstNonEmpty(
		anyString(candidate["profile_url"]), anyString(candidate["profileUrl"]),
		anyString(candidate["share_link"]), fallbackURL,
	)
	if userID != "" && (profileURL == "" || strings.Contains(profileURL, "xhslink.")) {
		profileURL = "https://www.xiaohongshu.com/user/profile/" + userID
	}
	return tikHubExtendedSocialUser{
		ID:          userID,
		Handle:      handle,
		DisplayName: firstNonEmpty(anyString(candidate["nickname"]), anyString(candidate["nick_name"]), anyString(candidate["name"]), handle, userID),
		AvatarURL: firstNonEmpty(
			xiaohongshuImageURL(candidate["avatar"]), xiaohongshuImageURL(candidate["avatar_url"]),
			xiaohongshuImageURL(candidate["image"]), xiaohongshuImageURL(candidate["images"]), xiaohongshuImageURL(candidate["imageb"]),
			firstRemoteImageInValue(candidate),
		),
		FollowerCount: firstNonZeroInt64(
			candidate["fans"], candidate["fans_count"], candidate["fansCount"],
			candidate["follower_count"], candidate["followers_count"], candidate["followers"],
			xiaohongshuInteractionCount(candidate, "fans", "followers"),
		),
		FollowingCount: firstNonZeroInt64(
			candidate["follows"], candidate["follows_count"], candidate["following_count"], candidate["following"],
			xiaohongshuInteractionCount(candidate, "follows", "following"),
		),
		PostCount: firstNonZeroInt64(
			candidate["note_count"], candidate["notes_count"], candidate["post_count"], candidate["posts_count"],
			xiaohongshuInteractionCount(candidate, "notes", "note"),
		),
		ProfileURL: profileURL,
	}
}

func xiaohongshuInteractionCount(row map[string]any, labels ...string) int64 {
	values, _ := row["interactions"].([]any)
	for _, value := range values {
		item, _ := value.(map[string]any)
		label := strings.ToLower(firstNonEmpty(anyString(item["type"]), anyString(item["name"]), anyString(item["label"])))
		for _, expected := range labels {
			if strings.Contains(label, strings.ToLower(expected)) {
				return firstNonZeroInt64(item["count"], item["value"], item["num"])
			}
		}
	}
	return 0
}

func normalizeTikHubXiaohongshuPosts(data map[string]any) []platformPost {
	seen := map[string]bool{}
	posts := make([]platformPost, 0)
	for _, rawCandidate := range collectNestedMaps(data) {
		candidate := xiaohongshuNoteCandidate(rawCandidate)
		id := firstNonEmpty(
			anyString(candidate["note_id"]), anyString(candidate["noteId"]),
			anyString(candidate["id"]), anyString(candidate["item_id"]),
		)
		title := firstNonEmpty(
			socialScalarString(candidate["title"]), socialScalarString(candidate["note_title"]),
			socialScalarString(candidate["display_title"]), socialScalarString(candidate["displayTitle"]),
		)
		description := firstNonEmpty(
			socialScalarString(candidate["desc"]), socialScalarString(candidate["description"]),
			socialScalarString(candidate["content"]), title,
		)
		if id == "" || (title == "" && description == "") || seen[id] {
			continue
		}
		seen[id] = true
		postURL := firstNonEmpty(
			anyString(candidate["share_link"]), anyString(candidate["shareLink"]),
			anyString(candidate["note_url"]), anyString(candidate["noteUrl"]), anyString(candidate["url"]),
			anyString(mapAt(candidate, "share_info")["link"]), anyString(mapAt(candidate, "shareInfo")["link"]),
		)
		if postURL == "" {
			postURL = "https://www.xiaohongshu.com/explore/" + id
		}
		posts = append(posts, platformPost{
			PlatformPostID: id,
			Title:          truncateText(firstNonEmpty(title, description), 120),
			Description:    description,
			PostURL:        postURL,
			CoverURL: firstNonEmpty(
				xiaohongshuImageURL(candidate["cover"]), xiaohongshuImageURL(candidate["cover_url"]),
				xiaohongshuImageURL(candidate["image_list"]), xiaohongshuImageURL(candidate["imageList"]),
				xiaohongshuImageURL(candidate["images"]), xiaohongshuImageURL(candidate["image"]),
				firstRemoteImageInValue(candidate),
			),
			MediaType:   xiaohongshuMediaType(candidate),
			PublishedAt: socialPlatformTime(candidate, "time", "timestamp", "create_time", "createTime", "publish_time", "publishTime"),
			ViewCount: firstNonZeroInt64(
				candidate["view_count"], candidate["views_count"], candidate["views"], candidate["read_count"],
			),
			LikeCount: firstNonZeroInt64(
				candidate["liked_count"], candidate["like_count"], candidate["likes_count"], candidate["likes"],
				nestedInt64(candidate, "interact_info", "liked_count"), nestedInt64(candidate, "interactInfo", "likedCount"),
			),
			CommentCount: firstNonZeroInt64(
				candidate["comments_count"], candidate["comment_count"], candidate["comments"],
				nestedInt64(candidate, "interact_info", "comment_count"), nestedInt64(candidate, "interactInfo", "commentCount"),
			),
			ShareCount: firstNonZeroInt64(
				candidate["shared_count"], candidate["share_count"], candidate["shares_count"],
				nestedInt64(candidate, "interact_info", "share_count"), nestedInt64(candidate, "interactInfo", "shareCount"),
			),
			SaveCount: firstNonZeroInt64(
				candidate["collected_count"], candidate["collect_count"], candidate["favorite_count"], candidate["save_count"],
				nestedInt64(candidate, "interact_info", "collected_count"), nestedInt64(candidate, "interactInfo", "collectedCount"),
			),
			Raw: rawCandidate,
		})
	}
	return posts
}

func xiaohongshuNoteCandidate(candidate map[string]any) map[string]any {
	noteCard := firstMapAt(candidate, "note_card", "noteCard", "note_info", "noteInfo")
	if len(noteCard) == 0 {
		return candidate
	}
	merged := make(map[string]any, len(candidate)+len(noteCard))
	for key, value := range candidate {
		merged[key] = value
	}
	for key, value := range noteCard {
		merged[key] = value
	}
	return merged
}

func xiaohongshuImageURL(value any) string {
	if image := imageURL(value); image != "" {
		return image
	}
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if image := xiaohongshuImageURL(item); image != "" {
				return image
			}
		}
	case map[string]any:
		for _, key := range []string{
			"url_default", "urlDefault", "url_pre", "urlPre", "image_url", "imageUrl",
			"url_size_large", "url_size_middle", "url_size_small", "master_url",
			"info_list", "infoList", "image_list", "imageList",
		} {
			if image := xiaohongshuImageURL(typed[key]); image != "" {
				return image
			}
		}
	}
	return ""
}

func xiaohongshuMediaType(row map[string]any) string {
	value := strings.ToLower(firstNonEmpty(
		anyString(row["type"]), anyString(row["note_type"]), anyString(row["media_type"]),
	))
	if strings.Contains(value, "video") || len(mapAt(row, "video")) > 0 || len(mapAt(row, "video_info")) > 0 {
		return "VIDEO"
	}
	return "IMAGE"
}

func (a *app) fetchXiaohongshuPostByURL(ctx context.Context, resourceID int, postID, postURL string) (platformPost, error) {
	apiKey := strings.TrimSpace(tikHubAPIKey(a.effectivePlatformAPIConfig(ctx)))
	if apiKey == "" {
		return platformPost{}, fmt.Errorf("未配置 TikHub API Key")
	}
	params := url.Values{}
	if strings.TrimSpace(postID) != "" {
		params.Set("note_id", strings.TrimSpace(postID))
	} else {
		params.Set("share_text", strings.TrimSpace(postURL))
	}
	client := &http.Client{Timeout: 45 * time.Second}
	post, err := fetchXiaohongshuPostDetail(ctx, client, apiKey, params, postID, postURL)
	if err != nil {
		return platformPost{}, err
	}
	if post.PostURL == "" || strings.TrimSpace(postID) == "" {
		post.PostURL = postURL
	}
	if err := a.upsertSingleContentPlatformPost(ctx, resourceID, "小红书", post); err != nil {
		return platformPost{}, err
	}
	return post, nil
}

func fetchXiaohongshuPostDetail(
	ctx context.Context,
	client *http.Client,
	apiKey string,
	primaryParams url.Values,
	postID string,
	postURL string,
) (platformPost, error) {
	attempts := []url.Values{primaryParams}
	if strings.TrimSpace(postID) != "" && strings.TrimSpace(postURL) != "" {
		attempts = append(attempts, url.Values{"share_text": []string{strings.TrimSpace(postURL)}})
	}
	var lastErr error
	for _, params := range attempts {
		data, err := tikhubGETWithTransientRetry(ctx, client, apiKey,
			"/xiaohongshu/app_v2/get_image_note_detail", params)
		if err != nil {
			lastErr = err
			continue
		}
		if serviceErr := xiaohongshuServiceError(data); serviceErr != "" {
			lastErr = fmt.Errorf("小红书笔记获取失败：%s", serviceErr)
			continue
		}
		posts := normalizeTikHubXiaohongshuPosts(data)
		if len(posts) == 0 {
			lastErr = fmt.Errorf("TikHub 未返回小红书笔记数据")
			continue
		}
		post := posts[0]
		if post.MediaType == "VIDEO" && normalizedRemoteImageURL(post.CoverURL) == "" {
			videoData, videoErr := tikhubGETWithTransientRetry(ctx, client, apiKey,
				"/xiaohongshu/app_v2/get_video_note_detail", params)
			if videoErr == nil && xiaohongshuServiceError(videoData) == "" {
				if videoPosts := normalizeTikHubXiaohongshuPosts(videoData); len(videoPosts) > 0 {
					post = mergePlatformPosts([]platformPost{post}, videoPosts)[0]
				}
			}
		}
		return post, nil
	}
	if lastErr != nil {
		return platformPost{}, lastErr
	}
	return platformPost{}, fmt.Errorf("TikHub 未返回小红书笔记数据")
}

func tikhubGETWithTransientRetry(
	ctx context.Context,
	client *http.Client,
	apiKey string,
	endpoint string,
	params url.Values,
) (map[string]any, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		data, err := tikhubGET(ctx, client, apiKey, endpoint, params)
		if err == nil {
			return data, nil
		}
		lastErr = err
		message := strings.ToLower(err.Error())
		if !strings.Contains(message, "请求失败，请重试") &&
			!strings.Contains(message, "please retry") &&
			!strings.Contains(message, "try again") {
			return nil, err
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 250 * time.Millisecond):
			}
		}
	}
	return nil, lastErr
}
