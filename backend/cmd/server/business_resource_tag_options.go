package main

import "net/http"

func (a *app) businessResourceTagOptions(w http.ResponseWriter, r *http.Request) {
	rows, err := a.queryMaps(r.Context(), `select id,name,category,color,status from biz_tags where status='启用' order by category,id`)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	writeOK(w, rows)
}
