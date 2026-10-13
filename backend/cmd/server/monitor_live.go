package main

import (
	"net/http"
	"strings"
	"time"
)

func monitorPage(body map[string]any) (size, page int) {
	size = intField(body, "pageSize")
	page = intField(body, "currentPage")
	if size <= 0 {
		size = 10
	}
	if size > 100 {
		size = 100
	}
	if page <= 0 {
		page = 1
	}
	maxInt := int(^uint(0) >> 1)
	if page > maxInt/size {
		page = maxInt / size
	}
	return
}

func (a *app) liveOnlineUsers(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	size, page := monitorPage(body)
	cutoff := time.Now().Add(-onlineActivityWindow).Unix()
	now := time.Now().Unix()
	where := ` where s.revoked_at_unix is null and s.expires_at_unix > ?
	  and s.last_seen_unix >= ? and u.status = 1`
	args := []any{now, cutoff}
	if username := strings.TrimSpace(stringField(body, "username")); username != "" {
		where += ` and (s.username like ? or u.nickname like ?)`
		args = append(args, "%"+username+"%", "%"+username+"%")
	}
	var total int
	if err := a.DB().QueryRowContext(r.Context(),
		`select count(*) from sys_sessions s join sys_users u on u.id = s.user_id`+where,
		args...,
	).Scan(&total); err != nil {
		writeDBError(w, err)
		return
	}
	queryArgs := append(append([]any{}, args...), size, (page-1)*size)
	rows, err := a.queryMaps(r.Context(),
		`select s.id, s.username, u.nickname, s.ip, s.address, s.`+"`system`"+` as `+"`system`"+`,
		        s.browser, 1 as status,
		        s.started_at_unix * 1000 as loginTime,
		        s.last_seen_unix * 1000 as lastActiveTime
		   from sys_sessions s join sys_users u on u.id = s.user_id`+where+`
		  order by s.last_seen_unix desc, s.id desc limit ? offset ?`,
		queryArgs...,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	writeOK(w, tableData{List: rows, Total: total, PageSize: size, CurrentPage: page})
}

func (a *app) forceSessionOffline(w http.ResponseWriter, r *http.Request) {
	id := intField(readBody(r), "id")
	if id <= 0 {
		writeError(w, http.StatusOK, 10001, "会话 ID 不能为空")
		return
	}
	result, err := a.DB().ExecContext(r.Context(),
		`update sys_sessions set revoked_at_unix = ?
		 where id = ? and revoked_at_unix is null and expires_at_unix > ?`,
		time.Now().Unix(), id, time.Now().Unix(),
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		writeDBError(w, err)
		return
	}
	if affected == 0 {
		writeError(w, http.StatusOK, 10004, "该会话已下线或不存在")
		return
	}
	writeOK(w, map[string]any{"offline": true})
}

func operationLogFilter(body map[string]any) (string, []any) {
	var clauses []string
	var args []any
	if module := strings.TrimSpace(stringField(body, "module")); module != "" {
		clauses = append(clauses, "module like ?")
		args = append(args, "%"+module+"%")
	}
	if status := strings.TrimSpace(stringField(body, "status")); status == "0" || status == "1" {
		clauses = append(clauses, "status = ?")
		args = append(args, status)
	}
	if start, end, ok := timeRangeField(body, "operatingTime"); ok {
		clauses = append(clauses, "operation_time between ? and ?")
		args = append(args, start, end)
	}
	if len(clauses) == 0 {
		return " where audit_version = 1", nil
	}
	return " where audit_version = 1 and " + strings.Join(clauses, " and "), args
}

func (a *app) filteredOperationLogs(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	size, page := monitorPage(body)
	where, args := operationLogFilter(body)
	var total int
	if err := a.DB().QueryRowContext(r.Context(),
		`select count(*) from sys_operation_logs`+where,
		args...,
	).Scan(&total); err != nil {
		writeDBError(w, err)
		return
	}
	queryArgs := append(append([]any{}, args...), size, (page-1)*size)
	rows, err := a.queryMaps(r.Context(),
		`select id, username, module, summary, ip, address, sys_operation_logs.`+"`system`"+` as `+"`system`"+`,
		        browser, method, status,
		        cast(unix_timestamp(operation_time) * 1000 as unsigned) as operationTime
		   from sys_operation_logs`+where+` order by id desc limit ? offset ?`,
		queryArgs...,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	writeOK(w, tableData{List: rows, Total: total, PageSize: size, CurrentPage: page})
}
