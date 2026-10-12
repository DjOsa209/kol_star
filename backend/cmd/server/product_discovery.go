package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"unicode"
)

const publishedCooperationClause = "c.status = '已发布' and trim(coalesce(c.final_link, '')) <> ''"

func discoveryPage(body map[string]any) (int, int, int) {
	pageSize := intField(body, "pageSize")
	if pageSize < 1 {
		pageSize = 12
	}
	if pageSize > 100 {
		pageSize = 100
	}
	page := intField(body, "currentPage")
	if page < 1 {
		page = 1
	}
	return pageSize, page, (page - 1) * pageSize
}

func discoveryKey(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

// Discovery candidates use the project's configured product-line vocabulary.
// An ambiguous title is left for human review instead of creating a guessed link.
func discoverProductLine(projectName, creativeName string, productLines []string) (string, string) {
	project := matchingProductLines(projectName, productLines)
	creative := matchingProductLines(creativeName, productLines)
	if len(project) == 1 && len(creative) == 1 && discoveryKey(project[0]) != discoveryKey(creative[0]) {
		return "", ""
	}
	if len(creative) == 1 {
		return creative[0], "creative_name"
	}
	if len(project) == 1 {
		return project[0], "project_name"
	}
	return "", ""
}

func matchingProductLines(text string, productLines []string) []string {
	haystack := " " + discoverySearchWords(text) + " "
	var matches []string
	for _, line := range productLines {
		needle := discoverySearchWords(line)
		if needle != "" && strings.Contains(haystack, " "+needle+" ") {
			matches = append(matches, line)
		}
	}
	// A more specific configured label is not ambiguous with its parent label.
	result := make([]string, 0, len(matches))
	for _, candidate := range matches {
		contained := false
		for _, other := range matches {
			if candidate != other && strings.Contains(" "+discoverySearchWords(other)+" ", " "+discoverySearchWords(candidate)+" ") {
				contained = true
				break
			}
		}
		if !contained {
			result = append(result, candidate)
		}
	}
	return result
}

func discoverySearchWords(value string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}), " ")
}

func (a *app) productDiscoverySearch(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	mode := str(body, "mode")
	if mode == "" {
		mode = "resource"
	}
	if mode != "resource" && mode != "brand" && mode != "product" {
		writeError(w, http.StatusOK, 10001, "搜索类型只能是 resource、brand 或 product")
		return
	}
	keyword := strings.TrimSpace(str(body, "keyword"))
	if len([]rune(keyword)) > 128 {
		writeError(w, http.StatusOK, 10001, "关键词过长")
		return
	}
	pageSize, page, offset := discoveryPage(body)
	var countQuery, query string
	var countArgs, queryArgs []any
	like := "%" + keyword + "%"
	switch mode {
	case "resource":
		where := " where r.name like ? or r.platform_handle like ? or r.media_outlet like ?"
		countQuery = "select count(*) from biz_resources r" + where
		countArgs = []any{like, like, like}
		query = `select r.id, r.name, r.resource_type as resourceType, r.platform,
		               r.country, r.avatar_url as avatarUrl, r.platform_handle as platformHandle,
		               (select group_concat(distinct concat(b.name, ' ', p.name) order by b.name, p.name separator ', ')
		                  from biz_cooperation_products cp
		                  join biz_discovery_products p on p.id = cp.product_id
		                  join biz_discovery_brands b on b.id = p.brand_id
		                  join biz_cooperations c on c.id = cp.cooperation_id
		                 where c.resource_id = r.id and cp.status = 'confirmed' and ` + publishedCooperationClause + `) as confirmedProducts,
		               (select count(distinct c.id) from biz_cooperation_products cp
		                  join biz_cooperations c on c.id = cp.cooperation_id
		                 where c.resource_id = r.id and cp.status = 'confirmed' and ` + publishedCooperationClause + `) as contentCount,
		               (select coalesce(sum(x.views), 0) from (
		                  select c.resource_id, c.id, max(greatest(c.views, c.impressions)) as views
		                    from biz_cooperation_products cp join biz_cooperations c on c.id = cp.cooperation_id
		                   where cp.status = 'confirmed' and ` + publishedCooperationClause + `
		                   group by c.resource_id, c.id) x where x.resource_id = r.id) as totalViews
		          from biz_resources r` + where + ` order by r.updated_at desc, r.id desc limit ? offset ?`
		queryArgs = []any{like, like, like, pageSize, offset}
	case "brand":
		countQuery = `select count(*) from biz_discovery_brands b
		               where b.name like ? and exists (
		                 select 1 from biz_discovery_products p
		                 join biz_cooperation_products cp on cp.product_id = p.id
		                 join biz_cooperations c on c.id = cp.cooperation_id
		                 where p.brand_id = b.id and cp.status = 'confirmed' and ` + publishedCooperationClause + `)`
		countArgs = []any{like}
		query = `select b.id, b.name,
		              (select count(distinct p2.id)
		                 from biz_discovery_products p2
		                 join biz_cooperation_products cp2 on cp2.product_id = p2.id and cp2.status = 'confirmed'
		                 join biz_cooperations c2 on c2.id = cp2.cooperation_id
		                where p2.brand_id = b.id and c2.status = '已发布' and trim(coalesce(c2.final_link, '')) <> '') as productCount,
		              count(distinct linked.resource_id) as resourceCount,
		              count(linked.cooperation_id) as contentCount,
		              coalesce(sum(linked.views), 0) as totalViews
		         from biz_discovery_brands b
		         join (select p.brand_id, c.id as cooperation_id, c.resource_id, max(greatest(c.views, c.impressions)) as views
		                 from biz_discovery_products p
		                 join biz_cooperation_products cp on cp.product_id = p.id and cp.status = 'confirmed'
		                 join biz_cooperations c on c.id = cp.cooperation_id and ` + publishedCooperationClause + `
		                group by p.brand_id, c.id, c.resource_id) linked on linked.brand_id = b.id
		        where b.name like ? group by b.id, b.name
		        order by contentCount desc, b.id desc limit ? offset ?`
		queryArgs = []any{like, pageSize, offset}
	case "product":
		countQuery = `select count(*) from biz_discovery_products p
		               join biz_discovery_brands b on b.id = p.brand_id
		              where (p.name like ? or b.name like ?) and exists (
		                select 1 from biz_cooperation_products cp
		                join biz_cooperations c on c.id = cp.cooperation_id
		                where cp.product_id = p.id and cp.status = 'confirmed' and ` + publishedCooperationClause + `)`
		countArgs = []any{like, like}
		query = `select p.id, p.name, b.id as brandId, b.name as brandName,
		                count(distinct c.resource_id) as resourceCount,
		                count(distinct c.id) as contentCount,
		                coalesce(sum(greatest(c.views, c.impressions)), 0) as totalViews,
		                coalesce(max(nullif(c.content_cover_url, '')), max(nullif(c.content_cover_remote_url, '')), '') as coverUrl
		           from biz_discovery_products p
		           join biz_discovery_brands b on b.id = p.brand_id
		           join biz_cooperation_products cp on cp.product_id = p.id and cp.status = 'confirmed'
		           join biz_cooperations c on c.id = cp.cooperation_id and ` + publishedCooperationClause + `
		          where p.name like ? or b.name like ?
		          group by p.id, p.name, b.id, b.name
		          order by contentCount desc, p.id desc limit ? offset ?`
		queryArgs = []any{like, like, pageSize, offset}
	}
	var total int
	if err := a.DB().QueryRowContext(r.Context(), countQuery, countArgs...).Scan(&total); err != nil {
		writeDBError(w, err)
		return
	}
	rows, err := a.queryMaps(r.Context(), query, queryArgs...)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, tableData{List: rows, Total: total, PageSize: pageSize, CurrentPage: page})
}

func (a *app) productDiscoveryProductDetail(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	id := intField(body, "productId")
	if id <= 0 {
		writeError(w, http.StatusOK, 10001, "产品 id 不能为空")
		return
	}
	products, err := a.queryMaps(r.Context(), `select p.id, p.name, b.id as brandId, b.name as brandName
		from biz_discovery_products p join biz_discovery_brands b on b.id = p.brand_id
		where p.id = ? and exists (
		  select 1 from biz_cooperation_products cp
		  join biz_cooperations c on c.id = cp.cooperation_id
		  where cp.product_id = p.id and cp.status = 'confirmed' and `+publishedCooperationClause+`)`, id)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if len(products) == 0 {
		writeError(w, http.StatusOK, 10004, "产品不存在或已无有效发布证据")
		return
	}
	rows, err := a.queryMaps(r.Context(), `select c.id as cooperationId, c.project_id as projectId, project.name as projectName,
		r.id as resourceId, r.name as resourceName, r.resource_type as resourceType,
		coalesce(nullif(c.content_platform, ''), r.platform) as platform,
		c.final_link as contentUrl,
		coalesce(nullif(c.content_cover_url, ''), nullif(c.content_cover_remote_url, ''), '') as coverUrl,
		c.views, c.impressions, c.engagement_count as engagementCount,
		cp.source_kind as sourceKind, cp.source_field as sourceField,
		cp.source_text as sourceText, cp.evidence_url as reviewedEvidenceUrl,
		cp.confirmed_by as confirmedBy,
		coalesce(nullif(reviewer.nickname, ''), reviewer.username, '') as confirmedByName,
		date_format(cp.confirmed_at, '%Y-%m-%d %H:%i') as confirmedAt,
		date_format(coalesce(c.publish_time, cast(c.release_date as datetime)), '%Y-%m-%d %H:%i:%s') as publishTime
		from biz_cooperation_products cp
		join biz_cooperations c on c.id = cp.cooperation_id
		join biz_projects project on project.id = c.project_id
		join biz_resources r on r.id = c.resource_id
		left join sys_users reviewer on reviewer.id = cp.confirmed_by
		where cp.product_id = ? and cp.status = 'confirmed' and `+publishedCooperationClause+`
		order by coalesce(c.publish_time, cast(c.release_date as datetime), c.updated_at) desc, c.id desc`, id)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"product": products[0], "cooperations": rows})
}

func (a *app) refreshProductDiscoveryCandidates(ctx context.Context) (int, int, error) {
	options, err := a.standardImportOptions(ctx)
	if err != nil {
		return 0, 0, err
	}
	rows, err := a.DB().QueryContext(ctx, `select c.id, p.name, c.creative_name, c.final_link
		from biz_cooperations c join biz_projects p on p.id = c.project_id
		where `+publishedCooperationClause+` order by c.id`)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	scanned, created := 0, 0
	for rows.Next() {
		var cooperationID int64
		var projectName, creativeName, evidenceURL string
		if err := rows.Scan(&cooperationID, &projectName, &creativeName, &evidenceURL); err != nil {
			return scanned, created, err
		}
		scanned++
		productName, sourceField := discoverProductLine(projectName, creativeName, options["projectProductLine"])
		if productName == "" {
			continue
		}
		sourceText := projectName
		if sourceField == "creative_name" {
			sourceText = creativeName
		}
		if sourceRunes := []rune(sourceText); len(sourceRunes) > 512 {
			sourceText = string(sourceRunes[:512])
		}
		result, err := a.DB().ExecContext(ctx, `insert ignore into biz_cooperation_products
			(cooperation_id, candidate_name, candidate_key, source_field, source_text, evidence_url)
			values (?, ?, ?, ?, ?, ?)`, cooperationID, productName, discoveryKey(productName), sourceField, sourceText, evidenceURL)
		if err != nil {
			return scanned, created, err
		}
		if affected, err := result.RowsAffected(); err == nil {
			created += int(affected)
		}
	}
	return scanned, created, rows.Err()
}

func (a *app) productDiscoveryCandidatesRefresh(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.discoveryReviewer(w, r); !ok {
		return
	}
	scanned, created, err := a.refreshProductDiscoveryCandidates(r.Context())
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"scanned": scanned, "created": created})
}

func (a *app) productDiscoveryCandidatesList(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.discoveryReviewer(w, r); !ok {
		return
	}
	body := readBody(r)
	status := str(body, "status")
	if status == "" {
		status = "pending"
	}
	if status != "pending" && status != "confirmed" && status != "rejected" {
		writeError(w, http.StatusOK, 10001, "无效的审核状态")
		return
	}
	pageSize, page, offset := discoveryPage(body)
	var total int
	if err := a.DB().QueryRowContext(r.Context(), `select count(*)
		from biz_cooperation_products cp
		join biz_cooperations c on c.id = cp.cooperation_id
		join biz_projects project on project.id = c.project_id
		join biz_resources resource on resource.id = c.resource_id
		where cp.status = ?`, status).Scan(&total); err != nil {
		writeDBError(w, err)
		return
	}
	rows, err := a.queryMaps(r.Context(), `select cp.id, cp.cooperation_id as cooperationId,
		project.name as projectName, resource.name as resourceName,
		resource.id as resourceId, cp.candidate_name as suggestedProduct,
		cp.source_kind as sourceKind, cp.source_field as sourceField,
		cp.source_text as sourceText, c.final_link as evidenceUrl,
		cp.status, p.name as productName, b.name as brandName,
		cp.confirmed_by as confirmedBy,
		cast(unix_timestamp(cp.confirmed_at) * 1000 as unsigned) as confirmedAt
		from biz_cooperation_products cp
		join biz_cooperations c on c.id = cp.cooperation_id
		join biz_projects project on project.id = c.project_id
		join biz_resources resource on resource.id = c.resource_id
		left join biz_discovery_products p on p.id = cp.product_id
		left join biz_discovery_brands b on b.id = p.brand_id
		where cp.status = ? order by cp.created_at desc, cp.id desc limit ? offset ?`, status, pageSize, offset)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, tableData{List: rows, Total: total, PageSize: pageSize, CurrentPage: page})
}

func (a *app) productDiscoveryCandidateDecision(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.discoveryReviewer(w, r)
	if !ok {
		return
	}
	body := readBody(r)
	id := intField(body, "candidateId")
	decision := str(body, "decision")
	if id <= 0 || (decision != "confirmed" && decision != "rejected") {
		writeError(w, http.StatusOK, 10001, "候选 id 或审核结果无效")
		return
	}
	brandName, productName := strings.TrimSpace(str(body, "brandName")), strings.TrimSpace(str(body, "productName"))
	if decision == "confirmed" && (brandName == "" || productName == "" || len([]rune(brandName)) > 128 || len([]rune(productName)) > 128) {
		writeError(w, http.StatusOK, 10001, "确认时需填写有效的品牌和产品名称")
		return
	}
	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()
	var cooperationID int64
	if err := tx.QueryRowContext(r.Context(), `select cooperation_id from biz_cooperation_products where id = ? for update`, id).Scan(&cooperationID); err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusOK, 10004, "候选不存在")
		} else {
			writeDBError(w, err)
		}
		return
	}
	if decision == "confirmed" {
		var eligible int
		if err := tx.QueryRowContext(r.Context(), `select count(*) from biz_cooperations c where c.id = ? and `+publishedCooperationClause, cooperationID).Scan(&eligible); err != nil {
			writeDBError(w, err)
			return
		}
		if eligible == 0 {
			writeError(w, http.StatusOK, 10001, "仅已发布且有作品链接的合作可确认产品")
			return
		}
		brandID, err := discoveryUpsertID(r.Context(), tx, `insert into biz_discovery_brands (name, name_key) values (?, ?)
			on duplicate key update id = last_insert_id(id)`, brandName, discoveryKey(brandName))
		if err != nil {
			writeDBError(w, err)
			return
		}
		productID, err := discoveryUpsertID(r.Context(), tx, `insert into biz_discovery_products (brand_id, name, name_key) values (?, ?, ?)
			on duplicate key update id = last_insert_id(id)`, brandID, productName, discoveryKey(productName))
		if err != nil {
			writeDBError(w, err)
			return
		}
		var duplicate int
		if err := tx.QueryRowContext(r.Context(), `select count(*) from biz_cooperation_products
			where cooperation_id = ? and product_id = ? and id <> ?`, cooperationID, productID, id).Scan(&duplicate); err != nil {
			writeDBError(w, err)
			return
		}
		if duplicate > 0 {
			writeError(w, http.StatusOK, 10001, "该合作已关联此产品")
			return
		}
		_, err = tx.ExecContext(r.Context(), `update biz_cooperation_products
			set status = 'confirmed', product_id = ?, confirmed_by = ?, confirmed_at = current_timestamp
			where id = ?`, productID, userID, id)
	} else {
		_, err = tx.ExecContext(r.Context(), `update biz_cooperation_products
			set status = 'rejected', product_id = null, confirmed_by = null, confirmed_at = null
			where id = ?`, id)
	}
	if err != nil {
		writeDBError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"updated": true})
}

// The resource library may be readable by other teams. Only operators and
// administrators can turn inferred product mentions into public facts.
func (a *app) discoveryReviewer(w http.ResponseWriter, r *http.Request) (int, bool) {
	userID, ok := a.currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, 401, "未登录或登录已过期")
		return 0, false
	}
	roles, err := a.roleCodes(r.Context(), userID)
	if err != nil {
		writeDBError(w, err)
		return 0, false
	}
	if !contains(roles, "admin") && !contains(roles, "operation") {
		writeError(w, http.StatusForbidden, 403, "仅运营或管理员可核实产品合作")
		return 0, false
	}
	return userID, true
}

func discoveryUpsertID(ctx context.Context, tx *sql.Tx, query string, args ...any) (int64, error) {
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("unable to resolve catalog id: %w", err)
	}
	if id <= 0 {
		return 0, fmt.Errorf("unable to resolve catalog id: invalid id")
	}
	return id, nil
}
