package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type app struct {
	db     atomic.Value
	config atomic.Value
}

type apiResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type tableData struct {
	List        any `json:"list"`
	Total       int `json:"total"`
	PageSize    int `json:"pageSize"`
	CurrentPage int `json:"currentPage"`
}

func main() {
	cfg, watcher, err := loadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := openDB(cfg.MySQL)
	if err != nil {
		log.Fatalf("open mysql: %v", err)
	}
	defer db.Close()

	a := newApp(db, cfg)
	watchConfig(watcher, func(next Config) {
		if err := a.reloadConfig(next); err != nil {
			log.Printf("reload config failed: %v", err)
			return
		}
		log.Printf("config reloaded")
	})

	mux := http.NewServeMux()
	a.routes(mux)

	log.Printf("kol-admin backend listening on %s", cfg.Server.Addr)
	if err := http.ListenAndServe(cfg.Server.Addr, a.withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

func newApp(db *sql.DB, cfg Config) *app {
	a := &app{}
	a.db.Store(db)
	a.config.Store(cfg)
	return a
}

func (a *app) DB() *sql.DB {
	return a.db.Load().(*sql.DB)
}

func (a *app) Config() Config {
	return a.config.Load().(Config)
}

func (a *app) reloadConfig(next Config) error {
	current := a.Config()
	if next.MySQL != current.MySQL {
		db, err := openDB(next.MySQL)
		if err != nil {
			return err
		}
		old := a.DB()
		a.db.Store(db)
		_ = old.Close()
	}
	a.config.Store(next)
	return nil
}

func (a *app) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", a.health)
	mux.Handle("GET /uploads/images/", http.StripPrefix("/uploads/images/", http.FileServer(http.Dir("uploads/images"))))
	mux.HandleFunc("POST /login", a.login)
	mux.HandleFunc("POST /refresh-token", a.refreshToken)
	mux.HandleFunc("GET /mine", a.mine)
	mux.HandleFunc("POST /upload/images", a.uploadImage)
	mux.HandleFunc("GET /get-async-routes", a.asyncRoutes)
	mux.HandleFunc("POST /user", a.requireMenu("/system/user/index", a.users))
	mux.HandleFunc("POST /user/create", a.requirePerm("system:user:add", a.createUser))
	mux.HandleFunc("POST /user/update", a.requirePerm("system:user:edit", a.updateUser))
	mux.HandleFunc("POST /user/delete", a.requirePerm("system:user:delete", a.deleteUser))
	mux.HandleFunc("POST /user/status", a.requirePerm("system:user:edit", a.updateUserStatus))
	mux.HandleFunc("POST /user/reset-password", a.requirePerm("system:user:reset-password", a.resetUserPassword))
	mux.HandleFunc("POST /user/roles", a.requirePerm("system:user:assign-role", a.updateUserRoles))
	mux.HandleFunc("GET /list-all-role", a.requireMenu("/system/user/index", a.allRoles))
	mux.HandleFunc("POST /list-role-ids", a.requireMenu("/system/user/index", a.roleIDs))
	mux.HandleFunc("POST /role", a.requireMenu("/system/role/index", a.roles))
	mux.HandleFunc("POST /role/create", a.requirePerm("system:role:add", a.createRole))
	mux.HandleFunc("POST /role/update", a.requirePerm("system:role:edit", a.updateRole))
	mux.HandleFunc("POST /role/delete", a.requirePerm("system:role:delete", a.deleteRole))
	mux.HandleFunc("POST /role/status", a.requirePerm("system:role:edit", a.updateRoleStatus))
	mux.HandleFunc("POST /role/menus", a.requirePerm("system:role:menu", a.updateRoleMenus))
	mux.HandleFunc("POST /menu", a.requireMenu("/system/menu/index", a.menus))
	mux.HandleFunc("POST /menu/create", a.requirePerm("system:menu:add", a.createMenu))
	mux.HandleFunc("POST /menu/update", a.requirePerm("system:menu:edit", a.updateMenu))
	mux.HandleFunc("POST /menu/delete", a.requirePerm("system:menu:delete", a.deleteMenu))
	mux.HandleFunc("GET /system/platform-sync-control", a.requireMenu("/system/platform-sync-control", a.platformSyncControl))
	mux.HandleFunc("POST /system/platform-sync-control/save", a.requireMenu("/system/platform-sync-control", a.savePlatformSyncControl))
	mux.HandleFunc("POST /dept", a.requireMenu("/system/dept/index", a.departments))
	mux.HandleFunc("POST /dept/create", a.requirePerm("system:dept:add", a.createDepartment))
	mux.HandleFunc("POST /dept/update", a.requirePerm("system:dept:edit", a.updateDepartment))
	mux.HandleFunc("POST /dept/delete", a.requirePerm("system:dept:delete", a.deleteDepartment))
	mux.HandleFunc("POST /role-menu", a.requireMenu("/system/role/index", a.roleMenu))
	mux.HandleFunc("POST /role-menu-ids", a.requirePerm("system:role:menu", a.roleMenuIDs))
	mux.HandleFunc("POST /online-logs", a.requireMenu("/monitor/online-user", a.onlineLogs))
	mux.HandleFunc("POST /login-logs", a.requireMenu("/monitor/login-logs", a.loginLogs))
	mux.HandleFunc("POST /operation-logs", a.requireMenu("/monitor/operation-logs", a.operationLogs))
	mux.HandleFunc("POST /system-logs", a.requireMenu("/monitor/system-logs", a.systemLogs))
	mux.HandleFunc("POST /system-logs-detail", a.requireMenu("/monitor/system-logs", a.systemLogDetail))
	mux.HandleFunc("POST /business/resources", a.requireMenu("/business/resources", a.businessResources))
	mux.HandleFunc("POST /business/resources/create", a.requireMenu("/business/resources", a.createBusinessResource))
	mux.HandleFunc("POST /business/resources/update", a.requireMenu("/business/resources", a.updateBusinessResource))
	mux.HandleFunc("POST /business/resources/delete", a.requireMenu("/business/resources", a.deleteBusinessResource))
	mux.HandleFunc("POST /business/resources/sync", a.requireMenu("/business/resources", a.syncBusinessResource))
	mux.HandleFunc("POST /business/resources/sync-all", a.requireMenu("/business/resources", a.startBusinessResourcesSyncAll))
	mux.HandleFunc("GET /business/resources/sync-status", a.requireMenu("/business/resources", a.businessResourcesSyncStatus))
	mux.HandleFunc("GET /business/resources/extra-fields", a.requireMenu("/business/resources", a.businessResourceExtraFields))
	mux.HandleFunc("POST /business/resources/import", a.requireMenu("/business/resources", a.importBusinessResources))
	mux.HandleFunc("POST /business/resource-posts", a.requireMenu("/business/resource-posts", a.businessResourcePosts))
	mux.HandleFunc("POST /business/assistant/recommend", a.requireMenu("/business/assistant", a.businessAssistantRecommend))
	mux.HandleFunc("POST /business/project-resources/create", a.requireMenu("/business/projects", a.createBusinessProjectResource))
	mux.HandleFunc("GET /business/markets", a.requireAnyMenu([]string{"/business/projects", "/business/assistant"}, a.businessMarketOptions))
	mux.HandleFunc("POST /business/markets/create", a.requireAnyMenu([]string{"/business/projects", "/business/assistant"}, a.createBusinessMarketOption))
	mux.HandleFunc("POST /business/markets/delete", a.requireAnyMenu([]string{"/business/projects", "/business/assistant"}, a.deleteBusinessMarketOption))
	mux.HandleFunc("GET /business/tags", a.requireMenu("/business/tags", a.businessTags))
	mux.HandleFunc("POST /business/tags/create", a.requireMenu("/business/tags", a.createBusinessTag))
	mux.HandleFunc("POST /business/projects", a.requireMenu("/business/projects", a.businessProjects))
	mux.HandleFunc("POST /business/projects/create", a.requireMenu("/business/projects", a.createBusinessProject))
	mux.HandleFunc("POST /business/projects/update", a.requireMenu("/business/projects", a.updateBusinessProject))
	mux.HandleFunc("POST /business/cooperations", a.requireMenu("/business/projects", a.businessCooperations))
	mux.HandleFunc("POST /business/cooperations/create", a.requireMenu("/business/projects", a.createBusinessCooperation))
	mux.HandleFunc("POST /business/cooperations/update", a.requireMenu("/business/projects", a.updateBusinessCooperation))
	mux.HandleFunc("POST /business/cooperations/import", a.requireMenu("/business/projects", a.importBusinessCooperations))
	mux.HandleFunc("POST /business/brief-templates", a.requireMenu("/business/briefs", a.businessBriefTemplates))
	mux.HandleFunc("POST /business/brief-templates/create", a.requireMenu("/business/briefs", a.createBusinessBriefTemplate))
	mux.HandleFunc("GET /business/dashboard", a.requireMenu("/business/dashboard", a.businessDashboard))
	mux.HandleFunc("POST /collector/tiktok/callback", a.tikTokCollectorCallback)
	mux.HandleFunc("GET /business/ai-model", a.requireMenu("/business/ai-model", a.businessAIModelConfig))
	mux.HandleFunc("POST /business/ai-model/save", a.requireMenu("/business/ai-model", a.saveBusinessAIModelConfig))
	mux.HandleFunc("POST /business/ai-model/test", a.requireMenu("/business/ai-model", a.testBusinessAIModelConfig))
	mux.HandleFunc("GET /business/governance", a.requireMenu("/business/governance", a.businessGovernanceRules))
	mux.HandleFunc("POST /business/governance/save", a.requireMenu("/business/governance", a.saveBusinessGovernanceRule))
}

func (a *app) health(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]any{"status": "ok"})
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	username := stringField(body, "username")
	password := stringField(body, "password")
	if username == "" || password == "" {
		writeError(w, http.StatusOK, 10001, "用户名或密码不能为空")
		return
	}

	var user struct {
		ID           int
		Avatar       string
		Username     string
		Nickname     string
		PasswordHash string
		Status       int
	}
	err := a.DB().QueryRowContext(
		r.Context(),
		`select id, avatar, username, nickname, password_hash, status
		   from sys_users where username = ? limit 1`,
		username,
	).Scan(&user.ID, &user.Avatar, &user.Username, &user.Nickname, &user.PasswordHash, &user.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusOK, 10002, "用户名或密码错误")
			return
		}
		writeDBError(w, err)
		return
	}
	if user.Status != 1 || user.PasswordHash != sha256Hex(password) {
		writeError(w, http.StatusOK, 10002, "用户名或密码错误")
		return
	}

	roles, err := a.roleCodes(r.Context(), user.ID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	permissions, err := a.userPermissions(r.Context(), user.ID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if contains(roles, "admin") {
		permissions = []string{"*:*:*"}
	}

	now := time.Now()
	data := map[string]any{
		"avatar":       user.Avatar,
		"username":     user.Username,
		"nickname":     user.Nickname,
		"roles":        roles,
		"permissions":  permissions,
		"accessToken":  fmt.Sprintf("kol.%d.%d", user.ID, now.Unix()),
		"refreshToken": fmt.Sprintf("kol.%d.refresh.%d", user.ID, now.Unix()),
		"expires":      now.Add(2 * time.Hour).Format("2006/01/02 15:04:05"),
	}
	writeOK(w, data)
}

func (a *app) refreshToken(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	writeOK(w, map[string]any{
		"accessToken":  fmt.Sprintf("kol.refresh.%d", now.Unix()),
		"refreshToken": fmt.Sprintf("kol.refresh.next.%d", now.Unix()),
		"expires":      now.Add(2 * time.Hour).Format("2006/01/02 15:04:05"),
	})
}

func (a *app) uploadImage(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, 401, "未登录或登录已过期")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusOK, 10001, "图片上传失败："+err.Error())
		return
	}
	file, _, err := r.FormFile("files")
	if err != nil {
		file, _, err = r.FormFile("file")
	}
	if err != nil {
		writeError(w, http.StatusOK, 10002, "未找到上传文件")
		return
	}
	defer file.Close()

	contentType, err := sniffSupportedImageContentType(file)
	if err != nil {
		writeError(w, http.StatusOK, 10001, "图片上传失败："+err.Error())
		return
	}
	if contentType == "" {
		writeError(w, http.StatusOK, 10003, "仅支持图片文件")
		return
	}
	ext := imageExt(contentType)
	if ext == "" {
		writeError(w, http.StatusOK, 10004, "图片格式仅支持 jpg、png、gif、webp")
		return
	}

	dir := filepath.Join("uploads", "images")
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeDBError(w, err)
		return
	}
	filename := fmt.Sprintf("avatar_%d_%d%s", userID, time.Now().UnixNano(), ext)
	path := filepath.Join(dir, filename)
	dst, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		writeDBError(w, err)
		return
	}

	url := "/api/uploads/images/" + filename
	_, _ = a.DB().ExecContext(r.Context(), `update sys_users set avatar = ? where id = ?`, url, userID)
	writeOK(w, map[string]any{"url": url})
}

func sniffSupportedImageContentType(file io.ReadSeeker) (string, error) {
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return normalizeImageContentType(buf[:n]), nil
}

func normalizeImageContentType(header []byte) string {
	if len(header) >= 12 && string(header[:4]) == "RIFF" && string(header[8:12]) == "WEBP" {
		return "image/webp"
	}
	switch strings.ToLower(http.DetectContentType(header)) {
	case "image/jpeg":
		return "image/jpeg"
	case "image/png":
		return "image/png"
	case "image/gif":
		return "image/gif"
	case "image/webp":
		return "image/webp"
	default:
		return ""
	}
}

func imageExt(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}

func (a *app) mine(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, 401, "未登录或登录已过期")
		return
	}
	var user struct {
		Avatar   string
		Username string
		Nickname string
		Email    string
		Phone    string
		Remark   string
	}
	err := a.DB().QueryRowContext(r.Context(),
		`select avatar, username, nickname, email, phone, remark from sys_users where id = ? limit 1`,
		userID,
	).Scan(&user.Avatar, &user.Username, &user.Nickname, &user.Email, &user.Phone, &user.Remark)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{
		"avatar":      user.Avatar,
		"username":    user.Username,
		"nickname":    user.Nickname,
		"email":       user.Email,
		"phone":       user.Phone,
		"description": user.Remark,
	})
}

func (a *app) asyncRoutes(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, 401, "未登录或登录已过期")
		return
	}
	menus, err := a.userRouteMenus(r.Context(), userID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, buildRouteTree(menus))
}

func (a *app) users(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	where, args := filters(body, map[string]string{
		"username": "u.username like ?",
		"phone":    "u.phone = ?",
		"status":   "u.status = ?",
		"deptId":   "u.dept_id = ?",
	})
	rows, err := a.queryMaps(r.Context(),
		`select u.id, u.avatar, u.username, u.nickname, u.phone, u.email, u.sex, u.status,
		        u.remark, cast(unix_timestamp(u.create_time) * 1000 as unsigned) as createTime,
		        d.id as deptId, d.name as deptName
		   from sys_users u left join sys_departments d on d.id = u.dept_id`+where+` order by u.id asc`,
		args...,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	for _, row := range rows {
		row["dept"] = map[string]any{"id": row["deptId"], "name": row["deptName"]}
		delete(row, "deptId")
		delete(row, "deptName")
	}
	writeTable(w, rows)
}

func (a *app) allRoles(w http.ResponseWriter, r *http.Request) {
	rows, err := a.queryMaps(r.Context(), `select id, name from sys_roles order by id`)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, rows)
}

func (a *app) roleIDs(w http.ResponseWriter, r *http.Request) {
	userID := intField(readBody(r), "userId")
	rows, err := a.DB().QueryContext(r.Context(), `select role_id from sys_user_roles where user_id = ?`, userID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			writeDBError(w, err)
			return
		}
		ids = append(ids, id)
	}
	writeOK(w, ids)
}

func (a *app) roles(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	where, args := filters(body, map[string]string{
		"name":   "name like ?",
		"code":   "code = ?",
		"status": "status = ?",
	})
	rows, err := a.queryMaps(r.Context(),
		`select id, name, code, status, remark,
		        cast(unix_timestamp(create_time) * 1000 as unsigned) as createTime,
		        cast(unix_timestamp(update_time) * 1000 as unsigned) as updateTime
		   from sys_roles`+where+` order by id asc`,
		args...,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeTable(w, rows)
}

func (a *app) menus(w http.ResponseWriter, r *http.Request) {
	rows, err := a.menuRows(r.Context())
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, rows)
}

func (a *app) departments(w http.ResponseWriter, r *http.Request) {
	rows, err := a.queryMaps(r.Context(),
		`select id, parent_id as parentId, name, principal, phone, email, sort, status, remark,
		        cast(unix_timestamp(create_time) * 1000 as unsigned) as createTime
		   from sys_departments order by sort asc, id asc`,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, rows)
}

func (a *app) roleMenu(w http.ResponseWriter, r *http.Request) {
	rows, err := a.menuRows(r.Context())
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, rows)
}

func (a *app) roleMenuIDs(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	roleID := intField(body, "id")
	if roleID == 0 {
		roleID = intField(body, "roleId")
	}
	rows, err := a.DB().QueryContext(r.Context(), `select menu_id from sys_role_menus where role_id = ?`, roleID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			writeDBError(w, err)
			return
		}
		ids = append(ids, id)
	}
	writeOK(w, ids)
}

func (a *app) updateRoleMenus(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	roleID := intField(body, "roleId")
	if roleID == 0 {
		roleID = intField(body, "id")
	}
	if roleID == 0 {
		writeError(w, http.StatusOK, 10001, "角色 id 不能为空")
		return
	}
	menuIDs := intSliceField(body, "menuIds")
	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `delete from sys_role_menus where role_id = ?`, roleID); err != nil {
		writeDBError(w, err)
		return
	}
	for _, menuID := range menuIDs {
		if _, err := tx.ExecContext(r.Context(), `insert into sys_role_menus (role_id, menu_id) values (?, ?)`, roleID, menuID); err != nil {
			writeDBError(w, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"updated": true})
}

func (a *app) onlineLogs(w http.ResponseWriter, r *http.Request) {
	a.table(w, r, `select id, username, ip, address, sys_online_users.system, browser, status,
	                     cast(unix_timestamp(login_time) * 1000 as unsigned) as loginTime
	                from sys_online_users order by id desc`)
}

func (a *app) loginLogs(w http.ResponseWriter, r *http.Request) {
	a.table(w, r, `select id, username, ip, address, sys_login_logs.system, browser, status, behavior,
	                     cast(unix_timestamp(login_time) * 1000 as unsigned) as loginTime
	                from sys_login_logs order by id desc`)
}

func (a *app) operationLogs(w http.ResponseWriter, r *http.Request) {
	a.table(w, r, `select id, username, module, summary, ip, address, sys_operation_logs.system, browser, method,
	                     cast(unix_timestamp(operation_time) * 1000 as unsigned) as operationTime
	                from sys_operation_logs order by id desc`)
}

func (a *app) systemLogs(w http.ResponseWriter, r *http.Request) {
	a.table(w, r, `select id, module, url, method, ip, address, sys_system_logs.system, browser, takes_time as takesTime,
	                     cast(unix_timestamp(request_time) * 1000 as unsigned) as requestTime
	                from sys_system_logs order by id desc`)
}

func (a *app) systemLogDetail(w http.ResponseWriter, r *http.Request) {
	id := intField(readBody(r), "id")
	rows, err := a.queryMaps(r.Context(),
		`select id, module, url, method, ip, address, sys_system_logs.system, browser, takes_time as takesTime,
		        request_body as requestBody, response_body as responseBody,
		        cast(unix_timestamp(request_time) * 1000 as unsigned) as requestTime
		   from sys_system_logs where id = ? limit 1`,
		id,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if len(rows) == 0 {
		writeOK(w, map[string]any{})
		return
	}
	writeOK(w, rows[0])
}

func (a *app) table(w http.ResponseWriter, r *http.Request, query string) {
	rows, err := a.queryMaps(r.Context(), query)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeTable(w, rows)
}

func (a *app) menuRows(ctx context.Context) ([]map[string]any, error) {
	return a.queryMaps(ctx,
		"select id, parent_id as parentId, menu_type as menuType, title, path, name, component, "+
			"`rank` as `rank`, icon, auths, show_link as showLink, redirect, extra_icon as extraIcon, "+
			"enter_transition as enterTransition, leave_transition as leaveTransition, active_path as activePath, "+
			"frame_src as frameSrc, frame_loading as frameLoading, keep_alive as keepAlive, hidden_tag as hiddenTag, "+
			"fixed_tag as fixedTag, show_parent as showParent "+
			"from sys_menus order by parent_id asc, `rank` asc, id asc",
	)
}

func (a *app) roleCodes(ctx context.Context, userID int) ([]string, error) {
	rows, err := a.DB().QueryContext(ctx,
		`select r.code from sys_roles r
		  join sys_user_roles ur on ur.role_id = r.id
		 where ur.user_id = ? and r.status = 1`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		roles = append(roles, code)
	}
	return roles, rows.Err()
}

func (a *app) userRouteMenus(ctx context.Context, userID int) ([]map[string]any, error) {
	return a.queryMaps(ctx,
		`select distinct m.id, m.parent_id as parentId, m.menu_type as menuType, m.title, m.path, m.name, m.component,
		        m.`+"`rank`"+` as `+"`rank`"+`, m.icon, m.auths, m.show_link as showLink
		   from sys_menus m
		   join sys_role_menus rm on rm.menu_id = m.id
		   join sys_user_roles ur on ur.role_id = rm.role_id
		   join sys_roles r on r.id = ur.role_id
		  where ur.user_id = ? and r.status = 1 and m.menu_type <> 3 and m.show_link = 1
		  order by m.parent_id asc, m.`+"`rank`"+` asc, m.id asc`,
		userID,
	)
}

func buildRouteTree(rows []map[string]any) []any {
	nodes := make(map[int]map[string]any, len(rows))
	parentIDs := make(map[int]int, len(rows))
	order := make([]int, 0, len(rows))
	for _, row := range rows {
		id := toInt(row["id"])
		parentID := toInt(row["parentId"])
		nodes[id] = routeNode(row)
		parentIDs[id] = parentID
		order = append(order, id)
	}

	var roots []any
	for _, id := range order {
		node := nodes[id]
		parentID := parentIDs[id]
		if parent, ok := nodes[parentID]; ok {
			children, _ := parent["children"].([]any)
			parent["children"] = append(children, node)
			continue
		}
		roots = append(roots, node)
	}
	return roots
}

func routeNode(row map[string]any) map[string]any {
	node := map[string]any{
		"path": row["path"],
		"meta": routeMeta(row),
	}
	if name := stringValue(row["name"]); name != "" {
		node["name"] = name
	}
	if component := stringValue(row["component"]); component != "" {
		node["component"] = component
	}
	if redirect := stringValue(row["redirect"]); redirect != "" {
		node["redirect"] = redirect
	}
	return node
}

func routeMeta(row map[string]any) map[string]any {
	meta := map[string]any{
		"title": row["title"],
	}
	if icon := stringValue(row["icon"]); icon != "" {
		meta["icon"] = icon
	}
	if rank := toInt(row["rank"]); rank != 0 {
		meta["rank"] = rank
	}
	if activePath := stringValue(row["activePath"]); activePath != "" {
		meta["activePath"] = activePath
	}
	if frameSrc := stringValue(row["frameSrc"]); frameSrc != "" {
		meta["frameSrc"] = frameSrc
	}
	if extraIcon := stringValue(row["extraIcon"]); extraIcon != "" {
		meta["extraIcon"] = extraIcon
	}
	boolMeta := map[string]string{
		"frameLoading": "frameLoading",
		"keepAlive":    "keepAlive",
		"hiddenTag":    "hiddenTag",
		"fixedTag":     "fixedTag",
		"showParent":   "showParent",
	}
	for source, target := range boolMeta {
		if value, ok := row[source].(bool); ok && value {
			meta[target] = value
		}
	}
	return meta
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func toInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 0
	}
}

func (a *app) userPermissions(ctx context.Context, userID int) ([]string, error) {
	rows, err := a.DB().QueryContext(ctx,
		`select distinct m.auths
		   from sys_menus m
		   join sys_role_menus rm on rm.menu_id = m.id
		   join sys_user_roles ur on ur.role_id = rm.role_id
		   join sys_roles r on r.id = ur.role_id
		  where ur.user_id = ? and r.status = 1 and m.auths <> ''`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []string
	for rows.Next() {
		var auth string
		if err := rows.Scan(&auth); err != nil {
			return nil, err
		}
		permissions = append(permissions, auth)
	}
	return permissions, rows.Err()
}

func (a *app) currentUserID(r *http.Request) (int, bool) {
	token := strings.TrimSpace(r.Header.Get("Authorization"))
	token = strings.TrimPrefix(token, "Bearer ")
	parts := strings.Split(token, ".")
	if len(parts) < 3 || parts[0] != "kol" {
		return 0, false
	}
	userID, err := strconv.Atoi(parts[1])
	return userID, err == nil && userID > 0
}

func (a *app) requirePerm(permission string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := a.currentUserID(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, 401, "未登录或登录已过期")
			return
		}
		allowed, err := a.hasPermission(r.Context(), userID, permission)
		if err != nil {
			writeDBError(w, err)
			return
		}
		if !allowed {
			writeError(w, http.StatusForbidden, 403, "无操作权限")
			return
		}
		next(w, r)
	}
}

func (a *app) requireMenu(path string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := a.currentUserID(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, 401, "未登录或登录已过期")
			return
		}
		allowed, err := a.hasMenu(r.Context(), userID, path)
		if err != nil {
			writeDBError(w, err)
			return
		}
		if !allowed {
			writeError(w, http.StatusForbidden, 403, "无菜单权限")
			return
		}
		next(w, r)
	}
}

func (a *app) requireAnyMenu(paths []string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := a.currentUserID(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, 401, "未登录或登录已过期")
			return
		}
		for _, path := range paths {
			allowed, err := a.hasMenu(r.Context(), userID, path)
			if err != nil {
				writeDBError(w, err)
				return
			}
			if allowed {
				next(w, r)
				return
			}
		}
		writeError(w, http.StatusForbidden, 403, "无菜单权限")
	}
}

func (a *app) hasPermission(ctx context.Context, userID int, permission string) (bool, error) {
	roles, err := a.roleCodes(ctx, userID)
	if err != nil {
		return false, err
	}
	if contains(roles, "admin") {
		return true, nil
	}
	var count int
	err = a.DB().QueryRowContext(ctx,
		`select count(*)
		   from sys_menus m
		   join sys_role_menus rm on rm.menu_id = m.id
		   join sys_user_roles ur on ur.role_id = rm.role_id
		   join sys_roles r on r.id = ur.role_id
		  where ur.user_id = ? and r.status = 1 and m.auths = ?`,
		userID,
		permission,
	).Scan(&count)
	return count > 0, err
}

func (a *app) hasMenu(ctx context.Context, userID int, path string) (bool, error) {
	roles, err := a.roleCodes(ctx, userID)
	if err != nil {
		return false, err
	}
	if contains(roles, "admin") {
		return true, nil
	}
	var count int
	err = a.DB().QueryRowContext(ctx,
		`select count(*)
		   from sys_menus m
		   join sys_role_menus rm on rm.menu_id = m.id
		   join sys_user_roles ur on ur.role_id = rm.role_id
		   join sys_roles r on r.id = ur.role_id
		  where ur.user_id = ? and r.status = 1 and m.path = ?`,
		userID,
		path,
	).Scan(&count)
	return count > 0, err
}

func (a *app) queryMaps(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	rows, err := a.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var list []map[string]any
	for rows.Next() {
		raw := make([]sql.NullString, len(cols))
		dest := make([]any, len(cols))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		item := make(map[string]any, len(cols))
		for i, col := range cols {
			if !raw[i].Valid {
				item[col] = nil
				continue
			}
			item[col] = parseColumnValue(col, raw[i].String)
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func filters(body map[string]any, rules map[string]string) (string, []any) {
	var where []string
	var args []any
	for field, clause := range rules {
		value := strings.TrimSpace(fmt.Sprint(body[field]))
		if value == "" || value == "<nil>" {
			continue
		}
		where = append(where, clause)
		if strings.Contains(clause, "like") {
			args = append(args, "%"+value+"%")
		} else {
			args = append(args, value)
		}
	}
	if len(where) == 0 {
		return "", nil
	}
	return " where " + strings.Join(where, " and "), args
}

func parseColumnValue(col string, value string) any {
	intColumns := map[string]bool{
		"id": true, "parentId": true, "menuType": true, "rank": true, "sort": true,
		"status": true, "sex": true, "takesTime": true, "createTime": true,
		"updateTime": true, "loginTime": true, "operationTime": true, "requestTime": true,
	}
	if intColumns[col] {
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
	}
	if col == "showLink" {
		return value == "1" || value == "true"
	}
	boolColumns := map[string]bool{
		"frameLoading": true,
		"keepAlive":    true,
		"hiddenTag":    true,
		"fixedTag":     true,
		"showParent":   true,
	}
	if boolColumns[col] {
		return value == "1" || value == "true"
	}
	return value
}

func route(path, component, name, icon, title string) map[string]any {
	r := map[string]any{
		"path": path,
		"name": name,
		"meta": map[string]any{
			"icon":  icon,
			"title": title,
			"roles": []string{"admin"},
		},
	}
	if component != "" {
		r["component"] = component
	}
	return r
}

func writeTable(w http.ResponseWriter, rows []map[string]any) {
	writeOK(w, tableData{List: rows, Total: len(rows), PageSize: 10, CurrentPage: 1})
}

func writeOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "操作成功", Data: data})
}

func writeError(w http.ResponseWriter, status int, code int, message string) {
	writeJSON(w, status, apiResponse{Code: code, Message: message})
}

func writeDBError(w http.ResponseWriter, err error) {
	writeError(w, http.StatusInternalServerError, 50000, err.Error())
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func readBody(r *http.Request) map[string]any {
	defer r.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return map[string]any{}
	}
	return body
}

func stringField(body map[string]any, key string) string {
	return strings.TrimSpace(fmt.Sprint(body[key]))
}

func intField(body map[string]any, key string) int {
	switch v := body[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 0
	}
}

func intSliceField(body map[string]any, key string) []int {
	raw, ok := body[key]
	if !ok {
		return nil
	}
	values, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]int, 0, len(values))
	for _, value := range values {
		switch v := value.(type) {
		case float64:
			result = append(result, int(v))
		case int:
			result = append(result, v)
		case string:
			n, _ := strconv.Atoi(v)
			if n != 0 {
				result = append(result, n)
			}
		}
	}
	return result
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (a *app) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := a.Config().CORS
		origin := r.Header.Get("Origin")
		w.Header().Set("Access-Control-Allow-Origin", corsOrigin(origin, cfg.AllowedOrigins))
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func corsOrigin(origin string, allowed []string) string {
	if len(allowed) == 0 {
		return "*"
	}
	for _, item := range allowed {
		if item == "*" {
			return "*"
		}
		if origin != "" && item == origin {
			return origin
		}
	}
	return allowed[0]
}
