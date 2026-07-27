package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type onlineProjectResourceSeed struct {
	Platform       string
	Query          string
	ResourceType   string
	Name           string
	PlatformURL    string
	PlatformUserID string
	PlatformHandle string
}

func (a *app) searchOnlineProjectResource(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	projectID := intField(body, "projectId")
	if projectID <= 0 {
		writeError(w, http.StatusOK, 10001, "项目不能为空")
		return
	}
	var projectExists int
	if err := a.DB().QueryRowContext(r.Context(), `select count(*) from biz_projects where id = ?`, projectID).Scan(&projectExists); err != nil {
		writeDBError(w, err)
		return
	}
	if projectExists == 0 {
		writeError(w, http.StatusOK, 10002, "项目不存在")
		return
	}

	seed, err := normalizeOnlineProjectResourceSeed(str(body, "platform"), str(body, "query"), str(body, "resourceType"))
	if err != nil {
		writeError(w, http.StatusOK, 10001, err.Error())
		return
	}

	resourceID, err := a.findOnlineProjectResource(r.Context(), seed)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeDBError(w, err)
		return
	}
	created := false
	if errors.Is(err, sql.ErrNoRows) {
		resourceID, err = a.createOnlineProjectResource(r.Context(), seed)
		if err != nil {
			writeDBError(w, err)
			return
		}
		created = true
		resource := syncResourceRow{
			ID:             resourceID,
			Name:           seed.Name,
			Platform:       seed.Platform,
			PlatformURL:    seed.PlatformURL,
			PlatformUserID: seed.PlatformUserID,
			PlatformHandle: seed.PlatformHandle,
		}
		if err = a.syncResourceByPlatform(r.Context(), resource); err != nil {
			a.cleanupFailedOnlineProjectResource(r.Context(), resourceID)
			writeError(w, http.StatusOK, 10003, redactSensitiveText(err.Error()))
			return
		}
	}

	linked, err := a.projectHasResource(r.Context(), projectID, resourceID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if linked {
		writeError(w, http.StatusOK, 10004, "该达人/媒体已经在当前项目中")
		return
	}
	resource, err := a.projectResourceOption(r.Context(), resourceID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{
		"resource": resource,
		"created":  created,
		"source":   map[bool]string{true: "全网搜索", false: "全球资源库"}[created],
	})
}

func normalizeOnlineProjectResourceSeed(platformValue, queryValue, resourceTypeValue string) (onlineProjectResourceSeed, error) {
	platform := platformDisplayName(platformValue)
	if platform == "" {
		return onlineProjectResourceSeed{}, fmt.Errorf("请选择 Instagram、TikTok 或 YouTube")
	}
	query := strings.TrimSpace(queryValue)
	if query == "" {
		return onlineProjectResourceSeed{}, fmt.Errorf("请输入主页链接、@handle 或平台账号")
	}
	resourceType := strings.TrimSpace(resourceTypeValue)
	if resourceType == "" {
		resourceType = "KOL"
	}
	if resourceType != "KOL" && resourceType != "媒体" && resourceType != "艺术家" {
		return onlineProjectResourceSeed{}, fmt.Errorf("不支持的达人/媒体类型")
	}

	seed := onlineProjectResourceSeed{Platform: platform, Query: query, ResourceType: resourceType}
	switch platform {
	case "Instagram":
		handle := instagramHandleIdentifier(query)
		if handle == "" {
			return onlineProjectResourceSeed{}, fmt.Errorf("请输入有效的 Instagram 主页链接或 @handle")
		}
		seed.Name = handle
		seed.PlatformHandle = handle
		seed.PlatformURL = "https://www.instagram.com/" + handle + "/"
	case "TikTok":
		username, secUID := tiktokUserIdentifier(query)
		if username == "" && secUID == "" {
			return onlineProjectResourceSeed{}, fmt.Errorf("请输入有效的 TikTok 主页链接、@handle 或 secUid")
		}
		seed.Name = firstNonEmpty(username, truncateText(secUID, 32))
		seed.PlatformHandle = username
		seed.PlatformUserID = secUID
		if username != "" {
			seed.PlatformURL = "https://www.tiktok.com/@" + username
		}
	case "YouTube":
		seed.Name = strings.TrimSpace(strings.TrimPrefix(query, "@"))
		if paramName, identifier, err := youtubeChannelIdentifier(query, query); err == nil {
			seed.PlatformURL = query
			if paramName == "id" {
				seed.PlatformUserID = identifier
			} else {
				seed.PlatformHandle = strings.TrimPrefix(identifier, "@")
			}
		}
	}
	return seed, nil
}

func (a *app) findOnlineProjectResource(ctx context.Context, seed onlineProjectResourceSeed) (int, error) {
	clauses := make([]string, 0, 4)
	args := []any{seed.Platform}
	if seed.PlatformHandle != "" {
		clauses = append(clauses, "lower(trim(leading '@' from platform_handle)) = lower(?)")
		args = append(args, seed.PlatformHandle)
	}
	if seed.PlatformUserID != "" {
		clauses = append(clauses, "lower(trim(platform_user_id)) = lower(?)")
		args = append(args, seed.PlatformUserID)
	}
	if seed.PlatformURL != "" {
		clauses = append(clauses, "lower(trim(trailing '/' from platform_url)) = lower(trim(trailing '/' from ?))")
		args = append(args, seed.PlatformURL)
	}
	if len(clauses) == 0 || seed.Platform == "YouTube" {
		clauses = append(clauses, "lower(trim(name)) = lower(?)")
		args = append(args, seed.Name)
	}
	var resourceID int
	err := a.DB().QueryRowContext(ctx,
		`select id from biz_resources
		  where lower(trim(platform)) = lower(?)
		    and (`+strings.Join(clauses, " or ")+`)
		  order by id asc
		  limit 1`,
		args...,
	).Scan(&resourceID)
	return resourceID, err
}

func (a *app) createOnlineProjectResource(ctx context.Context, seed onlineProjectResourceSeed) (int, error) {
	referenceSource := ""
	if seed.ResourceType == "媒体" {
		referenceSource = "Similarweb（预置，未接入）"
	}
	result, err := a.DB().ExecContext(ctx,
		`insert into biz_resources
		  (name, resource_type, platform, platform_url, platform_user_id, platform_handle,
		   status, reference_source, last_sync_status)
		 values (?, ?, ?, ?, ?, ?, '可合作', ?, '同步中')`,
		seed.Name, seed.ResourceType, seed.Platform, seed.PlatformURL, seed.PlatformUserID,
		seed.PlatformHandle, referenceSource,
	)
	if err != nil {
		return 0, err
	}
	resourceID, err := result.LastInsertId()
	return int(resourceID), err
}

func (a *app) cleanupFailedOnlineProjectResource(ctx context.Context, resourceID int) {
	_, _ = a.DB().ExecContext(ctx, `delete from biz_resource_platform_posts where resource_id = ?`, resourceID)
	_, _ = a.DB().ExecContext(ctx, `delete from biz_resources where id = ?`, resourceID)
	_ = refreshAllResourceAudienceClassifications(ctx, a.DB())
}

func (a *app) projectHasResource(ctx context.Context, projectID, resourceID int) (bool, error) {
	var linked int
	err := a.DB().QueryRowContext(ctx,
		`select count(*) from (
		   select resource_id from biz_project_resources where project_id = ? and resource_id = ?
		   union
		   select resource_id from biz_cooperations where project_id = ? and resource_id = ?
		 ) linked`,
		projectID, resourceID, projectID, resourceID,
	).Scan(&linked)
	return linked > 0, err
}

func (a *app) projectResourceOption(ctx context.Context, resourceID int) (map[string]any, error) {
	rows, err := a.queryMaps(ctx,
		`select r.id as resourceId, r.name as resourceName, r.resource_type as resourceType,
		        r.platform, r.platform_handle as platformHandle, r.platform_url as platformUrl,
		        r.country, r.market, r.category, r.followers, r.audience_size as audienceSize,
		        r.audience_size_unit as audienceSizeUnit, r.tier as collaboratorTier,
		        r.contact as primaryContact, r.avatar_url as resourceAvatarUrl
		   from biz_resources r
		  where r.id = ?
		  limit 1`,
		resourceID,
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	return rows[0], nil
}
