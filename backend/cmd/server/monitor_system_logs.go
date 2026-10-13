package main

import (
	"net/http"
	"strings"
)

func systemLogFilter(body map[string]any) (string, []any) {
	var clauses []string
	var args []any
	if module := strings.TrimSpace(stringField(body, "module")); module != "" && module != "<nil>" {
		clauses = append(clauses, "module like ?")
		args = append(args, "%"+module+"%")
	}
	if start, end, ok := timeRangeField(body, "requestTime"); ok {
		clauses = append(clauses, "request_time between ? and ?")
		args = append(args, start, end)
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return " where " + strings.Join(clauses, " and "), args
}

func (a *app) filteredSystemLogs(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	size, page := monitorPage(body)
	where, args := systemLogFilter(body)
	var total int
	if err := a.DB().QueryRowContext(r.Context(),
		"select count(*) from sys_system_logs"+where,
		args...,
	).Scan(&total); err != nil {
		writeDBError(w, err)
		return
	}

	queryArgs := append(append([]any{}, args...), size, (page-1)*size)
	rows, err := a.queryMaps(r.Context(),
		`select id, module, url, method, ip, address, sys_system_logs.`+"`system`"+` as `+"`system`"+`, browser,
		        takes_time as takesTime,
		        cast(unix_timestamp(request_time) * 1000 as unsigned) as requestTime
		   from sys_system_logs`+where+` order by id desc limit ? offset ?`,
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
