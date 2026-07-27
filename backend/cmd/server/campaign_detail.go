package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

func (a *app) businessProjectDetail(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("id")))
	if projectID <= 0 {
		writeError(w, http.StatusOK, 10001, "Campaign id 不能为空")
		return
	}

	projects, err := a.queryMaps(r.Context(),
		`select id, name, target_market as targetMarket, language, platform, campaign_type as campaignType,
		        budget, currency, status, owner, brief,
		        date_format(cycle_start_date, '%Y-%m-%d') as cycleStartDate,
		        date_format(cycle_end_date, '%Y-%m-%d') as cycleEndDate,
		        date_format(report_update_date, '%Y-%m-%d') as reportUpdateDate,
		        cast(unix_timestamp(paused_at) * 1000 as unsigned) as pausedAt,
		        cast(unix_timestamp(created_at) * 1000 as unsigned) as createdAt,
		        cast(unix_timestamp(updated_at) * 1000 as unsigned) as updatedAt
		   from biz_projects
		  where id = ?
		  limit 1`,
		projectID,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if len(projects) == 0 {
		writeError(w, http.StatusOK, 10004, "Campaign 不存在")
		return
	}

	cooperations, err := a.projectCooperationRows(r.Context(), projectID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	projectResources, err := a.projectResourceRows(r.Context(), projectID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	deliverables, err := a.queryMaps(r.Context(),
		`select id, project_id as projectId, cooperation_id as cooperationId, stage_key as stageKey,
		        title, status, date_format(submitted_at, '%Y-%m-%d %H:%i:%s') as submittedAt,
		        link, caption, note, rejection_reason as rejectionReason, sort_order as sortOrder
		   from biz_campaign_deliverables
		  where project_id = ?
		  order by cooperation_id asc, sort_order asc, id asc`,
		projectID,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	segments, err := a.queryMaps(r.Context(),
		`select id, project_id as projectId, audience_segment as audienceSegment, platform,
		        creative_name as creativeName, forecast_views as forecastViews, actual_views as actualViews,
		        forecast_clicks as forecastClicks, actual_clicks as actualClicks,
		        forecast_cost as forecastCost, actual_cost as actualCost
		   from biz_campaign_report_segments
		  where project_id = ?
		  order by audience_segment asc, platform asc, creative_name asc`,
		projectID,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	billingEvents, err := a.queryMaps(r.Context(),
		`select id, project_id as projectId, event_type as eventType, amount, currency,
		        description, date_format(occurred_at, '%Y-%m-%d %H:%i:%s') as occurredAt
		   from biz_campaign_billing_events
		  where project_id = ?
		  order by occurred_at desc, id desc`,
		projectID,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	contentPosts, err := a.queryMaps(r.Context(),
		`select distinct p.id, p.resource_id as resourceId, r.name as resourceName,
		        r.avatar_url as resourceAvatarUrl, r.platform_handle as platformHandle,
		        p.platform, p.platform_post_id as platformPostId, p.title, p.description,
		        p.post_url as postUrl,
		        coalesce(nullif(p.cover_remote_url, ''), nullif(p.cover_url, ''), '') as coverUrl,
		        p.cover_remote_url as coverRemoteUrl,
		        case when p.cover_url like '/api/uploads/resource-images/%' then p.cover_url else '' end as coverLocalUrl,
		        p.media_type as mediaType,
		        cast(unix_timestamp(p.published_at) * 1000 as unsigned) as publishedAt,
		        p.duration_seconds as durationSeconds, p.view_count as viewCount,
		        p.like_count as likeCount, p.comment_count as commentCount, p.share_count as shareCount
	   from biz_resource_platform_posts p
	   join biz_cooperations c on c.resource_id = p.resource_id and c.project_id = ?
	    and (
	      lower(trim(trailing '/' from c.final_link)) = lower(trim(trailing '/' from p.post_url))
	      or locate(lower(trim(trailing '/' from p.post_url)), lower(ifnull(c.deliverable_links, ''))) > 0
	    )
	   left join biz_resources r on r.id = p.resource_id
	  order by p.published_at desc, p.id desc
	  limit 120`,
		projectID,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}

	writeOK(w, map[string]any{
		"project":          projects[0],
		"stats":            buildCampaignDetailStats(projects[0], cooperations, segments),
		"cooperations":     cooperations,
		"projectResources": projectResources,
		"deliverables":     deliverables,
		"reportSummary":    buildCampaignReportSummary(segments),
		"budget":           buildCampaignBudget(projects[0], billingEvents),
		"billingEvents":    billingEvents,
		"contentPosts":     contentPosts,
	})
}

func (a *app) projectResourceRows(ctx context.Context, projectID int) ([]map[string]any, error) {
	return a.queryMaps(ctx,
		`select relation.resource_id as resourceId, r.name as resourceName,
		        r.avatar_url as resourceAvatarUrl, r.platform_handle as platformHandle,
		        r.platform_url as platformUrl, r.country, r.market, r.language, r.platform, r.followers,
		        r.resource_type as resourceType, r.category, r.audience_size as audienceSize,
		        r.audience_size_unit as audienceSizeUnit, r.tier as collaboratorTier, r.contact as primaryContact,
		        r.engagement_rate as engagementRate, r.score, r.level,
		        ifnull(c.id, 0) as id, ifnull(c.cooperation_type, '') as cooperationType,
		        ifnull(c.quote_amount, 0) as quoteAmount, ifnull(c.currency, 'USD') as currency,
		        ifnull(c.status, '候选') as status, ifnull(c.deliverable_status, '未开始') as deliverableStatus,
		        ifnull(c.impressions, 0) as impressions, ifnull(c.views, 0) as views,
		        ifnull(c.engagement_count, 0) as engagementCount, ifnull(c.comments_count, 0) as commentsCount,
		        c.release_date as releaseDate, c.deliverable_links as deliverableLinks, c.final_link as finalLink,
		        c.owner, c.vendor, c.notes,
		        case when c.views > 0 then c.quote_amount * 1000 / c.views else 0 end as cpm
	   from (
		  select resource_id from biz_project_resources where project_id = ?
		  union
		  select resource_id from biz_cooperations where project_id = ?
	   ) relation
	   join biz_resources r on r.id = relation.resource_id
	   left join biz_cooperations c on c.id = (
		  select c2.id from biz_cooperations c2
		   where c2.project_id = ? and c2.resource_id = relation.resource_id
		   order by c2.updated_at desc, c2.id desc
		   limit 1
	   )
	  order by r.name asc, relation.resource_id asc`,
		projectID, projectID, projectID,
	)
}

func (a *app) projectCooperationRows(ctx context.Context, projectID int) ([]map[string]any, error) {
	return a.queryMaps(ctx,
		`select c.id, c.project_id as projectId, p.name as projectName, c.resource_id as resourceId,
		        r.name as resourceName, r.avatar_url as resourceAvatarUrl, r.platform_handle as platformHandle,
		        r.platform_url as platformUrl, r.country, r.market, r.language, r.platform, r.followers,
		        r.resource_type as resourceType, r.category, r.audience_size as audienceSize,
		        r.audience_size_unit as audienceSizeUnit, r.tier as collaboratorTier, r.contact as primaryContact,
		        r.engagement_rate as engagementRate, r.score, r.level,
		        c.cooperation_type as cooperationType, c.owner, c.vendor, c.audience_segment as audienceSegment,
		        c.creative_name as creativeName, c.quote_amount as quoteAmount,
		        c.currency, c.status, c.deliverable_status as deliverableStatus,
		        c.impressions, c.views, c.clicks, c.conversions, c.engagement_count as engagementCount,
		        c.comments_count as commentsCount, c.roi, c.team_rating as teamRating,
		        c.release_date as releaseDate, c.deliverable_links as deliverableLinks,
		        c.final_link as finalLink, c.top_geographies as topGeographies,
		        date_format(c.publish_time, '%Y-%m-%d %H:%i:%s') as publishTime,
		        c.tracking_link as trackingLink, c.ad_authorization_code as adAuthorizationCode,
		        c.import_batch_id as importBatchId, c.notes,
		        case when c.views > 0 then c.quote_amount * 1000 / c.views else 0 end as cpm,
		        cast(unix_timestamp(c.created_at) * 1000 as unsigned) as createdAt,
		        cast(unix_timestamp(c.updated_at) * 1000 as unsigned) as updatedAt
		   from biz_cooperations c
		   left join biz_projects p on p.id = c.project_id
		   left join biz_resources r on r.id = c.resource_id
		  where c.project_id = ?
		  order by c.updated_at desc`,
		projectID,
	)
}

func (a *app) updateBusinessProjectStatus(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	projectID := intField(body, "id")
	action := strings.TrimSpace(str(body, "action"))
	if projectID <= 0 {
		writeError(w, http.StatusOK, 10001, "Campaign id 不能为空")
		return
	}
	var status string
	var query string
	if action == "pause" {
		status = "Paused"
		query = `update biz_projects set status = ?, paused_at = now() where id = ?`
	} else if action == "resume" {
		status = "Active"
		query = `update biz_projects set status = ?, paused_at = null where id = ?`
	} else {
		writeError(w, http.StatusOK, 10001, "状态操作只支持 pause 或 resume")
		return
	}
	if _, err := a.DB().ExecContext(r.Context(), query, status, projectID); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"updated": true, "status": status})
}

func (a *app) renewBusinessProject(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	projectID := intField(body, "id")
	if projectID <= 0 {
		writeError(w, http.StatusOK, 10001, "Campaign id 不能为空")
		return
	}
	startDate := strings.TrimSpace(str(body, "cycleStartDate"))
	endDate := strings.TrimSpace(str(body, "cycleEndDate"))
	if startDate == "" || endDate == "" {
		writeError(w, http.StatusOK, 10001, "新周期开始和结束日期不能为空")
		return
	}
	_, err := a.DB().ExecContext(r.Context(),
		`update biz_projects
		    set cycle_start_date = ?, cycle_end_date = ?, report_update_date = current_date(), status = 'Active', paused_at = null
		  where id = ?`,
		nullableDate(startDate), nullableDate(endDate), projectID,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"updated": true})
}

func (a *app) updateBusinessProjectBudget(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	projectID := intField(body, "id")
	budget := floatField(body, "budget")
	if projectID <= 0 {
		writeError(w, http.StatusOK, 10001, "Campaign id 不能为空")
		return
	}
	if budget < 0 {
		writeError(w, http.StatusOK, 10001, "预算不能为负数")
		return
	}
	if _, err := a.DB().ExecContext(r.Context(), `update biz_projects set budget = ? where id = ?`, budget, projectID); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"updated": true, "budget": budget})
}

func (a *app) createBusinessInfluencerReport(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	projectID := intField(body, "projectId")
	cooperationID := intField(body, "cooperationId")
	resourceID := intField(body, "resourceId")
	reason := strings.TrimSpace(str(body, "reason"))
	if projectID <= 0 || cooperationID <= 0 || resourceID <= 0 {
		writeError(w, http.StatusOK, 10001, "Campaign、合作记录和达人不能为空")
		return
	}
	if reason == "" {
		reason = "质量或数据异常"
	}
	result, err := a.DB().ExecContext(r.Context(),
		`insert into biz_campaign_influencer_reports
		 (project_id, cooperation_id, resource_id, reason, detail, status)
		 values (?, ?, ?, ?, ?, '待处理')`,
		projectID, cooperationID, resourceID, reason, str(body, "detail"),
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	writeOK(w, map[string]any{"created": true, "id": id})
}

func (a *app) downloadBusinessProjectReport(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("projectId")))
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	if projectID <= 0 {
		writeError(w, http.StatusOK, 10001, "Campaign id 不能为空")
		return
	}
	if scope == "" {
		scope = "campaign"
	}
	if scope == "project" {
		a.downloadProjectWorkbook(w, r, projectID)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="campaign-%d-%s-report.csv"`, projectID, scope))
	writer := csv.NewWriter(w)
	defer writer.Flush()
	if scope == "influencer" {
		rows, err := a.projectCooperationRows(r.Context(), projectID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		_ = writer.Write([]string{"Influencer", "Final link", "Order price", "Language", "Top geographies", "Publish time"})
		for _, row := range rows {
			_ = writer.Write([]string{
				stringValue(row["resourceName"]),
				firstNonEmpty(stringValue(row["finalLink"]), stringValue(row["deliverableLinks"])),
				fmt.Sprintf("%s %v", stringValue(row["currency"]), row["quoteAmount"]),
				stringValue(row["language"]),
				stringValue(row["topGeographies"]),
				stringValue(row["publishTime"]),
			})
		}
		return
	}
	segments, err := a.queryMaps(r.Context(),
		`select audience_segment as audienceSegment, platform, creative_name as creativeName,
		        forecast_views as forecastViews, actual_views as actualViews,
		        forecast_clicks as forecastClicks, actual_clicks as actualClicks,
		        forecast_cost as forecastCost, actual_cost as actualCost
		   from biz_campaign_report_segments
		  where project_id = ?
		  order by audience_segment asc, platform asc, creative_name asc`,
		projectID,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	_ = writer.Write([]string{"Audience", "Platform", "Creative", "Forecast views", "Actual views", "Forecast clicks", "Actual clicks", "Forecast cost", "Actual cost"})
	for _, row := range segments {
		_ = writer.Write([]string{
			stringValue(row["audienceSegment"]),
			stringValue(row["platform"]),
			stringValue(row["creativeName"]),
			fmt.Sprint(row["forecastViews"]),
			fmt.Sprint(row["actualViews"]),
			fmt.Sprint(row["forecastClicks"]),
			fmt.Sprint(row["actualClicks"]),
			fmt.Sprint(row["forecastCost"]),
			fmt.Sprint(row["actualCost"]),
		})
	}
}

type projectReportContentRow struct {
	resourceName string
	platform     string
	contentType  string
	title        string
	contentURL   string
	publishedAt  string
	views        float64
	engagement   float64
	comments     float64
	notes        string
}

func (a *app) downloadProjectWorkbook(w http.ResponseWriter, r *http.Request, projectID int) {
	projects, err := a.queryMaps(r.Context(),
		`select id, name, target_market as targetMarket, language, platform,
		        campaign_type as campaignType, budget, currency, status, owner, brief,
		        date_format(created_at, '%Y-%m-%d %H:%i:%s') as createdAt
		   from biz_projects where id = ? limit 1`, projectID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if len(projects) == 0 {
		writeError(w, http.StatusOK, 10004, "项目不存在")
		return
	}
	resources, err := a.projectResourceRows(r.Context(), projectID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	cooperations, err := a.projectCooperationRows(r.Context(), projectID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	options, err := a.standardImportOptions(r.Context())
	if err != nil {
		writeDBError(w, err)
		return
	}

	exportRows := buildStandardProjectExportRows(projects[0], resources, cooperations)
	book, err := buildStandardProjectExportWorkbook(options, exportRows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, 10006, "项目标准数据生成失败")
		return
	}
	defer book.Close()
	buffer, err := book.WriteToBuffer()
	if err != nil {
		writeError(w, http.StatusInternalServerError, 10006, "项目标准数据生成失败")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="project-%d-standard.xlsx"`, projectID))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buffer.Bytes())
}

func buildStandardProjectExportRows(project map[string]any, resources, cooperations []map[string]any) []map[string]any {
	cooperationsByResource := make(map[int][]map[string]any)
	for _, cooperation := range cooperations {
		resourceID := intField(cooperation, "resourceId")
		cooperationsByResource[resourceID] = append(cooperationsByResource[resourceID], cooperation)
	}

	rows := make([]map[string]any, 0, len(resources)+len(cooperations))
	seenResources := make(map[int]bool)
	for _, resource := range resources {
		resourceID := intField(resource, "resourceId")
		seenResources[resourceID] = true
		linked := cooperationsByResource[resourceID]
		if len(linked) == 0 {
			rows = append(rows, standardProjectExportRow(project, resource))
			continue
		}
		for _, cooperation := range linked {
			rows = append(rows, standardProjectExportRow(project, cooperation))
		}
	}
	for _, cooperation := range cooperations {
		if resourceID := intField(cooperation, "resourceId"); !seenResources[resourceID] {
			rows = append(rows, standardProjectExportRow(project, cooperation))
		}
	}
	return rows
}

func standardProjectExportRow(project, source map[string]any) map[string]any {
	resourceType := defaultString(strings.TrimSpace(stringValue(source["resourceType"])), "KOL")
	audienceSize := floatFromAny(source["audienceSize"])
	followers := floatFromAny(source["followers"])
	if resourceType != "媒体" && followers > 0 {
		audienceSize = followers
	} else if audienceSize <= 0 {
		audienceSize = followers
	}
	tier := strings.TrimSpace(stringValue(source["collaboratorTier"]))
	if tier == "" {
		tier = collaboratorTierForAudience(audienceSize)
	}
	views := maxFloat(floatFromAny(source["views"]), floatFromAny(source["impressions"]))
	cost := floatFromAny(source["quoteAmount"])
	cpm := float64(0)
	if views > 0 {
		cpm = cost * 1000 / views
	}
	return map[string]any{
		"resourceName":     stringValue(source["resourceName"]),
		"profileUrl":       standardExportProfileURL(source),
		"resourceType":     resourceType,
		"category":         stringValue(source["category"]),
		"market":           firstNonEmpty(stringValue(source["market"]), stringValue(source["country"]), stringValue(project["targetMarket"])),
		"audienceSize":     audienceSize,
		"collaboratorTier": tier,
		"platform":         stringValue(source["platform"]),
		"cooperationType":  stringValue(source["cooperationType"]),
		"contentUrl":       firstNonEmpty(stringValue(source["finalLink"]), stringValue(source["deliverableLinks"])),
		"cost":             cost,
		"views":            views,
		"engagement":       floatFromAny(source["engagementCount"]),
		"primaryContact":   stringValue(source["primaryContact"]),
		"owner":            firstNonEmpty(stringValue(source["owner"]), stringValue(project["owner"])),
		"vendor":           stringValue(source["vendor"]),
		"notes":            stringValue(source["notes"]),
		"cpm":              cpm,
	}
}

func standardExportProfileURL(source map[string]any) string {
	if profileURL := firstHTTPExcelURL(stringValue(source["platformUrl"])); profileURL != "" {
		return profileURL
	}
	handle := strings.TrimSpace(strings.TrimPrefix(stringValue(source["platformHandle"]), "@"))
	if handle == "" {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(stringValue(source["platform"]))) {
	case "youtube":
		return "https://www.youtube.com/@" + handle
	case "tiktok":
		return "https://www.tiktok.com/@" + handle
	case "instagram", "ins":
		return "https://www.instagram.com/" + handle + "/"
	case "facebook":
		return "https://www.facebook.com/" + handle
	case "x", "twitter":
		return "https://x.com/" + handle
	case "linkedin":
		return "https://www.linkedin.com/in/" + handle
	case "reddit":
		return "https://www.reddit.com/user/" + handle
	default:
		return ""
	}
}

func collaboratorTierForAudience(audienceSize float64) string {
	switch {
	case audienceSize > 1000000:
		return "头部"
	case audienceSize >= 100000:
		return "腰部"
	default:
		return "尾部"
	}
}

func buildProjectReportContentRows(posts, cooperations []map[string]any) []projectReportContentRow {
	rows := make([]projectReportContentRow, 0, len(posts)+len(cooperations))
	seenURLs := make(map[string]bool)
	for _, post := range posts {
		contentURL := strings.TrimSpace(stringValue(post["postUrl"]))
		if contentURL != "" {
			seenURLs[strings.ToLower(contentURL)] = true
		}
		rows = append(rows, projectReportContentRow{
			resourceName: stringValue(post["resourceName"]), platform: stringValue(post["platform"]),
			contentType: stringValue(post["mediaType"]), title: stringValue(post["title"]),
			contentURL: contentURL, publishedAt: stringValue(post["publishedAt"]),
			views:      floatFromAny(post["viewCount"]),
			engagement: floatFromAny(post["likeCount"]) + floatFromAny(post["commentCount"]) + floatFromAny(post["shareCount"]),
			comments:   floatFromAny(post["commentCount"]), notes: stringValue(post["description"]),
		})
	}
	for _, cooperation := range cooperations {
		contentURL := firstNonEmpty(stringValue(cooperation["finalLink"]), stringValue(cooperation["deliverableLinks"]))
		if contentURL == "" || seenURLs[strings.ToLower(strings.TrimSpace(contentURL))] {
			continue
		}
		seenURLs[strings.ToLower(strings.TrimSpace(contentURL))] = true
		rows = append(rows, projectReportContentRow{
			resourceName: stringValue(cooperation["resourceName"]), platform: stringValue(cooperation["platform"]),
			contentType: stringValue(cooperation["cooperationType"]), title: stringValue(cooperation["creativeName"]),
			contentURL: contentURL, publishedAt: firstNonEmpty(stringValue(cooperation["publishTime"]), stringValue(cooperation["releaseDate"])),
			views:      maxFloat(floatFromAny(cooperation["views"]), floatFromAny(cooperation["impressions"])),
			engagement: floatFromAny(cooperation["engagementCount"]), comments: floatFromAny(cooperation["commentsCount"]),
			notes: stringValue(cooperation["notes"]),
		})
	}
	return rows
}

func buildProjectReportWorkbook(project map[string]any, resources []map[string]any, contents []projectReportContentRow) (*excelize.File, error) {
	book := excelize.NewFile()
	if err := book.SetSheetName(book.GetSheetName(0), "项目概览"); err != nil {
		return nil, err
	}
	for _, sheet := range []string{"合作达人媒体", "合作内容", "平台表现"} {
		if _, err := book.NewSheet(sheet); err != nil {
			return nil, err
		}
	}
	headerStyle, _ := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1}, Alignment: &excelize.Alignment{Vertical: "center"}})
	labelStyle, _ := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "44546A"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"D9EAF7"}, Pattern: 1}})
	percentStyle, _ := book.NewStyle(&excelize.Style{NumFmt: 10})
	currencyStyle, _ := book.NewStyle(&excelize.Style{NumFmt: 4})

	var totalViews, totalEngagement, totalCost float64
	for _, row := range contents {
		totalViews += row.views
		totalEngagement += row.engagement
	}
	for _, row := range resources {
		totalCost += floatFromAny(row["quoteAmount"])
	}
	cpm, cpe := float64(0), float64(0)
	if totalViews > 0 {
		cpm = totalCost * 1000 / totalViews
	}
	if totalEngagement > 0 {
		cpe = totalCost / totalEngagement
	}
	overview := [][]any{
		{"项目基本信息", ""},
		{"项目名称", project["name"]}, {"创建日期", project["createdAt"]},
		{"市场", defaultString(stringValue(project["targetMarket"]), "未设置")},
		{"平台", defaultString(stringValue(project["platform"]), "全平台")},
		{"语言", defaultString(stringValue(project["language"]), "多语言")},
		{"项目类型", defaultString(stringValue(project["campaignType"]), "合作项目")},
		{"状态", projectStatusText(stringValue(project["status"]))},
		{"负责人", defaultString(stringValue(project["owner"]), "未设置")},
		{"项目说明", stringValue(project["brief"])},
		{"数据概览", ""},
		{"合作达人 / 媒体数", len(resources)}, {"合作内容数", len(contents)},
		{"总曝光 / 播放量", totalViews}, {"总互动量", totalEngagement},
		{"平均互动率", safeRatio(totalEngagement, totalViews)},
		{"合作费用", totalCost}, {"CPM", cpm}, {"CPE", cpe},
		{"导出时间", time.Now().Format("2006-01-02 15:04:05")},
	}
	for index, row := range overview {
		cell, _ := excelize.CoordinatesToCellName(1, index+1)
		_ = book.SetSheetRow("项目概览", cell, &row)
		if index == 0 || index == 10 {
			_ = book.MergeCell("项目概览", cell, fmt.Sprintf("B%d", index+1))
			_ = book.SetCellStyle("项目概览", cell, fmt.Sprintf("B%d", index+1), headerStyle)
		} else {
			_ = book.SetCellStyle("项目概览", cell, cell, labelStyle)
		}
	}
	_ = book.SetCellStyle("项目概览", "B16", "B16", percentStyle)
	_ = book.SetCellStyle("项目概览", "B17", "B19", currencyStyle)
	_ = book.SetColWidth("项目概览", "A", "A", 24)
	_ = book.SetColWidth("项目概览", "B", "B", 52)

	resourceHeaders := []any{"合作方", "类型", "领域", "市场", "粉丝数/访问量", "层级", "平台", "合作类型", "内容链接", "合作费用", "曝光/播放量", "互动量", "联系方式", "对接人", "供应商", "备注", "CPM"}
	_ = book.SetSheetRow("合作达人媒体", "A1", &resourceHeaders)
	for index, row := range resources {
		values := []any{row["resourceName"], row["resourceType"], row["category"], firstNonEmpty(stringValue(row["market"]), stringValue(row["country"])), row["audienceSize"], row["collaboratorTier"], row["platform"], row["cooperationType"], firstNonEmpty(stringValue(row["finalLink"]), stringValue(row["deliverableLinks"])), row["quoteAmount"], maxFloat(floatFromAny(row["views"]), floatFromAny(row["impressions"])), row["engagementCount"], row["primaryContact"], row["owner"], row["vendor"], row["notes"], row["cpm"]}
		cell, _ := excelize.CoordinatesToCellName(1, index+2)
		_ = book.SetSheetRow("合作达人媒体", cell, &values)
	}
	formatReportSheet(book, "合作达人媒体", len(resourceHeaders), len(resources)+1, headerStyle)

	contentHeaders := []any{"合作方", "平台", "内容类型", "内容标题", "内容链接", "发布时间", "曝光/播放量", "互动量", "评论量", "备注"}
	_ = book.SetSheetRow("合作内容", "A1", &contentHeaders)
	for index, row := range contents {
		values := []any{row.resourceName, row.platform, row.contentType, row.title, row.contentURL, row.publishedAt, row.views, row.engagement, row.comments, row.notes}
		cell, _ := excelize.CoordinatesToCellName(1, index+2)
		_ = book.SetSheetRow("合作内容", cell, &values)
	}
	formatReportSheet(book, "合作内容", len(contentHeaders), len(contents)+1, headerStyle)

	type platformSummary struct {
		content           int
		views, engagement float64
	}
	platforms := make(map[string]*platformSummary)
	for _, row := range contents {
		name := defaultString(strings.TrimSpace(row.platform), "未设置平台")
		if platforms[name] == nil {
			platforms[name] = &platformSummary{}
		}
		platforms[name].content++
		platforms[name].views += row.views
		platforms[name].engagement += row.engagement
	}
	names := make([]string, 0, len(platforms))
	for name := range platforms {
		names = append(names, name)
	}
	sort.Strings(names)
	platformHeaders := []any{"平台", "内容数", "内容占比", "曝光/播放量", "曝光占比", "互动量", "互动占比"}
	_ = book.SetSheetRow("平台表现", "A1", &platformHeaders)
	for index, name := range names {
		row := platforms[name]
		values := []any{name, row.content, safeRatio(float64(row.content), float64(len(contents))), row.views, safeRatio(row.views, totalViews), row.engagement, safeRatio(row.engagement, totalEngagement)}
		cell, _ := excelize.CoordinatesToCellName(1, index+2)
		_ = book.SetSheetRow("平台表现", cell, &values)
	}
	formatReportSheet(book, "平台表现", len(platformHeaders), len(names)+1, headerStyle)
	if len(names) > 0 {
		_ = book.SetCellStyle("平台表现", "C2", fmt.Sprintf("C%d", len(names)+1), percentStyle)
		_ = book.SetCellStyle("平台表现", "E2", fmt.Sprintf("G%d", len(names)+1), percentStyle)
	}
	book.SetActiveSheet(0)
	return book, nil
}

func formatReportSheet(book *excelize.File, sheet string, columns, rows, headerStyle int) {
	end, _ := excelize.CoordinatesToCellName(columns, 1)
	_ = book.SetCellStyle(sheet, "A1", end, headerStyle)
	_ = book.SetRowHeight(sheet, 1, 26)
	_ = book.SetColWidth(sheet, "A", end[:1], 18)
	if columns >= 5 {
		_ = book.SetColWidth(sheet, "E", "E", 42)
	}
	if rows > 1 {
		last, _ := excelize.CoordinatesToCellName(columns, rows)
		_ = book.AutoFilter(sheet, "A1:"+last, nil)
	}
	_ = book.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
}

func projectStatusText(status string) string {
	if strings.Contains(strings.ToLower(status), "pause") || strings.Contains(status, "暂停") {
		return "已暂停"
	}
	return "进行中"
}

func safeRatio(numerator, denominator float64) float64 {
	if denominator <= 0 {
		return 0
	}
	return numerator / denominator
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

func buildCampaignDetailStats(project map[string]any, cooperations []map[string]any, segments []map[string]any) map[string]any {
	var cost, reach, clicks, engagements float64
	published := 0
	missingData := 0
	resourceIDs := map[string]bool{}
	for _, row := range cooperations {
		resourceIDs[fmt.Sprint(row["resourceId"])] = true
		rowReach := floatFromAny(row["impressions"])
		if rowReach <= 0 {
			rowReach = floatFromAny(row["views"])
		}
		reach += rowReach
		cost += floatFromAny(row["quoteAmount"])
		clicks += floatFromAny(row["clicks"])
		engagements += floatFromAny(row["engagementCount"]) + floatFromAny(row["commentsCount"])
		stageText := fmt.Sprintf("%v %v", row["status"], row["deliverableStatus"])
		if strings.Contains(stageText, "已发布") || strings.Contains(stageText, "已完成") {
			published++
			if rowReach <= 0 && floatFromAny(row["clicks"]) <= 0 {
				missingData++
			}
		}
	}
	completionRate := 0
	if len(cooperations) > 0 {
		completionRate = int(float64(published) / float64(len(cooperations)) * 100)
	}
	return map[string]any{
		"resourceCount":    len(resourceIDs),
		"cooperationCount": len(cooperations),
		"totalReach":       reach,
		"totalClicks":      clicks,
		"totalEngagements": engagements,
		"totalCost":        cost,
		"budget":           project["budget"],
		"published":        published,
		"completionRate":   completionRate,
		"missingData":      missingData,
		"segmentCount":     len(segments),
	}
}

func buildCampaignReportSummary(segments []map[string]any) map[string]any {
	summary := map[string]float64{}
	for _, row := range segments {
		summary["forecastViews"] += floatFromAny(row["forecastViews"])
		summary["actualViews"] += floatFromAny(row["actualViews"])
		summary["forecastClicks"] += floatFromAny(row["forecastClicks"])
		summary["actualClicks"] += floatFromAny(row["actualClicks"])
		summary["forecastCost"] += floatFromAny(row["forecastCost"])
		summary["actualCost"] += floatFromAny(row["actualCost"])
	}
	forecastCPM := ratioPerThousand(summary["forecastCost"], summary["forecastViews"])
	actualCPM := ratioPerThousand(summary["actualCost"], summary["actualViews"])
	forecastCPC := ratio(summary["forecastCost"], summary["forecastClicks"])
	actualCPC := ratio(summary["actualCost"], summary["actualClicks"])
	return map[string]any{
		"forecastViews":  summary["forecastViews"],
		"actualViews":    summary["actualViews"],
		"forecastClicks": summary["forecastClicks"],
		"actualClicks":   summary["actualClicks"],
		"forecastCost":   summary["forecastCost"],
		"actualCost":     summary["actualCost"],
		"forecastCPM":    forecastCPM,
		"actualCPM":      actualCPM,
		"forecastCPC":    forecastCPC,
		"actualCPC":      actualCPC,
		"segments":       segments,
	}
}

func buildCampaignBudget(project map[string]any, billingEvents []map[string]any) map[string]any {
	var cost float64
	for _, row := range billingEvents {
		cost += floatFromAny(row["amount"])
	}
	return map[string]any{
		"costToDate": cost,
		"budget":     project["budget"],
		"currency":   defaultString(stringValue(project["currency"]), "USD"),
	}
}

func floatFromAny(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case []byte:
		n, _ := strconv.ParseFloat(string(v), 64)
		return n
	case string:
		n, _ := strconv.ParseFloat(v, 64)
		return n
	default:
		n, _ := strconv.ParseFloat(fmt.Sprint(value), 64)
		return n
	}
}

func ratio(cost, denominator float64) float64 {
	if cost <= 0 || denominator <= 0 {
		return 0
	}
	return cost / denominator
}

func ratioPerThousand(cost, denominator float64) float64 {
	if cost <= 0 || denominator <= 0 {
		return 0
	}
	return cost / denominator * 1000
}
