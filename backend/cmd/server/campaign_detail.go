package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

	writeOK(w, map[string]any{
		"project":       projects[0],
		"stats":         buildCampaignDetailStats(projects[0], cooperations, segments),
		"cooperations":  cooperations,
		"deliverables":  deliverables,
		"reportSummary": buildCampaignReportSummary(segments),
		"budget":        buildCampaignBudget(projects[0], billingEvents),
		"billingEvents": billingEvents,
	})
}

func (a *app) projectCooperationRows(ctx context.Context, projectID int) ([]map[string]any, error) {
	return a.queryMaps(ctx,
		`select c.id, c.project_id as projectId, p.name as projectName, c.resource_id as resourceId,
		        r.name as resourceName, r.avatar_url as resourceAvatarUrl, r.platform_handle as platformHandle,
		        r.platform_url as platformUrl, r.country, r.language, r.platform, r.followers,
		        r.engagement_rate as engagementRate, r.score, r.level,
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
