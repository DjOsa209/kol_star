package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

type creatorDiscoveryReviewInput struct {
	ID             int64   `json:"id"`
	Action         string  `json:"action"`
	Country        string  `json:"country"`
	Language       string  `json:"language"`
	Industry       string  `json:"industry"`
	Category       string  `json:"category"`
	TagIDs         []int64 `json:"tagIds"`
	Note           string  `json:"note"`
	EvidenceURL    string  `json:"evidenceUrl"`
	TopicConfirmed bool    `json:"topicConfirmed"`
}

type creatorDiscoveryReviewResult struct {
	ID         int64  `json:"id"`
	Status     string `json:"status"`
	ResourceID int64  `json:"resourceId,omitempty"`
	Reused     bool   `json:"reused,omitempty"`
}

type creatorDiscoveryReviewError struct {
	Status  int
	Message string
}

func (e *creatorDiscoveryReviewError) Error() string { return e.Message }

func discoveryReviewError(status int, message string) error {
	return &creatorDiscoveryReviewError{Status: status, Message: message}
}

func (a *app) creatorDiscoveryReview(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.recommendationAccess(w, r, "creator")
	if !ok {
		return
	}
	var input creatorDiscoveryReviewInput
	if !readRecommendationJSON(w, r, &input) {
		return
	}
	input.Action = strings.TrimSpace(input.Action)
	if input.ID <= 0 || (input.Action != "approve" && input.Action != "reject") {
		writeError(w, http.StatusBadRequest, 400, "请选择有效候选和审核操作")
		return
	}
	if input.Action == "approve" {
		// Resource creation is guarded by the resource menu throughout this app.
		allowed, err := a.hasMenu(r.Context(), userID, "/business/resources")
		if err != nil {
			writeDBError(w, err)
			return
		}
		if !allowed {
			writeError(w, http.StatusForbidden, 403, "没有资源库权限，无法确认入库")
			return
		}
	}
	result, err := a.reviewCreatorDiscoveryCandidate(r.Context(), userID, input)
	if err != nil {
		var reviewErr *creatorDiscoveryReviewError
		if errors.As(err, &reviewErr) {
			writeError(w, reviewErr.Status, reviewErr.Status, reviewErr.Message)
		} else {
			writeDBError(w, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "审核已保存", Data: result})
}

func (a *app) reviewCreatorDiscoveryCandidate(ctx context.Context, userID int, input creatorDiscoveryReviewInput) (creatorDiscoveryReviewResult, error) {
	result := creatorDiscoveryReviewResult{ID: input.ID}
	tx, err := a.DB().BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	var status, jobStatus, channelID string
	var snapshotJSON []byte
	var resourceID sql.NullInt64
	err = tx.QueryRowContext(ctx, `select c.status,c.resource_id,c.channel_id,c.snapshot,j.status
	 from biz_creator_discovery_candidates c
	 join biz_creator_discovery_jobs j on j.id=c.job_id
	 where c.id=? and j.user_id=? for update`, input.ID, userID).
		Scan(&status, &resourceID, &channelID, &snapshotJSON, &jobStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return result, discoveryReviewError(http.StatusNotFound, "候选不存在")
	}
	if err != nil {
		return result, err
	}
	if !creatorDiscoveryReviewableJob(jobStatus) {
		return result, discoveryReviewError(http.StatusConflict, "请等待发现任务结束后再核实候选")
	}
	if status == "approved" {
		if input.Action != "approve" {
			return result, discoveryReviewError(http.StatusConflict, "已入库候选不能改为不采用")
		}
		if !resourceID.Valid || resourceID.Int64 <= 0 {
			return result, discoveryReviewError(http.StatusConflict, "候选入库记录不完整，请联系管理员")
		}
		result.Status, result.ResourceID, result.Reused = status, resourceID.Int64, true
		return result, tx.Commit()
	}
	if status == "rejected" && input.Action == "reject" {
		result.Status = status
		return result, tx.Commit()
	}
	if status != "pending" {
		return result, discoveryReviewError(http.StatusConflict, "该候选已处理，请重新发现后核实")
	}
	input.Note = strings.TrimSpace(input.Note)
	if utf8.RuneCountInString(input.Note) > 4000 {
		return result, discoveryReviewError(http.StatusBadRequest, "核实说明最多4000字")
	}
	if input.Action == "reject" {
		_, err = tx.ExecContext(ctx, `update biz_creator_discovery_candidates
		 set status='rejected',note=?,reviewed_by=?,reviewed_at=now() where id=?`, input.Note, userID, input.ID)
		if err != nil {
			return result, err
		}
		if err := invalidateCreatorDiscoveryRecommendation(ctx, tx, input.ID); err != nil {
			return result, err
		}
		result.Status = "rejected"
		return result, tx.Commit()
	}
	var snapshot creatorDiscoverySnapshot
	if err := json.Unmarshal(snapshotJSON, &snapshot); err != nil {
		return result, fmt.Errorf("decode discovery candidate: %w", err)
	}
	if snapshot.ChannelID != channelID || !creatorDiscoveryImportChannelID.MatchString(channelID) {
		return result, discoveryReviewError(http.StatusConflict, "候选频道身份无效，请重新发现")
	}
	if err := validateCreatorDiscoveryReview(&input, snapshot); err != nil {
		return result, err
	}
	if err := validateCreatorDiscoveryTags(ctx, tx, input.TagIDs); err != nil {
		return result, err
	}
	result.ResourceID, result.Reused, err = importCreatorDiscoverySnapshot(ctx, tx, snapshot, input)
	if err != nil {
		return result, err
	}
	_, err = tx.ExecContext(ctx, `update biz_creator_discovery_candidates
	 set status='approved',resource_id=?,country=?,language=?,industry=?,category=?,tag_ids=?,note=?,evidence_url=?,reviewed_by=?,reviewed_at=now()
	 where id=?`, result.ResourceID, input.Country, input.Language, input.Industry, input.Category, creatorDiscoveryTagJSON(input.TagIDs), input.Note, input.EvidenceURL, userID, input.ID)
	if err != nil {
		return result, err
	}
	// Approval links this staged candidate to a resource without replacing the
	// model's assessment. Recommendation readers resolve its new resource ID.
	result.Status = "approved"
	return result, tx.Commit()
}

func invalidateCreatorDiscoveryRecommendation(ctx context.Context, tx *sql.Tx, candidateID int64) error {
	_, err := tx.ExecContext(ctx, `update biz_creator_discovery_jobs j
	 join biz_creator_discovery_candidates c on c.job_id=j.id
	 set j.recommendation=NULL where c.id=?`, candidateID)
	return err
}

func creatorDiscoveryReviewableJob(status string) bool {
	switch status {
	case "completed", "partial", "failed", "cancelled":
		return true
	default:
		return false
	}
}

var creatorDiscoveryImportChannelID = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)

func validateCreatorDiscoveryReview(input *creatorDiscoveryReviewInput, snapshot creatorDiscoverySnapshot) error {
	input.Country = strings.TrimSpace(input.Country)
	input.Language = strings.TrimSpace(input.Language)
	input.Industry = strings.TrimSpace(input.Industry)
	input.Category = strings.TrimSpace(input.Category)
	input.Note = strings.TrimSpace(input.Note)
	input.EvidenceURL = strings.TrimSpace(input.EvidenceURL)
	if input.Country == "" || input.Language == "" || input.Note == "" || !input.TopicConfirmed {
		return discoveryReviewError(http.StatusBadRequest, "请核实市场、内容语言与主题，并填写依据说明")
	}
	if utf8.RuneCountInString(input.Country) > 64 || utf8.RuneCountInString(input.Language) > 64 || utf8.RuneCountInString(input.Note) > 4000 || len(input.EvidenceURL) > 1024 {
		return discoveryReviewError(http.StatusBadRequest, "核实字段超出长度限制")
	}
	if !creatorDiscoveryEvidenceAllowed(snapshot, input.EvidenceURL) {
		return discoveryReviewError(http.StatusBadRequest, "请选择本候选已采集的主页或作品作为核实依据")
	}
	for _, value := range []string{input.Industry, input.Category} {
		if utf8.RuneCountInString(value) > 128 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return discoveryReviewError(http.StatusBadRequest, "行业与分类须为128字以内的单行文本")
		}
	}
	if len(input.TagIDs) > 50 {
		return discoveryReviewError(http.StatusBadRequest, "最多选择50个资源标签")
	}
	seen := make(map[int64]bool, len(input.TagIDs))
	tagIDs := make([]int64, 0, len(input.TagIDs))
	for _, id := range input.TagIDs {
		if id <= 0 {
			return discoveryReviewError(http.StatusBadRequest, "请选择有效资源标签")
		}
		if !seen[id] {
			tagIDs = append(tagIDs, id)
			seen[id] = true
		}
	}
	sort.Slice(tagIDs, func(i, j int) bool { return tagIDs[i] < tagIDs[j] })
	input.TagIDs = tagIDs
	return nil
}

func creatorDiscoveryTagJSON(tagIDs []int64) string {
	if len(tagIDs) == 0 {
		return "[]"
	}
	raw, _ := json.Marshal(tagIDs)
	return string(raw)
}

func validateCreatorDiscoveryTags(ctx context.Context, tx *sql.Tx, tagIDs []int64) error {
	if len(tagIDs) == 0 {
		return nil
	}
	args := make([]any, len(tagIDs))
	for i, id := range tagIDs {
		args[i] = id
	}
	// biz_tags is the shared resource taxonomy, separate from IP categories.
	// Lock selected enabled tags until the resource associations are committed.
	rows, err := tx.QueryContext(ctx, `select id from biz_tags where status='启用' and id in (`+strings.TrimRight(strings.Repeat("?,", len(tagIDs)), ",")+`) order by id for update`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count != len(tagIDs) {
		return discoveryReviewError(http.StatusBadRequest, "所选资源标签已停用或不存在，请刷新后重选")
	}
	return nil
}

func creatorDiscoveryEvidenceAllowed(snapshot creatorDiscoverySnapshot, evidenceURL string) bool {
	key := creatorDiscoveryEvidenceURLKey(evidenceURL)
	if key == "" {
		return false
	}
	urls := []string{snapshot.ProfileURL}
	for _, item := range snapshot.Evidence {
		urls = append(urls, item.URL)
	}
	for _, item := range snapshot.RecentPosts {
		urls = append(urls, item.PostURL)
	}
	for _, candidate := range urls {
		if key == creatorDiscoveryEvidenceURLKey(candidate) {
			return true
		}
	}
	return false
}

func creatorDiscoveryEvidenceURLKey(value string) string {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return ""
	}
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	return u.String()
}

func importCreatorDiscoverySnapshot(ctx context.Context, tx *sql.Tx, snapshot creatorDiscoverySnapshot, review creatorDiscoveryReviewInput) (int64, bool, error) {
	// The unique identity row serializes approvals from different jobs/users.
	// Inserting it within this transaction also makes failure fully reversible.
	_, err := tx.ExecContext(ctx, `insert into biz_creator_discovery_identities (platform,channel_id)
	 values ('YouTube',?) on duplicate key update channel_id=values(channel_id)`, snapshot.ChannelID)
	if err != nil {
		return 0, false, err
	}
	var mappedID sql.NullInt64
	if err := tx.QueryRowContext(ctx, `select resource_id from biz_creator_discovery_identities
	 where platform='YouTube' and channel_id=? for update`, snapshot.ChannelID).Scan(&mappedID); err != nil {
		return 0, false, err
	}
	canonicalURL := "https://www.youtube.com/channel/" + snapshot.ChannelID
	var resourceID int64
	if mappedID.Valid {
		err = tx.QueryRowContext(ctx, `select id from biz_resources where id=? and platform='YouTube'
		 and (platform_user_id=? or (platform_user_id='' and platform_url in (?,?,?,?))) for update`, mappedID.Int64, snapshot.ChannelID,
			canonicalURL, canonicalURL+"/", strings.Replace(canonicalURL, "www.", "", 1), strings.Replace(canonicalURL, "www.", "", 1)+"/").Scan(&resourceID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return 0, false, err
		}
	}
	if resourceID == 0 {
		err = tx.QueryRowContext(ctx, `select id from biz_resources
		 where platform='YouTube' and (platform_user_id=? or (platform_user_id='' and platform_url in (?,?,?,?)))
		 order by id limit 1 for update`, snapshot.ChannelID, canonicalURL, canonicalURL+"/", strings.Replace(canonicalURL, "www.", "", 1), strings.Replace(canonicalURL, "www.", "", 1)+"/").Scan(&resourceID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return 0, false, err
		}
	}
	reused := resourceID > 0
	avatarURL := normalizedRemoteImageURL(snapshot.AvatarURL)
	if !reused {
		followers := int64(0)
		if snapshot.Followers != nil && *snapshot.Followers > 0 {
			followers = *snapshot.Followers
		}
		note := "发现来源：YouTube；人工核实依据：" + review.EvidenceURL + "\n" + review.Note + "\n报价与档期待确认。"
		if snapshot.Followers == nil {
			note += "\n未获取公开订阅数。"
		}
		syncStatus, syncWarning := "成功", strings.TrimSpace(snapshot.Warning)
		if syncWarning != "" {
			syncStatus = "部分成功"
		}
		created, err := tx.ExecContext(ctx, `insert into biz_resources
		 (name,resource_type,country,language,industry,category,platform,status,followers,audience_size,audience_size_unit,
		 platform_user_id,platform_handle,platform_url,avatar_url,avatar_remote_url,total_views,video_count,
		 reference_source,score,level,risk_level,notes,last_sync_status,last_sync_error,last_sync_at)
		 values (?,'达人',?,?,?,?,'YouTube','观察中',?,?,'Followers',?,?,?,?,?,?,?,'YouTube Data API',0,?,'待核实',?,?,?,?)`,
			creatorDiscoveryImportText(firstNonEmpty(snapshot.Name, snapshot.ChannelID), 128), review.Country, review.Language, review.Industry, review.Category,
			followers, followers, snapshot.ChannelID, creatorDiscoveryImportText(snapshot.Handle, 128), canonicalURL,
			avatarURL, avatarURL, snapshot.TotalViews, snapshot.VideoCount, calcLevel(0), note, syncStatus, syncWarning,
			parsePlatformTime(snapshot.CollectedAt))
		if err != nil {
			return 0, false, err
		}
		resourceID, err = created.LastInsertId()
		if err != nil {
			return 0, false, err
		}
	} else {
		_, err = tx.ExecContext(ctx, `update biz_resources set
		 avatar_url=if(trim(avatar_url)='',coalesce(nullif(avatar_remote_url,''),?),avatar_url),
		 avatar_remote_url=if(trim(avatar_remote_url)='',?,avatar_remote_url),
		 country=if(trim(country)='',?,country),language=if(trim(language)='',?,language),
		 industry=if(trim(industry)='',?,industry),category=if(trim(category)='',?,category)
		 where id=?`, avatarURL, avatarURL, review.Country, review.Language, review.Industry, review.Category, resourceID)
		if err != nil {
			return 0, false, err
		}
	}
	for _, tagID := range review.TagIDs {
		if _, err := tx.ExecContext(ctx, `insert into biz_resource_tags(resource_id,tag_id) values(?,?) on duplicate key update tag_id=values(tag_id)`, resourceID, tagID); err != nil {
			return 0, false, err
		}
	}
	if err := refreshCreatorDiscoveryAudience(ctx, tx, resourceID); err != nil {
		return 0, false, err
	}
	if _, err := tx.ExecContext(ctx, `update biz_creator_discovery_identities set resource_id=? where platform='YouTube' and channel_id=?`, resourceID, snapshot.ChannelID); err != nil {
		return 0, false, err
	}
	if err := insertCreatorDiscoveryPosts(ctx, tx, resourceID, snapshot.RecentPosts); err != nil {
		return 0, false, err
	}
	return resourceID, reused, nil
}

func refreshCreatorDiscoveryAudience(ctx context.Context, tx *sql.Tx, resourceID int64) error {
	// Match the resource library's same-name/type audience grouping, but only
	// update this imported resource and preserve any existing manual tier.
	_, err := tx.ExecContext(ctx, `update biz_resources r join (
	 select case when lower(trim(peers.resource_type)) in ('kol','达人') then 'creator' else peers.resource_type end as resource_kind,lower(trim(peers.name)) as collaborator_key,
	 case when min(peers.resource_type)='媒体' then max(greatest(case when lower(trim(peers.platform)) in ('website','web') then peers.monthly_visits else peers.audience_size end,0))
	 else sum(greatest(peers.followers,0)) end as total_audience
	 from biz_resources peers join biz_resources imported on imported.id=?
	 and (peers.resource_type=imported.resource_type or (lower(trim(peers.resource_type)) in ('kol','达人') and lower(trim(imported.resource_type)) in ('kol','达人')))
	 and ((trim(imported.name)<>'' and lower(trim(peers.name))=lower(trim(imported.name))) or peers.id=imported.id)
	 group by resource_kind,lower(trim(peers.name))
	 ) totals on totals.resource_kind=case when lower(trim(r.resource_type)) in ('kol','达人') then 'creator' else r.resource_type end and totals.collaborator_key=lower(trim(r.name))
	 set r.audience_size=totals.total_audience,
	 r.audience_size_unit=case when r.resource_type='媒体' and lower(trim(r.platform)) in ('website','web') then 'Monthly Visits' when r.resource_type='媒体' then 'UMV' else 'Followers' end,
	 r.tier=if(trim(r.tier)<>'',r.tier,case when totals.total_audience<=0 then '' when totals.total_audience>1000000 then '头部' when totals.total_audience>=100000 then '腰部' else '尾部' end)
	 where r.id=?`, resourceID, resourceID)
	return err
}

func insertCreatorDiscoveryPosts(ctx context.Context, tx *sql.Tx, resourceID int64, posts []platformPost) error {
	for _, post := range posts {
		if post.PlatformPostID == "" {
			continue
		}
		// Existing content may have been curated or synced more recently. An
		// approval adds missing source works without replacing those records.
		_, err := tx.ExecContext(ctx, `insert into biz_resource_platform_posts
		 (resource_id,platform,platform_post_id,title,description,post_url,cover_remote_url,
		 media_type,published_at,duration_seconds,view_count,like_count,comment_count,share_count,save_count,synced_at)
		 values (?,'YouTube',?,?,?,?,?,?,?,?,?,?,?,?,?,now()) on duplicate key update id=id`,
			resourceID, post.PlatformPostID, creatorDiscoveryImportText(post.Title, 255), post.Description, post.PostURL,
			normalizedRemoteImageURL(firstNonEmpty(post.CoverRemoteURL, post.CoverURL)), firstNonEmpty(post.MediaType, "VIDEO"),
			post.PublishedAt, post.Duration, post.ViewCount, post.LikeCount, post.CommentCount, post.ShareCount, post.SaveCount)
		if err != nil {
			return err
		}
	}
	return nil
}

func creatorDiscoveryImportText(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return string(runes)
}
