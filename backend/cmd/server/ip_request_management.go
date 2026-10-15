package main

import (
	"database/sql"
	"errors"
	"net/http"
)

// Management belongs to the authenticated creator, at every workflow stage.
func ipRequestManagementActions(row map[string]any, userID int) {
	owned := userID > 0 && anyInt64(row["createdBy"]) == int64(userID)
	row["canEdit"] = owned
	row["canDelete"] = owned
	row["candidatesLocked"] = row["status"] != "draft" && (row["status"] != "submitted" || anyInt64(row["feedbackStarted"]) > 0)
}

func (a *app) ipRequestDelete(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID int64 `json:"id"`
	}
	if !readIPJSON(w, r, &input) {
		return
	}
	if input.ID <= 0 {
		writeError(w, 400, 400, "缺少需求编号")
		return
	}
	userID, ok := a.currentUserID(r)
	if !ok {
		writeError(w, 401, 401, "请重新登录")
		return
	}
	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()
	var creatorID int64
	err = tx.QueryRowContext(r.Context(), "select created_by from biz_ip_requests where id=? for update", input.ID).Scan(&creatorID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 404, 404, "需求不存在或已删除")
		return
	}
	if err != nil {
		writeDBError(w, err)
		return
	}
	if creatorID != int64(userID) {
		writeError(w, 403, 403, "只能删除本人发起的需求")
		return
	}
	// Keep resource-library records and AI conversation history; detach the demand.
	for _, query := range []string{
		"update biz_recommendation_subscriptions set request_id=null,active=0 where request_id=? and domain='ip'",
		"delete from biz_ip_workflow_notifications where request_id=?",
		"delete from biz_ip_request_candidates where request_id=?",
		"delete from biz_ip_requests where id=?",
	} {
		if _, err = tx.ExecContext(r.Context(), query, input.ID); err != nil {
			writeDBError(w, err)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, nil)
}
