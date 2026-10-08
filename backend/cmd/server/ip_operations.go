package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ipResourceInput struct {
	ID                int64          `json:"id"`
	Name              string         `json:"name"`
	IPType            string         `json:"ipType"`
	RightsOwner       string         `json:"rightsOwner"`
	Contact           string         `json:"contact"`
	Markets           []string       `json:"markets"`
	Audience          string         `json:"audience"`
	Summary           string         `json:"summary"`
	CooperationStatus string         `json:"cooperationStatus"`
	Currency          string         `json:"currency"`
	PriceMin          *float64       `json:"priceMin"`
	PriceMax          *float64       `json:"priceMax"`
	LicenseNotes      string         `json:"licenseNotes"`
	Profile           map[string]any `json:"profile"`
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
		if !ok || strings.TrimSpace(fmt.Sprint(candidate["name"])) == "" || strings.TrimSpace(fmt.Sprint(candidate["reason"])) == "" || candidate["name"] == nil || candidate["reason"] == nil {
			return false
		}
	}
	return true
}

func (a *app) ipResourcesList(w http.ResponseWriter, r *http.Request) {
	var filter struct {
		Keyword  string `json:"keyword"`
		IPType   string `json:"ipType"`
		Status   string `json:"status"`
		Page     int    `json:"page"`
		PageSize int    `json:"pageSize"`
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
	var total int
	if err := a.DB().QueryRowContext(r.Context(), "select count(*) from biz_ip_resources"+where, args...).Scan(&total); err != nil {
		writeDBError(w, err)
		return
	}
	listArgs := append(append([]any{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := a.queryMaps(r.Context(), `select id, name, ip_type as ipType, rights_owner as rightsOwner,
		markets, audience, cooperation_status as cooperationStatus, currency,
		price_min as priceMin, price_max as priceMax,
		date_format(updated_at, '%Y-%m-%d %H:%i') as updatedAt
		from biz_ip_resources`+where+` order by updated_at desc, id desc limit ? offset ?`, listArgs...)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if rows == nil {
		rows = []map[string]any{}
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
	if item.Currency == "" {
		item.Currency = "CNY"
	}
	if item.CooperationStatus == "" {
		item.CooperationStatus = "待评估"
	}
	var id int64
	if item.ID > 0 {
		var exists int
		if err := a.DB().QueryRowContext(r.Context(), "select count(*) from biz_ip_resources where id=?", item.ID).Scan(&exists); err != nil {
			writeDBError(w, err)
			return
		}
		if exists == 0 {
			writeError(w, http.StatusNotFound, 404, "IP不存在")
			return
		}
		result, err := a.DB().ExecContext(r.Context(), `update biz_ip_resources set name=?, ip_type=?, rights_owner=?, contact=?, markets=?, audience=?, summary=?, cooperation_status=?, currency=?, price_min=?, price_max=?, license_notes=?, profile=? where id=?`,
			item.Name, item.IPType, item.RightsOwner, item.Contact, ipJSON(item.Markets), item.Audience, item.Summary, item.CooperationStatus, item.Currency, item.PriceMin, item.PriceMax, item.LicenseNotes, ipJSON(item.Profile), item.ID)
		if err != nil {
			writeIPSaveError(w, err)
			return
		}
		_ = result
		id = item.ID
	} else {
		userID, _ := a.currentUserID(r)
		result, err := a.DB().ExecContext(r.Context(), `insert into biz_ip_resources (name,ip_type,rights_owner,contact,markets,audience,summary,cooperation_status,currency,price_min,price_max,license_notes,profile,created_by) values (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			item.Name, item.IPType, item.RightsOwner, item.Contact, ipJSON(item.Markets), item.Audience, item.Summary, item.CooperationStatus, item.Currency, item.PriceMin, item.PriceMax, item.LicenseNotes, ipJSON(item.Profile), userID)
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
		markets, audience, summary, cooperation_status as cooperationStatus, currency,
		price_min as priceMin, price_max as priceMax, license_notes as licenseNotes, profile,
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
	cases, err := a.queryMaps(r.Context(), `select id, ip_id as ipId, title, summary,
		date_format(created_at, '%Y-%m-%d') as createdAt from biz_ip_cases where ip_id=? order by id desc`, id)
	if err != nil {
		writeDBError(w, err)
		return
	}
	files, err := a.queryMaps(r.Context(), `select id, ip_id as ipId, case_id as caseId, file_kind as fileKind,
		original_name as originalName, size_bytes as sizeBytes,
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

func (a *app) ipFileUpload(w http.ResponseWriter, r *http.Request) {
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
	if header.Size <= 0 || header.Size > maxSize || (kind == "visual" && ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp") || (kind != "visual" && ext != ".pdf") {
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
	userID, _ := a.currentUserID(r)
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

func (a *app) validateIPImport(r *http.Request, rows []ipResourceInput) ([]ipImportResult, error) {
	results := make([]ipImportResult, 0, len(rows))
	seen := map[string]bool{}
	for index, row := range rows {
		result := ipImportResult{Row: index + 2, Name: strings.TrimSpace(row.Name), Status: "ready"}
		key := strings.ToLower(result.Name + "\x00" + strings.TrimSpace(row.RightsOwner))
		switch {
		case result.Name == "" || strings.TrimSpace(row.IPType) == "" || len(row.Markets) == 0:
			result.Status, result.Message = "error", "缺少IP名称、类型或覆盖市场"
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
	userID, _ := a.currentUserID(r)
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
		if row.Currency == "" {
			row.Currency = "CNY"
		}
		if row.CooperationStatus == "" {
			row.CooperationStatus = "待评估"
		}
		inserted, err := tx.ExecContext(r.Context(), `insert ignore into biz_ip_resources (name,ip_type,rights_owner,contact,markets,audience,summary,cooperation_status,currency,price_min,price_max,license_notes,profile,created_by) values (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			strings.TrimSpace(row.Name), strings.TrimSpace(row.IPType), strings.TrimSpace(row.RightsOwner), row.Contact, ipJSON(row.Markets), row.Audience, row.Summary, row.CooperationStatus, row.Currency, row.PriceMin, row.PriceMax, row.LicenseNotes, ipJSON(row.Profile), userID)
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
		Keyword  string `json:"keyword"`
		Status   string `json:"status"`
		Page     int    `json:"page"`
		PageSize int    `json:"pageSize"`
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
	if filter.Status != "" {
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
		status, created_by as createdBy, date_format(created_at, '%Y-%m-%d %H:%i') as createdAt,
		date_format(updated_at, '%Y-%m-%d %H:%i') as updatedAt,
		(select count(*) from biz_ip_request_candidates c where c.request_id=biz_ip_requests.id) as candidateCount
		from biz_ip_requests`+where+` order by updated_at desc, id desc limit ? offset ?`, listArgs...)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	writeOK(w, tableData{List: rows, Total: total, PageSize: filter.PageSize, CurrentPage: filter.Page})
}

func (a *app) ipRequestSave(w http.ResponseWriter, r *http.Request) {
	var item ipRequestInput
	if !readIPJSON(w, r, &item) {
		return
	}
	item.ProjectName = strings.TrimSpace(item.ProjectName)
	if item.ProjectName == "" || item.Department == "" || len(item.Markets) == 0 || item.Goal == "" || item.Description == "" || !ipValidRange(item.BudgetMin, item.BudgetMax) || (item.Submit && (item.BudgetMin == nil || item.BudgetMax == nil)) {
		writeError(w, http.StatusBadRequest, 400, "请填写项目、市场、目标、说明及有效授权预算")
		return
	}
	if len(item.CandidateIDs) > 5 {
		writeError(w, http.StatusBadRequest, 400, "最多选择5个意向IP")
		return
	}
	if ipExternalCount(item.Brief) > 3 {
		writeError(w, http.StatusBadRequest, 400, "最多推荐3个库外IP")
		return
	}
	if !ipExternalValid(item.Brief) {
		writeError(w, http.StatusBadRequest, 400, "库外IP需填写名称和选择理由")
		return
	}
	if item.Submit && len(item.CandidateIDs) == 0 && strings.TrimSpace(item.ExternalRecommendation) == "" && ipExternalCount(item.Brief) == 0 {
		writeError(w, http.StatusBadRequest, 400, "请选择意向IP或填写库外IP推荐")
		return
	}
	if item.BudgetCurrency == "" {
		item.BudgetCurrency = "CNY"
	}
	status := "draft"
	if item.Submit {
		status = "submitted"
	}
	var launch any
	if item.ExpectedLaunch != "" {
		launch = item.ExpectedLaunch
	}
	userID, _ := a.currentUserID(r)
	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()
	id := item.ID
	if id > 0 {
		var exists int
		if err := tx.QueryRowContext(r.Context(), "select count(*) from biz_ip_requests where id=? and status='draft' and created_by=?", id, userID).Scan(&exists); err != nil {
			writeDBError(w, err)
			return
		}
		if exists == 0 {
			writeError(w, http.StatusConflict, 409, "只能修改本人草稿")
			return
		}
		result, err := tx.ExecContext(r.Context(), `update biz_ip_requests set project_name=?,department=?,markets=?,expected_launch=?,goal=?,description=?,budget_currency=?,budget_min=?,budget_max=?,external_recommendation=?,brief=?,status=? where id=? and status='draft' and created_by=?`,
			item.ProjectName, item.Department, ipJSON(item.Markets), launch, item.Goal, item.Description, item.BudgetCurrency, item.BudgetMin, item.BudgetMax, item.ExternalRecommendation, ipJSON(item.Brief), status, id, userID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		_ = result
		if _, err := tx.ExecContext(r.Context(), "delete from biz_ip_request_candidates where request_id=?", id); err != nil {
			writeDBError(w, err)
			return
		}
	} else {
		result, err := tx.ExecContext(r.Context(), `insert into biz_ip_requests (project_name,department,markets,expected_launch,goal,description,budget_currency,budget_min,budget_max,external_recommendation,brief,status,created_by) values (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			item.ProjectName, item.Department, ipJSON(item.Markets), launch, item.Goal, item.Description, item.BudgetCurrency, item.BudgetMin, item.BudgetMax, item.ExternalRecommendation, ipJSON(item.Brief), status, userID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		id, _ = result.LastInsertId()
	}
	seen := map[int64]bool{}
	for index, ipID := range item.CandidateIDs {
		if ipID <= 0 || seen[ipID] {
			continue
		}
		seen[ipID] = true
		result, err := tx.ExecContext(r.Context(), `insert into biz_ip_request_candidates (request_id,ip_id,priority_order) select ?,id,? from biz_ip_resources where id=?`, id, index+1, ipID)
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
		created_by as createdBy,date_format(created_at, '%Y-%m-%d %H:%i') as createdAt
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
		c.priority_order as priorityOrder,c.feasibility,c.recommendation,c.reason,c.assessment
		from biz_ip_request_candidates c join biz_ip_resources r on r.id=c.ip_id where c.request_id=? order by c.priority_order,c.id`, id)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if candidates == nil {
		candidates = []map[string]any{}
	}
	writeOK(w, map[string]any{"request": rows[0], "candidates": candidates})
}

func (a *app) ipRequestFeedback(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RequestID  int64 `json:"requestId"`
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
	if input.RequestID <= 0 || len(input.Candidates) == 0 || len(input.Candidates) > 5 {
		writeError(w, http.StatusBadRequest, 400, "请选择待评估IP")
		return
	}
	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()
	for _, candidate := range input.Candidates {
		if candidate.Feasibility == "" || candidate.Recommendation == "" || strings.TrimSpace(candidate.Reason) == "" {
			writeError(w, http.StatusBadRequest, 400, "请填写每个IP的可行性、推荐意见和理由")
			return
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
	result, err := tx.ExecContext(r.Context(), "update biz_ip_requests set status='ip_reviewed' where id=? and status='submitted'", input.RequestID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusConflict, 409, "需求未处于待IP反馈状态")
		return
	}
	if err := tx.Commit(); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, nil)
}

func (a *app) ipRequestMarketing(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RequestID         int64          `json:"requestId"`
		MarketHeat        string         `json:"marketHeat"`
		FanAudience       string         `json:"fanAudience"`
		CommercialValue   string         `json:"commercialValue"`
		MarketingRisks    string         `json:"marketingRisks"`
		MarketingChannels []string       `json:"marketingChannels"`
		MarketingComments string         `json:"marketingComments"`
		MarketingProfile  map[string]any `json:"marketingProfile"`
	}
	if !readIPJSON(w, r, &input) {
		return
	}
	if input.RequestID <= 0 || strings.TrimSpace(input.MarketingComments) == "" {
		writeError(w, http.StatusBadRequest, 400, "请填写营销补充意见")
		return
	}
	result, err := a.DB().ExecContext(r.Context(), `update biz_ip_requests set market_heat=?,fan_audience=?,commercial_value=?,marketing_risks=?,marketing_channels=?,marketing_comments=?,marketing_profile=?,status='marketing_reviewed' where id=? and status='ip_reviewed'`,
		input.MarketHeat, input.FanAudience, input.CommercialValue, input.MarketingRisks, ipJSON(input.MarketingChannels), input.MarketingComments, ipJSON(input.MarketingProfile), input.RequestID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusConflict, 409, "需求未处于待营销补充状态")
		return
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
