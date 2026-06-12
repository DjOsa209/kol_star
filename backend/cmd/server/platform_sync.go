package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

type syncResourceRow struct {
	ID             int
	Name           string
	Platform       string
	PlatformURL    string
	PlatformUserID string
	PlatformHandle string
}

func (a *app) platformSyncControl(w http.ResponseWriter, r *http.Request) {
	data, err := a.platformSyncStatus(r.Context())
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, data)
}

func (a *app) savePlatformSyncControl(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	settings, ok := body["settings"].([]any)
	if !ok {
		writeError(w, http.StatusOK, 10001, "settings 不能为空")
		return
	}
	if err := a.ensurePlatformSyncSettings(r.Context()); err != nil {
		writeDBError(w, err)
		return
	}
	if apiConfigRaw, ok := body["apiConfig"].(map[string]any); ok {
		if err := a.savePlatformAPIConfig(r.Context(), apiConfigRaw); err != nil {
			writeDBError(w, err)
			return
		}
	}
	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()
	for _, item := range settings {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		platform := platformDisplayName(str(row, "platform"))
		if platform == "" {
			continue
		}
		if _, err := tx.ExecContext(r.Context(),
			`insert into biz_platform_sync_settings
			  (platform, enabled, sync_profile, sync_posts, post_limit)
			 values (?, ?, ?, ?, ?)
			 on duplicate key update
			   enabled = values(enabled),
			   sync_profile = values(sync_profile),
			   sync_posts = values(sync_posts),
			   post_limit = values(post_limit)`,
			platform, boolInt(row, "enabled"), boolInt(row, "syncProfile"), boolInt(row, "syncPosts"), clampInt(intField(row, "postLimit"), 1, 50),
		); err != nil {
			writeDBError(w, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeDBError(w, err)
		return
	}
	data, err := a.platformSyncStatus(r.Context())
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, data)
}

func (a *app) startBusinessResourcesSyncAll(w http.ResponseWriter, r *http.Request) {
	if err := a.ensurePlatformSyncSettings(r.Context()); err != nil {
		writeDBError(w, err)
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
		  (job_type, status, started_at, message)
		 values ('resource_sync_all', '运行中', now(), '任务已启动')`,
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
	go a.runBusinessResourcesSyncAll(int(jobID))
	writeOK(w, map[string]any{
		"started": true,
		"jobId":   jobID,
		"message": "异步同步任务已启动",
	})
}

func (a *app) businessResourcesSyncStatus(w http.ResponseWriter, r *http.Request) {
	data, err := a.platformSyncStatus(r.Context())
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, data)
}

func (a *app) runBusinessResourcesSyncAll(jobID int) {
	ctx := context.Background()
	defer func() {
		if recovered := recover(); recovered != nil {
			_, _ = a.DB().ExecContext(ctx,
				`update biz_platform_sync_jobs
				   set status = '失败', message = ?, finished_at = now()
				 where id = ?`,
				fmt.Sprintf("任务异常：%v", recovered), jobID,
			)
		}
	}()
	settings, err := a.platformSyncSettingsMap(ctx)
	if err != nil {
		a.finishPlatformSyncJob(ctx, jobID, "失败", fmt.Sprintf("读取抓取设置失败：%v", err))
		return
	}
	resources, err := a.syncableResources(ctx)
	if err != nil {
		a.finishPlatformSyncJob(ctx, jobID, "失败", fmt.Sprintf("读取资源失败：%v", err))
		return
	}
	_, _ = a.DB().ExecContext(ctx, `update biz_platform_sync_jobs set total_count = ? where id = ?`, len(resources), jobID)

	successCount := 0
	failedCount := 0
	skippedCount := 0
	for _, resource := range resources {
		platform := platformDisplayName(resource.Platform)
		enabled := settings[platform]
		if !enabled {
			skippedCount++
			a.updatePlatformSyncJobProgress(ctx, jobID, resource, successCount, failedCount, skippedCount, fmt.Sprintf("%s 已在抓取控制中停用", platform))
			continue
		}
		a.updatePlatformSyncJobProgress(ctx, jobID, resource, successCount, failedCount, skippedCount, "同步中")
		err := a.syncResourceByPlatform(ctx, resource)
		if err != nil {
			failedCount++
			a.markResourceSyncFailed(ctx, resource.ID, err.Error())
			a.updatePlatformSyncJobProgress(ctx, jobID, resource, successCount, failedCount, skippedCount, err.Error())
			continue
		}
		successCount++
		a.updatePlatformSyncJobProgress(ctx, jobID, resource, successCount, failedCount, skippedCount, "同步成功")
	}

	status := "成功"
	if failedCount > 0 && successCount == 0 {
		status = "失败"
	} else if failedCount > 0 {
		status = "部分失败"
	}
	message := fmt.Sprintf("同步完成：成功 %d，失败 %d，跳过 %d", successCount, failedCount, skippedCount)
	a.finishPlatformSyncJob(ctx, jobID, status, message)
}

func (a *app) syncResourceByPlatform(ctx context.Context, resource syncResourceRow) error {
	switch platformDisplayName(resource.Platform) {
	case "YouTube":
		_, err := a.syncYouTubeResource(ctx, resource.ID, resource.Name, resource.PlatformURL)
		return err
	case "Instagram":
		_, err := a.syncInstagramResource(ctx, resource.ID, resource.Name, resource.PlatformURL, resource.PlatformUserID, resource.PlatformHandle)
		return err
	case "TikTok":
		_, err := a.syncTikTokResource(ctx, resource.ID)
		return err
	default:
		return fmt.Errorf("当前平台暂不支持官方 API 同步")
	}
}

func (a *app) syncableResources(ctx context.Context) ([]syncResourceRow, error) {
	rows, err := a.DB().QueryContext(ctx,
		`select id, name, platform, platform_url, platform_user_id, platform_handle
		   from biz_resources
		  where lower(platform) in ('youtube', 'instagram', 'ins', 'tiktok')
		  order by id asc`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var resources []syncResourceRow
	for rows.Next() {
		var resource syncResourceRow
		if err := rows.Scan(&resource.ID, &resource.Name, &resource.Platform, &resource.PlatformURL, &resource.PlatformUserID, &resource.PlatformHandle); err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, rows.Err()
}

func (a *app) updatePlatformSyncJobProgress(ctx context.Context, jobID int, resource syncResourceRow, successCount, failedCount, skippedCount int, message string) {
	_, _ = a.DB().ExecContext(ctx,
		`update biz_platform_sync_jobs
		    set success_count = ?, failed_count = ?, skipped_count = ?,
		        current_resource_id = ?, current_resource_name = ?, message = ?
		  where id = ?`,
		successCount, failedCount, skippedCount, resource.ID, resource.Name, message, jobID,
	)
}

func (a *app) finishPlatformSyncJob(ctx context.Context, jobID int, status, message string) {
	_, _ = a.DB().ExecContext(ctx,
		`update biz_platform_sync_jobs
		    set status = ?, message = ?, current_resource_id = null,
		        current_resource_name = '', finished_at = now()
		  where id = ?`,
		status, message, jobID,
	)
}

func (a *app) platformSyncStatus(ctx context.Context) (map[string]any, error) {
	if err := a.ensurePlatformSyncSettings(ctx); err != nil {
		return nil, err
	}
	settings, err := a.queryMaps(ctx,
		`select platform, enabled as enabled, sync_profile as syncProfile,
		        sync_posts as syncPosts, post_limit as postLimit,
		        cast(unix_timestamp(updated_at) * 1000 as unsigned) as updatedAt
		   from biz_platform_sync_settings
		  order by field(platform, 'YouTube', 'Instagram', 'TikTok'), platform`,
	)
	if err != nil {
		return nil, err
	}
	latestJob, err := a.latestPlatformSyncJob(ctx)
	if err != nil {
		return nil, err
	}
	var lastResourceSyncAt sql.NullString
	if err := a.DB().QueryRowContext(ctx, `select cast(unix_timestamp(max(last_sync_at)) * 1000 as unsigned) from biz_resources`).Scan(&lastResourceSyncAt); err != nil {
		return nil, err
	}
	counts, err := a.queryMaps(ctx,
		`select platform, count(*) as total,
		        sum(case when last_sync_status = '成功' then 1 else 0 end) as synced
		   from biz_resources
		  where lower(platform) in ('youtube', 'instagram', 'ins', 'tiktok')
		  group by platform
		  order by platform`,
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"settings":           settings,
		"latestJob":          latestJob,
		"lastResourceSyncAt": nullStringValue(lastResourceSyncAt),
		"resourceCounts":     counts,
		"apiConfig":          a.platformAPIConfigStatus(ctx),
		"tokenStatus":        a.platformTokenStatus(ctx),
	}, nil
}

func (a *app) latestPlatformSyncJob(ctx context.Context) (map[string]any, error) {
	rows, err := a.queryMaps(ctx,
		`select id, job_type as jobType, status, total_count as totalCount,
		        success_count as successCount, failed_count as failedCount,
		        skipped_count as skippedCount, current_resource_id as currentResourceId,
		        current_resource_name as currentResourceName, message,
		        cast(unix_timestamp(started_at) * 1000 as unsigned) as startedAt,
		        cast(unix_timestamp(finished_at) * 1000 as unsigned) as finishedAt,
		        cast(unix_timestamp(updated_at) * 1000 as unsigned) as updatedAt
		   from biz_platform_sync_jobs
		  order by id desc
		  limit 1`,
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (a *app) latestRunningPlatformSyncJob(ctx context.Context) (map[string]any, error) {
	rows, err := a.queryMaps(ctx,
		`select id, job_type as jobType, status, total_count as totalCount,
		        success_count as successCount, failed_count as failedCount,
		        skipped_count as skippedCount, current_resource_id as currentResourceId,
		        current_resource_name as currentResourceName, message,
		        cast(unix_timestamp(started_at) * 1000 as unsigned) as startedAt,
		        cast(unix_timestamp(updated_at) * 1000 as unsigned) as updatedAt
		   from biz_platform_sync_jobs
		  where status = '运行中'
		  order by id desc
		  limit 1`,
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (a *app) platformSyncSettingsMap(ctx context.Context) (map[string]bool, error) {
	if err := a.ensurePlatformSyncSettings(ctx); err != nil {
		return nil, err
	}
	rows, err := a.DB().QueryContext(ctx, `select platform, enabled from biz_platform_sync_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	settings := map[string]bool{}
	for rows.Next() {
		var platform string
		var enabled int
		if err := rows.Scan(&platform, &enabled); err != nil {
			return nil, err
		}
		settings[platformDisplayName(platform)] = enabled == 1
	}
	return settings, rows.Err()
}

func (a *app) ensurePlatformSyncSettings(ctx context.Context) error {
	_, err := a.DB().ExecContext(ctx,
		`insert ignore into biz_platform_sync_settings
		  (platform, enabled, sync_profile, sync_posts, post_limit)
		 values
		  ('YouTube', 1, 1, 1, 25),
		  ('Instagram', 1, 1, 1, 25),
		  ('TikTok', 1, 1, 1, 20)`,
	)
	return err
}

func (a *app) platformTokenStatus(ctx context.Context) map[string]any {
	cfg := a.effectivePlatformAPIConfig(ctx)
	return map[string]any{
		"YouTube": map[string]any{
			"configured": strings.TrimSpace(cfg.YouTubeAPIKey) != "",
			"message":    tokenStatusMessage(strings.TrimSpace(cfg.YouTubeAPIKey) != "", "YouTube API Key 未配置"),
		},
		"Instagram": map[string]any{
			"configured": strings.TrimSpace(tikHubAPIKey(cfg)) != "",
			"message":    tokenStatusMessage(strings.TrimSpace(tikHubAPIKey(cfg)) != "", "TikHub API Key 未配置"),
		},
		"TikTok": map[string]any{
			"configured": strings.TrimSpace(tikHubAPIKey(cfg)) != "",
			"message":    tokenStatusMessage(strings.TrimSpace(tikHubAPIKey(cfg)) != "", "TikHub API Key 未配置"),
		},
	}
}

func (a *app) savePlatformAPIConfig(ctx context.Context, data map[string]any) error {
	current := a.effectivePlatformAPIConfig(ctx)
	if key := strings.TrimSpace(str(data, "youtubeApiKey")); key != "" {
		current.YouTubeAPIKey = key
	}
	if proxyURL, ok := data["youtubeProxyUrl"]; ok {
		current.YouTubeProxyURL = strings.TrimSpace(anyString(proxyURL))
	}
	if version := strings.TrimSpace(str(data, "metaGraphApiVersion")); version != "" {
		current.MetaGraphAPIVersion = version
	}
	if token := strings.TrimSpace(str(data, "instagramAccessToken")); token != "" {
		current.InstagramAccessToken = token
	}
	if userID := strings.TrimSpace(str(data, "instagramUserId")); userID != "" {
		current.InstagramUserID = userID
	}
	if token := strings.TrimSpace(str(data, "tiktokAccessToken")); token != "" {
		current.TikTokAccessToken = token
	}
	if token := strings.TrimSpace(str(data, "tikhubApiKey")); token != "" {
		current.TikHubAPIKey = token
		current.TikTokAccessToken = token
	}
	content := platformAPIConfigStoredContent(current)
	raw, err := json.Marshal(content)
	if err != nil {
		return err
	}
	if err := writePlatformAPIConfigToConfig(current); err != nil {
		return fmt.Errorf("写入 config.yaml 失败：%w", err)
	}
	cfg := a.Config()
	cfg.PlatformAPIs = current
	a.config.Store(cfg)
	return a.saveBusinessRuleRecord(
		ctx,
		"platform_api",
		"平台 API 配置",
		string(raw),
		true,
		"立即生效",
		"平台 API 配置已保存，抓取任务将使用新配置",
		"",
	)
}

func writePlatformAPIConfigToConfig(cfg PlatformAPIConfig) error {
	path := configFilePath()
	raw := map[string]any{}
	data, err := os.ReadFile(path)
	if err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return err
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	platformAPIs, ok := raw["platform_apis"].(map[string]any)
	if !ok {
		platformAPIs = map[string]any{}
	}
	platformAPIs["youtube_api_key"] = strings.TrimSpace(cfg.YouTubeAPIKey)
	platformAPIs["youtube_proxy_url"] = strings.TrimSpace(cfg.YouTubeProxyURL)
	platformAPIs["meta_graph_api_version"] = defaultString(strings.TrimSpace(cfg.MetaGraphAPIVersion), "v21.0")
	platformAPIs["instagram_access_token"] = strings.TrimSpace(cfg.InstagramAccessToken)
	platformAPIs["instagram_user_id"] = strings.TrimSpace(cfg.InstagramUserID)
	platformAPIs["tiktok_access_token"] = strings.TrimSpace(cfg.TikTokAccessToken)
	platformAPIs["tikhub_api_key"] = strings.TrimSpace(tikHubAPIKey(cfg))
	raw["platform_apis"] = platformAPIs

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

func (a *app) platformAPIConfigStatus(ctx context.Context) map[string]any {
	return platformAPIConfigPublicContent(a.effectivePlatformAPIConfig(ctx))
}

func platformAPIConfigPublicContent(cfg PlatformAPIConfig) map[string]any {
	return map[string]any{
		"youtubeApiKeyConfigured":        strings.TrimSpace(cfg.YouTubeAPIKey) != "",
		"youtubeApiKeyLast4":             lastN(strings.TrimSpace(cfg.YouTubeAPIKey), 4),
		"youtubeProxyUrl":                strings.TrimSpace(cfg.YouTubeProxyURL),
		"metaGraphApiVersion":            defaultString(strings.TrimSpace(cfg.MetaGraphAPIVersion), "v21.0"),
		"instagramAccessTokenConfigured": strings.TrimSpace(cfg.InstagramAccessToken) != "",
		"instagramAccessTokenLast4":      lastN(strings.TrimSpace(cfg.InstagramAccessToken), 4),
		"instagramUserId":                strings.TrimSpace(cfg.InstagramUserID),
		"tiktokAccessTokenConfigured":    strings.TrimSpace(tikHubAPIKey(cfg)) != "",
		"tiktokAccessTokenLast4":         lastN(strings.TrimSpace(tikHubAPIKey(cfg)), 4),
		"tikhubApiKeyConfigured":         strings.TrimSpace(tikHubAPIKey(cfg)) != "",
		"tikhubApiKeyLast4":              lastN(strings.TrimSpace(tikHubAPIKey(cfg)), 4),
	}
}

func platformAPIConfigStoredContent(cfg PlatformAPIConfig) map[string]any {
	public := platformAPIConfigPublicContent(cfg)
	public["youtubeApiKey"] = strings.TrimSpace(cfg.YouTubeAPIKey)
	public["instagramAccessToken"] = strings.TrimSpace(cfg.InstagramAccessToken)
	public["tiktokAccessToken"] = strings.TrimSpace(cfg.TikTokAccessToken)
	public["tikhubApiKey"] = strings.TrimSpace(tikHubAPIKey(cfg))
	return public
}

func (a *app) effectivePlatformAPIConfig(ctx context.Context) PlatformAPIConfig {
	cfg := a.Config().PlatformAPIs
	if cfg.MetaGraphAPIVersion == "" {
		cfg.MetaGraphAPIVersion = "v21.0"
	}
	var content string
	err := a.DB().QueryRowContext(ctx,
		`select content from biz_governance_rules where rule_type = 'platform_api' limit 1`,
	).Scan(&content)
	if err != nil || strings.TrimSpace(content) == "" {
		return cfg
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return cfg
	}
	if value := strings.TrimSpace(str(data, "youtubeApiKey")); value != "" {
		cfg.YouTubeAPIKey = value
	}
	if value, ok := data["youtubeProxyUrl"]; ok {
		cfg.YouTubeProxyURL = strings.TrimSpace(anyString(value))
	}
	if value := strings.TrimSpace(str(data, "metaGraphApiVersion")); value != "" {
		cfg.MetaGraphAPIVersion = value
	}
	if value := strings.TrimSpace(str(data, "instagramAccessToken")); value != "" {
		cfg.InstagramAccessToken = value
	}
	if value := strings.TrimSpace(str(data, "instagramUserId")); value != "" {
		cfg.InstagramUserID = value
	}
	if value := strings.TrimSpace(str(data, "tiktokAccessToken")); value != "" {
		cfg.TikTokAccessToken = value
	}
	if value := strings.TrimSpace(str(data, "tikhubApiKey")); value != "" {
		cfg.TikHubAPIKey = value
	}
	return cfg
}

func tikHubAPIKey(cfg PlatformAPIConfig) string {
	return firstNonEmpty(strings.TrimSpace(cfg.TikHubAPIKey), strings.TrimSpace(cfg.TikTokAccessToken))
}

func platformDisplayName(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "youtube":
		return "YouTube"
	case "instagram", "ins":
		return "Instagram"
	case "tiktok":
		return "TikTok"
	default:
		return ""
	}
}

func tokenStatusMessage(configured bool, missing string) string {
	if configured {
		return "已配置"
	}
	return missing
}

func nullStringValue(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}
