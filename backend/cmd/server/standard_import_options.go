package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

var standardImportFieldLabels = map[string]string{
	"resourceType":       "类型",
	"category":           "领域",
	"collaboratorTier":   "层级",
	"platform":           "平台",
	"cooperationType":    "合作类型",
	"contentType":        "内容类型",
	"projectDivision":    "项目一级分类",
	"projectProductLine": "项目产品线",
}

func (a *app) businessProjectNameOptions(w http.ResponseWriter, r *http.Request) {
	options, err := a.standardImportOptions(r.Context())
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{
		"divisions":    options["projectDivision"],
		"productLines": options["projectProductLine"],
	})
}

var systemCalculatedStandardImportFields = map[string]bool{
	"collaboratorTier": true,
}

func (a *app) standardImportOptions(ctx context.Context) (map[string][]string, error) {
	if err := a.ensureStandardImportOptions(ctx); err != nil {
		return nil, err
	}
	rows, err := a.DB().QueryContext(ctx,
		`select field_key, value
		   from biz_standard_import_options
		  where status = '启用'
		  order by field_key, sort_order, id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := cloneStandardImportOptions(defaultStandardProjectImportOptions)
	for field := range result {
		result[field] = nil
	}
	for rows.Next() {
		var field, value string
		if err := rows.Scan(&field, &value); err != nil {
			return nil, err
		}
		if _, allowed := standardImportFieldLabels[field]; allowed {
			result[field] = append(result[field], value)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for field, defaults := range defaultStandardProjectImportOptions {
		if len(result[field]) == 0 {
			result[field] = append([]string(nil), defaults...)
		}
	}
	return result, nil
}

func (a *app) ensureStandardImportOptions(ctx context.Context) error {
	if _, err := a.DB().ExecContext(ctx,
		`create table if not exists biz_standard_import_options (
		  id bigint primary key auto_increment,
		  field_key varchar(64) not null,
		  value varchar(128) not null,
		  status varchar(16) not null default '启用',
		  source varchar(32) not null default '系统预置',
		  sort_order int not null default 100,
		  created_at datetime not null default current_timestamp,
		  updated_at datetime not null default current_timestamp on update current_timestamp,
		  unique key uk_biz_standard_import_option (field_key, value),
		  index idx_biz_standard_import_option_status (field_key, status, sort_order)
		)`,
	); err != nil {
		return err
	}
	for field, values := range defaultStandardProjectImportOptions {
		for index, value := range values {
			source := "系统预置"
			if systemCalculatedStandardImportFields[field] {
				source = "系统计算"
			}
			if _, err := a.DB().ExecContext(ctx,
				`insert into biz_standard_import_options (field_key, value, status, source, sort_order)
				 values (?, ?, '启用', ?, ?)
				 on duplicate key update updated_at = updated_at`,
				field, value, source, (index+1)*10,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *app) standardImportOptionList(w http.ResponseWriter, r *http.Request) {
	if err := a.ensureStandardImportOptions(r.Context()); err != nil {
		writeDBError(w, err)
		return
	}
	rows, err := a.queryMaps(r.Context(),
		`select id, field_key as fieldKey, value, status, source, sort_order as sortOrder,
		        cast(unix_timestamp(updated_at) * 1000 as unsigned) as updatedAt
		   from biz_standard_import_options
		  where status = '启用'
		  order by field_key, sort_order, id`,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, rows)
}

func (a *app) createStandardImportOption(w http.ResponseWriter, r *http.Request) {
	if err := a.ensureStandardImportOptions(r.Context()); err != nil {
		writeDBError(w, err)
		return
	}
	body := readBody(r)
	field := strings.TrimSpace(str(body, "fieldKey"))
	value := strings.TrimSpace(str(body, "value"))
	if _, allowed := standardImportFieldLabels[field]; !allowed {
		writeError(w, http.StatusOK, 10001, "不支持的标准字段")
		return
	}
	if systemCalculatedStandardImportFields[field] {
		writeError(w, http.StatusOK, 10001, "层级由系统按受众量自动计算，不能手动新增")
		return
	}
	if value == "" {
		writeError(w, http.StatusOK, 10001, "选项值不能为空")
		return
	}
	if len([]rune(value)) > 128 {
		writeError(w, http.StatusOK, 10001, "选项值不能超过 128 个字符")
		return
	}
	sortOrder := intField(body, "sortOrder")
	if sortOrder <= 0 {
		_ = a.DB().QueryRowContext(r.Context(),
			`select coalesce(max(sort_order), 0) + 10 from biz_standard_import_options where field_key = ?`,
			field,
		).Scan(&sortOrder)
	}
	_, err := a.DB().ExecContext(r.Context(),
		`insert into biz_standard_import_options (field_key, value, status, source, sort_order)
		 values (?, ?, '启用', '管理员新增', ?)
		 on duplicate key update status = '启用', sort_order = values(sort_order), updated_at = now()`,
		field, value, sortOrder,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"created": true})
}

func (a *app) updateStandardImportOption(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	id := intField(body, "id")
	value := strings.TrimSpace(str(body, "value"))
	if id <= 0 || value == "" {
		writeError(w, http.StatusOK, 10001, "选项 id 和值不能为空")
		return
	}
	if len([]rune(value)) > 128 {
		writeError(w, http.StatusOK, 10001, "选项值不能超过 128 个字符")
		return
	}
	var field string
	if err := a.DB().QueryRowContext(r.Context(),
		`select field_key from biz_standard_import_options where id = ? and status = '启用'`, id,
	).Scan(&field); err != nil {
		writeError(w, http.StatusOK, 10004, "选项不存在")
		return
	}
	if systemCalculatedStandardImportFields[field] {
		writeError(w, http.StatusOK, 10001, "层级为固定系统规则，不能编辑")
		return
	}
	result, err := a.DB().ExecContext(r.Context(),
		`update biz_standard_import_options
		    set value = ?, sort_order = if(? > 0, ?, sort_order), source = '管理员维护', updated_at = now()
		  where id = ? and status = '启用'`,
		value, intField(body, "sortOrder"), intField(body, "sortOrder"), id,
	)
	if err != nil {
		writeError(w, http.StatusOK, 10001, "该选项已存在")
		return
	}
	affected, _ := result.RowsAffected()
	writeOK(w, map[string]any{"updated": affected > 0})
}

func (a *app) deleteStandardImportOption(w http.ResponseWriter, r *http.Request) {
	id := intField(readBody(r), "id")
	if id <= 0 {
		writeError(w, http.StatusOK, 10001, "选项 id 不能为空")
		return
	}
	var field string
	if err := a.DB().QueryRowContext(r.Context(),
		`select field_key from biz_standard_import_options where id = ? and status = '启用'`, id,
	).Scan(&field); err != nil {
		writeError(w, http.StatusOK, 10004, "选项不存在")
		return
	}
	if systemCalculatedStandardImportFields[field] {
		writeError(w, http.StatusOK, 10001, "层级为固定系统规则，不能删除")
		return
	}
	var count int
	if err := a.DB().QueryRowContext(r.Context(),
		`select count(*) from biz_standard_import_options where field_key = ? and status = '启用'`, field,
	).Scan(&count); err != nil {
		writeDBError(w, err)
		return
	}
	if count <= 1 {
		writeError(w, http.StatusOK, 10001, fmt.Sprintf("%s至少保留一个选项", standardImportFieldLabels[field]))
		return
	}
	_, err := a.DB().ExecContext(r.Context(),
		`update biz_standard_import_options set status = '停用', source = '管理员维护', updated_at = now() where id = ?`, id,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"deleted": true})
}

func cloneStandardImportOptions(source map[string][]string) map[string][]string {
	result := make(map[string][]string, len(source))
	for field, values := range source {
		result[field] = append([]string(nil), values...)
	}
	return result
}
