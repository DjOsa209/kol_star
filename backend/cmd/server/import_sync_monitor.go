package main

import (
	"fmt"
	"net/http"
	"strings"
)

func (a *app) projectImportSyncJobs(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	pageSize := intField(body, "pageSize")
	currentPage := intField(body, "currentPage")
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if currentPage <= 0 {
		currentPage = 1
	}

	conditions := []string{"j.job_type = 'project_import_sync'"}
	args := make([]any, 0, 4)
	if status := strings.TrimSpace(stringField(body, "status")); status != "" {
		conditions = append(conditions, "j.status = ?")
		args = append(args, status)
	}
	if uploader := strings.TrimSpace(stringField(body, "uploader")); uploader != "" {
		conditions = append(conditions, "coalesce(nullif(j.created_by_name, ''), nullif(u.nickname, ''), u.username, '') like ?")
		args = append(args, "%"+uploader+"%")
	}
	where := strings.Join(conditions, " and ")

	var total int
	if err := a.DB().QueryRowContext(r.Context(),
		`select count(*)
		   from biz_platform_sync_jobs j
		   left join sys_users u on u.id = j.created_by
		  where `+where,
		args...,
	).Scan(&total); err != nil {
		writeDBError(w, err)
		return
	}

	query := fmt.Sprintf(
		`select j.id, j.status, j.total_count as totalCount,
		        j.success_count as successCount, j.failed_count as failedCount,
		        j.current_resource_name as currentStage, j.message,
		        coalesce(nullif(j.created_by_name, ''), nullif(u.nickname, ''), u.username, '未知用户') as uploader,
		        cast(unix_timestamp(j.started_at) * 1000 as unsigned) as startedAt,
		        cast(unix_timestamp(j.finished_at) * 1000 as unsigned) as finishedAt,
		        cast(unix_timestamp(j.updated_at) * 1000 as unsigned) as updatedAt
		   from biz_platform_sync_jobs j
		   left join sys_users u on u.id = j.created_by
		  where %s
		  order by j.id desc
		  limit ? offset ?`,
		where,
	)
	queryArgs := append(args, pageSize, (currentPage-1)*pageSize)
	rows, err := a.queryMaps(r.Context(), query, queryArgs...)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	writeOK(w, tableData{
		List:        rows,
		Total:       total,
		PageSize:    pageSize,
		CurrentPage: currentPage,
	})
}
