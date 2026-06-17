package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

func (a *app) businessResources(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	pageSize := intField(body, "pageSize")
	currentPage := intField(body, "currentPage")
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 200 {
		pageSize = 200
	}
	if currentPage <= 0 {
		currentPage = 1
	}
	offset := (currentPage - 1) * pageSize
	sequenceSortOrder := strings.ToLower(str(body, "sequenceSortOrder"))
	if sequenceSortOrder != "desc" {
		sequenceSortOrder = "asc"
	}
	where, args := businessFilters(body, map[string]string{
		"name":         "name like ?",
		"resourceType": "resource_type = ?",
		"country":      "country like ?",
		"language":     "language like ?",
		"platform":     "platform = ?",
		"industry":     "concat(industry, category) like ?",
		"tier":         "tier = ?",
		"status":       "status = ?",
		"level":        "level = ?",
		"riskLevel":    "risk_level = ?",
		"owner":        "owner like ?",
		"regionTeam":   "region_team like ?",
	})
	var total int
	if err := a.DB().QueryRowContext(r.Context(), `select count(*) from biz_resources`+where, args...).Scan(&total); err != nil {
		writeDBError(w, err)
		return
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, pageSize, offset)
	rows, err := a.queryMaps(r.Context(),
		`select id, name, resource_type as resourceType, media_outlet as mediaOutlet,
		        tier, country, region, city, language, platform, industry, category, title,
		        contact, owner, region_team as regionTeam, reference_source as referenceSource,
		        shipping_address as shippingAddress, website, import_source_sheet as importSourceSheet,
		        status, followers, engagement_rate as engagementRate, avg_views as avgViews,
		        content_types as contentTypes, platform_url as platformUrl,
		        platform_user_id as platformUserId, platform_handle as platformHandle,
		        avatar_url as avatarUrl, total_views as totalViews, video_count as videoCount,
		        last_sync_status as lastSyncStatus, last_sync_error as lastSyncError,
		        cast(unix_timestamp(last_sync_at) * 1000 as unsigned) as lastSyncAt,
		        score, level, risk_level as riskLevel, notes,
		        cast(unix_timestamp(updated_at) * 1000 as unsigned) as updatedAt
		   from biz_resources`+where+` order by id `+sequenceSortOrder+` limit ? offset ?`,
		queryArgs...,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if err := a.attachResourceTags(r.Context(), rows); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, tableData{List: rows, Total: total, PageSize: pageSize, CurrentPage: currentPage})
}

func (a *app) createBusinessResource(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(),
		`insert into biz_resources
		 (name, resource_type, media_outlet, tier, country, region, city, language, platform,
		  industry, category, title, contact, owner, region_team, reference_source,
		  shipping_address, status, followers, engagement_rate, avg_views, content_types,
		  platform_url, website, score, level, risk_level, notes)
		 values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		str(body, "name"), str(body, "resourceType"), str(body, "mediaOutlet"), str(body, "tier"),
		str(body, "country"), str(body, "region"), str(body, "city"), str(body, "language"),
		str(body, "platform"), str(body, "industry"), str(body, "category"), str(body, "title"),
		str(body, "contact"), str(body, "owner"), str(body, "regionTeam"), str(body, "referenceSource"),
		str(body, "shippingAddress"), defaultString(str(body, "status"), "可合作"),
		intField(body, "followers"), floatField(body, "engagementRate"), intField(body, "avgViews"),
		str(body, "contentTypes"), str(body, "platformUrl"), str(body, "website"),
		intField(body, "score"), calcLevel(intField(body, "score")),
		defaultString(str(body, "riskLevel"), "低"), str(body, "notes"),
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		writeDBError(w, err)
		return
	}
	if err := saveResourceTags(r.Context(), tx, id, intSliceField(body, "tagIds")); err != nil {
		writeDBError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"created": true})
}

func (a *app) updateBusinessResource(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	id := intField(body, "id")
	if id == 0 {
		writeError(w, http.StatusOK, 10001, "资源 id 不能为空")
		return
	}
	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(),
		`update biz_resources set
		  name=?, resource_type=?, media_outlet=?, tier=?, country=?, region=?, city=?,
		  language=?, platform=?, industry=?, category=?, title=?, contact=?, owner=?,
		  region_team=?, reference_source=?, shipping_address=?, status=?, followers=?,
		  engagement_rate=?, avg_views=?, content_types=?, platform_url=?, website=?,
		  score=?, level=?, risk_level=?, notes=?
		 where id=?`,
		str(body, "name"), str(body, "resourceType"), str(body, "mediaOutlet"), str(body, "tier"),
		str(body, "country"), str(body, "region"), str(body, "city"), str(body, "language"),
		str(body, "platform"), str(body, "industry"), str(body, "category"), str(body, "title"),
		str(body, "contact"), str(body, "owner"), str(body, "regionTeam"), str(body, "referenceSource"),
		str(body, "shippingAddress"), defaultString(str(body, "status"), "可合作"),
		intField(body, "followers"), floatField(body, "engagementRate"), intField(body, "avgViews"),
		str(body, "contentTypes"), str(body, "platformUrl"), str(body, "website"),
		intField(body, "score"), calcLevel(intField(body, "score")),
		defaultString(str(body, "riskLevel"), "低"), str(body, "notes"), id,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if err := saveResourceTags(r.Context(), tx, int64(id), intSliceField(body, "tagIds")); err != nil {
		writeDBError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"updated": true})
}

func (a *app) deleteBusinessResource(w http.ResponseWriter, r *http.Request) {
	id := intField(readBody(r), "id")
	if id == 0 {
		writeError(w, http.StatusOK, 10001, "资源 id 不能为空")
		return
	}
	_, err := a.DB().ExecContext(r.Context(), `delete from biz_resources where id = ?`, id)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"deleted": true})
}

func (a *app) businessResourceExtraFields(w http.ResponseWriter, r *http.Request) {
	rows, err := a.queryMaps(r.Context(),
		`select id, field_key as fieldKey, label, source_header as sourceHeader, status
		   from biz_resource_extra_fields
		  where status = '启用'
		  order by id asc`,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, rows)
}

func (a *app) businessResourcePosts(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	pageSize := intField(body, "pageSize")
	currentPage := intField(body, "currentPage")
	if pageSize <= 0 {
		pageSize = 12
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if currentPage <= 0 {
		currentPage = 1
	}
	offset := (currentPage - 1) * pageSize
	resourceID := intField(body, "resourceId")
	platform := str(body, "platform")
	keyword := str(body, "keyword")

	where := " where 1 = 1"
	args := []any{}
	if resourceID > 0 {
		where += " and p.resource_id = ?"
		args = append(args, resourceID)
	}
	if platform != "" {
		where += " and p.platform = ?"
		args = append(args, platform)
	}
	if keyword != "" {
		where += " and (p.title like ? or p.description like ? or r.name like ?)"
		like := "%" + keyword + "%"
		args = append(args, like, like, like)
	}

	var total int
	if err := a.DB().QueryRowContext(r.Context(),
		`select count(*)
		   from biz_resource_platform_posts p
		   left join biz_resources r on r.id = p.resource_id`+where,
		args...,
	).Scan(&total); err != nil {
		writeDBError(w, err)
		return
	}

	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, pageSize, offset)
	rows, err := a.queryMaps(r.Context(),
		`select p.id, p.resource_id as resourceId, r.name as resourceName, r.avatar_url as resourceAvatarUrl,
		        r.platform_handle as platformHandle, p.platform, p.platform_post_id as platformPostId,
		        p.title, p.description, p.post_url as postUrl, p.cover_url as coverUrl,
		        p.media_type as mediaType, cast(unix_timestamp(p.published_at) * 1000 as unsigned) as publishedAt,
		        p.duration_seconds as durationSeconds, p.view_count as viewCount, p.like_count as likeCount,
		        p.comment_count as commentCount, p.share_count as shareCount,
		        cast(unix_timestamp(p.synced_at) * 1000 as unsigned) as syncedAt
		   from biz_resource_platform_posts p
		   left join biz_resources r on r.id = p.resource_id`+where+`
		  order by p.published_at desc, p.id desc
		  limit ? offset ?`,
		queryArgs...,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}

	resource := map[string]any{}
	stats := map[string]any{}
	if resourceID > 0 {
		resourceRows, err := a.queryMaps(r.Context(),
			`select id, name, resource_type as resourceType, country, language, platform,
			        platform_user_id as platformUserId, platform_handle as platformHandle,
			        avatar_url as avatarUrl, followers, total_views as totalViews,
			        video_count as videoCount, avg_views as avgViews,
			        engagement_rate as engagementRate, last_sync_status as lastSyncStatus,
			        last_sync_error as lastSyncError,
			        cast(unix_timestamp(last_sync_at) * 1000 as unsigned) as lastSyncAt
			   from biz_resources where id = ? limit 1`,
			resourceID,
		)
		if err != nil {
			writeDBError(w, err)
			return
		}
		if len(resourceRows) > 0 {
			resource = resourceRows[0]
		}
		statsRows, err := a.queryMaps(r.Context(),
			`select count(*) as postCount,
			        coalesce(sum(view_count), 0) as totalViews,
			        coalesce(sum(like_count), 0) as totalLikes,
			        coalesce(sum(comment_count), 0) as totalComments,
			        coalesce(sum(share_count), 0) as totalShares,
			        coalesce(round(avg(nullif(view_count, 0))), 0) as avgViews,
			        cast(unix_timestamp(max(published_at)) * 1000 as unsigned) as latestPublishedAt
			   from biz_resource_platform_posts
			  where resource_id = ?`,
			resourceID,
		)
		if err != nil {
			writeDBError(w, err)
			return
		}
		if len(statsRows) > 0 {
			stats = statsRows[0]
		}
	}

	writeOK(w, map[string]any{
		"list":        rows,
		"total":       total,
		"pageSize":    pageSize,
		"currentPage": currentPage,
		"resource":    resource,
		"stats":       stats,
	})
}

func (a *app) attachResourceTags(ctx context.Context, rows []map[string]any) error {
	if len(rows) == 0 {
		return nil
	}
	placeholders := make([]string, 0, len(rows))
	args := make([]any, 0, len(rows))
	byID := make(map[int]map[string]any, len(rows))
	for _, row := range rows {
		id := intField(row, "id")
		if id == 0 {
			continue
		}
		row["tagIds"] = []int{}
		row["tagNames"] = []string{}
		row["tags"] = []map[string]any{}
		placeholders = append(placeholders, "?")
		args = append(args, id)
		byID[id] = row
	}
	if len(placeholders) == 0 {
		return nil
	}
	query := `select rt.resource_id, t.id, t.name, t.category, t.color
	            from biz_resource_tags rt
	            join biz_tags t on t.id = rt.tag_id
	           where rt.resource_id in (` + strings.Join(placeholders, ",") + `)
	           order by t.category, t.id`
	tagRows, err := a.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer tagRows.Close()
	for tagRows.Next() {
		var resourceID int
		var tagID int
		var name string
		var category string
		var color string
		if err := tagRows.Scan(&resourceID, &tagID, &name, &category, &color); err != nil {
			return err
		}
		row := byID[resourceID]
		if row == nil {
			continue
		}
		tagIDs, _ := row["tagIds"].([]int)
		tagNames, _ := row["tagNames"].([]string)
		tags, _ := row["tags"].([]map[string]any)
		row["tagIds"] = append(tagIDs, tagID)
		row["tagNames"] = append(tagNames, name)
		row["tags"] = append(tags, map[string]any{
			"id":       tagID,
			"name":     name,
			"category": category,
			"color":    color,
		})
	}
	return tagRows.Err()
}

func saveResourceTags(ctx context.Context, tx *sql.Tx, resourceID int64, tagIDs []int) error {
	if _, err := tx.ExecContext(ctx, `delete from biz_resource_tags where resource_id = ?`, resourceID); err != nil {
		return err
	}
	seen := make(map[int]bool, len(tagIDs))
	for _, tagID := range tagIDs {
		if tagID <= 0 || seen[tagID] {
			continue
		}
		seen[tagID] = true
		if _, err := tx.ExecContext(ctx,
			`insert into biz_resource_tags (resource_id, tag_id) values (?, ?)`,
			resourceID, tagID,
		); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) importBusinessResources(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	rows, ok := body["rows"].([]any)
	if !ok || len(rows) == 0 {
		writeError(w, http.StatusOK, 10001, "导入数据不能为空")
		return
	}

	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()

	imported := 0
	created := 0
	updated := 0
	var errors []map[string]any
	for index, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			errors = append(errors, map[string]any{"row": index + 1, "message": "行数据格式错误"})
			continue
		}
		resourceID, isCreated, err := upsertImportMediaResource(r.Context(), tx, row)
		if err != nil {
			errors = append(errors, map[string]any{"row": intField(row, "rowNo"), "message": err.Error()})
			continue
		}
		_ = resourceID
		imported++
		if isCreated {
			created++
		} else {
			updated++
		}
	}

	if imported == 0 {
		message := "没有可导入的有效数据"
		if len(errors) > 0 {
			message = fmt.Sprintf("%s：%s", message, errors[0]["message"])
		}
		writeError(w, http.StatusOK, 10002, message)
		return
	}
	if err := tx.Commit(); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{
		"imported": imported,
		"created":  created,
		"updated":  updated,
		"failed":   len(errors),
		"errors":   errors,
	})
}

func (a *app) attachResourceExtraFields(ctx context.Context, rows []map[string]any) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]string, 0, len(rows))
	byID := make(map[int]map[string]any, len(rows))
	for _, row := range rows {
		id := intField(row, "id")
		if id == 0 {
			continue
		}
		row["extraFields"] = map[string]any{}
		ids = append(ids, "?")
		byID[id] = row
	}
	if len(ids) == 0 {
		return nil
	}
	args := make([]any, 0, len(byID))
	for id := range byID {
		args = append(args, id)
	}
	query := `select v.resource_id, f.field_key, v.value
	            from biz_resource_extra_values v
	            join biz_resource_extra_fields f on f.id = v.field_id
	           where v.resource_id in (` + strings.Join(ids, ",") + `)`
	valueRows, err := a.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer valueRows.Close()
	for valueRows.Next() {
		var resourceID int
		var fieldKey string
		var value sql.NullString
		if err := valueRows.Scan(&resourceID, &fieldKey, &value); err != nil {
			return err
		}
		row := byID[resourceID]
		if row == nil {
			continue
		}
		extraFields, _ := row["extraFields"].(map[string]any)
		if value.Valid {
			extraFields[fieldKey] = value.String
		}
	}
	return valueRows.Err()
}

func upsertImportMediaResource(ctx context.Context, tx *sql.Tx, row map[string]any) (int64, bool, error) {
	name := cleanImportString(str(row, "name"))
	email := cleanImportString(str(row, "email"))
	if name == "" {
		return 0, false, fmt.Errorf("Name 不能为空")
	}
	if email == "" {
		return 0, false, fmt.Errorf("Email 不能为空")
	}
	mediaOutlet := cleanImportString(str(row, "mediaOutlet"))
	tier := cleanImportString(str(row, "tier"))
	platform := cleanImportString(str(row, "platform"))
	category := cleanImportString(str(row, "category"))
	base := cleanImportString(str(row, "base"))
	title := cleanImportString(str(row, "title"))
	followers := intField(row, "followers")
	owner := cleanImportString(str(row, "pic"))
	status := cleanImportString(str(row, "status"))
	sourceSheet := cleanImportString(str(row, "sourceSheet"))
	shippingAddress := cleanImportString(str(row, "shippingAddress"))
	website := cleanImportString(str(row, "website"))
	notes := cleanImportString(str(row, "notes"))
	reference := cleanImportString(str(row, "reference"))
	resourceType := "KOL"
	categoryLower := strings.ToLower(category)
	if strings.Contains(categoryLower, "media") || strings.Contains(category, "媒体") {
		resourceType = "媒体"
	}

	id, found, err := findImportResource(ctx, tx, name, email)
	if err != nil {
		return 0, false, err
	}
	if found {
		_, err = tx.ExecContext(ctx,
			`update biz_resources set
			  resource_type = if(? <> '', ?, resource_type),
			  media_outlet = if(? <> '', ?, media_outlet),
			  tier = if(? <> '', ?, tier),
			  country = if(? <> '', ?, country),
			  platform = if(? <> '', ?, platform),
			  industry = if(? <> '', ?, industry),
			  category = if(? <> '', ?, category),
			  title = if(? <> '', ?, title),
			  contact = if(? <> '', ?, contact),
			  followers = if(? > 0, ?, followers),
			  owner = if(? <> '', ?, owner),
			  status = if(? <> '', ?, status),
			  reference_source = if(? <> '', ?, reference_source),
			  shipping_address = if(? <> '', ?, shipping_address),
			  website = if(? <> '', ?, website),
			  import_source_sheet = if(? <> '', ?, import_source_sheet),
			  notes = if(? <> '', ?, notes)
			 where id = ?`,
			resourceType, resourceType, mediaOutlet, mediaOutlet, tier, tier, base, base,
			platform, platform, category, category, title, title, title, title, email, email,
			followers, followers, owner, owner, status, status, reference, reference,
			shippingAddress, shippingAddress, website, website, sourceSheet, sourceSheet, notes, notes, id,
		)
		return id, false, err
	}

	result, err := tx.ExecContext(ctx,
		`insert into biz_resources
		 (name, resource_type, media_outlet, tier, country, platform, industry, category, title,
		  contact, followers, owner, status, reference_source, shipping_address, website,
		  import_source_sheet, notes, score, level, risk_level)
		 values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 70, 'B', '低')`,
		name, resourceType, mediaOutlet, tier, base, platform, category, title, title, email,
		followers, owner, defaultString(status, "可合作"), reference, shippingAddress,
		website, sourceSheet, notes,
	)
	if err != nil {
		return 0, false, err
	}
	id, err = result.LastInsertId()
	return id, true, err
}

func findImportResource(ctx context.Context, tx *sql.Tx, name, email string) (int64, bool, error) {
	var id int64
	err := tx.QueryRowContext(ctx,
		`select id from biz_resources where lower(name) = lower(?) and lower(contact) = lower(?) limit 1`,
		name, email,
	).Scan(&id)
	if err == nil {
		return id, true, nil
	}
	if err != sql.ErrNoRows {
		return 0, false, err
	}
	return 0, false, nil
}

func upsertResourceExtraValues(ctx context.Context, tx *sql.Tx, resourceID int64, row map[string]any) error {
	extra, ok := row["extra"].(map[string]any)
	if !ok {
		return nil
	}
	for label, rawValue := range extra {
		label = cleanImportString(label)
		value := cleanImportString(fmt.Sprint(rawValue))
		if label == "" || value == "" || value == "<nil>" {
			continue
		}
		fieldKey := extraFieldKey(label)
		fieldID, err := upsertResourceExtraField(ctx, tx, fieldKey, label)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`insert into biz_resource_extra_values (resource_id, field_id, value)
			 values (?, ?, ?)
			 on duplicate key update value = values(value)`,
			resourceID, fieldID, value,
		); err != nil {
			return err
		}
	}
	return nil
}

func upsertResourceExtraField(ctx context.Context, tx *sql.Tx, fieldKey, label string) (int64, error) {
	_, err := tx.ExecContext(ctx,
		`insert into biz_resource_extra_fields (field_key, label, source_header)
		 values (?, ?, ?)
		 on duplicate key update label = values(label), source_header = values(source_header)`,
		fieldKey, label, label,
	)
	if err != nil {
		return 0, err
	}
	var id int64
	err = tx.QueryRowContext(ctx,
		`select id from biz_resource_extra_fields where field_key = ? limit 1`,
		fieldKey,
	).Scan(&id)
	return id, err
}

func extraFieldKey(label string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(label) {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r >= '\u4e00' && r <= '\u9fff':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}
	key := strings.Trim(builder.String(), "_")
	for strings.Contains(key, "__") {
		key = strings.ReplaceAll(key, "__", "_")
	}
	if key == "" {
		key = "field"
	}
	if len(key) > 120 {
		key = key[:120]
	}
	return key
}

func cleanImportString(value string) string {
	value = strings.TrimSpace(value)
	if value == "<nil>" {
		return ""
	}
	return value
}

func (a *app) syncBusinessResource(w http.ResponseWriter, r *http.Request) {
	id := intField(readBody(r), "id")
	if id == 0 {
		writeError(w, http.StatusOK, 10001, "资源 id 不能为空")
		return
	}
	resource, err := a.syncableResourceByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusOK, 10002, "资源不存在或当前平台暂不支持同步")
			return
		}
		writeDBError(w, err)
		return
	}
	if platformDisplayName(resource.Platform) == "" {
		writeError(w, http.StatusOK, 10003, "当前平台暂不支持官方 API 同步")
		return
	}
	running, err := a.latestRunningPlatformSyncJob(r.Context())
	if err != nil {
		writeDBError(w, err)
		return
	}
	if running != nil {
		writeOK(w, map[string]any{
			"started": false,
			"message": "已有同步任务正在运行",
			"job":     running,
		})
		return
	}
	result, err := a.DB().ExecContext(r.Context(),
		`insert into biz_platform_sync_jobs
		  (job_type, status, total_count, current_resource_id, current_resource_name, started_at, message)
		 values ('resource_sync_one', '运行中', 1, ?, ?, now(), '任务已启动')`,
		resource.ID, resource.Name,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	jobID, err := result.LastInsertId()
	if err != nil {
		writeDBError(w, err)
		return
	}
	go a.runBusinessResourceSyncOne(int(jobID), resource)
	job, err := a.platformSyncJob(r.Context(), int(jobID))
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{
		"started": true,
		"jobId":   jobID,
		"message": "单个资源同步任务已启动",
		"job":     job,
	})
}

func (a *app) syncYouTubeResource(ctx context.Context, id int, name, platformURL string) (map[string]any, error) {
	cfg := a.effectivePlatformAPIConfig(ctx)
	apiKey := strings.TrimSpace(cfg.YouTubeAPIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("未配置 YouTube API Key，请在系统管理/抓取控制中配置")
	}
	paramName, identifier, err := youtubeChannelIdentifier(name, platformURL)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("part", "snippet,statistics,contentDetails")
	params.Set("key", apiKey)
	params.Set(paramName, identifier)
	apiURL := "https://www.googleapis.com/youtube/v3/channels?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	client, err := youtubeHTTPClient(cfg)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title      string `json:"title"`
				CustomURL  string `json:"customUrl"`
				Country    string `json:"country"`
				Thumbnails map[string]struct {
					URL string `json:"url"`
				} `json:"thumbnails"`
			} `json:"snippet"`
			Statistics struct {
				ViewCount       string `json:"viewCount"`
				SubscriberCount string `json:"subscriberCount"`
				VideoCount      string `json:"videoCount"`
			} `json:"statistics"`
			ContentDetails struct {
				RelatedPlaylists struct {
					Uploads string `json:"uploads"`
				} `json:"relatedPlaylists"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	if err := decodePlatformResponse(resp, "YouTube", &payload); err != nil {
		return nil, err
	}
	if len(payload.Items) == 0 {
		return nil, fmt.Errorf("YouTube 未找到对应频道，请检查频道 ID 或 @handle")
	}
	item := payload.Items[0]
	followers := parseCount(item.Statistics.SubscriberCount)
	totalViews := parseCount(item.Statistics.ViewCount)
	videoCount := parseCount(item.Statistics.VideoCount)
	avgViews := int64(0)
	if videoCount > 0 {
		avgViews = totalViews / videoCount
	}
	avatarURL := bestThumbnailURL(item.Snippet.Thumbnails)
	syncPosts, postLimit := a.platformPostSyncOptions(ctx, "YouTube", 25)
	posts := []platformPost{}
	if syncPosts {
		posts, err = a.fetchYouTubePosts(ctx, client, apiKey, id, item.ContentDetails.RelatedPlaylists.Uploads, postLimit)
		if err != nil {
			return nil, err
		}
	}
	if avgViews == 0 && len(posts) > 0 {
		avgViews = averagePostViews(posts)
	}
	engagementRate := platformPostEngagementRate(posts, followers)
	avatarURL = localizeResourceImage(ctx, id, "avatar", avatarURL)
	_, err = a.DB().ExecContext(ctx,
		`update biz_resources set
		  name = if(? <> '', ?, name),
		  country = if(country = '' and ? <> '', ?, country),
		  followers = ?, total_views = ?, video_count = ?, avg_views = ?,
		  engagement_rate = if(? > 0, ?, engagement_rate),
		  platform_user_id = ?, platform_handle = ?, avatar_url = ?, last_sync_status = '成功',
		  last_sync_error = '', last_sync_at = now()
		 where id = ?`,
		item.Snippet.Title, item.Snippet.Title, item.Snippet.Country, item.Snippet.Country,
		followers, totalViews, videoCount, avgViews, engagementRate, engagementRate,
		item.ID, item.Snippet.CustomURL, avatarURL, id,
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"platform":       "YouTube",
		"platformUserId": item.ID,
		"platformHandle": item.Snippet.CustomURL,
		"name":           item.Snippet.Title,
		"followers":      followers,
		"totalViews":     totalViews,
		"videoCount":     videoCount,
		"avgViews":       avgViews,
		"engagementRate": engagementRate,
		"avatarUrl":      avatarURL,
		"syncedPosts":    len(posts),
		"posts":          posts,
		"syncedAt":       time.Now().Format(time.RFC3339),
	}, nil
}

func youtubeHTTPClient(cfg PlatformAPIConfig) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL := strings.TrimSpace(cfg.YouTubeProxyURL); proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("YouTube 代理地址无效：%w", err)
		}
		transport.Proxy = http.ProxyURL(parsed)
	}
	return &http.Client{Transport: transport, Timeout: 20 * time.Second}, nil
}

type platformPost struct {
	PlatformPostID string     `json:"platformPostId"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	PostURL        string     `json:"postUrl"`
	CoverURL       string     `json:"coverUrl"`
	MediaType      string     `json:"mediaType"`
	PublishedAt    *time.Time `json:"publishedAt"`
	Duration       int        `json:"duration"`
	ViewCount      int64      `json:"viewCount"`
	LikeCount      int64      `json:"likeCount"`
	CommentCount   int64      `json:"commentCount"`
	ShareCount     int64      `json:"shareCount"`
	Raw            any        `json:"-"`
}

func (a *app) fetchYouTubePosts(ctx context.Context, client *http.Client, apiKey string, resourceID int, uploadsPlaylistID string, postLimit int) ([]platformPost, error) {
	uploadsPlaylistID = strings.TrimSpace(uploadsPlaylistID)
	if uploadsPlaylistID == "" {
		return nil, nil
	}
	params := url.Values{}
	params.Set("part", "contentDetails")
	params.Set("playlistId", uploadsPlaylistID)
	params.Set("maxResults", strconv.Itoa(clampInt(postLimit, 1, 50)))
	params.Set("key", apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/youtube/v3/playlistItems?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var listPayload struct {
		Items []struct {
			ContentDetails struct {
				VideoID string `json:"videoId"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	if err := decodePlatformResponse(resp, "YouTube", &listPayload); err != nil {
		return nil, err
	}
	videoIDs := make([]string, 0, len(listPayload.Items))
	for _, item := range listPayload.Items {
		if item.ContentDetails.VideoID != "" {
			videoIDs = append(videoIDs, item.ContentDetails.VideoID)
		}
	}
	if len(videoIDs) == 0 {
		return nil, nil
	}

	videoParams := url.Values{}
	videoParams.Set("part", "snippet,statistics,contentDetails")
	videoParams.Set("id", strings.Join(videoIDs, ","))
	videoParams.Set("key", apiKey)
	videoReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/youtube/v3/videos?"+videoParams.Encode(), nil)
	if err != nil {
		return nil, err
	}
	videoResp, err := client.Do(videoReq)
	if err != nil {
		return nil, err
	}
	defer videoResp.Body.Close()
	var videoPayload struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				PublishedAt string `json:"publishedAt"`
				Title       string `json:"title"`
				Description string `json:"description"`
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
	if err := decodePlatformResponse(videoResp, "YouTube", &videoPayload); err != nil {
		return nil, err
	}
	posts := make([]platformPost, 0, len(videoPayload.Items))
	for _, item := range videoPayload.Items {
		publishedAt := parsePlatformTime(item.Snippet.PublishedAt)
		posts = append(posts, platformPost{
			PlatformPostID: item.ID,
			Title:          item.Snippet.Title,
			Description:    item.Snippet.Description,
			PostURL:        "https://www.youtube.com/watch?v=" + item.ID,
			CoverURL:       bestThumbnailURL(item.Snippet.Thumbnails),
			MediaType:      "VIDEO",
			PublishedAt:    publishedAt,
			Duration:       parseYouTubeDurationSeconds(item.ContentDetails.Duration),
			ViewCount:      parseCount(item.Statistics.ViewCount),
			LikeCount:      parseCount(item.Statistics.LikeCount),
			CommentCount:   parseCount(item.Statistics.CommentCount),
			Raw:            item,
		})
	}
	if err := a.upsertResourcePlatformPosts(ctx, resourceID, "YouTube", posts); err != nil {
		return nil, err
	}
	return posts, nil
}

func (a *app) fetchYouTubePostByID(ctx context.Context, resourceID int, postID string) (platformPost, error) {
	cfg := a.effectivePlatformAPIConfig(ctx)
	apiKey := strings.TrimSpace(cfg.YouTubeAPIKey)
	if apiKey == "" {
		return platformPost{}, fmt.Errorf("未配置 YouTube API Key")
	}
	client, err := youtubeHTTPClient(cfg)
	if err != nil {
		return platformPost{}, err
	}
	params := url.Values{}
	params.Set("part", "snippet,statistics,contentDetails")
	params.Set("id", postID)
	params.Set("key", apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/youtube/v3/videos?"+params.Encode(), nil)
	if err != nil {
		return platformPost{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return platformPost{}, err
	}
	defer resp.Body.Close()
	var payload struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				PublishedAt string `json:"publishedAt"`
				Title       string `json:"title"`
				Description string `json:"description"`
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
	if err := decodePlatformResponse(resp, "YouTube", &payload); err != nil {
		return platformPost{}, err
	}
	if len(payload.Items) == 0 {
		return platformPost{}, fmt.Errorf("YouTube 未找到对应作品")
	}
	item := payload.Items[0]
	post := platformPost{
		PlatformPostID: item.ID,
		Title:          item.Snippet.Title,
		Description:    item.Snippet.Description,
		PostURL:        "https://www.youtube.com/watch?v=" + item.ID,
		CoverURL:       bestThumbnailURL(item.Snippet.Thumbnails),
		MediaType:      "VIDEO",
		PublishedAt:    parsePlatformTime(item.Snippet.PublishedAt),
		Duration:       parseYouTubeDurationSeconds(item.ContentDetails.Duration),
		ViewCount:      parseCount(item.Statistics.ViewCount),
		LikeCount:      parseCount(item.Statistics.LikeCount),
		CommentCount:   parseCount(item.Statistics.CommentCount),
		Raw:            item,
	}
	if err := a.upsertResourcePlatformPosts(ctx, resourceID, "YouTube", []platformPost{post}); err != nil {
		return platformPost{}, err
	}
	return post, nil
}

func (a *app) syncInstagramResource(ctx context.Context, id int, name, platformURL, platformUserID, platformHandle string) (map[string]any, error) {
	cfg := a.effectivePlatformAPIConfig(ctx)
	apiKey := strings.TrimSpace(tikHubAPIKey(cfg))
	if apiKey == "" {
		return nil, fmt.Errorf("未配置 TikHub API Key，请在系统管理/抓取控制中配置")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	handle := instagramHandleIdentifier(platformHandle, platformURL, name)
	endpoint := "/instagram/v1/fetch_user_info_by_username_v3"
	params := url.Values{}
	fallback := handle
	if handle != "" {
		params.Set("username", handle)
	} else if userID := strings.TrimSpace(platformUserID); userID != "" {
		endpoint = "/instagram/v1/fetch_user_info_by_id_v2"
		params.Set("user_id", userID)
		fallback = userID
	} else {
		return nil, fmt.Errorf("请先填写 Instagram 主页链接、@handle 或 user_id")
	}
	data, err := tikhubGET(ctx, client, apiKey, endpoint, params)
	if err != nil {
		return nil, err
	}
	user := normalizeTikHubInstagramUser(data, fallback)
	if user.ID == "" && user.Username == "" {
		return nil, fmt.Errorf("TikHub 未返回 Instagram 账号数据，请检查用户名")
	}
	syncPosts, postLimit := a.platformPostSyncOptions(ctx, "Instagram", 25)
	warnings := make([]string, 0, 2)
	postLists := make([][]platformPost, 0, 3)
	if syncPosts {
		postLists = append(postLists, user.Posts)
	}
	if syncPosts && user.ID != "" {
		count := strconv.Itoa(postLimit)
		postsData, postsErr := tikhubGET(ctx, client, apiKey, "/instagram/v1/fetch_user_posts", url.Values{"user_id": []string{user.ID}, "count": []string{count}})
		if postsErr != nil {
			warnings = append(warnings, "普通作品获取失败："+postsErr.Error())
		} else {
			postLists = append(postLists, normalizeTikHubInstagramPostsData(postsData, user.Username))
		}
		reelsData, reelsErr := tikhubGET(ctx, client, apiKey, "/instagram/v1/fetch_user_reels", url.Values{"user_id": []string{user.ID}, "count": []string{count}})
		if reelsErr != nil {
			warnings = append(warnings, "Reels 获取失败："+reelsErr.Error())
		} else {
			postLists = append(postLists, normalizeTikHubInstagramPostsData(reelsData, user.Username))
		}
	} else if syncPosts {
		warnings = append(warnings, "账号资料未返回 user_id，已跳过普通作品和 Reels 接口")
	}
	user.Posts = limitPlatformPosts(mergePlatformPosts(postLists...), postLimit)
	if user.MediaCount == 0 && len(user.Posts) > 0 {
		user.MediaCount = int64(len(user.Posts))
	}
	return a.persistTikHubInstagramUser(ctx, id, user, warnings, syncPosts)
}

func (a *app) syncInstagramBusinessDiscovery(ctx context.Context, client *http.Client, resourceID int, apiVersion, accessToken, igUserID, handle string) (map[string]any, error) {
	fields := fmt.Sprintf("business_discovery.username(%s){id,username,name,biography,profile_picture_url,followers_count,follows_count,media_count,media.limit(25){id,caption,comments_count,like_count,media_product_type,media_type,media_url,permalink,thumbnail_url,timestamp}}", handle)
	params := url.Values{}
	params.Set("fields", fields)
	params.Set("access_token", accessToken)
	reqURL := fmt.Sprintf("https://graph.facebook.com/%s/%s?%s", apiVersion, igUserID, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var payload struct {
		BusinessDiscovery instagramUserPayload `json:"business_discovery"`
	}
	if err := decodePlatformResponse(resp, "Instagram", &payload); err != nil {
		return nil, err
	}
	return a.persistInstagramUser(ctx, resourceID, payload.BusinessDiscovery)
}

func (a *app) syncInstagramOwnedAccount(ctx context.Context, client *http.Client, resourceID int, apiVersion, accessToken, igUserID string) (map[string]any, error) {
	params := url.Values{}
	params.Set("fields", "id,username,name,profile_picture_url,followers_count,follows_count,media_count,media.limit(25){id,caption,comments_count,like_count,media_product_type,media_type,media_url,permalink,thumbnail_url,timestamp}")
	params.Set("access_token", accessToken)
	reqURL := fmt.Sprintf("https://graph.facebook.com/%s/%s?%s", apiVersion, igUserID, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var payload instagramUserPayload
	if err := decodePlatformResponse(resp, "Instagram", &payload); err != nil {
		return nil, err
	}
	return a.persistInstagramUser(ctx, resourceID, payload)
}

type instagramUserPayload struct {
	ID                string `json:"id"`
	Username          string `json:"username"`
	Name              string `json:"name"`
	Biography         string `json:"biography"`
	ProfilePictureURL string `json:"profile_picture_url"`
	FollowersCount    int64  `json:"followers_count"`
	FollowsCount      int64  `json:"follows_count"`
	MediaCount        int64  `json:"media_count"`
	Media             struct {
		Data []struct {
			ID               string `json:"id"`
			Caption          string `json:"caption"`
			CommentsCount    int64  `json:"comments_count"`
			LikeCount        int64  `json:"like_count"`
			MediaProductType string `json:"media_product_type"`
			MediaType        string `json:"media_type"`
			MediaURL         string `json:"media_url"`
			Permalink        string `json:"permalink"`
			ThumbnailURL     string `json:"thumbnail_url"`
			Timestamp        string `json:"timestamp"`
		} `json:"data"`
	} `json:"media"`
}

func (a *app) persistInstagramUser(ctx context.Context, resourceID int, user instagramUserPayload) (map[string]any, error) {
	if user.ID == "" {
		return nil, fmt.Errorf("Instagram 未返回账号数据，请检查授权范围和目标账号是否为专业账号")
	}
	posts := make([]platformPost, 0, len(user.Media.Data))
	totalEngagement := int64(0)
	for _, item := range user.Media.Data {
		publishedAt := parsePlatformTime(item.Timestamp)
		totalEngagement += item.LikeCount + item.CommentsCount
		posts = append(posts, platformPost{
			PlatformPostID: item.ID,
			Title:          truncateText(item.Caption, 120),
			Description:    item.Caption,
			PostURL:        item.Permalink,
			CoverURL:       firstNonEmpty(item.ThumbnailURL, item.MediaURL),
			MediaType:      firstNonEmpty(item.MediaProductType, item.MediaType),
			PublishedAt:    publishedAt,
			LikeCount:      item.LikeCount,
			CommentCount:   item.CommentsCount,
			Raw:            item,
		})
	}
	if err := a.upsertResourcePlatformPosts(ctx, resourceID, "Instagram", posts); err != nil {
		return nil, err
	}
	engagementRate := 0.0
	if user.FollowersCount > 0 && len(posts) > 0 {
		engagementRate = float64(totalEngagement) / float64(user.FollowersCount) / float64(len(posts))
	}
	displayName := firstNonEmpty(user.Name, user.Username)
	avatarURL := localizeResourceImage(ctx, resourceID, "avatar", user.ProfilePictureURL)
	_, err := a.DB().ExecContext(ctx,
		`update biz_resources set
		  name = if(? <> '', ?, name),
		  followers = ?, video_count = ?, engagement_rate = if(? > 0, ?, engagement_rate),
		  platform_user_id = ?, platform_handle = ?, avatar_url = ?, last_sync_status = '成功',
		  last_sync_error = '', last_sync_at = now()
		 where id = ?`,
		displayName, displayName, user.FollowersCount, user.MediaCount, engagementRate, engagementRate,
		user.ID, user.Username, avatarURL, resourceID,
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"platform":       "Instagram",
		"platformUserId": user.ID,
		"platformHandle": user.Username,
		"name":           displayName,
		"followers":      user.FollowersCount,
		"videoCount":     user.MediaCount,
		"engagementRate": engagementRate,
		"avatarUrl":      avatarURL,
		"syncedPosts":    len(posts),
		"posts":          posts,
		"syncedAt":       time.Now().Format(time.RFC3339),
	}, nil
}

func (a *app) syncTikTokResource(ctx context.Context, id int) (map[string]any, error) {
	apiKey := strings.TrimSpace(tikHubAPIKey(a.effectivePlatformAPIConfig(ctx)))
	if apiKey == "" {
		return nil, fmt.Errorf("未配置 TikHub API Key，请在系统管理/抓取控制中配置")
	}
	var resource struct {
		Name           string
		PlatformURL    string
		PlatformUserID string
		PlatformHandle string
	}
	if err := a.DB().QueryRowContext(ctx,
		`select name, platform_url, platform_user_id, platform_handle from biz_resources where id = ? limit 1`,
		id,
	).Scan(&resource.Name, &resource.PlatformURL, &resource.PlatformUserID, &resource.PlatformHandle); err != nil {
		return nil, err
	}
	username, secUID := tiktokUserIdentifier(resource.PlatformHandle, resource.PlatformURL, resource.PlatformUserID, resource.Name)
	if username == "" && secUID == "" {
		return nil, fmt.Errorf("请先填写 TikTok 主页链接、@handle 或 secUid")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	params := url.Values{}
	if username != "" {
		params.Set("uniqueId", username)
	}
	if secUID != "" {
		params.Set("secUid", secUID)
	}
	profileData, err := tikhubGET(ctx, client, apiKey, "/tiktok/web/fetch_user_profile", params)
	if err != nil {
		return nil, err
	}
	user := normalizeTikHubTikTokUser(profileData, username, secUID)
	if user.UserID == "" && user.SecUID == "" && user.Username == "" {
		return nil, fmt.Errorf("TikHub 未返回 TikTok 账号数据，请检查用户名或 secUid")
	}
	syncPosts, postLimit := a.platformPostSyncOptions(ctx, "TikTok", 20)
	warnings := make([]string, 0, 1)
	posts := []platformPost{}
	postsFetched := false
	if syncPosts && user.SecUID != "" {
		var postWarnings []string
		posts, postWarnings, err = a.fetchTikTokPosts(ctx, client, apiKey, id, user.SecUID, user.Username, postLimit)
		warnings = append(warnings, postWarnings...)
		if err != nil {
			warnings = append(warnings, "作品获取失败："+err.Error())
		} else {
			postsFetched = true
		}
	} else if syncPosts {
		warnings = append(warnings, "账号资料未返回 secUid，已跳过作品接口")
	}
	recentViews := sumPostViews(posts)
	avgViews := averagePostViews(posts)
	engagementRate := platformPostEngagementRate(posts, user.FollowerCount)
	if !postsFetched {
		if err := a.DB().QueryRowContext(ctx,
			`select total_views, avg_views, engagement_rate from biz_resources where id = ? limit 1`,
			id,
		).Scan(&recentViews, &avgViews, &engagementRate); err != nil {
			return nil, err
		}
	}
	avatarURL := localizeResourceImage(ctx, id, "avatar", user.AvatarURL)
	_, err = a.DB().ExecContext(ctx,
		`update biz_resources set
		  name = if(? <> '', ?, name),
		  followers = ?, total_views = ?, video_count = ?, avg_views = ?,
		  engagement_rate = if(? > 0, ?, engagement_rate),
		  platform_user_id = ?, platform_handle = ?, avatar_url = ?, last_sync_status = '成功',
		  last_sync_error = '', last_sync_at = now()
		 where id = ?`,
		user.DisplayName, user.DisplayName, user.FollowerCount, recentViews, user.VideoCount, avgViews,
		engagementRate, engagementRate,
		firstNonEmpty(user.SecUID, user.UserID), user.Username, avatarURL, id,
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"platform":       "TikTok",
		"platformUserId": firstNonEmpty(user.SecUID, user.UserID),
		"platformHandle": user.Username,
		"name":           user.DisplayName,
		"followers":      user.FollowerCount,
		"totalViews":     recentViews,
		"videoCount":     user.VideoCount,
		"avgViews":       avgViews,
		"engagementRate": engagementRate,
		"avatarUrl":      avatarURL,
		"syncedPosts":    len(posts),
		"posts":          posts,
		"warnings":       warnings,
		"syncedAt":       time.Now().Format(time.RFC3339),
	}, nil
}

func (a *app) fetchTikTokPosts(ctx context.Context, client *http.Client, apiKey string, resourceID int, secUID, username string, postLimit int) ([]platformPost, []string, error) {
	secUID = strings.TrimSpace(secUID)
	if secUID == "" {
		return nil, nil, nil
	}
	params := url.Values{}
	params.Set("secUid", secUID)
	params.Set("cursor", "0")
	params.Set("count", "20")
	params.Set("coverFormat", "2")
	params.Set("post_item_list_request_type", "0")
	data, err := tikhubGET(ctx, client, apiKey, "/tiktok/web/fetch_user_post", params)
	if err != nil {
		fallbackParams := url.Values{}
		fallbackParams.Set("sec_user_id", secUID)
		fallbackParams.Set("unique_id", "")
		fallbackParams.Set("max_cursor", "0")
		fallbackParams.Set("count", strconv.Itoa(clampInt(postLimit, 1, 20)))
		fallbackParams.Set("sort_type", "0")
		data, fallbackErr := tikhubGET(ctx, client, apiKey, "/tiktok/app/v3/fetch_user_post_videos_v3", fallbackParams)
		if fallbackErr != nil {
			return nil, nil, fmt.Errorf("Web 接口失败：%v；App V3 fallback 失败：%v", err, fallbackErr)
		}
		posts := limitPlatformPosts(normalizeTikHubTikTokPosts(data, username), postLimit)
		if err := a.upsertResourcePlatformPosts(ctx, resourceID, "TikTok", posts); err != nil {
			return nil, nil, err
		}
		return posts, []string{"TikTok Web 作品接口失败，已使用 App V3 fallback 同步"}, nil
	}
	posts := limitPlatformPosts(normalizeTikHubTikTokPosts(data, username), postLimit)
	if err := a.upsertResourcePlatformPosts(ctx, resourceID, "TikTok", posts); err != nil {
		return nil, nil, err
	}
	return posts, nil, nil
}

type tikHubTikTokUser struct {
	UserID         string
	SecUID         string
	Username       string
	DisplayName    string
	AvatarURL      string
	FollowerCount  int64
	FollowingCount int64
	LikesCount     int64
	VideoCount     int64
}

type tikHubInstagramUser struct {
	ID             string
	Username       string
	DisplayName    string
	AvatarURL      string
	FollowerCount  int64
	FollowingCount int64
	MediaCount     int64
	Posts          []platformPost
}

func tikhubGET(ctx context.Context, client *http.Client, apiKey, endpoint string, params url.Values) (map[string]any, error) {
	reqURL := "https://api.tikhub.io/api/v1" + endpoint
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		message := tikhubErrorMessage(body)
		if len(message) > 500 {
			message = message[:500]
		}
		if message == "" {
			message = resp.Status
		}
		return nil, fmt.Errorf("TikHub API 请求失败：%s", message)
	}
	var envelope struct {
		Code      int             `json:"code"`
		Message   string          `json:"message"`
		MessageZH string          `json:"message_zh"`
		Data      json.RawMessage `json:"data"`
	}
	if err := decodeJSON(body, &envelope); err != nil {
		return nil, fmt.Errorf("TikHub API 响应解析失败：%w", err)
	}
	if envelope.Code != 0 && envelope.Code != 200 {
		return nil, fmt.Errorf("TikHub API 返回异常：%s", firstNonEmpty(envelope.MessageZH, envelope.Message, fmt.Sprintf("code=%d", envelope.Code)))
	}
	raw := envelope.Data
	if len(raw) == 0 {
		raw = body
	}
	if string(raw) == "null" {
		return map[string]any{}, nil
	}
	var data any
	if err := decodeJSON(raw, &data); err != nil {
		return nil, fmt.Errorf("TikHub API data 解析失败：%w", err)
	}
	result, ok := data.(map[string]any)
	if !ok {
		return map[string]any{"value": data}, nil
	}
	return result, nil
}

func tikhubErrorMessage(body []byte) string {
	var payload map[string]any
	if err := decodeJSON(body, &payload); err != nil {
		return strings.TrimSpace(string(body))
	}
	detail := mapAt(payload, "detail")
	if len(detail) > 0 {
		return firstNonEmpty(
			anyString(detail["message_zh"]),
			anyString(detail["message"]),
			strings.TrimSpace(string(body)),
		)
	}
	return firstNonEmpty(
		anyString(payload["message_zh"]),
		anyString(payload["message"]),
		strings.TrimSpace(string(body)),
	)
}

func decodeJSON(data []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	return decoder.Decode(target)
}

func normalizeTikHubTikTokUser(data map[string]any, fallbackUsername, fallbackSecUID string) tikHubTikTokUser {
	userInfo := mapAt(data, "userInfo")
	if len(userInfo) == 0 {
		userInfo = data
	}
	user := firstMapAt(userInfo, "user", "user_info")
	if len(user) == 0 {
		user = userInfo
	}
	stats := firstMapAt(userInfo, "stats", "statsV2", "statistics")
	username := normalizeTikTokUsername(firstNonEmpty(
		anyString(user["uniqueId"]),
		anyString(user["unique_id"]),
		anyString(user["username"]),
		fallbackUsername,
	))
	return tikHubTikTokUser{
		UserID:         firstNonEmpty(anyString(user["id"]), anyString(user["uid"]), anyString(user["user_id"])),
		SecUID:         firstNonEmpty(anyString(user["secUid"]), anyString(user["sec_uid"]), fallbackSecUID),
		Username:       username,
		DisplayName:    firstNonEmpty(anyString(user["nickname"]), anyString(user["displayName"]), anyString(user["display_name"]), username),
		AvatarURL:      firstNonEmpty(imageURL(user["avatarLarger"]), imageURL(user["avatarMedium"]), imageURL(user["avatarThumb"]), imageURL(user["avatar_url"])),
		FollowerCount:  firstNonZeroInt64(stats["followerCount"], stats["follower_count"], user["follower_count"]),
		FollowingCount: firstNonZeroInt64(stats["followingCount"], stats["following_count"], user["following_count"]),
		LikesCount:     firstNonZeroInt64(stats["heartCount"], stats["diggCount"], stats["likes_count"], user["likes_count"]),
		VideoCount:     firstNonZeroInt64(stats["videoCount"], stats["video_count"], user["video_count"]),
	}
}

func normalizeTikHubTikTokPosts(data map[string]any, username string) []platformPost {
	items := firstListAt(data, "itemList", "items", "aweme_list", "videos", "data", "list")
	posts := make([]platformPost, 0, len(items))
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		stats := firstMapAt(item, "stats", "statsV2", "statistics")
		video := mapAt(item, "video")
		author := mapAt(item, "author")
		shareInfo := mapAt(item, "share_info")
		postID := firstNonEmpty(anyString(item["id"]), anyString(item["aweme_id"]), anyString(item["awemeId"]))
		if postID == "" {
			continue
		}
		postUsername := normalizeTikTokUsername(firstNonEmpty(anyString(author["uniqueId"]), anyString(author["unique_id"]), username))
		publishedAt := unixPlatformTime(firstNonZeroInt64(item["createTime"], item["create_time"]))
		duration := int(firstNonZeroInt64(video["duration"], item["duration"]))
		if duration > 1000 {
			duration = duration / 1000
		}
		description := firstNonEmpty(anyString(item["desc"]), anyString(item["description"]), anyString(item["video_description"]))
		posts = append(posts, platformPost{
			PlatformPostID: postID,
			Title:          firstNonEmpty(anyString(item["title"]), truncateText(description, 120)),
			Description:    description,
			PostURL:        firstNonEmpty(anyString(item["shareUrl"]), anyString(item["share_url"]), anyString(shareInfo["share_url"]), tikTokVideoURL(postUsername, postID)),
			CoverURL:       firstNonEmpty(imageURL(video["cover"]), imageURL(video["dynamicCover"]), imageURL(video["dynamic_cover"]), imageURL(video["originCover"]), imageURL(video["origin_cover"]), imageURL(item["cover_image_url"])),
			MediaType:      "VIDEO",
			PublishedAt:    publishedAt,
			Duration:       duration,
			ViewCount:      firstNonZeroInt64(stats["playCount"], stats["play_count"], stats["viewCount"], stats["view_count"], item["view_count"]),
			LikeCount:      firstNonZeroInt64(stats["diggCount"], stats["digg_count"], stats["likeCount"], stats["like_count"], item["like_count"]),
			CommentCount:   firstNonZeroInt64(stats["commentCount"], stats["comment_count"], item["comment_count"]),
			ShareCount:     firstNonZeroInt64(stats["shareCount"], stats["share_count"], item["share_count"]),
			Raw:            item,
		})
	}
	return posts
}

func normalizeTikHubInstagramUser(data map[string]any, fallbackUsername string) tikHubInstagramUser {
	user := firstMapAt(data, "user", "user_info")
	if len(user) == 0 {
		user = data
	}
	timeline := firstMapAt(user, "edge_owner_to_timeline_media", "timeline_media")
	posts := normalizeTikHubInstagramPosts(timeline, fallbackUsername)
	username := sanitizeInstagramHandle(firstNonEmpty(anyString(user["username"]), fallbackUsername))
	return tikHubInstagramUser{
		ID:             firstNonEmpty(anyString(user["id"]), anyString(user["pk"]), anyString(user["pk_id"]), anyString(user["user_id"])),
		Username:       username,
		DisplayName:    firstNonEmpty(anyString(user["full_name"]), anyString(user["name"]), username),
		AvatarURL:      firstNonEmpty(imageURL(user["hd_profile_pic_url_info"]), imageURL(user["profile_pic_url_hd"]), imageURL(user["profile_pic_url"]), imageURL(user["profile_picture_url"])),
		FollowerCount:  firstNonZeroInt64(user["follower_count"], nestedInt64(user, "edge_followed_by", "count"), user["followers_count"]),
		FollowingCount: firstNonZeroInt64(user["following_count"], nestedInt64(user, "edge_follow", "count"), user["follows_count"]),
		MediaCount:     firstNonZeroInt64(user["media_count"], nestedInt64(user, "edge_owner_to_timeline_media", "count")),
		Posts:          posts,
	}
}

func normalizeTikHubInstagramPosts(timeline map[string]any, username string) []platformPost {
	rawItems := firstListAt(timeline, "edges", "items", "data", "value", "reels", "clips", "medias")
	posts := make([]platformPost, 0, len(rawItems))
	for _, raw := range rawItems {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if node := mapAt(item, "node"); len(node) > 0 {
			item = node
		}
		if media := mapAt(item, "media"); len(media) > 0 {
			item = media
		}
		postID := firstNonEmpty(anyString(item["id"]), anyString(item["pk"]), anyString(item["media_id"]), anyString(item["shortcode"]), anyString(item["code"]))
		if postID == "" {
			continue
		}
		shortcode := firstNonEmpty(anyString(item["shortcode"]), anyString(item["code"]))
		caption := instagramCaption(item)
		publishedAt := unixPlatformTime(firstNonZeroInt64(item["taken_at_timestamp"], item["taken_at"], item["timestamp"]))
		posts = append(posts, platformPost{
			PlatformPostID: postID,
			Title:          truncateText(caption, 120),
			Description:    caption,
			PostURL:        firstNonEmpty(anyString(item["permalink"]), instagramPostURL(username, shortcode)),
			CoverURL:       firstNonEmpty(imageURL(item["display_url"]), imageURL(item["thumbnail_src"]), imageURL(item["thumbnail_url"]), imageURL(item["media_url"]), imageURL(item["image_versions2"]), imageURL(item["video_versions"])),
			MediaType:      instagramMediaType(item),
			PublishedAt:    publishedAt,
			Duration:       int(firstNonZeroInt64(item["video_duration"], item["duration"])),
			ViewCount:      firstNonZeroInt64(item["play_count"], item["video_play_count"], item["ig_play_count"], item["video_view_count"], item["view_count"]),
			LikeCount:      firstNonZeroInt64(item["like_count"], nestedInt64(item, "edge_liked_by", "count"), nestedInt64(item, "edge_media_preview_like", "count")),
			CommentCount:   firstNonZeroInt64(item["comment_count"], nestedInt64(item, "edge_media_to_comment", "count")),
			ShareCount:     firstNonZeroInt64(item["share_count"]),
			Raw:            item,
		})
	}
	return posts
}

func normalizeTikHubInstagramPostsData(data map[string]any, username string) []platformPost {
	for _, key := range []string{"data", "items", "value", "reels", "clips", "medias", "edge_owner_to_timeline_media"} {
		if nested := mapAt(data, key); len(nested) > 0 {
			if posts := normalizeTikHubInstagramPosts(nested, username); len(posts) > 0 {
				return posts
			}
		}
	}
	return normalizeTikHubInstagramPosts(data, username)
}

func mergePlatformPosts(lists ...[]platformPost) []platformPost {
	byID := make(map[string]platformPost)
	order := make([]string, 0)
	for _, posts := range lists {
		for _, post := range posts {
			id := strings.TrimSpace(post.PlatformPostID)
			if id == "" {
				continue
			}
			if existing, ok := byID[id]; ok {
				byID[id] = mergePlatformPost(existing, post)
				continue
			}
			order = append(order, id)
			byID[id] = post
		}
	}
	result := make([]platformPost, 0, len(order))
	for _, id := range order {
		result = append(result, byID[id])
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].PublishedAt == nil {
			return false
		}
		if result[j].PublishedAt == nil {
			return true
		}
		return result[i].PublishedAt.After(*result[j].PublishedAt)
	})
	return result
}

func limitPlatformPosts(posts []platformPost, limit int) []platformPost {
	if limit <= 0 || len(posts) <= limit {
		return posts
	}
	return posts[:limit]
}

func (a *app) platformPostSyncOptions(ctx context.Context, platform string, defaultLimit int) (bool, int) {
	var syncPosts int
	var postLimit int
	err := a.DB().QueryRowContext(ctx,
		`select sync_posts, post_limit from biz_platform_sync_settings where platform = ? limit 1`,
		platform,
	).Scan(&syncPosts, &postLimit)
	if err != nil {
		return true, defaultLimit
	}
	return syncPosts == 1, clampInt(postLimit, 1, 50)
}

func mergePlatformPost(current, incoming platformPost) platformPost {
	current.Title = firstNonEmpty(incoming.Title, current.Title)
	current.Description = firstNonEmpty(incoming.Description, current.Description)
	current.PostURL = firstNonEmpty(incoming.PostURL, current.PostURL)
	current.CoverURL = firstNonEmpty(incoming.CoverURL, current.CoverURL)
	current.MediaType = firstNonEmpty(incoming.MediaType, current.MediaType)
	if incoming.PublishedAt != nil {
		current.PublishedAt = incoming.PublishedAt
	}
	if incoming.Duration > 0 {
		current.Duration = incoming.Duration
	}
	if incoming.ViewCount > 0 {
		current.ViewCount = incoming.ViewCount
	}
	if incoming.LikeCount > 0 {
		current.LikeCount = incoming.LikeCount
	}
	if incoming.CommentCount > 0 {
		current.CommentCount = incoming.CommentCount
	}
	if incoming.ShareCount > 0 {
		current.ShareCount = incoming.ShareCount
	}
	if incoming.Raw != nil {
		current.Raw = incoming.Raw
	}
	return current
}

func (a *app) persistTikHubInstagramUser(ctx context.Context, resourceID int, user tikHubInstagramUser, warnings []string, syncPosts bool) (map[string]any, error) {
	if user.ID == "" && user.Username == "" {
		return nil, fmt.Errorf("TikHub 未返回 Instagram 账号数据，请检查用户名")
	}
	if err := a.upsertResourcePlatformPosts(ctx, resourceID, "Instagram", user.Posts); err != nil {
		return nil, err
	}
	engagementRate := platformPostEngagementRate(user.Posts, user.FollowerCount)
	totalViews := sumPostViews(user.Posts)
	avgViews := averageViewedPostViews(user.Posts)
	active30D, active90D := recentPostCounts(user.Posts, time.Now())
	postFrequency := ""
	if len(user.Posts) > 0 {
		postFrequency = fmt.Sprintf("%d/月", active30D)
	}
	if !syncPosts {
		if err := a.DB().QueryRowContext(ctx,
			`select total_views, avg_views, engagement_rate, active_30d, active_90d, post_frequency
			   from biz_resources where id = ? limit 1`,
			resourceID,
		).Scan(&totalViews, &avgViews, &engagementRate, &active30D, &active90D, &postFrequency); err != nil {
			return nil, err
		}
	}
	avatarURL := localizeResourceImage(ctx, resourceID, "avatar", user.AvatarURL)
	_, err := a.DB().ExecContext(ctx,
		`update biz_resources set
		  name = if(? <> '', ?, name),
		  followers = ?, total_views = ?, avg_views = ?, video_count = ?,
		  engagement_rate = if(? > 0, ?, engagement_rate),
		  active_30d = ?, active_90d = ?, post_frequency = if(? <> '', ?, post_frequency),
		  platform_user_id = ?, platform_handle = ?, avatar_url = ?, last_sync_status = '成功',
		  last_sync_error = '', last_sync_at = now()
		 where id = ?`,
		user.DisplayName, user.DisplayName, user.FollowerCount, totalViews, avgViews, user.MediaCount, engagementRate, engagementRate,
		active30D, active90D, postFrequency, postFrequency,
		user.ID, user.Username, avatarURL, resourceID,
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"platform":       "Instagram",
		"platformUserId": user.ID,
		"platformHandle": user.Username,
		"name":           user.DisplayName,
		"followers":      user.FollowerCount,
		"totalViews":     totalViews,
		"avgViews":       avgViews,
		"videoCount":     user.MediaCount,
		"engagementRate": engagementRate,
		"active30d":      active30D,
		"active90d":      active90D,
		"postFrequency":  postFrequency,
		"avatarUrl":      avatarURL,
		"syncedPosts":    len(user.Posts),
		"posts":          user.Posts,
		"warnings":       warnings,
		"syncedAt":       time.Now().Format(time.RFC3339),
	}, nil
}

func (a *app) upsertResourcePlatformPosts(ctx context.Context, resourceID int, platform string, posts []platformPost) error {
	for index := range posts {
		post := &posts[index]
		if strings.TrimSpace(post.PlatformPostID) == "" {
			continue
		}
		post.CoverURL = localizeResourceImage(
			ctx,
			resourceID,
			filepath.Join("posts", platform+"_"+post.PlatformPostID),
			post.CoverURL,
		)
		var rawJSON any
		if post.Raw != nil {
			data, err := json.Marshal(post.Raw)
			if err != nil {
				return err
			}
			rawJSON = string(data)
		}
		_, err := a.DB().ExecContext(ctx,
			`insert into biz_resource_platform_posts
			  (resource_id, platform, platform_post_id, title, description, post_url, cover_url,
			   media_type, published_at, duration_seconds, view_count, like_count, comment_count,
			   share_count, raw_json, synced_at)
			 values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, now())
			 on duplicate key update
			   title = values(title),
			   description = values(description),
			   post_url = values(post_url),
			   cover_url = values(cover_url),
			   media_type = values(media_type),
			   published_at = values(published_at),
			   duration_seconds = values(duration_seconds),
			   view_count = values(view_count),
			   like_count = values(like_count),
			   comment_count = values(comment_count),
			   share_count = values(share_count),
			   raw_json = values(raw_json),
			   synced_at = now()`,
			resourceID, platform, post.PlatformPostID, post.Title, post.Description, post.PostURL, post.CoverURL,
			post.MediaType, post.PublishedAt, post.Duration, post.ViewCount, post.LikeCount, post.CommentCount,
			post.ShareCount, rawJSON,
		)
		if err != nil {
			return err
		}
	}
	return a.syncCooperationsFromStoredPostsForResource(ctx, resourceID)
}

func decodePlatformResponse(resp *http.Response, platform string, target any) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		message := strings.TrimSpace(string(body))
		if len(message) > 500 {
			message = message[:500]
		}
		if message == "" {
			message = resp.Status
		}
		return fmt.Errorf("%s API 请求失败：%s", platform, message)
	}
	if len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("%s API 响应解析失败：%w", platform, err)
	}
	return nil
}

func bestThumbnailURL(thumbnails map[string]struct {
	URL string `json:"url"`
}) string {
	for _, key := range []string{"maxres", "standard", "high", "medium", "default"} {
		if item, ok := thumbnails[key]; ok && strings.TrimSpace(item.URL) != "" {
			return item.URL
		}
	}
	for _, item := range thumbnails {
		if strings.TrimSpace(item.URL) != "" {
			return item.URL
		}
	}
	return ""
}

func parsePlatformTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	layouts := []string{time.RFC3339, "2006-01-02T15:04:05-0700", "2006-01-02T15:04:05+0000"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}

func parseYouTubeDurationSeconds(value string) int {
	value = strings.TrimSpace(strings.TrimPrefix(value, "P"))
	if value == "" {
		return 0
	}
	total := 0
	number := strings.Builder{}
	inTime := false
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
			number.WriteRune(r)
		case r == 'T':
			inTime = true
		default:
			n, _ := strconv.Atoi(number.String())
			number.Reset()
			switch r {
			case 'D':
				total += n * 86400
			case 'H':
				total += n * 3600
			case 'M':
				if inTime {
					total += n * 60
				}
			case 'S':
				total += n
			}
		}
	}
	return total
}

func averagePostViews(posts []platformPost) int64 {
	if len(posts) == 0 {
		return 0
	}
	return sumPostViews(posts) / int64(len(posts))
}

func sumPostViews(posts []platformPost) int64 {
	total := int64(0)
	for _, post := range posts {
		total += post.ViewCount
	}
	return total
}

func platformPostEngagementRate(posts []platformPost, followers int64) float64 {
	if followers <= 0 || len(posts) == 0 {
		return 0
	}
	total := int64(0)
	for _, post := range posts {
		total += post.LikeCount + post.CommentCount + post.ShareCount
	}
	return float64(total) / float64(followers) / float64(len(posts))
}

func truncateText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len([]rune(value)) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max])
}

func instagramHandleIdentifier(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "<nil>" {
			continue
		}
		if strings.HasPrefix(value, "@") {
			return sanitizeInstagramHandle(strings.TrimPrefix(value, "@"))
		}
		parsed, err := url.Parse(value)
		if err == nil && parsed.Host != "" {
			host := strings.ToLower(parsed.Host)
			if strings.Contains(host, "instagram.com") {
				segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
				if len(segments) > 0 && segments[0] != "" {
					switch strings.ToLower(segments[0]) {
					case "p", "reel", "reels", "tv", "stories", "explore":
						continue
					default:
						return sanitizeInstagramHandle(segments[0])
					}
				}
			}
			continue
		}
		if !strings.Contains(value, " ") && !strings.Contains(value, "/") {
			return sanitizeInstagramHandle(strings.TrimPrefix(value, "@"))
		}
	}
	return ""
}

func sanitizeInstagramHandle(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	value = strings.TrimPrefix(value, "@")
	builder := strings.Builder{}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func tiktokUserIdentifier(values ...string) (string, string) {
	username := ""
	secUID := ""
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "<nil>" {
			continue
		}
		if strings.Contains(value, "tiktok.com") {
			if parsed, err := url.Parse(value); err == nil {
				for _, segment := range strings.Split(strings.Trim(parsed.Path, "/"), "/") {
					if strings.HasPrefix(segment, "@") {
						username = normalizeTikTokUsername(segment)
						break
					}
				}
			}
			continue
		}
		if strings.HasPrefix(value, "MS4") || strings.Contains(value, "secUid") {
			secUID = value
			continue
		}
		if !strings.Contains(value, " ") && !strings.Contains(value, "/") {
			username = normalizeTikTokUsername(value)
		}
	}
	return username, secUID
}

func normalizeTikTokUsername(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	value = strings.TrimPrefix(value, "@")
	return strings.TrimSpace(value)
}

func firstMapAt(row map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if value := mapAt(row, key); len(value) > 0 {
			return value
		}
	}
	return map[string]any{}
}

func mapAt(row map[string]any, key string) map[string]any {
	if row == nil {
		return map[string]any{}
	}
	value, _ := row[key].(map[string]any)
	if value == nil {
		return map[string]any{}
	}
	return value
}

func firstListAt(row map[string]any, keys ...string) []any {
	for _, key := range keys {
		if row == nil {
			continue
		}
		switch value := row[key].(type) {
		case []any:
			if len(value) > 0 {
				return value
			}
		case map[string]any:
			if nested := firstListAt(value, "edges", "items", "data", "itemList", "aweme_list", "videos", "list"); len(nested) > 0 {
				return nested
			}
		}
	}
	return nil
}

func firstNonZeroInt64(values ...any) int64 {
	for _, value := range values {
		n := anyInt64(value)
		if n != 0 {
			return n
		}
	}
	return 0
}

func nestedInt64(row map[string]any, path ...string) int64 {
	var current any = row
	for _, key := range path {
		next, ok := current.(map[string]any)
		if !ok {
			return 0
		}
		current = next[key]
	}
	return anyInt64(current)
}

func imageURL(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		for _, item := range v {
			if url := imageURL(item); url != "" {
				return url
			}
		}
	case map[string]any:
		for _, key := range []string{"url", "uri", "display_url", "thumbnail_src", "profile_pic_url", "url_list", "candidates", "image_versions2", "video_versions"} {
			if url := imageURL(v[key]); url != "" {
				return url
			}
		}
	}
	return ""
}

func unixPlatformTime(value int64) *time.Time {
	if value <= 0 {
		return nil
	}
	if value > 1000000000000 {
		value = value / 1000
	}
	parsed := time.Unix(value, 0).UTC()
	return &parsed
}

func instagramCaption(item map[string]any) string {
	if captionMap := mapAt(item, "caption"); len(captionMap) > 0 {
		if text := firstNonEmpty(anyString(captionMap["text"]), anyString(captionMap["caption_text"])); text != "" {
			return text
		}
	}
	if caption := firstNonEmpty(anyString(item["caption_text"]), anyString(item["accessibility_caption"])); caption != "" {
		return caption
	}
	edges := firstListAt(mapAt(item, "edge_media_to_caption"), "edges")
	for _, raw := range edges {
		edge, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if text := anyString(mapAt(edge, "node")["text"]); text != "" {
			return text
		}
	}
	return ""
}

func instagramMediaType(item map[string]any) string {
	if anyInt64(item["is_video"]) == 1 || fmt.Sprint(item["is_video"]) == "true" {
		return "VIDEO"
	}
	if value := anyString(item["media_type"]); value != "" {
		return strings.ToUpper(value)
	}
	if typename := anyString(item["__typename"]); strings.Contains(strings.ToLower(typename), "video") {
		return "VIDEO"
	}
	return "IMAGE"
}

func instagramPostURL(username, shortcode string) string {
	if shortcode == "" {
		return ""
	}
	return "https://www.instagram.com/p/" + shortcode + "/"
}

func averageViewedPostViews(posts []platformPost) int64 {
	total := int64(0)
	count := int64(0)
	for _, post := range posts {
		if post.ViewCount <= 0 {
			continue
		}
		total += post.ViewCount
		count++
	}
	if count == 0 {
		return 0
	}
	return total / count
}

func recentPostCounts(posts []platformPost, now time.Time) (int, int) {
	active30D := 0
	active90D := 0
	cutoff30D := now.AddDate(0, 0, -30)
	cutoff90D := now.AddDate(0, 0, -90)
	for _, post := range posts {
		if post.PublishedAt == nil {
			continue
		}
		if !post.PublishedAt.Before(cutoff90D) {
			active90D++
		}
		if !post.PublishedAt.Before(cutoff30D) {
			active30D++
		}
	}
	return active30D, active90D
}

func (a *app) markResourceSyncFailed(ctx context.Context, id int, message string) {
	_, _ = a.DB().ExecContext(ctx,
		`update biz_resources set last_sync_status = '失败', last_sync_error = ?, last_sync_at = now() where id = ?`,
		redactSensitiveText(message), id,
	)
}

func (a *app) businessAssistantRecommend(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	demandText := str(body, "demandText")
	if demandText == "" {
		writeError(w, http.StatusOK, 10001, "项目需求不能为空")
		return
	}
	parsed := parseAssistantDemand(body, demandText)
	includeWatch := boolField(body, "includeWatch")
	budget := floatField(body, "budget")
	if budget <= 0 {
		budget = floatField(parsed, "budget")
	}

	rows, err := a.queryMaps(r.Context(),
		`select id, name, avatar_url as avatarUrl, resource_type as resourceType, country, language, platform,
		        industry, category, content_types as contentTypes, status, followers,
		        engagement_rate as engagementRate, avg_views as avgViews, score, level,
		        risk_level as riskLevel, owner, notes
		   from biz_resources
		  where status not in ('黑名单', '暂停合作', '已归档', '待补全')
		  order by score desc, updated_at desc
		  limit 300`,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}

	recommendations, filtered := buildLocalRecommendations(rows, parsed, includeWatch, budget)
	fallback := true
	message := "当前使用本地规则推荐；AI 模型可在 AI 模型配置中接入。"
	if model, ok := a.assistantAIModel(r.Context()); ok {
		result, err := a.requestAIRecommendations(r.Context(), model, demandText, parsed, recommendations)
		if err != nil {
			message = "模型调用失败，已回退本地规则推荐：" + err.Error()
		} else if len(result.Recommendations) > 0 {
			recommendations = mergeAIRecommendations(recommendations, result.Recommendations)
			if len(result.Parsed) > 0 {
				parsed = mergeParsedDemand(parsed, result.Parsed)
			}
			fallback = false
			message = defaultString(result.Summary, fmt.Sprintf("已调用 %s/%s 生成达人推荐。", model.Provider, model.Model))
		}
	}
	writeOK(w, map[string]any{
		"parsed":          parsed,
		"recommendations": recommendations,
		"filteredSummary": filtered,
		"fallback":        fallback,
		"message":         message,
	})
}

func buildLocalRecommendations(rows []map[string]any, parsed map[string]any, includeWatch bool, budget float64) ([]map[string]any, map[string]int) {
	var recommendations []map[string]any
	filtered := map[string]int{}
	for _, row := range rows {
		status := fmt.Sprint(row["status"])
		if status == "观察中" && !includeWatch {
			filtered["观察中默认不推荐"]++
			continue
		}
		if levelRank(fmt.Sprint(row["level"])) > levelRank("B") {
			filtered["低于最低等级"]++
			continue
		}
		score, reasons, downgrades := recommendationScore(row, parsed)
		if score < 20 {
			filtered["匹配度不足"]++
			continue
		}
		estimatedCost := estimateResourceCost(row)
		if budget > 0 && float64(estimatedCost) > budget {
			filtered["预算超限"]++
			continue
		}
		priority := recommendationPriority(score)
		recommendations = append(recommendations, map[string]any{
			"id":             row["id"],
			"name":           row["name"],
			"avatarUrl":      row["avatarUrl"],
			"resourceType":   row["resourceType"],
			"country":        row["country"],
			"language":       row["language"],
			"platform":       row["platform"],
			"industry":       row["industry"],
			"category":       row["category"],
			"contentTypes":   row["contentTypes"],
			"status":         row["status"],
			"level":          row["level"],
			"score":          row["score"],
			"riskLevel":      row["riskLevel"],
			"followers":      row["followers"],
			"engagementRate": row["engagementRate"],
			"avgViews":       row["avgViews"],
			"owner":          row["owner"],
			"notes":          row["notes"],
			"estimatedCost":  estimatedCost,
			"matchScore":     score,
			"priority":       priority,
			"reason":         strings.Join(reasons, "；"),
			"hitRules":       reasons,
			"downReasons":    downgrades,
			"riskTip":        riskTip(row),
		})
	}
	sortRecommendationRows(recommendations)
	if len(recommendations) > 20 {
		recommendations = recommendations[:20]
	}
	return recommendations, filtered
}

func recommendationPriority(score int) string {
	if score >= 75 {
		return "高"
	}
	if score >= 45 {
		return "中"
	}
	return "低"
}

func parseAssistantDemand(body map[string]any, demandText string) map[string]any {
	text := strings.ToLower(demandText)
	parsed := map[string]any{
		"targetMarket": firstNonEmpty(str(body, "targetMarket"), detectMarket(demandText)),
		"language":     firstNonEmpty(str(body, "language"), detectLanguage(demandText)),
		"platform":     firstNonEmpty(str(body, "platform"), detectPlatform(demandText)),
		"resourceType": firstNonEmpty(str(body, "resourceType"), detectResourceType(demandText)),
		"industry":     detectIndustry(demandText),
		"goal":         detectGoal(demandText),
		"budget":       floatField(body, "budget"),
	}
	if floatField(parsed, "budget") <= 0 {
		parsed["budget"] = detectBudget(text)
	}
	return parsed
}

func detectMarket(text string) string {
	rules := map[string][]string{
		"美国":   {"美国", "北美", "united states", "usa", " u.s.", " us "},
		"德国":   {"德国", "germany"},
		"日本":   {"日本", "japan"},
		"英国":   {"英国", "uk", "united kingdom"},
		"欧洲":   {"欧洲", "europe"},
		"中东北非": {"中东北非", "中东", "北非", "mena", "middle east", "north africa"},
		"东非":   {"东非", "east africa"},
		"西非":   {"西非", "west africa"},
		"东南亚":  {"东南亚", "sea", "southeast asia", "asean"},
		"拉美":   {"拉美", "拉丁美洲", "latin america", "latam"},
	}
	return detectByKeywords(text, rules)
}

func detectLanguage(text string) string {
	rules := map[string][]string{
		"英语": {"英语", "英文", "english"},
		"德语": {"德语", "german"},
		"日语": {"日语", "japanese"},
		"中文": {"中文", "chinese"},
	}
	return detectByKeywords(text, rules)
}

func detectPlatform(text string) string {
	rules := map[string][]string{
		"YouTube":   {"youtube", "油管"},
		"TikTok":    {"tiktok", "抖音"},
		"Instagram": {"instagram", "ins"},
		"媒体网站":      {"媒体", "media", "网站", "报道"},
	}
	return detectByKeywords(text, rules)
}

func detectResourceType(text string) string {
	rules := map[string][]string{
		"媒体":  {"媒体", "media", "报道"},
		"创作者": {"创作者", "creator", "youtube", "tiktok", "评测"},
		"KOL": {"kol", "达人", "博主"},
	}
	return detectByKeywords(text, rules)
}

func detectIndustry(text string) string {
	rules := map[string][]string{
		"AI/科技/消费电子": {"ai", "人工智能", "科技", "录音笔", "硬件", "消费电子", "gadget"},
		"游戏":         {"游戏", "game", "gaming"},
		"汽车":         {"汽车", "car", "auto"},
		"美妆":         {"美妆", "beauty"},
	}
	return detectByKeywords(text, rules)
}

func detectGoal(text string) string {
	rules := map[string][]string{
		"新品曝光/点击转化": {"曝光", "点击", "转化", "新品", "conversion", "click"},
		"品牌背书":      {"背书", "品牌", "pr", "发布"},
		"下载/注册":     {"下载", "注册", "download", "signup"},
	}
	return detectByKeywords(text, rules)
}

func detectByKeywords(text string, rules map[string][]string) string {
	lower := strings.ToLower(" " + text + " ")
	for value, keywords := range rules {
		for _, keyword := range keywords {
			if strings.Contains(lower, strings.ToLower(keyword)) {
				return value
			}
		}
	}
	return ""
}

func detectBudget(text string) float64 {
	cleaned := strings.ReplaceAll(text, ",", "")
	fields := strings.Fields(cleaned)
	for _, field := range fields {
		field = strings.Trim(field, "$美元usd预算")
		var n float64
		if _, err := fmt.Sscan(field, &n); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

func recommendationScore(row, parsed map[string]any) (int, []string, []string) {
	score := intField(row, "score") / 2
	var reasons []string
	var downgrades []string
	score += matchField(row, parsed, "country", "targetMarket", 18, "市场匹配", &reasons)
	score += matchField(row, parsed, "language", "language", 12, "语言匹配", &reasons)
	score += matchField(row, parsed, "platform", "platform", 14, "平台匹配", &reasons)
	score += matchText(row, parsed, "industry", 18, "行业/内容方向匹配", &reasons)
	score += matchText(row, parsed, "resourceType", 8, "资源类型匹配", &reasons)
	if intField(row, "avgViews") > 0 || intField(row, "followers") > 0 {
		score += 6
		reasons = append(reasons, "有平台表现数据")
	}
	switch fmt.Sprint(row["riskLevel"]) {
	case "高":
		score -= 20
		downgrades = append(downgrades, "高风险资源降权")
	case "中":
		score -= 8
		downgrades = append(downgrades, "中风险资源降权")
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "综合评分较高")
	}
	return score, reasons, downgrades
}

func matchField(row, parsed map[string]any, rowKey, parsedKey string, points int, reason string, reasons *[]string) int {
	want := strings.TrimSpace(fmt.Sprint(parsed[parsedKey]))
	have := strings.TrimSpace(fmt.Sprint(row[rowKey]))
	if want == "" || have == "" || have == "<nil>" {
		return 0
	}
	if strings.Contains(strings.ToLower(have), strings.ToLower(want)) || strings.Contains(strings.ToLower(want), strings.ToLower(have)) {
		*reasons = append(*reasons, reason)
		return points
	}
	return 0
}

func matchText(row, parsed map[string]any, rowKey string, points int, reason string, reasons *[]string) int {
	want := strings.TrimSpace(fmt.Sprint(parsed[rowKey]))
	if want == "" || want == "<nil>" {
		return 0
	}
	have := strings.Join([]string{
		fmt.Sprint(row["industry"]),
		fmt.Sprint(row["category"]),
		fmt.Sprint(row["contentTypes"]),
		fmt.Sprint(row["resourceType"]),
		fmt.Sprint(row["name"]),
	}, " ")
	for _, token := range strings.FieldsFunc(strings.ToLower(want), func(r rune) bool {
		return r == '/' || r == ',' || r == '，' || r == ' ' || r == '|'
	}) {
		if token != "" && strings.Contains(strings.ToLower(have), token) {
			*reasons = append(*reasons, reason)
			return points
		}
	}
	return 0
}

func estimateResourceCost(row map[string]any) int {
	followers := intField(row, "followers")
	avgViews := intField(row, "avgViews")
	base := 1000
	if avgViews > 0 {
		base += avgViews / 20
	} else {
		base += followers / 250
	}
	switch fmt.Sprint(row["level"]) {
	case "S":
		base += 4000
	case "A":
		base += 2500
	case "B":
		base += 1200
	}
	if base < 500 {
		base = 500
	}
	return base
}

func riskTip(row map[string]any) string {
	switch fmt.Sprint(row["riskLevel"]) {
	case "高":
		return "高风险资源，建议复核历史合作和数据来源"
	case "中":
		return "存在一定风险，建议确认报价、交付和数据可信度"
	default:
		return "暂无明显风险"
	}
}

func levelRank(level string) int {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "S":
		return 1
	case "A":
		return 2
	case "B":
		return 3
	case "C":
		return 4
	case "D":
		return 5
	default:
		return 9
	}
}

func sortRecommendationRows(rows []map[string]any) {
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if intField(rows[j], "matchScore") > intField(rows[i], "matchScore") {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
}

type assistantAIModel struct {
	Provider                   string
	Model                      string
	BaseURL                    string
	APIKey                     string
	EnableDemandParsing        bool
	EnableRecommendationReason bool
	Timeout                    time.Duration
	Temperature                float64
}

type aiRecommendationResult struct {
	Parsed          map[string]any   `json:"parsed"`
	Recommendations []map[string]any `json:"recommendations"`
	Summary         string           `json:"summary"`
}

func (a *app) assistantAIModel(ctx context.Context) (assistantAIModel, bool) {
	var enabled int
	var content string
	err := a.DB().QueryRowContext(ctx,
		`select enabled, content
		   from biz_governance_rules
		  where rule_type = 'ai_model'
		  limit 1`,
	).Scan(&enabled, &content)
	if err != nil || enabled != 1 {
		return assistantAIModel{}, false
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return assistantAIModel{}, false
	}
	model := assistantAIModel{
		Provider:                   str(data, "provider"),
		Model:                      str(data, "model"),
		BaseURL:                    strings.TrimRight(str(data, "baseUrl"), "/"),
		APIKey:                     strings.TrimSpace(a.Config().AIModel.APIKey),
		EnableDemandParsing:        true,
		EnableRecommendationReason: true,
		Timeout:                    30 * time.Second,
		Temperature:                0.2,
	}
	if _, ok := data["enableDemandParsing"]; ok {
		model.EnableDemandParsing = boolField(data, "enableDemandParsing")
	}
	if _, ok := data["enableRecommendationReason"]; ok {
		model.EnableRecommendationReason = boolField(data, "enableRecommendationReason")
	}
	if seconds := intField(data, "timeoutSeconds"); seconds > 0 {
		model.Timeout = time.Duration(seconds) * time.Second
	}
	if model.Timeout > 120*time.Second {
		model.Timeout = 120 * time.Second
	}
	if _, ok := data["temperature"]; ok {
		if temperature := floatField(data, "temperature"); temperature >= 0 {
			model.Temperature = temperature
		}
	}
	if model.Provider == "" || model.Model == "" || model.BaseURL == "" || model.APIKey == "" {
		return assistantAIModel{}, false
	}
	return model, true
}

func (a *app) requestAIRecommendations(ctx context.Context, model assistantAIModel, demandText string, parsed map[string]any, candidates []map[string]any) (aiRecommendationResult, error) {
	if len(candidates) == 0 {
		return aiRecommendationResult{}, nil
	}
	endpoint := model.BaseURL
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}
	userPayload := map[string]any{
		"demandText":      demandText,
		"parsedByRules":   parsed,
		"candidateKOLs":   candidates,
		"maxReturnCount":  12,
		"outputLanguage":  "zh-CN",
		"enabledFeatures": map[string]bool{"demandParsing": model.EnableDemandParsing, "recommendationReason": model.EnableRecommendationReason},
		"selectionPolicy": "只能从 candidateKOLs 中推荐达人，禁止新增、杜撰或修改资源 id。",
		"businessContext": "全球 KOL/媒体资源库，用于新品推广、内容测评、传播曝光和转化项目的达人推荐。",
		"riskRequirement": "必须考虑风险等级、预算、市场、语言、平台、内容方向、历史评分和可解释性。",
	}
	userContent, _ := json.Marshal(userPayload)
	requestBody, _ := json.Marshal(map[string]any{
		"model": model.Model,
		"messages": []map[string]string{
			{"role": "system", "content": assistantRecommendationSystemPrompt()},
			{"role": "user", "content": string(userContent)},
		},
		"temperature": model.Temperature,
		"max_tokens":  2400,
		"response_format": map[string]string{
			"type": "json_object",
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return aiRecommendationResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+model.APIKey)
	resp, err := (&http.Client{Timeout: model.Timeout}).Do(req)
	if err != nil {
		return aiRecommendationResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		message := strings.TrimSpace(string(bodyBytes))
		if message == "" {
			message = resp.Status
		}
		return aiRecommendationResult{}, fmt.Errorf("模型服务返回异常：%s", message)
	}
	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return aiRecommendationResult{}, err
	}
	if len(completion.Choices) == 0 {
		return aiRecommendationResult{}, fmt.Errorf("模型未返回推荐内容")
	}
	content := cleanJSONContent(completion.Choices[0].Message.Content)
	var result aiRecommendationResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return aiRecommendationResult{}, fmt.Errorf("模型推荐 JSON 解析失败：%w", err)
	}
	return result, nil
}

func assistantRecommendationSystemPrompt() string {
	return strings.TrimSpace(`你是全球 KOL/达人资源推荐助手，负责根据品牌项目需求，从候选资源池中选择最适合的达人、媒体或创作者。

工作目标：
1. 解析用户自然语言需求，补全目标市场、语言、平台、资源类型、行业方向、投放目标和预算等字段。
2. 只允许从用户提供的 candidateKOLs 中选择推荐对象，严禁编造不存在的达人、媒体、账号、数据或资源 id。
3. 根据市场/语言/平台/内容方向/资源类型/粉丝或均观看/内部评分/等级/风险/预估成本综合排序。
4. 高风险资源只能在理由充分时保留，并必须写明 riskTip；预算明显不匹配、市场平台不匹配或风险过高的资源要降权。
5. 推荐理由要面向运营人员，具体说明“为什么适合本项目”，避免空泛词。

输出要求：
只返回严格 JSON，不要 Markdown，不要解释性前后缀。JSON 结构必须为：
{
  "parsed": {
    "targetMarket": "string",
    "language": "string",
    "platform": "string",
    "resourceType": "string",
    "industry": "string",
    "goal": "string",
    "budget": number,
    "mustHave": ["string"],
    "avoid": ["string"]
  },
  "recommendations": [
    {
      "id": number,
      "matchScore": number,
      "priority": "高|中|低",
      "reason": "不超过80字的推荐理由",
      "hitRules": ["具体命中的匹配点"],
      "downReasons": ["降权原因，没有则空数组"],
      "riskTip": "风险提示，没有明显风险也要说明暂无明显风险"
    }
  ],
  "summary": "一句话总结推荐策略"
}

字段规则：
- recommendations 最多返回 12 条，按 matchScore 从高到低排序。
- id 必须来自 candidateKOLs。
- matchScore 必须是 0 到 100 的整数。
- priority 按匹配度和可执行性综合判断，只能是 高、中、低。
- 如果无法判断某个 parsed 字段，返回空字符串或 0，不要猜测事实。`)
}

func cleanJSONContent(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	return strings.TrimSpace(content)
}

func mergeAIRecommendations(localRows []map[string]any, aiRows []map[string]any) []map[string]any {
	byID := map[string]map[string]any{}
	for _, row := range localRows {
		byID[fmt.Sprint(row["id"])] = cloneMap(row)
	}
	used := map[string]bool{}
	merged := make([]map[string]any, 0, len(localRows))
	for _, aiRow := range aiRows {
		id := fmt.Sprint(aiRow["id"])
		local, ok := byID[id]
		if !ok || used[id] {
			continue
		}
		if score := intField(aiRow, "matchScore"); score > 0 {
			local["matchScore"] = clampInt(score, 0, 100)
		}
		if priority := str(aiRow, "priority"); priority == "高" || priority == "中" || priority == "低" {
			local["priority"] = priority
		} else {
			local["priority"] = recommendationPriority(intField(local, "matchScore"))
		}
		if reason := str(aiRow, "reason"); reason != "" && reason != "<nil>" {
			local["reason"] = reason
		}
		if hitRules := stringSliceField(aiRow, "hitRules"); hitRules != nil {
			local["hitRules"] = hitRules
		}
		if downReasons := stringSliceField(aiRow, "downReasons"); downReasons != nil {
			local["downReasons"] = downReasons
		}
		if risk := str(aiRow, "riskTip"); risk != "" && risk != "<nil>" {
			local["riskTip"] = risk
		}
		used[id] = true
		merged = append(merged, local)
	}
	for _, row := range localRows {
		if len(merged) >= 20 {
			break
		}
		id := fmt.Sprint(row["id"])
		if !used[id] {
			merged = append(merged, row)
		}
	}
	return merged
}

func mergeParsedDemand(local map[string]any, ai map[string]any) map[string]any {
	merged := cloneMap(local)
	for key, value := range ai {
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" || text == "<nil>" || text == "0" {
			continue
		}
		merged[key] = value
	}
	return merged
}

func cloneMap(row map[string]any) map[string]any {
	next := make(map[string]any, len(row))
	for key, value := range row {
		next[key] = value
	}
	return next
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func stringSliceField(body map[string]any, key string) []string {
	raw, ok := body[key]
	if !ok || raw == nil {
		return nil
	}
	switch values := raw.(type) {
	case []string:
		return values
	case []any:
		result := make([]string, 0, len(values))
		for _, value := range values {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				result = append(result, text)
			}
		}
		return result
	case string:
		if strings.TrimSpace(values) == "" {
			return nil
		}
		return []string{strings.TrimSpace(values)}
	default:
		return nil
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func youtubeChannelIdentifier(name, platformURL string) (string, string, error) {
	raw := strings.TrimSpace(platformURL)
	if raw == "" || raw == "<nil>" {
		raw = strings.TrimSpace(name)
	}
	if raw == "" {
		return "", "", fmt.Errorf("请先填写 YouTube 频道链接、频道 ID 或 @handle")
	}
	if strings.HasPrefix(raw, "UC") && !strings.Contains(raw, "/") {
		return "id", raw, nil
	}
	if strings.HasPrefix(raw, "@") {
		return "forHandle", raw, nil
	}
	parsed, err := url.Parse(raw)
	if err == nil && parsed.Host != "" {
		segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		for i, segment := range segments {
			if segment == "channel" && i+1 < len(segments) {
				return "id", segments[i+1], nil
			}
			if strings.HasPrefix(segment, "@") {
				return "forHandle", segment, nil
			}
		}
	}
	return "", "", fmt.Errorf("暂只支持 YouTube 频道 ID 或 @handle，请填写 /channel/UC... 或 /@handle 链接")
}

func parseCount(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	n, _ := strconv.ParseInt(value, 10, 64)
	return n
}

func (a *app) businessTags(w http.ResponseWriter, r *http.Request) {
	rows, err := a.queryMaps(r.Context(), `select id, name, category, color, status from biz_tags order by category, id`)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, rows)
}

func (a *app) createBusinessTag(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	_, err := a.DB().ExecContext(r.Context(),
		`insert into biz_tags (name, category, color, status) values (?, ?, ?, ?)`,
		str(body, "name"), str(body, "category"), defaultString(str(body, "color"), "#409EFF"),
		defaultString(str(body, "status"), "启用"),
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"created": true})
}

func (a *app) businessProjects(w http.ResponseWriter, r *http.Request) {
	rows, err := a.queryMaps(r.Context(),
		`select id, name, target_market as targetMarket, language, platform, campaign_type as campaignType,
		        budget, currency, status, owner, brief,
		        date_format(cycle_start_date, '%Y-%m-%d') as cycleStartDate,
		        date_format(cycle_end_date, '%Y-%m-%d') as cycleEndDate,
		        date_format(report_update_date, '%Y-%m-%d') as reportUpdateDate,
		        cast(unix_timestamp(paused_at) * 1000 as unsigned) as pausedAt,
		        cast(unix_timestamp(created_at) * 1000 as unsigned) as createdAt
		   from biz_projects order by created_at desc`,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeTable(w, rows)
}

func (a *app) createBusinessProject(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	_, err := a.DB().ExecContext(r.Context(),
		`insert into biz_projects
		 (name, target_market, language, platform, campaign_type, budget, currency, status, owner, brief,
		  cycle_start_date, cycle_end_date, report_update_date)
		 values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		str(body, "name"), str(body, "targetMarket"), str(body, "language"), str(body, "platform"),
		str(body, "campaignType"), floatField(body, "budget"), defaultString(str(body, "currency"), "USD"),
		defaultString(str(body, "status"), "需求创建"), str(body, "owner"), str(body, "brief"),
		nullableDate(str(body, "cycleStartDate")), nullableDate(str(body, "cycleEndDate")),
		nullableDate(str(body, "reportUpdateDate")),
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"created": true})
}

func (a *app) updateBusinessProject(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	id := intField(body, "id")
	if id == 0 {
		writeError(w, http.StatusOK, 10001, "项目 id 不能为空")
		return
	}
	_, err := a.DB().ExecContext(r.Context(),
		`update biz_projects set
		  name = ?, target_market = ?, language = ?, platform = ?, campaign_type = ?,
		  budget = ?, currency = ?, status = ?, owner = ?, brief = ?,
		  cycle_start_date = ?, cycle_end_date = ?, report_update_date = ?
		 where id = ?`,
		str(body, "name"), str(body, "targetMarket"), str(body, "language"), str(body, "platform"),
		str(body, "campaignType"), floatField(body, "budget"), defaultString(str(body, "currency"), "USD"),
		defaultString(str(body, "status"), "需求创建"), str(body, "owner"), str(body, "brief"),
		nullableDate(str(body, "cycleStartDate")), nullableDate(str(body, "cycleEndDate")),
		nullableDate(str(body, "reportUpdateDate")), id,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"updated": true})
}

func (a *app) createBusinessProjectResource(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	projectID := intField(body, "projectId")
	resourceID := intField(body, "resourceId")
	if projectID == 0 || resourceID == 0 {
		writeError(w, http.StatusOK, 10001, "项目和资源不能为空")
		return
	}
	status := defaultString(str(body, "status"), "候选")
	reason := str(body, "reason")
	_, err := a.DB().ExecContext(r.Context(),
		`insert into biz_project_resources
		 (project_id, resource_id, status, source, recommend_reason, priority, estimated_cost, risk_tip)
		 values (?, ?, ?, ?, ?, ?, ?, ?)
		 on duplicate key update
		   status = values(status),
		   source = values(source),
		   recommend_reason = values(recommend_reason),
		   priority = values(priority),
		   estimated_cost = values(estimated_cost),
		   risk_tip = values(risk_tip),
		   updated_at = now()`,
		projectID, resourceID, status, defaultString(str(body, "source"), "智能助手"),
		reason, str(body, "priority"), floatField(body, "estimatedCost"), str(body, "riskTip"),
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"created": true})
}

func (a *app) businessMarketOptions(w http.ResponseWriter, r *http.Request) {
	if err := a.ensureBusinessMarketOptions(r.Context()); err != nil {
		writeDBError(w, err)
		return
	}
	rows, err := a.queryMaps(r.Context(),
		`select id, name, region_group as regionGroup, status, source, sort_order as sortOrder
		   from biz_market_options
		  where status = '启用'
		  order by sort_order asc, name asc`,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, rows)
}

func (a *app) createBusinessMarketOption(w http.ResponseWriter, r *http.Request) {
	if err := a.ensureBusinessMarketOptions(r.Context()); err != nil {
		writeDBError(w, err)
		return
	}
	body := readBody(r)
	name := strings.TrimSpace(str(body, "name"))
	if name == "" {
		writeError(w, http.StatusOK, 10001, "市场名称不能为空")
		return
	}
	sortOrder := intField(body, "sortOrder")
	if sortOrder <= 0 {
		sortOrder = 200
	}
	_, err := a.DB().ExecContext(r.Context(),
		`insert into biz_market_options
		 (name, region_group, status, source, sort_order)
		 values (?, ?, '启用', '用户新增', ?)
		 on duplicate key update
		   region_group = if(values(region_group) = '', region_group, values(region_group)),
		   status = '启用',
		   updated_at = now()`,
		name, strings.TrimSpace(str(body, "regionGroup")), sortOrder,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"created": true, "name": name})
}

func (a *app) deleteBusinessMarketOption(w http.ResponseWriter, r *http.Request) {
	if err := a.ensureBusinessMarketOptions(r.Context()); err != nil {
		writeDBError(w, err)
		return
	}
	body := readBody(r)
	name := strings.TrimSpace(str(body, "name"))
	if name == "" {
		writeError(w, http.StatusOK, 10001, "市场名称不能为空")
		return
	}
	result, err := a.DB().ExecContext(r.Context(),
		`update biz_market_options
		    set status = '停用', updated_at = now()
		  where name = ?`,
		name,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	affected, _ := result.RowsAffected()
	writeOK(w, map[string]any{"deleted": affected > 0, "name": name})
}

func (a *app) ensureBusinessMarketOptions(ctx context.Context) error {
	_, err := a.DB().ExecContext(ctx,
		`create table if not exists biz_market_options (
		  id bigint primary key auto_increment,
		  name varchar(128) not null,
		  region_group varchar(64) not null default '',
		  status varchar(16) not null default '启用',
		  source varchar(32) not null default '系统预置',
		  sort_order int not null default 100,
		  created_at datetime not null default current_timestamp,
		  updated_at datetime not null default current_timestamp on update current_timestamp,
		  unique key uk_biz_market_options_name (name),
		  index idx_biz_market_options_status_sort (status, sort_order)
		)`,
	)
	if err != nil {
		return err
	}
	defaults := []struct {
		name        string
		regionGroup string
		sortOrder   int
	}{
		{"美国", "欧美", 10},
		{"英国", "欧美", 20},
		{"欧洲", "欧美", 30},
		{"德国", "欧美", 40},
		{"日本", "亚太", 50},
		{"中东北非", "MENA", 60},
		{"东非", "非洲", 70},
		{"西非", "非洲", 80},
		{"东南亚", "亚太", 90},
		{"拉美", "拉美", 100},
	}
	for _, item := range defaults {
		_, err := a.DB().ExecContext(ctx,
			`insert into biz_market_options
			 (name, region_group, status, source, sort_order)
			 values (?, ?, '启用', '系统预置', ?)
			 on duplicate key update
			   region_group = values(region_group),
			   sort_order = values(sort_order),
			   updated_at = now()`,
			item.name, item.regionGroup, item.sortOrder,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *app) businessCooperations(w http.ResponseWriter, r *http.Request) {
	rows, err := a.queryMaps(r.Context(),
		`select c.id, c.project_id as projectId, p.name as projectName, c.resource_id as resourceId,
		        r.name as resourceName, r.avatar_url as resourceAvatarUrl, r.platform_handle as platformHandle,
		        r.platform_url as platformUrl, r.country, r.language, r.platform,
		        c.cooperation_type as cooperationType, c.audience_segment as audienceSegment,
		        c.creative_name as creativeName, c.quote_amount as quoteAmount,
		        c.currency, c.status, c.deliverable_status as deliverableStatus,
		        c.impressions, c.views, c.clicks, c.conversions, c.engagement_count as engagementCount,
		        c.comments_count as commentsCount, c.roi, c.team_rating as teamRating,
		        c.release_date as releaseDate, c.deliverable_links as deliverableLinks,
		        c.final_link as finalLink, c.top_geographies as topGeographies,
		        date_format(c.publish_time, '%Y-%m-%d %H:%i:%s') as publishTime,
		        c.tracking_link as trackingLink, c.ad_authorization_code as adAuthorizationCode,
		        c.import_batch_id as importBatchId, c.notes,
		        cast(unix_timestamp(c.updated_at) * 1000 as unsigned) as updatedAt
		   from biz_cooperations c
		   left join biz_projects p on p.id = c.project_id
		   left join biz_resources r on r.id = c.resource_id
		  order by c.updated_at desc`,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeTable(w, rows)
}

func (a *app) createBusinessCooperation(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	result, err := a.DB().ExecContext(r.Context(),
		`insert into biz_cooperations
		 (project_id, resource_id, cooperation_type, audience_segment, creative_name, quote_amount, currency, status,
		  deliverable_status, impressions, views, clicks, conversions, engagement_count,
		  comments_count, roi, team_rating, release_date, deliverable_links, final_link, top_geographies,
		  publish_time, tracking_link, ad_authorization_code, import_batch_id, notes)
		 values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		intField(body, "projectId"), intField(body, "resourceId"), str(body, "cooperationType"),
		str(body, "audienceSegment"), str(body, "creativeName"),
		floatField(body, "quoteAmount"), defaultString(str(body, "currency"), "USD"),
		defaultString(str(body, "status"), "邀约中"), defaultString(str(body, "deliverableStatus"), "未开始"),
		intField(body, "impressions"), intField(body, "views"), intField(body, "clicks"),
		intField(body, "conversions"), intField(body, "engagementCount"), intField(body, "commentsCount"),
		floatField(body, "roi"), intField(body, "teamRating"), nullableDate(str(body, "releaseDate")),
		str(body, "deliverableLinks"), str(body, "finalLink"), str(body, "topGeographies"),
		nullableTime(str(body, "publishTime")), str(body, "trackingLink"), str(body, "adAuthorizationCode"),
		str(body, "importBatchId"), str(body, "notes"),
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		writeDBError(w, err)
		return
	}
	syncResult, err := a.syncCooperationPost(r.Context(), int(id), true)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"created": true, "id": id, "postSync": syncResult})
}

func (a *app) updateBusinessCooperation(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	id := intField(body, "id")
	if id == 0 {
		writeError(w, http.StatusOK, 10001, "合作记录 id 不能为空")
		return
	}
	_, err := a.DB().ExecContext(r.Context(),
		`update biz_cooperations set
		  project_id = ?, resource_id = ?, cooperation_type = ?, audience_segment = ?, creative_name = ?,
		  quote_amount = ?, currency = ?,
		  status = ?, deliverable_status = ?, impressions = ?, views = ?, clicks = ?,
		  conversions = ?, engagement_count = ?, comments_count = ?, roi = ?, team_rating = ?,
		  release_date = ?, deliverable_links = ?, final_link = ?, top_geographies = ?, publish_time = ?,
		  tracking_link = ?, ad_authorization_code = ?, notes = ?
		 where id = ?`,
		intField(body, "projectId"), intField(body, "resourceId"), str(body, "cooperationType"),
		str(body, "audienceSegment"), str(body, "creativeName"),
		floatField(body, "quoteAmount"), defaultString(str(body, "currency"), "USD"),
		defaultString(str(body, "status"), "邀约中"), defaultString(str(body, "deliverableStatus"), "未开始"),
		intField(body, "impressions"), intField(body, "views"), intField(body, "clicks"),
		intField(body, "conversions"), intField(body, "engagementCount"), intField(body, "commentsCount"),
		floatField(body, "roi"), intField(body, "teamRating"), nullableDate(str(body, "releaseDate")),
		str(body, "deliverableLinks"), str(body, "finalLink"), str(body, "topGeographies"),
		nullableTime(str(body, "publishTime")), str(body, "trackingLink"), str(body, "adAuthorizationCode"),
		str(body, "notes"), id,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	syncResult, err := a.syncCooperationPost(r.Context(), id, true)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"updated": true, "postSync": syncResult})
}

func (a *app) syncBusinessCooperation(w http.ResponseWriter, r *http.Request) {
	id := intField(readBody(r), "id")
	if id == 0 {
		writeError(w, http.StatusOK, 10001, "合作记录 id 不能为空")
		return
	}
	result, err := a.syncCooperationPost(r.Context(), id, true)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, result)
}

func (a *app) importBusinessCooperations(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	projectID := intField(body, "projectId")
	if projectID == 0 {
		writeError(w, http.StatusOK, 10001, "项目 id 不能为空")
		return
	}
	rows, ok := body["rows"].([]any)
	if !ok || len(rows) == 0 {
		writeError(w, http.StatusOK, 10001, "导入数据不能为空")
		return
	}

	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()

	batchID := fmt.Sprintf("IMP%s", time.Now().Format("20060102150405"))
	imported := 0
	createdResources := 0
	var errors []map[string]any
	for index, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			errors = append(errors, map[string]any{"row": index + 2, "message": "行数据格式错误"})
			continue
		}
		influencer := strings.TrimSpace(fmt.Sprint(row["influencer"]))
		if influencer == "" || influencer == "<nil>" {
			errors = append(errors, map[string]any{"row": index + 2, "message": "姓名/Influencer 不能为空"})
			continue
		}
		resourceID, created, err := upsertImportResource(r.Context(), tx, row)
		if err != nil {
			errors = append(errors, map[string]any{"row": index + 2, "message": err.Error()})
			continue
		}
		if created {
			createdResources++
		}
		if err := insertImportCooperation(r.Context(), tx, projectID, resourceID, batchID, row); err != nil {
			errors = append(errors, map[string]any{"row": index + 2, "message": err.Error()})
			continue
		}
		imported++
	}

	if imported == 0 {
		writeError(w, http.StatusOK, 10002, "没有可导入的有效数据")
		return
	}
	if err := tx.Commit(); err != nil {
		writeDBError(w, err)
		return
	}
	synced, syncWarnings := a.syncImportedCooperations(r.Context(), batchID)
	writeOK(w, map[string]any{
		"batchId":          batchID,
		"imported":         imported,
		"failed":           len(errors),
		"createdResources": createdResources,
		"syncedPosts":      synced,
		"syncWarnings":     syncWarnings,
		"errors":           errors,
	})
}

func upsertImportResource(ctx context.Context, tx *sql.Tx, row map[string]any) (int64, bool, error) {
	name := strings.TrimSpace(fmt.Sprint(row["influencer"]))
	category := strings.TrimSpace(fmt.Sprint(row["category"]))
	platform := strings.TrimSpace(fmt.Sprint(row["platform"]))
	followers := intField(row, "followerNumber")
	views := intField(row, "views")
	score, level, hasRating := ratingScoreLevel(strings.TrimSpace(fmt.Sprint(row["rating"])))

	var id int64
	var query string
	var args []any
	if platform == "" || platform == "<nil>" {
		query = `select id from biz_resources where name = ? limit 1`
		args = []any{name}
	} else {
		query = `select id from biz_resources where name = ? and platform = ? limit 1`
		args = []any{name, platform}
	}
	err := tx.QueryRowContext(ctx, query, args...).Scan(&id)
	if err == nil {
		if hasRating {
			_, err = tx.ExecContext(ctx,
				`update biz_resources set industry = ?, category = ?, platform = ?, followers = ?,
				 avg_views = ?, score = ?, level = ? where id = ?`,
				category, category, platform, followers, views, score, level, id,
			)
		} else {
			_, err = tx.ExecContext(ctx,
				`update biz_resources set industry = ?, category = ?, platform = ?, followers = ?,
				 avg_views = ? where id = ?`,
				category, category, platform, followers, views, id,
			)
		}
		return id, false, err
	}
	if err != nil && err != sql.ErrNoRows {
		return 0, false, err
	}

	if !hasRating {
		score = 70
		level = calcLevel(score)
	}
	result, err := tx.ExecContext(ctx,
		`insert into biz_resources
		 (name, resource_type, platform, industry, category, followers, avg_views, score, level, status, risk_level)
		 values (?, 'KOL', ?, ?, ?, ?, ?, ?, ?, '可合作', '低')`,
		name, platform, category, category, followers, views, score, level,
	)
	if err != nil {
		return 0, false, err
	}
	id, err = result.LastInsertId()
	return id, true, err
}

func insertImportCooperation(ctx context.Context, tx *sql.Tx, projectID int, resourceID int64, batchID string, row map[string]any) error {
	views := intField(row, "views")
	_, err := tx.ExecContext(ctx,
		`insert into biz_cooperations
		 (project_id, resource_id, cooperation_type, quote_amount, currency, status, deliverable_status,
		  impressions, views, engagement_count, comments_count, release_date, deliverable_links,
		  import_batch_id, notes)
		 values (?, ?, ?, ?, 'USD', '已发布', '已完成', ?, ?, ?, ?, ?, ?, ?, ?)`,
		projectID, resourceID, str(row, "category"), floatField(row, "quoteAmount"), views, views, intField(row, "engagementCount"),
		intField(row, "commentsCount"), nullableDate(str(row, "releaseDate")),
		str(row, "deliverableLinks"), batchID, defaultString(str(row, "rating"), "表格导入"),
	)
	return err
}

func (a *app) businessBriefTemplates(w http.ResponseWriter, r *http.Request) {
	rows, err := a.queryMaps(r.Context(),
		`select id, name, platform, market, content_type as contentType, language, status, owner, template,
		        cast(unix_timestamp(updated_at) * 1000 as unsigned) as updatedAt
		   from biz_brief_templates order by updated_at desc`,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeTable(w, rows)
}

func (a *app) createBusinessBriefTemplate(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	_, err := a.DB().ExecContext(r.Context(),
		`insert into biz_brief_templates
		 (name, platform, market, content_type, language, status, owner, template)
		 values (?, ?, ?, ?, ?, ?, ?, ?)`,
		str(body, "name"), str(body, "platform"), str(body, "market"), str(body, "contentType"),
		str(body, "language"), defaultString(str(body, "status"), "启用"), str(body, "owner"),
		str(body, "template"),
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"created": true})
}

func (a *app) businessDashboard(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{}
	stats := []struct {
		key   string
		query string
	}{
		{"resourceTotal", "select count(*) from biz_resources"},
		{"activeResourceTotal", "select count(*) from biz_resources where status = '可合作'"},
		{"saResourceTotal", "select count(*) from biz_resources where level in ('S','A')"},
		{"riskResourceTotal", "select count(*) from biz_resources where risk_level in ('中','高')"},
		{"projectTotal", "select count(*) from biz_projects"},
		{"cooperationTotal", "select count(*) from biz_cooperations"},
		{"totalFollowers", "select coalesce(sum(followers), 0) from biz_resources"},
		{"totalAvgViews", "select coalesce(sum(avg_views), 0) from biz_resources"},
		{"totalExposure", "select coalesce(sum(greatest(impressions, views)), 0) from biz_cooperations"},
		{"totalEngagements", "select coalesce(sum(engagement_count + comments_count), 0) from biz_cooperations"},
		{"totalCooperationCost", "select coalesce(sum(case when currency = 'USD' then quote_amount else 0 end), 0) from biz_cooperations"},
	}
	for _, stat := range stats {
		var n float64
		if err := a.DB().QueryRowContext(r.Context(), stat.query).Scan(&n); err != nil {
			writeDBError(w, err)
			return
		}
		data[stat.key] = n
	}
	totalExposure, _ := data["totalExposure"].(float64)
	totalEngagements, _ := data["totalEngagements"].(float64)
	averageEngagementRate := float64(0)
	if totalExposure > 0 {
		averageEngagementRate = totalEngagements / totalExposure
	}
	data["averageEngagementRate"] = averageEngagementRate

	postWhere := " where p.published_at >= ? and p.published_at < date_add(?, interval 1 day)"
	startDate := strings.TrimSpace(r.URL.Query().Get("startDate"))
	endDate := strings.TrimSpace(r.URL.Query().Get("endDate"))
	if startDate == "" {
		startDate = time.Now().AddDate(0, 0, -29).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}
	postArgs := []any{startDate, endDate}
	if platform := strings.TrimSpace(r.URL.Query().Get("platform")); platform != "" {
		postWhere += " and p.platform = ?"
		postArgs = append(postArgs, platform)
	}
	if resourceType := strings.TrimSpace(r.URL.Query().Get("resourceType")); resourceType != "" {
		postWhere += " and r.resource_type = ?"
		postArgs = append(postArgs, resourceType)
	}
	if country := strings.TrimSpace(r.URL.Query().Get("country")); country != "" {
		postWhere += " and r.country = ?"
		postArgs = append(postArgs, country)
	}

	var totalPostCount, totalPostViews, totalPostInteractions, hotPostCount float64
	if err := a.DB().QueryRowContext(r.Context(),
		`select count(*) as postCount,
		        coalesce(sum(p.view_count), 0) as totalViews,
		        coalesce(sum(p.like_count + p.comment_count + p.share_count), 0) as totalInteractions,
		        coalesce(sum(case when p.view_count >= 1000000 then 1 else 0 end), 0) as hotPostCount
		   from biz_resource_platform_posts p
		   left join biz_resources r on r.id = p.resource_id`+postWhere,
		postArgs...,
	).Scan(&totalPostCount, &totalPostViews, &totalPostInteractions, &hotPostCount); err != nil {
		writeDBError(w, err)
		return
	}
	data["totalPostCount"] = totalPostCount
	data["totalPostViews"] = totalPostViews
	data["totalPostInteractions"] = totalPostInteractions
	data["hotPostCount"] = hotPostCount
	postEngagementRate := float64(0)
	if totalPostViews > 0 {
		postEngagementRate = totalPostInteractions / totalPostViews
	}
	data["postEngagementRate"] = postEngagementRate

	trend, err := a.queryMaps(r.Context(),
		`select date_format(date(p.published_at), '%Y-%m-%d') as date,
		        count(*) as postCount,
		        coalesce(sum(p.view_count), 0) as exposure,
		        coalesce(sum(p.like_count + p.comment_count + p.share_count), 0) as interactions
		   from biz_resource_platform_posts p
		   left join biz_resources r on r.id = p.resource_id`+postWhere+`
		  group by date(p.published_at)
		  order by date(p.published_at)`,
		postArgs...,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	topResources, err := a.queryMaps(r.Context(),
		`select r.id, r.name, r.platform, count(*) as postCount,
		        coalesce(sum(p.view_count), 0) as exposure,
		        coalesce(sum(p.like_count + p.comment_count + p.share_count), 0) as interactions,
		        case when sum(p.view_count) > 0
		             then sum(p.like_count + p.comment_count + p.share_count) / sum(p.view_count)
		             else 0 end as engagementRate
		   from biz_resource_platform_posts p
		   left join biz_resources r on r.id = p.resource_id`+postWhere+`
		  group by r.id, r.name, r.platform
		  order by exposure desc
		  limit 10`,
		postArgs...,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	data["trend"] = trend
	data["topResources"] = topResources

	byCountry, err := a.queryMaps(r.Context(), `select country as name, count(*) as value from biz_resources group by country order by value desc limit 10`)
	if err != nil {
		writeDBError(w, err)
		return
	}
	byPlatform, err := a.queryMaps(r.Context(), `select platform as name, count(*) as value from biz_resources group by platform order by value desc limit 10`)
	if err != nil {
		writeDBError(w, err)
		return
	}
	byLevel, err := a.queryMaps(r.Context(), `select level as name, count(*) as value from biz_resources group by level order by field(level, 'S','A','B','C','D')`)
	if err != nil {
		writeDBError(w, err)
		return
	}
	data["byCountry"] = byCountry
	data["byPlatform"] = byPlatform
	data["byLevel"] = byLevel
	data["filters"] = map[string]any{
		"startDate": startDate,
		"endDate":   endDate,
	}
	writeOK(w, data)
}

func (a *app) businessGovernanceRules(w http.ResponseWriter, r *http.Request) {
	rows, err := a.queryMaps(r.Context(),
		`select id, rule_type as ruleType, name, content, enabled,
		        cast(unix_timestamp(updated_at) * 1000 as unsigned) as updatedAt
		   from biz_governance_rules
		  order by field(rule_type, 'ai_model', 'scoring_model', 'level_threshold', 'required_fields', 'update_frequency', 'data_trust', 'recommendation', 'warning'), id`,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, rows)
}

func (a *app) businessAIModelConfig(w http.ResponseWriter, r *http.Request) {
	rows, err := a.queryMaps(r.Context(),
		`select id, rule_type as ruleType, name, content, enabled,
		        cast(unix_timestamp(updated_at) * 1000 as unsigned) as updatedAt
		   from biz_governance_rules
		  where rule_type = 'ai_model'
		  limit 1`,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if len(rows) > 0 {
		writeOK(w, a.withAIModelKeyStatus(rows[0]))
		return
	}
	content := map[string]any{
		"provider":                   "",
		"model":                      "",
		"baseUrl":                    "",
		"apiKeyConfigured":           false,
		"apiKeyLast4":                "",
		"enableDemandParsing":        true,
		"enableRecommendationReason": true,
		"timeoutSeconds":             30,
		"temperature":                0.2,
	}
	if key := strings.TrimSpace(a.Config().AIModel.APIKey); key != "" {
		content["apiKeyConfigured"] = true
		content["apiKeyLast4"] = lastN(key, 4)
	}
	contentBytes, _ := json.Marshal(content)
	writeOK(w, map[string]any{
		"ruleType": "ai_model",
		"name":     "AI 模型配置",
		"content":  string(contentBytes),
		"enabled":  true,
	})
}

func (a *app) saveBusinessAIModelConfig(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	name := defaultString(str(body, "name"), "AI 模型配置")
	content := str(body, "content")
	if content == "" {
		writeError(w, http.StatusOK, 10001, "模型配置不能为空")
		return
	}
	if !json.Valid([]byte(content)) {
		writeError(w, http.StatusOK, 10002, "模型配置必须是合法 JSON")
		return
	}
	apiKey := strings.TrimSpace(str(body, "apiKey"))
	if apiKey != "" {
		if err := writeAIModelAPIKeyToConfig(apiKey); err != nil {
			writeError(w, http.StatusOK, 10006, "API Key 写入 config.yaml 失败："+err.Error())
			return
		}
		cfg := a.Config()
		cfg.AIModel.APIKey = apiKey
		a.config.Store(cfg)
	}
	storedKey := apiKey
	if storedKey == "" {
		storedKey = strings.TrimSpace(a.Config().AIModel.APIKey)
	}
	nextContent, err := aiModelContentWithKeyStatus(content, storedKey)
	if err != nil {
		writeError(w, http.StatusOK, 10002, "模型配置必须是合法 JSON")
		return
	}
	content = nextContent
	if err := a.saveBusinessRuleRecord(
		r.Context(),
		"ai_model",
		name,
		content,
		boolField(body, "enabled"),
		"立即生效",
		"AI 模型配置已保存，推荐助手将按新配置尝试接入",
		str(body, "createdBy"),
	); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"saved": true})
}

func (a *app) testBusinessAIModelConfig(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	provider := str(body, "provider")
	model := str(body, "model")
	baseURL := strings.TrimRight(str(body, "baseUrl"), "/")
	apiKey := strings.TrimSpace(str(body, "apiKey"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(a.Config().AIModel.APIKey)
	}
	if provider == "" || model == "" {
		writeError(w, http.StatusOK, 10001, "供应商和模型不能为空")
		return
	}
	if baseURL == "" || apiKey == "" {
		writeOK(w, map[string]any{
			"realRequest": false,
			"message":     "基础配置已通过；填写 Base URL，并保存或临时填写 API Key 后可发起真实连接测试。",
		})
		return
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		writeError(w, http.StatusOK, 10002, "Base URL 格式不正确")
		return
	}

	timeout := time.Duration(intField(body, "timeoutSeconds")) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if timeout > 120*time.Second {
		timeout = 120 * time.Second
	}
	endpoint := baseURL
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}
	payload, _ := json.Marshal(map[string]any{
		"model":       model,
		"messages":    []map[string]string{{"role": "user", "content": "Reply exactly: ok"}},
		"max_tokens":  16,
		"temperature": 0,
	})
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		writeError(w, http.StatusOK, 10003, err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	start := time.Now()
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		writeError(w, http.StatusOK, 10004, "连接测试失败："+err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		message := strings.TrimSpace(string(bodyBytes))
		if message == "" {
			message = resp.Status
		}
		writeError(w, http.StatusOK, 10005, "模型服务返回异常："+message)
		return
	}
	writeOK(w, map[string]any{
		"realRequest": true,
		"message":     fmt.Sprintf("真实连接测试通过，用时 %dms。", time.Since(start).Milliseconds()),
	})
}

func (a *app) withAIModelKeyStatus(row map[string]any) map[string]any {
	apiKey := strings.TrimSpace(a.Config().AIModel.APIKey)
	next := make(map[string]any, len(row))
	for key, value := range row {
		next[key] = value
	}
	content, _ := aiModelContentWithKeyStatus(str(next, "content"), apiKey)
	if content != "" {
		next["content"] = content
	}
	return next
}

func aiModelContentWithKeyStatus(content string, apiKey string) (string, error) {
	var data map[string]any
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return "", err
	}
	if apiKey == "" {
		data["apiKeyConfigured"] = false
		data["apiKeyLast4"] = ""
	} else {
		data["apiKeyConfigured"] = true
		data["apiKeyLast4"] = lastN(apiKey, 4)
	}
	contentBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(contentBytes), nil
}

func writeAIModelAPIKeyToConfig(apiKey string) error {
	path := configFilePath()
	raw := map[string]any{}
	data, err := os.ReadFile(path)
	if err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return err
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	aiModel, ok := raw["ai_model"].(map[string]any)
	if !ok {
		aiModel = map[string]any{}
	}
	aiModel["api_key"] = apiKey
	raw["ai_model"] = aiModel

	output, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	mode := os.FileMode(0600)
	if stat, err := os.Stat(path); err == nil {
		mode = stat.Mode().Perm()
	}
	return os.WriteFile(path, output, mode)
}

func configFilePath() string {
	if file := strings.TrimSpace(os.Getenv("CONFIG_FILE")); file != "" {
		return file
	}
	return "config.yaml"
}

func lastN(value string, n int) string {
	if len(value) <= n {
		return value
	}
	return value[len(value)-n:]
}

func (a *app) saveBusinessGovernanceRule(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ruleType := str(body, "ruleType")
	name := str(body, "name")
	content := str(body, "content")
	if ruleType == "" || name == "" {
		writeError(w, http.StatusOK, 10001, "规则类型和名称不能为空")
		return
	}
	if !json.Valid([]byte(content)) {
		writeError(w, http.StatusOK, 10002, "规则内容必须是合法 JSON")
		return
	}
	if err := a.saveBusinessRuleRecord(
		r.Context(),
		ruleType,
		name,
		content,
		boolField(body, "enabled"),
		"下次推荐生效",
		defaultString(str(body, "impactSummary"), "保存配置，将在下次推荐任务生效"),
		str(body, "createdBy"),
	); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"saved": true})
}

func (a *app) saveBusinessRuleRecord(ctx context.Context, ruleType, name, content string, enabledValue bool, effectiveMode, impactSummary, createdBy string) error {
	enabled := 0
	if enabledValue {
		enabled = 1
	}
	tx, err := a.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx,
		`insert into biz_governance_rules (rule_type, name, content, enabled)
		 values (?, ?, ?, ?)
		 on duplicate key update name = values(name), content = values(content), enabled = values(enabled), updated_at = now()`,
		ruleType, name, content, enabled,
	)
	if err != nil {
		return err
	}
	ruleID, _ := result.LastInsertId()
	if ruleID == 0 {
		_ = tx.QueryRowContext(ctx, `select id from biz_governance_rules where rule_type = ? limit 1`, ruleType).Scan(&ruleID)
	}
	_, err = tx.ExecContext(ctx,
		`insert into biz_rule_versions (rule_id, rule_type, version_no, content, effective_mode, impact_summary, created_by)
		 values (?, ?, concat('v', date_format(now(), '%Y%m%d%H%i%s')), ?, ?, ?, ?)`,
		ruleID, ruleType, content, effectiveMode, impactSummary, createdBy,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func businessFilters(body map[string]any, rules map[string]string) (string, []any) {
	var where []string
	var args []any
	for field, clause := range rules {
		value := strings.TrimSpace(fmt.Sprint(body[field]))
		if value == "" || value == "<nil>" {
			continue
		}
		where = append(where, clause)
		if strings.Contains(clause, "like") {
			args = append(args, "%"+value+"%")
		} else {
			args = append(args, value)
		}
	}
	if len(where) == 0 {
		return "", nil
	}
	return " where " + strings.Join(where, " and "), args
}

func str(body map[string]any, key string) string {
	return strings.TrimSpace(fmt.Sprint(body[key]))
}

func floatField(body map[string]any, key string) float64 {
	switch v := body[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		var n sql.NullFloat64
		_, _ = fmt.Sscan(v, &n.Float64)
		n.Valid = true
		return n.Float64
	default:
		return 0
	}
}

func boolField(body map[string]any, key string) bool {
	switch v := body[key].(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case int:
		return v != 0
	case string:
		value := strings.ToLower(strings.TrimSpace(v))
		return value == "1" || value == "true" || value == "yes" || value == "启用"
	default:
		return false
	}
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" || value == "<nil>" {
		return fallback
	}
	return value
}

func calcLevel(score int) string {
	switch {
	case score >= 90:
		return "S"
	case score >= 80:
		return "A"
	case score >= 60:
		return "B"
	case score >= 40:
		return "C"
	default:
		return "D"
	}
}

func nullableDate(value string) any {
	value = strings.TrimSpace(value)
	if value == "" || value == "<nil>" {
		return nil
	}
	return value
}

func nullableTime(value string) any {
	value = strings.TrimSpace(value)
	if value == "" || value == "<nil>" {
		return nil
	}
	return value
}

func ratingScoreLevel(value string) (int, string, bool) {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" || value == "<NIL>" {
		return 0, "", false
	}
	switch value {
	case "S":
		return 95, "S", true
	case "A":
		return 85, "A", true
	case "B":
		return 70, "B", true
	case "C":
		return 50, "C", true
	case "D":
		return 30, "D", true
	}
	score, err := strconv.Atoi(value)
	if err != nil {
		return 0, "", false
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score, calcLevel(score), true
}
