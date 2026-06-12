package main

import (
	"net/http"
)

func (a *app) createUser(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	_, err := a.DB().ExecContext(r.Context(),
		`insert into sys_users
		 (dept_id, avatar, username, nickname, password_hash, phone, email, sex, status, remark)
		 values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		intField(body, "parentId"), "", stringField(body, "username"), stringField(body, "nickname"),
		sha256Hex(defaultString(stringField(body, "password"), "123456")),
		stringField(body, "phone"), stringField(body, "email"), intField(body, "sex"),
		defaultInt(intField(body, "status"), 1), stringField(body, "remark"),
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"created": true})
}

func (a *app) updateUser(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	id := intField(body, "id")
	if id == 0 {
		writeError(w, http.StatusOK, 10001, "用户 id 不能为空")
		return
	}
	_, err := a.DB().ExecContext(r.Context(),
		`update sys_users set dept_id=?, username=?, nickname=?, phone=?, email=?, sex=?, status=?, remark=? where id=?`,
		intField(body, "parentId"), stringField(body, "username"), stringField(body, "nickname"),
		stringField(body, "phone"), stringField(body, "email"), intField(body, "sex"),
		defaultInt(intField(body, "status"), 1), stringField(body, "remark"), id,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"updated": true})
}

func (a *app) deleteUser(w http.ResponseWriter, r *http.Request) {
	id := intField(readBody(r), "id")
	if id == 0 {
		writeError(w, http.StatusOK, 10001, "用户 id 不能为空")
		return
	}
	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `delete from sys_user_roles where user_id=?`, id); err != nil {
		writeDBError(w, err)
		return
	}
	if _, err := tx.ExecContext(r.Context(), `delete from sys_users where id=?`, id); err != nil {
		writeDBError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"deleted": true})
}

func (a *app) updateUserStatus(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	if _, err := a.DB().ExecContext(r.Context(), `update sys_users set status=? where id=?`, intField(body, "status"), intField(body, "id")); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"updated": true})
}

func (a *app) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	if _, err := a.DB().ExecContext(r.Context(), `update sys_users set password_hash=? where id=?`, sha256Hex(stringField(body, "password")), intField(body, "id")); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"updated": true})
}

func (a *app) updateUserRoles(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	userID := intField(body, "userId")
	roleIDs := intSliceField(body, "roleIds")
	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `delete from sys_user_roles where user_id=?`, userID); err != nil {
		writeDBError(w, err)
		return
	}
	for _, roleID := range roleIDs {
		if _, err := tx.ExecContext(r.Context(), `insert into sys_user_roles (user_id, role_id) values (?, ?)`, userID, roleID); err != nil {
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

func (a *app) createRole(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	_, err := a.DB().ExecContext(r.Context(),
		`insert into sys_roles (name, code, status, remark) values (?, ?, ?, ?)`,
		stringField(body, "name"), stringField(body, "code"), defaultInt(intField(body, "status"), 1), stringField(body, "remark"),
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"created": true})
}

func (a *app) updateRole(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	_, err := a.DB().ExecContext(r.Context(),
		`update sys_roles set name=?, code=?, status=?, remark=? where id=?`,
		stringField(body, "name"), stringField(body, "code"), defaultInt(intField(body, "status"), 1), stringField(body, "remark"), intField(body, "id"),
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"updated": true})
}

func (a *app) deleteRole(w http.ResponseWriter, r *http.Request) {
	id := intField(readBody(r), "id")
	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `delete from sys_user_roles where role_id=?`, id); err != nil {
		writeDBError(w, err)
		return
	}
	if _, err := tx.ExecContext(r.Context(), `delete from sys_role_menus where role_id=?`, id); err != nil {
		writeDBError(w, err)
		return
	}
	if _, err := tx.ExecContext(r.Context(), `delete from sys_roles where id=?`, id); err != nil {
		writeDBError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"deleted": true})
}

func (a *app) updateRoleStatus(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	if _, err := a.DB().ExecContext(r.Context(), `update sys_roles set status=? where id=?`, intField(body, "status"), intField(body, "id")); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"updated": true})
}

func (a *app) createMenu(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	_, err := a.DB().ExecContext(r.Context(),
		menuInsertSQL(),
		menuArgs(body)...,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"created": true})
}

func (a *app) updateMenu(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	args := menuArgs(body)
	args = append(args, intField(body, "id"))
	_, err := a.DB().ExecContext(r.Context(),
		`update sys_menus set parent_id=?, menu_type=?, title=?, path=?, name=?, component=?, `+"`rank`"+`=?, icon=?, auths=?,
		        show_link=?, redirect=?, extra_icon=?, enter_transition=?, leave_transition=?, active_path=?, frame_src=?,
		        frame_loading=?, keep_alive=?, hidden_tag=?, fixed_tag=?, show_parent=? where id=?`,
		args...,
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"updated": true})
}

func (a *app) deleteMenu(w http.ResponseWriter, r *http.Request) {
	id := intField(readBody(r), "id")
	if _, err := a.DB().ExecContext(r.Context(), `delete from sys_menus where id=? or parent_id=?`, id, id); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"deleted": true})
}

func (a *app) createDepartment(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	_, err := a.DB().ExecContext(r.Context(),
		`insert into sys_departments (parent_id, name, principal, phone, email, sort, status, remark) values (?, ?, ?, ?, ?, ?, ?, ?)`,
		intField(body, "parentId"), stringField(body, "name"), stringField(body, "principal"),
		stringField(body, "phone"), stringField(body, "email"), intField(body, "sort"),
		defaultInt(intField(body, "status"), 1), stringField(body, "remark"),
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"created": true})
}

func (a *app) updateDepartment(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	_, err := a.DB().ExecContext(r.Context(),
		`update sys_departments set parent_id=?, name=?, principal=?, phone=?, email=?, sort=?, status=?, remark=? where id=?`,
		intField(body, "parentId"), stringField(body, "name"), stringField(body, "principal"),
		stringField(body, "phone"), stringField(body, "email"), intField(body, "sort"),
		defaultInt(intField(body, "status"), 1), stringField(body, "remark"), intField(body, "id"),
	)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"updated": true})
}

func (a *app) deleteDepartment(w http.ResponseWriter, r *http.Request) {
	id := intField(readBody(r), "id")
	if _, err := a.DB().ExecContext(r.Context(), `delete from sys_departments where id=? or parent_id=?`, id, id); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"deleted": true})
}

func menuInsertSQL() string {
	return `insert into sys_menus
		(parent_id, menu_type, title, path, name, component, ` + "`rank`" + `, icon, auths, show_link,
		 redirect, extra_icon, enter_transition, leave_transition, active_path, frame_src,
		 frame_loading, keep_alive, hidden_tag, fixed_tag, show_parent)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
}

func menuArgs(body map[string]any) []any {
	return []any{
		intField(body, "parentId"), intField(body, "menuType"), stringField(body, "title"),
		stringField(body, "path"), stringField(body, "name"), stringField(body, "component"),
		intField(body, "rank"), stringField(body, "icon"), stringField(body, "auths"),
		boolInt(body, "showLink"), stringField(body, "redirect"), stringField(body, "extraIcon"),
		stringField(body, "enterTransition"), stringField(body, "leaveTransition"), stringField(body, "activePath"),
		stringField(body, "frameSrc"), boolInt(body, "frameLoading"), boolInt(body, "keepAlive"),
		boolInt(body, "hiddenTag"), boolInt(body, "fixedTag"), boolInt(body, "showParent"),
	}
}

func defaultInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func boolInt(body map[string]any, key string) int {
	value, ok := body[key]
	if !ok {
		return 0
	}
	if b, ok := value.(bool); ok && b {
		return 1
	}
	if n, ok := value.(float64); ok && n != 0 {
		return 1
	}
	if s, ok := value.(string); ok && (s == "true" || s == "1") {
		return 1
	}
	return 0
}
