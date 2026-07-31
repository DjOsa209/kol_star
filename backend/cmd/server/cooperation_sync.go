package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type cooperationPostLink struct {
	Platform string
	PostID   string
	URL      string
}

type cooperationPostSyncResult struct {
	Synced         bool   `json:"synced"`
	Source         string `json:"source,omitempty"`
	Platform       string `json:"platform,omitempty"`
	PostID         string `json:"postId,omitempty"`
	Message        string `json:"message,omitempty"`
	PreviewWarning string `json:"previewWarning,omitempty"`
}

func (a *app) syncCooperationPost(ctx context.Context, cooperationID int, allowAPI bool) (cooperationPostSyncResult, error) {
	var resourceID int
	var finalLink string
	var deliverableLinks string
	if err := a.DB().QueryRowContext(ctx,
		`select resource_id, coalesce(final_link, ''), coalesce(deliverable_links, '')
		   from biz_cooperations where id = ? limit 1`,
		cooperationID,
	).Scan(&resourceID, &finalLink, &deliverableLinks); err != nil {
		return cooperationPostSyncResult{}, err
	}
	postSource := cooperationPostSource(finalLink, deliverableLinks)
	if postSource == "" {
		return cooperationPostSyncResult{}, nil
	}
	link, err := parseCooperationPostLink(postSource)
	if err != nil {
		return cooperationPostSyncResult{Message: err.Error()}, nil
	}

	post, found, err := a.findStoredPlatformPost(ctx, resourceID, link)
	if err != nil {
		return cooperationPostSyncResult{}, err
	}
	source := "作品库"
	if allowAPI && cooperationPlatformSupportsSinglePostFetch(link.Platform) {
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
	if allowAPI {
		if err := a.applyPlatformPostToCooperation(ctx, cooperationID, resourceID, link, post); err != nil {
			return cooperationPostSyncResult{}, err
		}
	} else if err := a.applyPlatformPostMetricsToCooperation(ctx, cooperationID, post); err != nil {
		return cooperationPostSyncResult{}, err
	}
	previewWarning := ""
	if projectContentPlatformUsesPageScreenshot(link.Platform) {
		localCoverURL, warning := captureWebsiteScreenshot(ctx, resourceID, cooperationID, link.URL)
		previewWarning = warning
		if localCoverURL != "" {
			if err := storeCooperationPageScreenshot(
				ctx,
				a.DB(),
				cooperationID,
				resourceID,
				link.Platform,
				link.PostID,
				localCoverURL,
			); err != nil {
				return cooperationPostSyncResult{}, err
			}
		}
	}
	return cooperationPostSyncResult{
		Synced:         true,
		Source:         source,
		Platform:       link.Platform,
		PostID:         post.PlatformPostID,
		Message:        fmt.Sprintf("已通过%s同步合作作品数据", source),
		PreviewWarning: previewWarning,
	}, nil
}

func cooperationPlatformSupportsSinglePostFetch(platform string) bool {
	switch platform {
	case "YouTube", "TikTok", "Instagram":
		return true
	default:
		return false
	}
}

func storeCooperationPageScreenshot(
	ctx context.Context,
	db *sql.DB,
	cooperationID int,
	resourceID int,
	platform string,
	platformPostID string,
	localCoverURL string,
) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx,
		`update biz_cooperations
		    set content_platform = ?, content_cover_url = ?, content_cover_remote_url = ''
		  where id = ? and resource_id = ?`,
		platform, localCoverURL, cooperationID, resourceID,
	); err != nil {
		return err
	}
	if strings.TrimSpace(platformPostID) != "" {
		if _, err = tx.ExecContext(ctx,
			`update biz_resource_platform_posts
			    set cover_url = ?, cover_remote_url = ''
			  where resource_id = ? and platform = ? and platform_post_id = ?`,
			localCoverURL, resourceID, platform, platformPostID,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func cooperationPostSource(finalLink, deliverableLinks string) string {
	for _, raw := range []string{finalLink, deliverableLinks} {
		for _, candidate := range contentURLPattern.FindAllString(raw, -1) {
			if _, err := parseCooperationPostLink(candidate); err == nil {
				return candidate
			}
		}
	}
	return firstNonEmpty(strings.TrimSpace(finalLink), strings.TrimSpace(deliverableLinks))
}

func (a *app) syncCooperationsFromStoredPostsForResource(ctx context.Context, resourceID int) error {
	return a.syncCooperationsForResource(ctx, resourceID, false)
}

func (a *app) syncCooperationsForResource(ctx context.Context, resourceID int, allowAPI bool) error {
	rows, err := a.DB().QueryContext(ctx,
		`select id from biz_cooperations
		  where resource_id = ?
		    and coalesce(nullif(final_link, ''), nullif(deliverable_links, ''), '') <> ''`,
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
		result, err := a.syncCooperationPost(ctx, id, allowAPI)
		if err != nil {
			return err
		}
		if allowAPI {
			if err := cooperationPostSyncFailure(result); err != nil {
				return err
			}
		}
	}
	return nil
}

func cooperationPostSyncFailure(result cooperationPostSyncResult) error {
	if result.Synced || strings.TrimSpace(result.Message) == "" {
		return nil
	}
	return fmt.Errorf("%s", result.Message)
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

type importedWebsiteScreenshotCapture func(
	context.Context,
	int,
	int,
	string,
) (string, string)

func (a *app) captureImportedPageScreenshots(ctx context.Context, batchID string) (int, []string) {
	return a.captureImportedPageScreenshotsWith(ctx, batchID, captureWebsiteScreenshot)
}

func (a *app) captureImportedPageScreenshotsWith(
	ctx context.Context,
	batchID string,
	capture importedWebsiteScreenshotCapture,
) (int, []string) {
	rows, err := a.DB().QueryContext(ctx,
		`select c.id, c.resource_id,
		        coalesce(nullif(c.final_link, ''), nullif(c.deliverable_links, ''), '') as post_url,
		        coalesce(nullif(c.content_platform, ''), nullif(r.platform, ''), 'Website') as platform
		   from biz_cooperations c
		   left join biz_resources r on r.id = c.resource_id
		  where c.import_batch_id = ?
		    and coalesce(nullif(c.final_link, ''), nullif(c.deliverable_links, ''), '') <> ''
		    and coalesce(c.content_cover_url, '') = ''
		  order by c.id`,
		batchID,
	)
	if err != nil {
		return 0, []string{err.Error()}
	}
	defer rows.Close()

	type candidate struct {
		cooperationID int
		resourceID    int
		postURL       string
		platform      string
	}
	var candidates []candidate
	for rows.Next() {
		var item candidate
		if err := rows.Scan(
			&item.cooperationID,
			&item.resourceID,
			&item.postURL,
			&item.platform,
		); err != nil {
			return 0, []string{err.Error()}
		}
		item.platform = normalizeEditableContentPlatform(item.platform)
		if projectContentPlatformUsesPageScreenshot(item.platform) {
			candidates = append(candidates, item)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, []string{err.Error()}
	}

	captured := 0
	var warnings []string
	for _, item := range candidates {
		if !validEditableContentURL(item.postURL) {
			warnings = append(warnings, fmt.Sprintf("合作记录 %d：网页地址无效", item.cooperationID))
			continue
		}
		localCoverURL, warning := capture(
			ctx,
			item.resourceID,
			item.cooperationID,
			item.postURL,
		)
		if localCoverURL == "" {
			warnings = append(warnings, fmt.Sprintf(
				"合作记录 %d：%s",
				item.cooperationID,
				firstNonEmpty(warning, "网页缩略图抓取失败"),
			))
			continue
		}
		platformPostID := ""
		if link, err := parseCooperationPostLink(item.postURL); err == nil &&
			link.Platform == item.platform {
			platformPostID = link.PostID
		}
		if err := storeCooperationPageScreenshot(
			ctx,
			a.DB(),
			item.cooperationID,
			item.resourceID,
			item.platform,
			platformPostID,
			localCoverURL,
		); err != nil {
			warnings = append(warnings, fmt.Sprintf("合作记录 %d：%v", item.cooperationID, err))
			continue
		}
		captured++
	}
	return captured, warnings
}

func (a *app) syncImportedResources(ctx context.Context, resourceIDs []int) (int, []string) {
	synced := 0
	warnings := make([]string, 0)
	for _, resourceID := range resourceIDs {
		var resource syncResourceRow
		if err := a.DB().QueryRowContext(ctx,
			`select id, name, platform, platform_url, platform_user_id, platform_handle
			   from biz_resources where id = ? limit 1`,
			resourceID,
		).Scan(&resource.ID, &resource.Name, &resource.Platform, &resource.PlatformURL, &resource.PlatformUserID, &resource.PlatformHandle); err != nil {
			warnings = append(warnings, fmt.Sprintf("资源 %d：%v", resourceID, err))
			continue
		}
		if platformDisplayName(resource.Platform) == "" {
			message := fmt.Sprintf("%s 暂不支持账号数据自动抓取", resource.Platform)
			if strings.EqualFold(strings.TrimSpace(resource.Platform), "Website") {
				message = "Website 的 UMV 自动抓取需要配置 Similarweb API Key"
			}
			_, _ = a.DB().ExecContext(ctx,
				`update biz_resources set last_sync_status = '待配置', last_sync_error = ?, last_sync_at = now() where id = ?`,
				message, resourceID,
			)
			warnings = append(warnings, fmt.Sprintf("资源 %d：%s", resourceID, message))
			continue
		}
		if err := a.syncResourceByPlatform(ctx, resource); err != nil {
			a.markResourceSyncFailed(ctx, resourceID, err.Error())
			warnings = append(warnings, fmt.Sprintf("资源 %d：%v", resourceID, err))
			continue
		}
		synced++
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
		case (host == "x.com" || host == "twitter.com" || host == "mobile.twitter.com") && len(segments) >= 3:
			if strings.EqualFold(segments[1], "status") {
				return cooperationPostLink{Platform: "X", PostID: segments[2], URL: candidate}, nil
			}
		case strings.HasSuffix(host, "linkedin.com"):
			if strings.Contains(strings.ToLower(parsed.Path), "/feed/update/") || strings.Contains(strings.ToLower(parsed.Path), "/posts/") {
				return cooperationPostLink{Platform: "LinkedIn", URL: candidate}, nil
			}
		case strings.HasSuffix(host, "reddit.com"):
			if slices.Contains(segments, "comments") {
				return cooperationPostLink{Platform: "Reddit", URL: candidate}, nil
			}
		case strings.HasSuffix(host, "facebook.com"):
			return cooperationPostLink{Platform: "Facebook", URL: candidate}, nil
		}
	}
	return cooperationPostLink{}, fmt.Errorf("发布链接不是可识别的受支持平台内容链接")
}

func (a *app) findStoredPlatformPost(ctx context.Context, resourceID int, link cooperationPostLink) (platformPost, bool, error) {
	rows, err := a.DB().QueryContext(ctx,
		`select platform_post_id, title, description, post_url,
		        coalesce(nullif(cover_remote_url, ''), cover_url) as cover_url, media_type,
		        published_at, duration_seconds, view_count, like_count, comment_count, share_count,
		        save_count
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
		&post.CommentCount, &post.ShareCount, &post.SaveCount,
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
	if normalizedProjectContentURL(post.PostURL) == normalizedProjectContentURL(link.URL) {
		return true
	}
	postLink, err := parseCooperationPostLink(post.PostURL)
	return err == nil &&
		link.PostID != "" &&
		postLink.Platform == link.Platform &&
		postLink.PostID == link.PostID
}

type cooperationPostResourceIdentity struct {
	PlatformURL    string
	PlatformUserID string
	PlatformHandle string
	AvatarURL      string
}

func (a *app) applyPlatformPostToCooperation(
	ctx context.Context,
	cooperationID int,
	resourceID int,
	link cooperationPostLink,
	post platformPost,
) error {
	if err := a.applyPlatformPostMetricsToCooperation(ctx, cooperationID, post); err != nil {
		return err
	}

	remoteCoverURL := firstNonEmpty(
		normalizedRemoteImageURL(post.CoverRemoteURL),
		normalizedRemoteImageURL(post.CoverURL),
	)
	localCoverURL := ""
	if isLocalResourceImageURL(post.CoverLocalURL) {
		localCoverURL = post.CoverLocalURL
	} else if isLocalResourceImageURL(post.CoverURL) {
		localCoverURL = post.CoverURL
	}
	if localCoverURL == "" && link.PostID != "" {
		localCoverURL = existingLocalResourceImageURL(
			resourceID,
			filepath.Join("posts", link.Platform+"_"+link.PostID),
		)
	}
	if _, err := a.DB().ExecContext(ctx,
		`update biz_cooperations set content_platform = ?,
		  content_cover_url = ?, content_cover_remote_url = ?
		 where id = ?`,
		link.Platform, firstNonEmpty(localCoverURL, remoteCoverURL), remoteCoverURL, cooperationID,
	); err != nil {
		return err
	}

	identity := cooperationResourceIdentityFromPost(link, post)
	remoteAvatarURL := normalizedRemoteImageURL(identity.AvatarURL)
	avatarURL := firstNonEmpty(
		existingLocalResourceImageURL(resourceID, "avatar"),
		remoteAvatarURL,
	)
	_, err := a.DB().ExecContext(ctx,
		`update biz_resources set platform = ?,
		  platform_url = if(? <> '', ?, platform_url),
		  platform_user_id = if(? <> '', ?, platform_user_id),
		  platform_handle = if(? <> '', ?, platform_handle),
		  avatar_url = if(? <> '', ?, avatar_url),
		  avatar_remote_url = if(? <> '', ?, avatar_remote_url),
		  last_sync_status = '成功', last_sync_error = '', last_sync_at = now()
		 where id = ?`,
		link.Platform,
		identity.PlatformURL, identity.PlatformURL,
		identity.PlatformUserID, identity.PlatformUserID,
		identity.PlatformHandle, identity.PlatformHandle,
		avatarURL, avatarURL,
		remoteAvatarURL, remoteAvatarURL,
		resourceID,
	)
	return err
}

func (a *app) applyPlatformPostMetricsToCooperation(ctx context.Context, cooperationID int, post platformPost) error {
	var releaseDate any
	if post.PublishedAt != nil {
		releaseDate = post.PublishedAt.Format("2006-01-02")
	}
	_, err := a.DB().ExecContext(ctx,
		`update biz_cooperations set
		  views = ?, engagement_count = ?, comments_count = ?,
		  release_date = coalesce(?, release_date)
		 where id = ?`,
		post.ViewCount, post.LikeCount+post.ShareCount+post.SaveCount, post.CommentCount, releaseDate, cooperationID,
	)
	return err
}

func cooperationResourceIdentityFromPost(
	link cooperationPostLink,
	post platformPost,
) cooperationPostResourceIdentity {
	raw, _ := post.Raw.(map[string]any)
	author := cooperationPostAuthor(raw, link.Platform)
	identity := cooperationPostResourceIdentity{}
	switch link.Platform {
	case "TikTok":
		identity.PlatformHandle = normalizeTikTokUsername(firstNonEmpty(
			anyString(author["uniqueId"]),
			anyString(author["unique_id"]),
			tikTokHandleFromContentURL(link.URL),
		))
		identity.PlatformUserID = firstNonEmpty(
			anyString(author["secUid"]),
			anyString(author["sec_uid"]),
			anyString(author["id"]),
			anyString(author["uid"]),
		)
		identity.AvatarURL = firstNonEmpty(
			imageURL(author["avatarLarger"]),
			imageURL(author["avatarMedium"]),
			imageURL(author["avatarThumb"]),
			imageURL(author["avatar_url"]),
			imageURL(author["avatar_url_list"]),
		)
		if identity.PlatformHandle != "" {
			identity.PlatformURL = "https://www.tiktok.com/@" + identity.PlatformHandle
		}
	case "Instagram":
		identity.PlatformHandle = sanitizeInstagramHandle(firstNonEmpty(
			anyString(author["username"]),
			anyString(author["user_name"]),
		))
		identity.PlatformUserID = firstNonEmpty(
			anyString(author["pk"]),
			anyString(author["id"]),
			anyString(author["user_id"]),
		)
		identity.AvatarURL = firstNonEmpty(
			imageURL(author["profile_pic_url_hd"]),
			imageURL(author["profile_pic_url"]),
			imageURL(author["profile_picture_url"]),
			imageURL(author["hd_profile_pic_url_info"]),
		)
		if identity.PlatformHandle != "" {
			identity.PlatformURL = "https://www.instagram.com/" + identity.PlatformHandle + "/"
		}
	case "YouTube":
		identity.PlatformHandle = strings.TrimSpace(firstNonEmpty(
			anyString(author["customUrl"]),
			anyString(author["custom_url"]),
			anyString(author["handle"]),
		))
		identity.PlatformUserID = firstNonEmpty(
			anyString(author["channelId"]),
			anyString(author["channel_id"]),
			anyString(author["id"]),
		)
		identity.AvatarURL = firstNonEmpty(
			imageURL(author["avatar"]),
			imageURL(author["thumbnails"]),
			imageURL(author["thumbnail"]),
		)
		if identity.PlatformHandle != "" {
			identity.PlatformURL = "https://www.youtube.com/" + strings.TrimPrefix(identity.PlatformHandle, "@")
			if strings.HasPrefix(identity.PlatformHandle, "@") {
				identity.PlatformURL = "https://www.youtube.com/" + identity.PlatformHandle
			}
		} else if identity.PlatformUserID != "" {
			identity.PlatformURL = "https://www.youtube.com/channel/" + identity.PlatformUserID
		}
	}
	return identity
}

func cooperationPostAuthor(raw map[string]any, platform string) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	if author := firstMapAt(raw, "author", "user", "owner"); len(author) > 0 {
		return author
	}
	for _, candidate := range collectNestedMaps(raw) {
		if author := firstMapAt(candidate, "author", "user", "owner"); len(author) > 0 {
			return author
		}
		switch platform {
		case "TikTok":
			if firstNonEmpty(
				anyString(candidate["uniqueId"]),
				anyString(candidate["unique_id"]),
				anyString(candidate["secUid"]),
				anyString(candidate["sec_uid"]),
			) != "" {
				return candidate
			}
		case "Instagram":
			if anyString(candidate["username"]) != "" &&
				firstNonEmpty(
					imageURL(candidate["profile_pic_url"]),
					imageURL(candidate["profile_pic_url_hd"]),
					imageURL(candidate["profile_picture_url"]),
				) != "" {
				return candidate
			}
		case "YouTube":
			if firstNonEmpty(
				anyString(candidate["channelId"]),
				anyString(candidate["channel_id"]),
				anyString(candidate["customUrl"]),
			) != "" {
				return candidate
			}
		}
	}
	return map[string]any{}
}

func tikTokHandleFromContentURL(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) == 0 || !strings.HasPrefix(segments[0], "@") {
		return ""
	}
	return strings.TrimPrefix(segments[0], "@")
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
		return platformPost{}, fmt.Errorf("平台 %s 暂不支持按单条链接实时抓取", link.Platform)
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
	if err := a.upsertSingleContentPlatformPost(ctx, resourceID, "TikTok", post); err != nil {
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
	if err := a.upsertSingleContentPlatformPost(ctx, resourceID, "Instagram", post); err != nil {
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
	if media := mapAt(data, "media"); len(media) > 0 && singlePlatformItemID(media) != "" {
		// Instagram's single-post response can return the media and its author as
		// siblings. Keep the wrapper so normalization can retain the author.
		return data
	}
	if singlePlatformItemID(data) != "" {
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

func singlePlatformItemID(data map[string]any) string {
	return firstNonEmpty(
		anyString(data["id"]), anyString(data["pk"]), anyString(data["aweme_id"]),
		anyString(data["shortcode"]), anyString(data["code"]),
	)
}
