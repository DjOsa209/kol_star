package main

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type tikHubExtendedSocialUser struct {
	ID             string
	Handle         string
	DisplayName    string
	AvatarURL      string
	FollowerCount  int64
	FollowingCount int64
	PostCount      int64
	ProfileURL     string
}

func linkedinProfileIdentifier(values ...string) (handle, profileURL string, company bool) {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "<nil>" {
			continue
		}
		if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
			host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
			if host != "linkedin.com" && host != "m.linkedin.com" {
				continue
			}
			segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
			if len(segments) < 2 || (segments[0] != "in" && segments[0] != "company") {
				continue
			}
			handle = sanitizeSocialHandle(segments[1])
			if handle == "" {
				continue
			}
			company = segments[0] == "company"
			kind := "in"
			if company {
				kind = "company"
			}
			return handle, "https://www.linkedin.com/" + kind + "/" + handle + "/", company
		}
		if !strings.ContainsAny(value, " /") {
			if handle = sanitizeSocialHandle(value); handle != "" {
				return handle, "https://www.linkedin.com/in/" + handle + "/", false
			}
		}
	}
	return "", "", false
}

func redditUsernameIdentifier(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "<nil>" {
			continue
		}
		if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
			host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
			if host != "reddit.com" && host != "old.reddit.com" && host != "new.reddit.com" {
				continue
			}
			segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
			if len(segments) < 2 || (strings.ToLower(segments[0]) != "user" && strings.ToLower(segments[0]) != "u") {
				continue
			}
			if username := sanitizeSocialHandle(segments[1]); username != "" {
				return username
			}
		}
		value = strings.TrimPrefix(strings.TrimPrefix(value, "u/"), "/u/")
		if !strings.ContainsAny(value, " /") {
			if username := sanitizeSocialHandle(value); username != "" {
				return username
			}
		}
	}
	return ""
}

func sanitizeSocialHandle(value string) string {
	value = strings.TrimSpace(strings.Trim(value, "/"))
	value = strings.TrimPrefix(value, "@")
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func (a *app) syncLinkedInResource(ctx context.Context, id int) (map[string]any, error) {
	apiKey := strings.TrimSpace(tikHubAPIKey(a.effectivePlatformAPIConfig(ctx)))
	if apiKey == "" {
		return nil, fmt.Errorf("未配置 TikHub API Key，请在系统管理/抓取控制中配置")
	}
	resource, err := a.extendedSocialResource(ctx, id)
	if err != nil {
		return nil, err
	}
	handle, profileURL, company := linkedinProfileIdentifier(
		resource.PlatformURL,
		resource.PlatformHandle,
		resource.Name,
	)
	if handle == "" {
		return nil, fmt.Errorf("请先填写 LinkedIn 个人主页或公司主页链接")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	profileParams := url.Values{"url": []string{profileURL}}
	profileEndpoint := "/linkedin/web_v2/get_user_profile"
	postsEndpoint := "/linkedin/web_v2/get_user_posts"
	if company {
		profileEndpoint = "/linkedin/web_v2/get_company_profile"
		postsEndpoint = "/linkedin/web_v2/get_company_posts"
	}
	profileData, err := tikhubGET(ctx, client, apiKey, profileEndpoint, profileParams)
	if err != nil {
		return nil, err
	}
	user := normalizeTikHubLinkedInUser(profileData, handle, profileURL)
	if user.ID == "" && user.Handle == "" {
		return nil, fmt.Errorf("TikHub 未返回 LinkedIn 账号数据，请检查主页链接")
	}

	syncPosts, postLimit := a.platformPostSyncOptions(ctx, "LinkedIn", 20)
	posts := []platformPost{}
	warnings := []string{}
	if syncPosts {
		postData, postErr := tikhubGET(ctx, client, apiKey, postsEndpoint, url.Values{"url": []string{profileURL}})
		if postErr != nil {
			warnings = append(warnings, "LinkedIn 作品获取失败："+postErr.Error())
		} else {
			posts = limitPlatformPosts(normalizeTikHubLinkedInPosts(postData), postLimit)
			if err := a.upsertResourcePlatformPosts(ctx, id, "LinkedIn", posts); err != nil {
				return nil, err
			}
		}
	}
	if user.PostCount == 0 {
		user.PostCount = int64(len(posts))
	}
	return a.persistExtendedSocialUser(ctx, id, "LinkedIn", user, posts, warnings)
}

func (a *app) syncRedditResource(ctx context.Context, id int) (map[string]any, error) {
	apiKey := strings.TrimSpace(tikHubAPIKey(a.effectivePlatformAPIConfig(ctx)))
	if apiKey == "" {
		return nil, fmt.Errorf("未配置 TikHub API Key，请在系统管理/抓取控制中配置")
	}
	resource, err := a.extendedSocialResource(ctx, id)
	if err != nil {
		return nil, err
	}
	username := redditUsernameIdentifier(resource.PlatformHandle, resource.PlatformURL, resource.Name)
	if username == "" {
		return nil, fmt.Errorf("请先填写 Reddit 用户主页链接或用户名")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	profileParams := url.Values{}
	profileParams.Set("username", username)
	profileParams.Set("need_format", "true")
	profileData, err := tikhubGET(ctx, client, apiKey, "/reddit/app/fetch_user_profile", profileParams)
	if err != nil {
		return nil, err
	}
	user := normalizeTikHubRedditUser(profileData, username)
	if user.ID == "" && user.Handle == "" {
		return nil, fmt.Errorf("TikHub 未返回 Reddit 账号数据，请检查用户名")
	}

	syncPosts, postLimit := a.platformPostSyncOptions(ctx, "Reddit", 20)
	posts := []platformPost{}
	warnings := []string{}
	if syncPosts {
		postParams := url.Values{}
		postParams.Set("username", username)
		postParams.Set("sort", "NEW")
		postParams.Set("need_format", "true")
		postData, postErr := tikhubGET(ctx, client, apiKey, "/reddit/app/fetch_user_posts", postParams)
		if postErr != nil {
			warnings = append(warnings, "Reddit 帖子获取失败："+postErr.Error())
		} else {
			posts = limitPlatformPosts(normalizeTikHubRedditPosts(postData), postLimit)
			if err := a.upsertResourcePlatformPosts(ctx, id, "Reddit", posts); err != nil {
				return nil, err
			}
		}
	}
	if user.PostCount == 0 {
		user.PostCount = int64(len(posts))
	}
	return a.persistExtendedSocialUser(ctx, id, "Reddit", user, posts, warnings)
}

func (a *app) syncFacebookResource(context.Context, int) (map[string]any, error) {
	return nil, fmt.Errorf("TikHub 当前公开接口未提供 Facebook 账号及帖子数据，暂无法通过 TikHub 同步")
}

func (a *app) extendedSocialResource(ctx context.Context, id int) (syncResourceRow, error) {
	var resource syncResourceRow
	err := a.DB().QueryRowContext(ctx,
		`select id, name, platform, platform_url, platform_user_id, platform_handle
		   from biz_resources where id = ? limit 1`,
		id,
	).Scan(
		&resource.ID,
		&resource.Name,
		&resource.Platform,
		&resource.PlatformURL,
		&resource.PlatformUserID,
		&resource.PlatformHandle,
	)
	return resource, err
}

func normalizeTikHubLinkedInUser(data map[string]any, fallbackHandle, fallbackURL string) tikHubExtendedSocialUser {
	candidate := bestSocialProfileCandidate(data, []string{
		"public_identifier", "publicIdentifier", "full_name", "fullName", "follower_count", "followers_count", "urn",
	})
	handle := sanitizeSocialHandle(firstNonEmpty(
		anyString(candidate["public_identifier"]),
		anyString(candidate["publicIdentifier"]),
		anyString(candidate["username"]),
		anyString(candidate["slug"]),
		fallbackHandle,
	))
	firstName := firstNonEmpty(anyString(candidate["first_name"]), anyString(candidate["firstName"]))
	lastName := firstNonEmpty(anyString(candidate["last_name"]), anyString(candidate["lastName"]))
	name := firstNonEmpty(
		anyString(candidate["full_name"]),
		anyString(candidate["fullName"]),
		anyString(candidate["name"]),
		strings.TrimSpace(firstName+" "+lastName),
		handle,
	)
	return tikHubExtendedSocialUser{
		ID: firstNonEmpty(
			anyString(candidate["id"]),
			anyString(candidate["urn"]),
			anyString(candidate["entity_urn"]),
			anyString(candidate["entityUrn"]),
		),
		Handle:      handle,
		DisplayName: name,
		AvatarURL: firstNonEmpty(
			imageURL(candidate["profile_picture"]),
			imageURL(candidate["profilePicture"]),
			imageURL(candidate["profile_pic_url"]),
			imageURL(candidate["picture"]),
			imageURL(candidate["logo"]),
			imageURL(candidate["avatar"]),
			firstRemoteImageInValue(candidate),
		),
		FollowerCount: firstNonZeroInt64(
			candidate["follower_count"],
			candidate["followers_count"],
			candidate["followers"],
			nestedInt64(candidate, "follower_and_connection", "follower_count"),
			nestedInt64(candidate, "followerAndConnection", "followerCount"),
		),
		FollowingCount: firstNonZeroInt64(
			candidate["connection_count"],
			candidate["connections_count"],
			candidate["following_count"],
			nestedInt64(candidate, "follower_and_connection", "connection_count"),
		),
		PostCount: firstNonZeroInt64(candidate["post_count"], candidate["posts_count"], candidate["updates_count"]),
		ProfileURL: firstNonEmpty(
			anyString(candidate["profile_url"]),
			anyString(candidate["profileUrl"]),
			anyString(candidate["url"]),
			fallbackURL,
		),
	}
}

func normalizeTikHubRedditUser(data map[string]any, fallbackUsername string) tikHubExtendedSocialUser {
	candidate := bestSocialProfileCandidate(data, []string{
		"total_karma", "link_karma", "comment_karma", "icon_img", "snoovatar_img", "subreddit", "followers_count",
	})
	handle := sanitizeSocialHandle(firstNonEmpty(
		anyString(candidate["name"]),
		anyString(candidate["username"]),
		anyString(candidate["display_name"]),
		fallbackUsername,
	))
	return tikHubExtendedSocialUser{
		ID:          firstNonEmpty(anyString(candidate["id"]), anyString(candidate["user_id"]), anyString(candidate["fullname"])),
		Handle:      handle,
		DisplayName: firstNonEmpty(anyString(candidate["display_name_prefixed"]), anyString(candidate["display_name"]), handle),
		AvatarURL: firstNonEmpty(
			cleanRemoteImageURL(anyString(candidate["icon_img"])),
			cleanRemoteImageURL(anyString(candidate["snoovatar_img"])),
			imageURL(candidate["avatar"]),
			imageURL(candidate["profile_img"]),
			imageURL(mapAt(candidate, "subreddit")["icon_img"]),
		),
		FollowerCount: firstNonZeroInt64(
			candidate["followers_count"],
			candidate["follower_count"],
			candidate["subscribers"],
			nestedInt64(candidate, "subreddit", "subscribers"),
		),
		FollowingCount: firstNonZeroInt64(candidate["following_count"], candidate["friends_count"]),
		PostCount:      firstNonZeroInt64(candidate["post_count"], candidate["posts_count"]),
		ProfileURL:     "https://www.reddit.com/user/" + handle + "/",
	}
}

func bestSocialProfileCandidate(data map[string]any, keys []string) map[string]any {
	best := data
	bestScore := -1
	for _, candidate := range collectNestedMaps(data) {
		score := 0
		for _, key := range keys {
			if value, ok := candidate[key]; ok && strings.TrimSpace(anyString(value)) != "" {
				score += 2
			}
		}
		for _, key := range []string{"id", "name", "username", "avatar", "profile_url"} {
			if value, ok := candidate[key]; ok && strings.TrimSpace(anyString(value)) != "" {
				score++
			}
		}
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}
	return best
}

func normalizeTikHubLinkedInPosts(data map[string]any) []platformPost {
	seen := map[string]bool{}
	posts := make([]platformPost, 0)
	for _, candidate := range collectNestedMaps(data) {
		id := firstNonEmpty(
			anyString(candidate["id"]),
			anyString(candidate["post_id"]),
			anyString(candidate["activity_urn"]),
			anyString(candidate["activityUrn"]),
			anyString(candidate["urn"]),
			anyString(candidate["entity_urn"]),
		)
		text := firstNonEmpty(
			socialScalarString(candidate["commentary"]),
			socialScalarString(candidate["text"]),
			socialScalarString(candidate["title"]),
			socialScalarString(candidate["description"]),
			anyString(mapAt(candidate, "commentary")["text"]),
			anyString(mapAt(candidate, "content")["text"]),
		)
		if id == "" || text == "" || seen[id] {
			continue
		}
		seen[id] = true
		postURL := firstNonEmpty(
			anyString(candidate["post_url"]),
			anyString(candidate["postUrl"]),
			anyString(candidate["permalink"]),
			anyString(candidate["url"]),
			anyString(candidate["navigation_url"]),
		)
		if postURL == "" && strings.Contains(strings.ToLower(id), "urn:li:") {
			postURL = "https://www.linkedin.com/feed/update/" + id + "/"
		}
		posts = append(posts, platformPost{
			PlatformPostID: id,
			Title:          truncateText(firstNonEmpty(anyString(candidate["title"]), text), 120),
			Description:    text,
			PostURL:        postURL,
			CoverURL: firstNonEmpty(
				imageURL(candidate["image"]),
				imageURL(candidate["images"]),
				imageURL(candidate["media"]),
				imageURL(candidate["thumbnail"]),
				imageURL(candidate["content"]),
				firstRemoteImageInValue(candidate),
			),
			MediaType:   linkedInMediaType(candidate),
			PublishedAt: socialPlatformTime(candidate, "published_at", "publishedAt", "posted_at", "postedAt", "created_at", "createdAt", "timestamp"),
			ViewCount: firstNonZeroInt64(
				candidate["view_count"], candidate["views_count"], candidate["impression_count"], candidate["impressions_count"],
			),
			LikeCount: firstNonZeroInt64(
				candidate["like_count"], candidate["likes_count"], candidate["reaction_count"], candidate["reactions_count"],
				nestedInt64(candidate, "social_detail", "total_reaction_count"),
			),
			CommentCount: firstNonZeroInt64(
				candidate["comment_count"], candidate["comments_count"], nestedInt64(candidate, "social_detail", "comment_count"),
			),
			ShareCount: firstNonZeroInt64(
				candidate["share_count"], candidate["shares_count"], candidate["repost_count"], candidate["reposts_count"],
			),
			Raw: candidate,
		})
	}
	return posts
}

func normalizeTikHubRedditPosts(data map[string]any) []platformPost {
	seen := map[string]bool{}
	posts := make([]platformPost, 0)
	for _, candidate := range collectNestedMaps(data) {
		id := firstNonEmpty(anyString(candidate["id"]), anyString(candidate["name"]), anyString(candidate["post_id"]))
		title := firstNonEmpty(anyString(candidate["title"]), anyString(candidate["post_title"]))
		if id == "" || title == "" || seen[id] {
			continue
		}
		seen[id] = true
		permalink := firstNonEmpty(anyString(candidate["permalink"]), anyString(candidate["post_url"]))
		if strings.HasPrefix(permalink, "/") {
			permalink = "https://www.reddit.com" + permalink
		}
		description := firstNonEmpty(anyString(candidate["selftext"]), anyString(candidate["body"]), anyString(candidate["description"]))
		posts = append(posts, platformPost{
			PlatformPostID: id,
			Title:          truncateText(title, 120),
			Description:    description,
			PostURL: firstNonEmpty(
				permalink,
				anyString(candidate["url"]),
				anyString(candidate["url_overridden_by_dest"]),
			),
			CoverURL: firstNonEmpty(
				cleanRemoteImageURL(anyString(candidate["thumbnail"])),
				cleanRemoteImageURL(anyString(candidate["url_overridden_by_dest"])),
				imageURL(candidate["preview"]),
				imageURL(candidate["media"]),
			),
			MediaType:   redditMediaType(candidate),
			PublishedAt: socialPlatformTime(candidate, "created_utc", "created", "created_at", "timestamp"),
			ViewCount:   firstNonZeroInt64(candidate["view_count"], candidate["views"]),
			LikeCount: firstNonZeroInt64(
				candidate["ups"], candidate["upvote_count"], candidate["score"], candidate["likes"],
			),
			CommentCount: firstNonZeroInt64(candidate["num_comments"], candidate["comment_count"], candidate["comments_count"]),
			ShareCount:   firstNonZeroInt64(candidate["share_count"], candidate["crosspost_count"]),
			Raw:          candidate,
		})
	}
	return posts
}

func linkedInMediaType(row map[string]any) string {
	value := strings.ToLower(firstNonEmpty(
		anyString(row["media_type"]),
		anyString(row["type"]),
		anyString(row["content_type"]),
	))
	switch {
	case strings.Contains(value, "video"):
		return "VIDEO"
	case strings.Contains(value, "image") || strings.Contains(value, "photo"):
		return "IMAGE"
	case strings.Contains(value, "article") || strings.Contains(value, "link"):
		return "LINK"
	default:
		return "POST"
	}
}

func redditMediaType(row map[string]any) string {
	if fmt.Sprint(row["is_video"]) == "true" || anyInt64(row["is_video"]) == 1 {
		return "VIDEO"
	}
	value := strings.ToLower(firstNonEmpty(anyString(row["post_hint"]), anyString(row["type"]), anyString(row["media_type"])))
	switch {
	case strings.Contains(value, "video"):
		return "VIDEO"
	case strings.Contains(value, "image"):
		return "IMAGE"
	case strings.Contains(value, "link"):
		return "LINK"
	default:
		return "POST"
	}
}

func socialPlatformTime(row map[string]any, keys ...string) *time.Time {
	for _, key := range keys {
		value, ok := row[key]
		if !ok || value == nil {
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			value = firstNonEmpty(anyString(nested["timestamp"]), anyString(nested["time"]), anyString(nested["date"]))
		}
		if raw := strings.TrimSpace(anyString(value)); raw != "" {
			if parsed := parsePlatformTime(raw); parsed != nil {
				return parsed
			}
			if number, err := strconv.ParseInt(raw, 10, 64); err == nil {
				return unixPlatformTime(number)
			}
			for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02"} {
				if parsed, err := time.Parse(layout, raw); err == nil {
					utc := parsed.UTC()
					return &utc
				}
			}
		}
		if number := anyInt64(value); number > 0 {
			return unixPlatformTime(number)
		}
	}
	return nil
}

func cleanRemoteImageURL(value string) string {
	return normalizedRemoteImageURL(html.UnescapeString(strings.TrimSpace(value)))
}

func firstRemoteImageInValue(value any) string {
	switch typed := value.(type) {
	case string:
		return cleanRemoteImageURL(typed)
	case []any:
		for _, item := range typed {
			if image := firstRemoteImageInValue(item); image != "" {
				return image
			}
		}
	case map[string]any:
		for _, key := range []string{
			"profile_picture", "profilePicture",
			"icon_img", "snoovatar_img", "thumbnail", "image", "images", "media",
			"display_image", "displayImageReference", "vectorImage", "artifacts",
		} {
			if image := firstRemoteImageInValue(typed[key]); image != "" {
				return image
			}
		}
	}
	return ""
}

func socialScalarString(value any) string {
	if typed, ok := value.(string); ok {
		return strings.TrimSpace(typed)
	}
	return ""
}

func (a *app) persistExtendedSocialUser(
	ctx context.Context,
	resourceID int,
	platform string,
	user tikHubExtendedSocialUser,
	posts []platformPost,
	warnings []string,
) (map[string]any, error) {
	totalViews := sumPostViews(posts)
	avgViews := averageViewedPostViews(posts)
	engagementRate := platformPostEngagementRate(posts, user.FollowerCount)
	avatarURL := normalizedRemoteImageURL(user.AvatarURL)
	resourceName := syncedResourceName(user.ProfileURL, firstNonEmpty(user.DisplayName, user.Handle))
	_, err := a.DB().ExecContext(ctx,
		`update biz_resources set
		  name = if(? <> '', ?, name),
		  followers = ?, total_views = if(? > 0, ?, total_views),
		  avg_views = if(? > 0, ?, avg_views), video_count = ?,
		  engagement_rate = if(? > 0, ?, engagement_rate),
		  platform = ?, platform_user_id = ?, platform_handle = ?,
		  platform_url = if(? <> '', ?, platform_url),
		  avatar_url = if(? <> '', ?, avatar_url),
		  avatar_remote_url = if(? <> '', ?, avatar_remote_url),
		  audience_size_unit = if(resource_type = '媒体', 'UMV', 'Followers'),
		  last_sync_status = '成功', last_sync_error = '', last_sync_at = now()
		 where id = ?`,
		resourceName, resourceName, user.FollowerCount,
		totalViews, totalViews, avgViews, avgViews, user.PostCount,
		engagementRate, engagementRate,
		platform, user.ID, user.Handle,
		user.ProfileURL, user.ProfileURL,
		avatarURL, avatarURL, user.AvatarURL, user.AvatarURL,
		resourceID,
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"platform":       platform,
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
