package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ipWorkflowRequest struct {
	ID          int64
	ProjectName string
	Brief       map[string]any
	CreatorID   int64
}

// Called inside the same transaction as the transition. The unique key makes
// completion delivery durable and idempotent, even across server replicas.
func enqueueIPWorkflowNotices(ctx context.Context, tx *sql.Tx, item ipWorkflowRequest, stage string, now time.Time) error {
	predicates := []string{}
	args := []any{item.ID, stage, now.In(ipTimeZone).Format("2006-01-02 15:04:05")}
	if stage == "marketing_reviewed" {
		predicates = append(predicates, "u.id=?")
		args = append(args, item.CreatorID)
	} else {
		contacts, _ := item.Brief["contactsByStage"].(map[string]any)
		for _, key := range ipStageRecipientKeys[stage] {
			person, _ := contacts[key].(map[string]any)
			if id := anyInt64(person["id"]); id > 0 {
				predicates = append(predicates, "u.id=?")
				args = append(args, id)
			} else if subject := strings.TrimPrefix(ipMapText(person, "id"), "uac:"); subject != "" && strings.HasPrefix(ipMapText(person, "id"), "uac:") && ipMapText(person, "employeeNo") != "" {
				predicates = append(predicates, "(u.auth_provider='uac' and (u.external_subject=? or u.employee_no=?) and u.employee_no=?)")
				args = append(args, subject, subject, ipMapText(person, "employeeNo"))
			}
		}
	}
	if len(predicates) == 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, `insert ignore into biz_ip_workflow_notifications (request_id,stage,created_at,user_id)
        select ?,?,?,u.id from sys_users u where u.status=1 and (`+strings.Join(predicates, " or ")+`)`, args...)
	return err
}

func scanIPWorkflowRequest(row *sql.Row, id int64) (string, ipWorkflowRequest, error) {
	item := ipWorkflowRequest{ID: id}
	var status, brief string
	err := row.Scan(&status, &item.ProjectName, &brief, &item.CreatorID)
	if err == nil && brief != "" {
		err = json.Unmarshal([]byte(brief), &item.Brief)
	}
	return status, item, err
}

func (a *app) ipWorkflowNotifications(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.currentUserID(r)
	if !ok {
		writeError(w, 401, 401, "未登录或登录已过期")
		return
	}
	rows, err := a.queryMaps(r.Context(), `select n.id,n.request_id as requestId,n.stage,
        date_format(n.read_at,'%Y-%m-%d %H:%i:%s') as readAt,
        date_format(n.created_at,'%Y-%m-%d %H:%i:%s') as createdAt,
        q.project_name as projectName,q.status,
        date_format(q.submitted_at,'%Y-%m-%d %H:%i:%s') as submittedAt,
        date_format(q.ip_reviewed_at,'%Y-%m-%d %H:%i:%s') as ipReviewedAt,
        date_format(q.marketing_reviewed_at,'%Y-%m-%d %H:%i:%s') as marketingReviewedAt
        from biz_ip_workflow_notifications n join biz_ip_requests q on q.id=n.request_id
        where n.user_id=? and (n.stage='marketing_reviewed' or (n.stage='submitted' and q.status='submitted') or (n.stage='ip_reviewed' and q.status='ip_reviewed'))
        order by (n.read_at is null) desc,n.id desc limit 100`, userID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	unread := 0
	for _, row := range rows {
		a.enrichIPRequestDeadlines(row, time.Now())
		stage := fmt.Sprint(row["stage"])
		path := "/business/ip/requests"
		title := "IP 与营销意见已补充完整，请查看"
		description := fmt.Sprint(row["projectName"])
		if stage != "marketing_reviewed" {
			key := "ip"
			path = "/business/ip/feedback"
			if stage == "ip_reviewed" {
				key = "marketing"
				path = "/business/ip/marketing"
			}
			d := row["deadlines"].(map[string]ipStageDeadline)[key]
			title = d.Label + "待反馈"
			if d.DueAt != "" {
				description += " · 最晚反馈：" + ipStageTime(d.DueAt).Format("2006-01-02 15:04") + "（北京时间）"
			} else {
				description += " · " + d.Warning
			}
		}
		row["title"] = title
		row["description"] = description
		row["targetPath"] = fmt.Sprintf("%s?id=%v", path, row["requestId"])
		if row["readAt"] == nil || row["readAt"] == "" {
			unread++
		}
	}
	writeOK(w, map[string]any{"list": rows, "unread": unread})
}

func (a *app) ipWorkflowNotificationsRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.currentUserID(r)
	if !ok {
		writeError(w, 401, 401, "未登录或登录已过期")
		return
	}
	var input struct {
		IDs []int64 `json:"ids"`
	}
	if !readIPJSON(w, r, &input) {
		return
	}
	if len(input.IDs) == 0 || len(input.IDs) > 100 {
		writeError(w, 400, 400, "请选择1至100条通知")
		return
	}
	args := []any{time.Now().In(ipTimeZone).Format("2006-01-02 15:04:05"), userID}
	for _, id := range input.IDs {
		if id <= 0 {
			writeError(w, 400, 400, "通知编号不合法")
			return
		}
		args = append(args, id)
	}
	result, err := a.DB().ExecContext(r.Context(), `update biz_ip_workflow_notifications set read_at=? where user_id=? and read_at is null and id in (`+strings.TrimSuffix(strings.Repeat("?,", len(input.IDs)), ",")+`)`, args...)
	if err != nil {
		writeDBError(w, err)
		return
	}
	n, _ := result.RowsAffected()
	writeOK(w, map[string]any{"updated": n})
}
