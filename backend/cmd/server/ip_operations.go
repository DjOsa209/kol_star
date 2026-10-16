package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ipResourceInput struct {
	ID                 int64              `json:"id"`
	Name               string             `json:"name"`
	IPType             string             `json:"ipType"`
	RightsOwner        string             `json:"rightsOwner"`
	Contact            string             `json:"contact"`
	Markets            []string           `json:"markets"`
	MarketPenetrations map[string]float64 `json:"marketPenetrations"`
	Audience           string             `json:"audience"`
	Summary            string             `json:"summary"`
	CooperationStatus  string             `json:"cooperationStatus"`
	Currency           string             `json:"currency"`
	PriceMin           *float64           `json:"priceMin"`
	PriceMax           *float64           `json:"priceMax"`
	LicenseNotes       string             `json:"licenseNotes"`
	Profile            map[string]any     `json:"profile"`
}

type ipRequestInput struct {
	ID                     int64          `json:"id"`
	ProjectName            string         `json:"projectName"`
	Department             string         `json:"department"`
	Markets                []string       `json:"markets"`
	ExpectedLaunch         string         `json:"expectedLaunch"`
	Goal                   string         `json:"goal"`
	Description            string         `json:"description"`
	BudgetCurrency         string         `json:"budgetCurrency"`
	BudgetMin              *float64       `json:"budgetMin"`
	BudgetMax              *float64       `json:"budgetMax"`
	ExternalRecommendation string         `json:"externalRecommendation"`
	CandidateIDs           []int64        `json:"candidateIds"`
	Submit                 bool           `json:"submit"`
	Brief                  map[string]any `json:"brief"`
}

func readIPJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, 400, "请求数据无效")
		return false
	}
	return true
}

func ipJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func ipValidRange(min, max *float64) bool {
	return (min == nil || *min >= 0) && (max == nil || *max >= 0) &&
		(min == nil || max == nil || *min <= *max)
}

func ipMarketPenetrationIssue(markets []string, percentages map[string]float64) string {
	selected := make(map[string]bool, len(markets))
	for _, market := range markets {
		market = strings.TrimSpace(market)
		if market == "" {
			return "覆盖市场不能留空"
		}
		if selected[market] {
			return "覆盖市场不能重复"
		}
		selected[market] = true
	}
	for market, percentage := range percentages {
		if market == "" || market != strings.TrimSpace(market) || !selected[market] {
			return "渗透率的市场必须已选择为覆盖市场"
		}
		if math.IsNaN(percentage) || math.IsInf(percentage, 0) || percentage < 0 || percentage > 100 {
			return "市场渗透率必须在0至100之间"
		}
	}
	return ""
}

func ipMarketPenetrationsJSON(percentages map[string]float64) string {
	if percentages == nil {
		percentages = map[string]float64{}
	}
	return ipJSON(percentages)
}

func ipSameMarketSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[string]int, len(left))
	for _, market := range left {
		counts[market]++
	}
	for _, market := range right {
		if counts[market] == 0 {
			return false
		}
		counts[market]--
	}
	return true
}

func ipMarketPenetrationsUpdateValue(percentages map[string]float64, markets []string, previousMarkets string) any {
	if percentages != nil {
		return ipMarketPenetrationsJSON(percentages)
	}
	// Older clients do not send the field. Keep percentages only when their
	// associated market list is unchanged; otherwise clear orphaned values.
	var previous []string
	if json.Unmarshal([]byte(previousMarkets), &previous) == nil && ipSameMarketSet(previous, markets) {
		return nil
	}
	return "{}"
}

func ipMarketPenetrationsResponse(value any) map[string]float64 {
	result := map[string]float64{}
	if raw, ok := value.(string); ok && raw != "" {
		_ = json.Unmarshal([]byte(raw), &result)
	}
	if result == nil {
		return map[string]float64{}
	}
	return result
}

func ipExternalCount(brief map[string]any) int {
	items, ok := brief["externalIPs"].([]any)
	if !ok {
		return 0
	}
	return len(items)
}

func ipExternalValid(brief map[string]any) bool {
	items, ok := brief["externalIPs"].([]any)
	if !ok {
		return true
	}
	for _, item := range items {
		candidate, ok := item.(map[string]any)
		if !ok || ipMapText(candidate, "name") == "" || ipMapText(candidate, "rightsOwner") == "" || !ipHTTPURL(ipMapText(candidate, "imageUrl")) || ipMapText(candidate, "reason") == "" || ipMapText(candidate, "productEvaluation") == "" || ipMapText(candidate, "technicalFit") == "" {
			return false
		}
	}
	return true
}

func ipHTTPURL(raw string) bool {
	parsed, err := url.ParseRequestURI(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != "" && parsed.User == nil
}

func ipMapText(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func ipBriefValid(brief map[string]any, candidateIDs []int64) bool {
	for _, key := range []string{"owner", "projectPeriod", "linkedProduct", "cooperationMode", "targetIPType", "targetIPCount"} {
		if ipMapText(brief, key) == "" {
			return false
		}
	}
	if stages, present := brief["contactsByStage"]; present {
		contacts, ok := stages.(map[string]any)
		if !ok {
			return false
		}
		for _, stage := range []string{"product", "ipMarketing", "imc", "brand", "user", "media", "pr"} {
			contact, ok := contacts[stage].(map[string]any)
			if !ok || ipMapText(contact, "name") == "" {
				return false
			}
		}
	} else if ipMapText(brief, "contacts") == "" {
		// Historical drafts used one free-text contacts field.
		return false
	}
	evaluations, _ := brief["candidateEvaluations"].(map[string]any)
	for _, id := range candidateIDs {
		item, _ := evaluations[strconv.FormatInt(id, 10)].(map[string]any)
		for _, key := range []string{"reason", "productEvaluation", "technicalFit"} {
			if ipMapText(item, key) == "" {
				return false
			}
		}
	}
	return true
}

func ipWeightedScore(assessment map[string]any) (float64, bool) {
	dimensions, ok := assessment["scoreDimensions"].(map[string]any)
	if !ok {
		return 0, false
	}
	weighted := 0.0
	for _, field := range []struct {
		Key    string
		Weight float64
	}{
		{"audienceMatch", 0.3},
		{"marketCoverage", 0.25},
		{"scheduleAvailability", 0.25},
		{"licenseRisk", 0.2},
	} {
		score, ok := dimensions[field.Key].(float64)
		if !ok || math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score > 100 {
			return 0, false
		}
		weighted += score * field.Weight
	}
	return math.Round(weighted), true
}

const ipCompletenessSQL = `(case when rights_owner<>'' then 1 else 0 end +
	case when coalesce(summary,'')<>'' then 1 else 0 end +
	case when price_min is not null and price_max is not null then 1 else 0 end +
	case when coalesce(json_unquote(json_extract(profile,'$.lifecycle')),'')<>'' then 1 else 0 end +
	case when coalesce(json_unquote(json_extract(profile,'$.genderRatio')),'')<>'' then 1 else 0 end +
	case when coalesce(json_unquote(json_extract(profile,'$.ageRange')),'')<>'' then 1 else 0 end +
	case when exists (select 1 from biz_ip_files f where f.ip_id=biz_ip_resources.id and f.file_kind='copyright') then 1 else 0 end +
	case when exists (select 1 from biz_ip_files f where f.ip_id=biz_ip_resources.id and f.file_kind='visual') then 1 else 0 end)`

func (a *app) ipResourcesList(w http.ResponseWriter, r *http.Request) {
	var filter struct {
		Keyword      string `json:"keyword"`
		IPType       string `json:"ipType"`
		Status       string `json:"status"`
		Market       string `json:"market"`
		Completeness string `json:"completeness"`
		UpdatedFrom  string `json:"updatedFrom"`
		UpdatedTo    string `json:"updatedTo"`
		Page         int    `json:"page"`
		PageSize     int    `json:"pageSize"`
	}
	if !readIPJSON(w, r, &filter) {
		return
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 200 {
		filter.PageSize = 20
	}
	where := " where 1=1"
	args := []any{}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		where += " and (name like ? or rights_owner like ? or markets like ?)"
		term := "%" + keyword + "%"
		args = append(args, term, term, term)
	}
	if filter.IPType != "" {
		where += " and ip_type = ?"
		args = append(args, filter.IPType)
	}
	if filter.Status != "" {
		where += " and cooperation_status = ?"
		args = append(args, filter.Status)
	}
	if filter.Market != "" {
		where += " and markets like ?"
		args = append(args, "%"+strings.TrimSpace(filter.Market)+"%")
	}
	if filter.Completeness == "complete" {
		where += " and " + ipCompletenessSQL + " = 8"
	} else if filter.Completeness == "incomplete" {
		where += " and " + ipCompletenessSQL + " < 8"
	}
	if filter.UpdatedFrom != "" {
		where += " and date(updated_at) >= ?"
		args = append(args, filter.UpdatedFrom)
	}
	if filter.UpdatedTo != "" {
		where += " and date(updated_at) <= ?"
		args = append(args, filter.UpdatedTo)
	}
	var total int
	if err := a.DB().QueryRowContext(r.Context(), "select count(*) from biz_ip_resources"+where, args...).Scan(&total); err != nil {
		writeDBError(w, err)
		return
	}
	listArgs := append(append([]any{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := a.queryMaps(r.Context(), `select id, name, ip_type as ipType, rights_owner as rightsOwner,
		markets, market_penetrations as marketPenetrations, audience, summary, cooperation_status as cooperationStatus, currency,
		price_min as priceMin, price_max as priceMax,
		created_by as uploadedBy,
		coalesce((select coalesce(nullif(u.nickname,''), u.username) from sys_users u where u.id=biz_ip_resources.created_by), '') as uploaderName,
		(select f.id from biz_ip_files f where f.ip_id=biz_ip_resources.id and f.file_kind='visual' order by f.id desc limit 1) as visualFileId,
		(`+ipCompletenessSQL+` * 12.5) as completeness,
		date_format(updated_at, '%Y-%m-%d %H:%i') as updatedAt
		from biz_ip_resources`+where+` order by updated_at desc, id desc limit ? offset ?`, listArgs...)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	for _, row := range rows {
		row["marketPenetrations"] = ipMarketPenetrationsResponse(row["marketPenetrations"])
	}
	var allResources, cooperable, pendingFiles, recent int
	if err := a.DB().QueryRowContext(r.Context(), `select
		count(*),
		count(case when cooperation_status='可合作' then 1 end),
		count(case when not exists (select 1 from biz_ip_files f where f.ip_id=r.id and f.file_kind='copyright') then 1 end),
		count(case when created_at >= date_sub(now(), interval 30 day) then 1 end)
		from biz_ip_resources r`).Scan(&allResources, &cooperable, &pendingFiles, &recent); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"list": rows, "total": total, "pageSize": filter.PageSize,
		"currentPage": filter.Page, "stats": map[string]int{"total": allResources, "cooperable": cooperable, "pendingFiles": pendingFiles, "recent": recent}})
}

func (a *app) ipResourceSave(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, 401, "未登录或登录已过期")
		return
	}
	var item ipResourceInput
	if !readIPJSON(w, r, &item) {
		return
	}
	item.Name = strings.TrimSpace(item.Name)
	item.IPType = strings.TrimSpace(item.IPType)
	item.RightsOwner = strings.TrimSpace(item.RightsOwner)
	if item.Name == "" || item.IPType == "" || len(item.Markets) == 0 || len(item.Name) > 255 || !ipValidRange(item.PriceMin, item.PriceMax) {
		writeError(w, http.StatusBadRequest, 400, "请填写IP名称、类型、覆盖市场和有效价格范围")
		return
	}
	if issue := ipMarketPenetrationIssue(item.Markets, item.MarketPenetrations); issue != "" {
		writeError(w, http.StatusBadRequest, 400, issue)
		return
	}
	if item.Currency == "" {
		item.Currency = "CNY"
	}
	if item.CooperationStatus == "" {
		item.CooperationStatus = "待评估"
	}
	var id int64
	if item.ID > 0 {
		var previousMarkets sql.NullString
		err := a.DB().QueryRowContext(r.Context(), "select markets from biz_ip_resources where id=?", item.ID).Scan(&previousMarkets)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, 404, "IP不存在")
			return
		}
		if err != nil {
			writeDBError(w, err)
			return
		}
		result, err := a.DB().ExecContext(r.Context(), `update biz_ip_resources set name=?, ip_type=?, rights_owner=?, contact=?, markets=?, market_penetrations=coalesce(?,market_penetrations), audience=?, summary=?, cooperation_status=?, currency=?, price_min=?, price_max=?, license_notes=?, profile=? where id=?`,
			item.Name, item.IPType, item.RightsOwner, item.Contact, ipJSON(item.Markets), ipMarketPenetrationsUpdateValue(item.MarketPenetrations, item.Markets, previousMarkets.String), item.Audience, item.Summary, item.CooperationStatus, item.Currency, item.PriceMin, item.PriceMax, item.LicenseNotes, ipJSON(item.Profile), item.ID)
		if err != nil {
			writeIPSaveError(w, err)
			return
		}
		_ = result
		id = item.ID
	} else {
		result, err := a.DB().ExecContext(r.Context(), `insert into biz_ip_resources (name,ip_type,rights_owner,contact,markets,market_penetrations,audience,summary,cooperation_status,currency,price_min,price_max,license_notes,profile,created_by) values (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			item.Name, item.IPType, item.RightsOwner, item.Contact, ipJSON(item.Markets), ipMarketPenetrationsJSON(item.MarketPenetrations), item.Audience, item.Summary, item.CooperationStatus, item.Currency, item.PriceMin, item.PriceMax, item.LicenseNotes, ipJSON(item.Profile), userID)
		if err != nil {
			writeIPSaveError(w, err)
			return
		}
		id, _ = result.LastInsertId()
	}
	writeOK(w, map[string]any{"id": id})
}

func writeIPSaveError(w http.ResponseWriter, err error) {
	if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		writeError(w, http.StatusConflict, 409, "相同名称和版权方的IP已存在")
		return
	}
	writeDBError(w, err)
}

func (a *app) ipResourceDetail(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if id <= 0 {
		writeError(w, http.StatusBadRequest, 400, "缺少IP编号")
		return
	}
	rows, err := a.queryMaps(r.Context(), `select id, name, ip_type as ipType, rights_owner as rightsOwner, contact,
		markets, market_penetrations as marketPenetrations, audience, summary, cooperation_status as cooperationStatus, currency,
		price_min as priceMin, price_max as priceMax, license_notes as licenseNotes, profile,
		created_by as uploadedBy,
		coalesce((select coalesce(nullif(u.nickname,''), u.username) from sys_users u where u.id=biz_ip_resources.created_by), '') as uploaderName,
		date_format(created_at, '%Y-%m-%d %H:%i') as createdAt,
		date_format(updated_at, '%Y-%m-%d %H:%i') as updatedAt
		from biz_ip_resources where id=?`, id)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if len(rows) == 0 {
		writeError(w, http.StatusNotFound, 404, "IP不存在")
		return
	}
	rows[0]["marketPenetrations"] = ipMarketPenetrationsResponse(rows[0]["marketPenetrations"])
	cases, err := a.queryMaps(r.Context(), `select id, ip_id as ipId, title, summary,
		date_format(created_at, '%Y-%m-%d') as createdAt from biz_ip_cases where ip_id=? order by id desc`, id)
	if err != nil {
		writeDBError(w, err)
		return
	}
	files, err := a.queryMaps(r.Context(), `select id, ip_id as ipId, case_id as caseId, file_kind as fileKind,
		original_name as originalName, size_bytes as sizeBytes, uploaded_by as uploadedBy,
		coalesce((select coalesce(nullif(u.nickname,''), u.username) from sys_users u where u.id=biz_ip_files.uploaded_by), '') as uploaderName,
		date_format(created_at, '%Y-%m-%d %H:%i') as createdAt from biz_ip_files where ip_id=? order by id desc`, id)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if cases == nil {
		cases = []map[string]any{}
	}
	if files == nil {
		files = []map[string]any{}
	}
	writeOK(w, map[string]any{"resource": rows[0], "cases": cases, "files": files})
}

func (a *app) ipCaseSave(w http.ResponseWriter, r *http.Request) {
	var item struct {
		IPID    int64  `json:"ipId"`
		Title   string `json:"title"`
		Summary string `json:"summary"`
	}
	if !readIPJSON(w, r, &item) {
		return
	}
	item.Title = strings.TrimSpace(item.Title)
	if item.IPID <= 0 || item.Title == "" || len(item.Title) > 255 {
		writeError(w, http.StatusBadRequest, 400, "请填写合作案例标题")
		return
	}
	var exists int
	if err := a.DB().QueryRowContext(r.Context(), "select count(*) from biz_ip_resources where id=?", item.IPID).Scan(&exists); err != nil {
		writeDBError(w, err)
		return
	}
	if exists == 0 {
		writeError(w, http.StatusNotFound, 404, "IP不存在")
		return
	}
	result, err := a.DB().ExecContext(r.Context(), "insert into biz_ip_cases (ip_id,title,summary) values (?,?,?)", item.IPID, item.Title, item.Summary)
	if err != nil {
		writeDBError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	writeOK(w, map[string]any{"id": id})
}

var ipFileRoot = filepath.Join("uploads", "ip-documents")

func validIPDisplayFilename(name, currentExtension string) bool {
	if name == "" || name != strings.TrimSpace(name) || len(name) > 255 || name == "." || name == ".." ||
		strings.ContainsAny(name, `/\`) || strings.IndexFunc(name, func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
		return false
	}
	ext := strings.ToLower(filepath.Ext(name))
	return ext != "" && ext == strings.ToLower(currentExtension) && strings.TrimSpace(strings.TrimSuffix(name, filepath.Ext(name))) != ""
}

func (a *app) ipFileUpload(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, 401, "未登录或登录已过期")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 22<<20)
	if err := r.ParseMultipartForm(22 << 20); err != nil {
		writeError(w, http.StatusBadRequest, 400, "文件不能超过20MB")
		return
	}
	ipID, _ := strconv.ParseInt(r.FormValue("ipId"), 10, 64)
	caseID, _ := strconv.ParseInt(r.FormValue("caseId"), 10, 64)
	kind := r.FormValue("fileKind")
	if ipID <= 0 || (kind != "copyright" && kind != "case" && kind != "visual") || (kind == "case" && caseID <= 0) {
		writeError(w, http.StatusBadRequest, 400, "附件关联信息无效")
		return
	}
	var exists int
	query := "select count(*) from biz_ip_resources where id=?"
	args := []any{ipID}
	if kind == "case" {
		query = "select count(*) from biz_ip_cases where id=? and ip_id=?"
		args = []any{caseID, ipID}
	}
	if err := a.DB().QueryRowContext(r.Context(), query, args...).Scan(&exists); err != nil {
		writeDBError(w, err)
		return
	}
	if exists == 0 {
		writeError(w, http.StatusNotFound, 404, "关联IP或合作案例不存在")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, 400, "请选择PDF文件")
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	maxSize := int64(20 << 20)
	if kind == "visual" {
		maxSize = 5 << 20
	}
	if header.Size <= 0 || header.Size > maxSize || !validIPDisplayFilename(header.Filename, ext) || (kind == "visual" && ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp") || (kind != "visual" && ext != ".pdf") {
		writeError(w, http.StatusBadRequest, 400, "文件类型或大小不符合要求")
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxSize+1))
	contentType := http.DetectContentType(data)
	validContent := kind != "visual" && len(data) >= 5 && string(data[:5]) == "%PDF-"
	if kind == "visual" {
		validContent = (ext == ".png" && contentType == "image/png") || ((ext == ".jpg" || ext == ".jpeg") && contentType == "image/jpeg") || (ext == ".webp" && contentType == "image/webp")
	}
	if err != nil || int64(len(data)) > maxSize || !validContent {
		writeError(w, http.StatusBadRequest, 400, "文件内容无效")
		return
	}
	if err := os.MkdirAll(ipFileRoot, 0700); err != nil {
		writeDBError(w, err)
		return
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		writeDBError(w, err)
		return
	}
	storageName := hex.EncodeToString(random) + ext
	path := filepath.Join(ipFileRoot, storageName)
	if err := os.WriteFile(path, data, 0600); err != nil {
		writeDBError(w, err)
		return
	}
	var nullableCase any
	if kind == "case" {
		nullableCase = caseID
	}
	result, err := a.DB().ExecContext(r.Context(), `insert into biz_ip_files (ip_id,case_id,file_kind,original_name,storage_name,size_bytes,uploaded_by) values (?,?,?,?,?,?,?)`,
		ipID, nullableCase, kind, filepath.Base(header.Filename), storageName, len(data), userID)
	if err != nil {
		_ = os.Remove(path)
		writeDBError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	writeOK(w, map[string]any{"id": id})
}

func (a *app) ipFileRename(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID           int64  `json:"id"`
		OriginalName string `json:"originalName"`
	}
	if !readIPJSON(w, r, &input) {
		return
	}
	if input.ID <= 0 {
		writeError(w, http.StatusBadRequest, 400, "缺少文件编号")
		return
	}
	var oldName string
	err := a.DB().QueryRowContext(r.Context(), "select original_name from biz_ip_files where id=?", input.ID).Scan(&oldName)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, 404, "文件不存在")
		return
	}
	if err != nil {
		writeDBError(w, err)
		return
	}
	if !validIPDisplayFilename(input.OriginalName, filepath.Ext(oldName)) {
		writeError(w, http.StatusBadRequest, 400, "文件名无效或文件类型不可更改")
		return
	}
	if _, err := a.DB().ExecContext(r.Context(), "update biz_ip_files set original_name=? where id=?", input.OriginalName, input.ID); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"id": input.ID, "originalName": input.OriginalName})
}

func (a *app) ipColleagues(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > 80 {
		writeError(w, http.StatusBadRequest, 400, "搜索词过长")
		return
	}
	page := 1
	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(w, http.StatusBadRequest, 400, "页码无效")
			return
		}
		page = parsed
	}
	directory := newUACDirectoryClient(a.Config().SSO, nil)
	if query != "" && directory.configured() {
		ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
		people, hasNext, err := directory.search(ctx, query, page)
		cancel()
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"code": 0, "message": "操作成功", "data": people,
				"directorySource": "uac", "hasNext": hasNext,
			})
			return
		}
		log.Printf("[ip-directory] UAC query unavailable: %v", err)
		// Directory permissions or availability can fail independently of XMP.
		// Return the cached SSO accounts and clearly mark this limited result.
	}
	// UAC identities are saved here when colleagues sign in to XMP. Local
	// administrator accounts are not colleagues in the enterprise directory.
	where := " where u.status=1 and u.auth_provider='uac'"
	args := []any{}
	if query != "" {
		query = strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(query)
		where += " and (u.nickname like ? escape '!' or u.username like ? escape '!' or u.employee_no like ? escape '!' or u.department_name like ? escape '!')"
		for i := 0; i < 4; i++ {
			args = append(args, "%"+query+"%")
		}
	}
	args = append(args, (page-1)*uacDirectoryPageSize)
	rows, err := a.queryMaps(r.Context(), `select u.id,
		coalesce(nullif(u.nickname,''),u.username) as name,
		coalesce(nullif(u.department_name,''),nullif(d.name,''),'') as department,
		u.employee_no as employeeNo,
		case when coalesce(u.email,'')='' then 0 else 1 end as notifiable
		from sys_users u left join sys_departments d on d.id=u.dept_id`+where+
		` order by coalesce(nullif(u.nickname,''),u.username),u.id limit 21 offset ?`, args...)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	hasNext := len(rows) > uacDirectoryPageSize
	if hasNext {
		rows = rows[:uacDirectoryPageSize]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0, "message": "操作成功", "data": rows,
		"directorySource": "cache", "hasNext": hasNext,
		"directoryWarning": "企业目录暂不可用，仅显示已登录 XMP 的同事",
	})
}

func (a *app) ipFileRead(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	var storageName, originalName, kind string
	err := a.DB().QueryRowContext(r.Context(), "select storage_name, original_name, file_kind from biz_ip_files where id=?", id).Scan(&storageName, &originalName, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, 404, "文件不存在")
		return
	}
	if err != nil {
		writeDBError(w, err)
		return
	}
	file, err := os.Open(filepath.Join(ipFileRoot, filepath.Base(storageName)))
	if err != nil {
		writeError(w, http.StatusNotFound, 404, "文件不存在")
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		writeDBError(w, err)
		return
	}
	disposition := "inline"
	if r.URL.Query().Get("download") == "1" {
		disposition = "attachment"
	}
	contentType := "application/pdf"
	if kind == "visual" {
		contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(storageName)))
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": originalName}))
	http.ServeContent(w, r, originalName, stat.ModTime(), file)
}

func (a *app) ipFileDelete(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID int64 `json:"id"`
	}
	if !readIPJSON(w, r, &input) {
		return
	}
	var name string
	err := a.DB().QueryRowContext(r.Context(), "select storage_name from biz_ip_files where id=?", input.ID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, 404, "文件不存在")
		return
	}
	if err != nil {
		writeDBError(w, err)
		return
	}
	if _, err := a.DB().ExecContext(r.Context(), "delete from biz_ip_files where id=?", input.ID); err != nil {
		writeDBError(w, err)
		return
	}
	_ = os.Remove(filepath.Join(ipFileRoot, filepath.Base(name)))
	writeOK(w, nil)
}

type ipImportInput struct {
	Rows []ipResourceInput `json:"rows"`
}
type ipImportResult struct {
	Row     int    `json:"row"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func ipImportOptionIssue(row ipResourceInput) string {
	validType := map[string]bool{
		"游戏": true, "动画": true, "影视": true, "综艺": true, "体育": true,
		"潮牌": true, "潮玩": true, "品牌": true, "文化艺术": true, "线下活动": true, "其他": true,
		// Existing spreadsheets and saved IPs can retain their former categories.
		"体育IP": true, "动画/影视": true, "音乐/活动": true, "电竞": true,
	}
	if !validType[strings.TrimSpace(row.IPType)] {
		return "IP类型不在可选范围内"
	}
	for _, market := range row.Markets {
		// The single-IP form allows custom markets. The template's dropdowns are
		// suggestions, so pasted country names and other nonempty markets remain valid.
		if strings.TrimSpace(market) == "" {
			return "覆盖市场不能留空"
		}
		if strings.IndexFunc(market, func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
			return "覆盖市场包含无效字符"
		}
	}
	if issue := ipMarketPenetrationIssue(row.Markets, row.MarketPenetrations); issue != "" {
		return issue
	}
	status := strings.TrimSpace(row.CooperationStatus)
	if status != "" && status != "可合作" && status != "洽谈中" && status != "待评估" && status != "暂不可合作" {
		return "合作状态不在可选范围内"
	}
	return ""
}

func (a *app) validateIPImport(r *http.Request, rows []ipResourceInput) ([]ipImportResult, error) {
	results := make([]ipImportResult, 0, len(rows))
	seen := map[string]bool{}
	for index, row := range rows {
		result := ipImportResult{Row: index + 2, Name: strings.TrimSpace(row.Name), Status: "ready"}
		key := strings.ToLower(result.Name + "\x00" + strings.TrimSpace(row.RightsOwner))
		switch {
		case result.Name == "" || strings.TrimSpace(row.IPType) == "" || len(row.Markets) == 0:
			result.Status, result.Message = "error", "缺少IP名称、类型或覆盖市场"
		case ipImportOptionIssue(row) != "":
			result.Status, result.Message = "error", ipImportOptionIssue(row)
		case !ipValidRange(row.PriceMin, row.PriceMax):
			result.Status, result.Message = "error", "价格范围无效"
		case seen[key]:
			result.Status, result.Message = "duplicate", "文件内重复"
		default:
			var count int
			if err := a.DB().QueryRowContext(r.Context(), "select count(*) from biz_ip_resources where name=? and rights_owner=?", result.Name, strings.TrimSpace(row.RightsOwner)).Scan(&count); err != nil {
				return nil, err
			}
			if count > 0 {
				result.Status, result.Message = "duplicate", "资源库已有相同IP"
			}
		}
		seen[key] = true
		results = append(results, result)
	}
	return results, nil
}

func (a *app) ipImportPreview(w http.ResponseWriter, r *http.Request) {
	var input ipImportInput
	if !readIPJSON(w, r, &input) {
		return
	}
	if len(input.Rows) == 0 || len(input.Rows) > 500 {
		writeError(w, http.StatusBadRequest, 400, "每次导入1至500条IP")
		return
	}
	results, err := a.validateIPImport(r, input.Rows)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, results)
}

func (a *app) ipImport(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, 401, "未登录或登录已过期")
		return
	}
	var input ipImportInput
	if !readIPJSON(w, r, &input) {
		return
	}
	if len(input.Rows) == 0 || len(input.Rows) > 500 {
		writeError(w, http.StatusBadRequest, 400, "每次导入1至500条IP")
		return
	}
	results, err := a.validateIPImport(r, input.Rows)
	if err != nil {
		writeDBError(w, err)
		return
	}
	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()
	imported := 0
	created := []map[string]any{}
	for index, result := range results {
		if result.Status != "ready" {
			continue
		}
		row := input.Rows[index]
		for i := range row.Markets {
			row.Markets[i] = strings.TrimSpace(row.Markets[i])
		}
		if row.Currency == "" {
			row.Currency = "CNY"
		}
		if row.CooperationStatus == "" {
			row.CooperationStatus = "待评估"
		}
		inserted, err := tx.ExecContext(r.Context(), `insert ignore into biz_ip_resources (name,ip_type,rights_owner,contact,markets,market_penetrations,audience,summary,cooperation_status,currency,price_min,price_max,license_notes,profile,created_by) values (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			strings.TrimSpace(row.Name), strings.TrimSpace(row.IPType), strings.TrimSpace(row.RightsOwner), row.Contact, ipJSON(row.Markets), ipMarketPenetrationsJSON(row.MarketPenetrations), row.Audience, row.Summary, row.CooperationStatus, row.Currency, row.PriceMin, row.PriceMax, row.LicenseNotes, ipJSON(row.Profile), userID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		count, _ := inserted.RowsAffected()
		imported += int(count)
		if count > 0 {
			id, _ := inserted.LastInsertId()
			created = append(created, map[string]any{"row": index + 2, "id": id})
		}
	}
	if err := tx.Commit(); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"imported": imported, "results": results, "created": created})
}

func (a *app) ipRequestsList(w http.ResponseWriter, r *http.Request) {
	var filter struct {
		Keyword  string   `json:"keyword"`
		Status   string   `json:"status"`
		Statuses []string `json:"statuses"`
		Page     int      `json:"page"`
		PageSize int      `json:"pageSize"`
	}
	if !readIPJSON(w, r, &filter) {
		return
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 200 {
		filter.PageSize = 20
	}
	where := " where 1=1"
	args := []any{}
	if filter.Keyword != "" {
		where += " and project_name like ?"
		args = append(args, "%"+strings.TrimSpace(filter.Keyword)+"%")
	}
	if len(filter.Statuses) > 0 {
		placeholders := make([]string, 0, len(filter.Statuses))
		for _, status := range filter.Statuses {
			if strings.TrimSpace(status) == "" {
				continue
			}
			placeholders = append(placeholders, "?")
			args = append(args, status)
		}
		if len(placeholders) > 0 {
			where += " and status in (" + strings.Join(placeholders, ",") + ")"
		}
	} else if filter.Status != "" {
		where += " and status=?"
		args = append(args, filter.Status)
	}
	var total int
	if err := a.DB().QueryRowContext(r.Context(), "select count(*) from biz_ip_requests"+where, args...).Scan(&total); err != nil {
		writeDBError(w, err)
		return
	}
	listArgs := append(append([]any{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := a.queryMaps(r.Context(), `select id, project_name as projectName, department, markets, expected_launch as expectedLaunch,
		goal, budget_currency as budgetCurrency, budget_min as budgetMin, budget_max as budgetMax,
		status, date_format(submitted_at, '%Y-%m-%d %H:%i:%s') as submittedAt,
        date_format(ip_reviewed_at, '%Y-%m-%d %H:%i:%s') as ipReviewedAt,
        date_format(marketing_reviewed_at, '%Y-%m-%d %H:%i:%s') as marketingReviewedAt,
        created_by as createdBy, date_format(created_at, '%Y-%m-%d %H:%i') as createdAt,
		date_format(updated_at, '%Y-%m-%d %H:%i') as updatedAt,
		(select count(*) from biz_ip_request_candidates c where c.request_id=biz_ip_requests.id) as candidateCount
		, exists(select 1 from biz_ip_request_candidates c where c.request_id=biz_ip_requests.id and (c.feasibility<>'' or c.recommendation<>'' or coalesce(c.reason,'')<>'' or coalesce(c.assessment,'') not in ('','{}','null'))) as feedbackStarted
		from biz_ip_requests`+where+` order by updated_at desc, id desc limit ? offset ?`, listArgs...)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	userID, _ := a.currentUserID(r)
	for _, row := range rows {
		a.enrichIPRequestDeadlines(row, time.Now())
		ipRequestManagementActions(row, userID)
	}
	writeOK(w, tableData{List: rows, Total: total, PageSize: filter.PageSize, CurrentPage: filter.Page})
}

func (a *app) ipRequestSave(w http.ResponseWriter, r *http.Request) {
	var item ipRequestInput
	if !readIPJSON(w, r, &item) {
		return
	}
	item.ProjectName = strings.TrimSpace(item.ProjectName)
	if item.Brief == nil {
		item.Brief = map[string]any{}
	}
	item.Brief["notificationLanguage"] = ipRequestLanguage(r)
	if !ipValidRange(item.BudgetMin, item.BudgetMax) || (item.Submit && (item.ProjectName == "" || item.Department == "" || len(item.Markets) == 0 || item.Goal == "" || item.Description == "" || item.BudgetMin == nil || item.BudgetMax == nil)) {
		writeError(w, http.StatusBadRequest, 400, "请填写项目、市场、目标、说明及有效授权预算")
		return
	}
	if !item.Submit && item.ProjectName == "" {
		item.ProjectName = "未命名需求"
	}
	if item.ID <= 0 && len(item.CandidateIDs) > 5 {
		writeError(w, http.StatusBadRequest, 400, "最多选择5个意向IP")
		return
	}
	if ipExternalCount(item.Brief) > 3 {
		writeError(w, http.StatusBadRequest, 400, "最多推荐3个库外IP")
		return
	}
	if item.Submit && !ipExternalValid(item.Brief) {
		writeError(w, http.StatusBadRequest, 400, "库外IP信息不完整")
		return
	}
	if item.ID <= 0 && item.Submit && !ipBriefValid(item.Brief, item.CandidateIDs) {
		writeError(w, http.StatusBadRequest, 400, "请补齐项目与各意向IP的评估信息")
		return
	}
	if item.Submit && len(item.CandidateIDs) == 0 && ipExternalCount(item.Brief) == 0 {
		writeError(w, http.StatusBadRequest, 400, "请选择库内意向IP或添加库外IP意向")
		return
	}
	if item.BudgetCurrency == "" {
		item.BudgetCurrency = "CNY"
	}
	now := time.Now().In(ipTimeZone)
	var submittedAt any
	if item.Submit {
		submittedAt = now.Format("2006-01-02 15:04:05")
	}
	status := "draft"
	if item.Submit {
		status = "submitted"
	}
	var launch any
	if item.ExpectedLaunch != "" {
		launch = item.ExpectedLaunch
	}
	userID, authenticated := a.currentUserID(r)
	if !authenticated {
		writeError(w, http.StatusUnauthorized, 401, "请重新登录")
		return
	}
	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()
	id := item.ID
	submissionTransition := item.Submit
	candidatesLocked := false
	if id > 0 {
		var oldStatus, oldBrief string
		var creatorID int64
		var oldSubmitted sql.NullString
		var feedbackStarted bool
		err := tx.QueryRowContext(r.Context(), `select status,created_by,date_format(submitted_at,'%Y-%m-%d %H:%i:%s'),coalesce(brief,''),
            exists(select 1 from biz_ip_request_candidates c where c.request_id=biz_ip_requests.id and (c.feasibility<>'' or c.recommendation<>'' or coalesce(c.reason,'')<>'' or coalesce(c.assessment,'') not in ('','{}','null')))
            from biz_ip_requests where id=? for update`, id).Scan(&oldStatus, &creatorID, &oldSubmitted, &oldBrief, &feedbackStarted)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, 404, "需求不存在或已删除")
			return
		}
		if err != nil {
			writeDBError(w, err)
			return
		}
		if creatorID != int64(userID) {
			writeError(w, http.StatusForbidden, 403, "只能编辑本人发起的需求")
			return
		}
		submissionTransition = oldStatus == "draft" && item.Submit
		if oldStatus != "draft" {
			if !item.Submit {
				writeError(w, http.StatusConflict, 409, "已提交的需求不能改为草稿，请保存修改")
				return
			}
			status = oldStatus
			submittedAt = nil
			if oldSubmitted.Valid {
				submittedAt = oldSubmitted.String
			}
		}
		candidatesLocked = oldStatus != "draft" && (oldStatus != "submitted" || feedbackStarted)
		validationIDs := item.CandidateIDs
		if candidatesLocked {
			candidateRows, err := tx.QueryContext(r.Context(), "select ip_id from biz_ip_request_candidates where request_id=?", id)
			if err != nil {
				writeDBError(w, err)
				return
			}
			savedIDs := map[int64]bool{}
			for candidateRows.Next() {
				var candidateID int64
				if err = candidateRows.Scan(&candidateID); err != nil {
					break
				}
				savedIDs[candidateID] = true
			}
			if err == nil {
				err = candidateRows.Err()
			}
			candidateRows.Close()
			if err != nil {
				writeDBError(w, err)
				return
			}
			matches := len(savedIDs) == len(item.CandidateIDs)
			for _, candidateID := range item.CandidateIDs {
				if !savedIDs[candidateID] {
					matches = false
				}
				delete(savedIDs, candidateID)
			}
			if !matches || len(savedIDs) > 0 {
				writeError(w, http.StatusConflict, 409, "IP组已开始反馈，意向IP名单不可修改；请刷新后编辑其他需求内容")
				return
			}
			// Preserve original candidate briefings, including IPs added by reviewers.
			var savedBrief map[string]any
			if oldBrief != "" && json.Unmarshal([]byte(oldBrief), &savedBrief) != nil {
				writeError(w, 409, 409, "原需求资料无法读取，请联系管理员")
				return
			}
			if item.Brief == nil {
				item.Brief = map[string]any{}
			}
			item.Brief["candidateEvaluations"] = savedBrief["candidateEvaluations"]
			validationIDs = nil
		} else if len(item.CandidateIDs) > 5 {
			writeError(w, 400, 400, "最多选择5个意向IP")
			return
		}
		if item.Submit && !ipBriefValid(item.Brief, validationIDs) {
			writeError(w, 400, 400, "请补齐项目与各意向IP的评估信息")
			return
		}
		_, err = tx.ExecContext(r.Context(), `update biz_ip_requests set project_name=?,department=?,markets=?,expected_launch=?,goal=?,description=?,budget_currency=?,budget_min=?,budget_max=?,external_recommendation=?,brief=?,status=?,submitted_at=? where id=? and created_by=?`,
			item.ProjectName, item.Department, ipJSON(item.Markets), launch, item.Goal, item.Description, item.BudgetCurrency, item.BudgetMin, item.BudgetMax, item.ExternalRecommendation, ipJSON(item.Brief), status, submittedAt, id, userID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		if !candidatesLocked {
			if _, err := tx.ExecContext(r.Context(), "delete from biz_ip_request_candidates where request_id=?", id); err != nil {
				writeDBError(w, err)
				return
			}
		}
	} else {
		result, err := tx.ExecContext(r.Context(), `insert into biz_ip_requests (project_name,department,markets,expected_launch,goal,description,budget_currency,budget_min,budget_max,external_recommendation,brief,status,created_by,submitted_at) values (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			item.ProjectName, item.Department, ipJSON(item.Markets), launch, item.Goal, item.Description, item.BudgetCurrency, item.BudgetMin, item.BudgetMax, item.ExternalRecommendation, ipJSON(item.Brief), status, userID, submittedAt)
		if err != nil {
			writeDBError(w, err)
			return
		}
		id, _ = result.LastInsertId()
	}
	seen := map[int64]bool{}
	for _, ipID := range item.CandidateIDs {
		if candidatesLocked {
			break
		}
		if ipID <= 0 || seen[ipID] {
			continue
		}
		seen[ipID] = true
		result, err := tx.ExecContext(r.Context(), `insert into biz_ip_request_candidates (request_id,ip_id,priority_order) select ?,id,? from biz_ip_resources where id=?`, id, 1, ipID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			writeError(w, http.StatusBadRequest, 400, "所选IP不存在")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeDBError(w, err)
		return
	}
	if submissionTransition {
		a.notifyIPStageAsync(id, "submitted")
	}
	writeOK(w, map[string]any{"id": id, "status": status})
}

func (a *app) ipRequestDetail(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if id <= 0 {
		writeError(w, http.StatusBadRequest, 400, "缺少需求编号")
		return
	}
	rows, err := a.queryMaps(r.Context(), `select id,project_name as projectName,department,markets,expected_launch as expectedLaunch,
		goal,description,budget_currency as budgetCurrency,budget_min as budgetMin,budget_max as budgetMax,
		external_recommendation as externalRecommendation,brief,marketing_profile as marketingProfile,status,market_heat as marketHeat,
		fan_audience as fanAudience,commercial_value as commercialValue,marketing_risks as marketingRisks,
		marketing_channels as marketingChannels,marketing_comments as marketingComments,
        exists(select 1 from biz_ip_request_candidates c where c.request_id=biz_ip_requests.id and (c.feasibility<>'' or c.recommendation<>'' or coalesce(c.reason,'')<>'' or coalesce(c.assessment,'') not in ('','{}','null'))) as feedbackStarted,
		created_by as createdBy,date_format(created_at, '%Y-%m-%d %H:%i') as createdAt,
        date_format(submitted_at, '%Y-%m-%d %H:%i:%s') as submittedAt,
        date_format(ip_reviewed_at, '%Y-%m-%d %H:%i:%s') as ipReviewedAt,
        date_format(marketing_reviewed_at, '%Y-%m-%d %H:%i:%s') as marketingReviewedAt
		from biz_ip_requests where id=?`, id)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if len(rows) == 0 {
		writeError(w, http.StatusNotFound, 404, "需求不存在")
		return
	}
	candidates, err := a.queryMaps(r.Context(), `select c.id,c.ip_id as ipId,r.name,r.ip_type as ipType,r.markets,
		(select f.id from biz_ip_files f where f.ip_id=r.id and f.file_kind='visual' order by f.id desc limit 1) as visualFileId,
		least(2, greatest(0, c.priority_order)) as priorityOrder,c.feasibility,c.recommendation,c.reason,c.assessment
		from biz_ip_request_candidates c join biz_ip_resources r on r.id=c.ip_id where c.request_id=? order by c.priority_order,c.id`, id)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if candidates == nil {
		candidates = []map[string]any{}
	}
	a.enrichIPRequestDeadlines(rows[0], time.Now())
	userID, _ := a.currentUserID(r)
	ipRequestManagementActions(rows[0], userID)
	writeOK(w, map[string]any{"request": rows[0], "candidates": candidates})
}

func (a *app) ipRequestFeedback(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RequestID  int64 `json:"requestId"`
		SaveDraft  bool  `json:"saveDraft"`
		Candidates []struct {
			IPID           int64          `json:"ipId"`
			PriorityOrder  int            `json:"priorityOrder"`
			Feasibility    string         `json:"feasibility"`
			Recommendation string         `json:"recommendation"`
			Reason         string         `json:"reason"`
			Assessment     map[string]any `json:"assessment"`
		} `json:"candidates"`
	}
	if !readIPJSON(w, r, &input) {
		return
	}
	if input.RequestID <= 0 || len(input.Candidates) == 0 || len(input.Candidates) > 8 {
		writeError(w, http.StatusBadRequest, 400, "请选择待评估IP")
		return
	}
	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()
	var requestStatus string
	err = tx.QueryRowContext(r.Context(), "select status from biz_ip_requests where id=? for update", input.RequestID).Scan(&requestStatus)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if requestStatus != "submitted" {
		writeError(w, http.StatusConflict, 409, "需求未处于待IP反馈状态")
		return
	}
	seenIPs := map[int64]bool{}
	for _, candidate := range input.Candidates {
		if candidate.IPID <= 0 || seenIPs[candidate.IPID] {
			writeError(w, http.StatusBadRequest, 400, "评估IP不可重复")
			return
		}
		seenIPs[candidate.IPID] = true
		if !input.SaveDraft && (candidate.Feasibility == "" || candidate.Recommendation == "" || strings.TrimSpace(candidate.Reason) == "") {
			writeError(w, http.StatusBadRequest, 400, "请填写每个IP的可行性、推荐意见和理由")
			return
		}
		if candidate.PriorityOrder < 0 || candidate.PriorityOrder > 2 {
			writeError(w, http.StatusBadRequest, 400, "优先级只能选择T0、T1或T2")
			return
		}
		if !input.SaveDraft {
			for _, key := range []string{"licenseContent", "licenseTerritory", "ipResources", "brandResources", "businessRisk", "publicRisk", "ownershipClarity"} {
				if ipMapText(candidate.Assessment, key) == "" {
					writeError(w, http.StatusBadRequest, 400, "请补齐授权、资源和风险评估信息")
					return
				}
			}
			score, ok := ipWeightedScore(candidate.Assessment)
			if !ok {
				writeError(w, http.StatusBadRequest, 400, "请填写四项0至100分的IP评估分数")
				return
			}
			candidate.Assessment["overallScore"] = score
		}
		result, err := tx.ExecContext(r.Context(), `insert into biz_ip_request_candidates (request_id,ip_id,priority_order,feasibility,recommendation,reason,assessment)
			select ?,id,?,?,?,?,? from biz_ip_resources where id=?
			on duplicate key update priority_order=values(priority_order),feasibility=values(feasibility),recommendation=values(recommendation),reason=values(reason),assessment=values(assessment)`,
			input.RequestID, candidate.PriorityOrder, candidate.Feasibility, candidate.Recommendation, candidate.Reason, ipJSON(candidate.Assessment), candidate.IPID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			var exists int
			if err := tx.QueryRowContext(r.Context(), "select count(*) from biz_ip_resources where id=?", candidate.IPID).Scan(&exists); err != nil {
				writeDBError(w, err)
				return
			}
			if exists == 0 {
				writeError(w, http.StatusBadRequest, 400, "评估IP不存在")
				return
			}
		}
	}
	if !input.SaveDraft {
		rows, err := tx.QueryContext(r.Context(), "select ip_id from biz_ip_request_candidates where request_id=?", input.RequestID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		missing := false
		for rows.Next() {
			var id int64
			if err = rows.Scan(&id); err != nil {
				break
			}
			if !seenIPs[id] {
				missing = true
			}
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			writeDBError(w, err)
			return
		}
		if missing {
			writeError(w, 400, 400, "请完成所有意向IP的意见后再提交")
			return
		}
		now := time.Now().In(ipTimeZone)

		if _, err := tx.ExecContext(r.Context(), "update biz_ip_requests set status='ip_reviewed',ip_reviewed_at=? where id=?", now.Format("2006-01-02 15:04:05"), input.RequestID); err != nil {
			writeDBError(w, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeDBError(w, err)
		return
	}
	if !input.SaveDraft {
		a.notifyIPStageAsync(input.RequestID, "ip_reviewed")
	}
	writeOK(w, nil)
}

func ipFileURL(id any, download bool) string {
	url := fmt.Sprintf("/api/business/ip/resources/file?id=%v", id)
	if download {
		url += "&download=1"
	}
	return url
}
