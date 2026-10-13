package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestUsersReturnsRequestedPage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectQuery(`select count\(\*\) from sys_users u`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(23))
	mock.ExpectQuery(`from sys_users u left join sys_departments d on d.id = u.dept_id.*order by u.id asc limit \? offset \?`).
		WithArgs(10, 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "nickname", "deptId", "deptName"}).
			AddRow(11, "second-page-user", "第二页用户", 2, "测试部门"))

	req := httptest.NewRequest(http.MethodPost, "/user", bytes.NewBufferString(`{"pageSize":10,"currentPage":2}`))
	rec := httptest.NewRecorder()
	a.users(rec, req)
	var response struct {
		Code int `json:"code"`
		Data struct {
			List        []map[string]any `json:"list"`
			Total       int              `json:"total"`
			PageSize    int              `json:"pageSize"`
			CurrentPage int              `json:"currentPage"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response %s: %v", rec.Body.String(), err)
	}
	if rec.Code != http.StatusOK || response.Code != 0 || response.Data.Total != 23 || response.Data.PageSize != 10 || response.Data.CurrentPage != 2 || len(response.Data.List) != 1 || response.Data.List[0]["username"] != "second-page-user" {
		t.Fatalf("user page response = %s; want page 2 with one user and total 23", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoginLogsShowsNicknameAndFallsBackForUnknownAccount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectQuery(`select count\(\*\) from sys_login_logs l`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery(`(?s)select l.id, l.username, coalesce\(nullif\(u.nickname, ''\), l.username\) as nickname,.*from sys_login_logs l left join sys_users u on u.username = l.username.*order by l.id desc limit \? offset \?`).
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "nickname", "status"}).
			AddRow(2, "employee-1", "张三", 1).
			AddRow(1, "unknown", "unknown", 0))

	req := httptest.NewRequest(http.MethodPost, "/login-logs", bytes.NewBufferString(`{"pageSize":10,"currentPage":1}`))
	rec := httptest.NewRecorder()
	a.loginLogs(rec, req)
	var response struct {
		Code int `json:"code"`
		Data struct {
			List []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response %s: %v", rec.Body.String(), err)
	}
	if rec.Code != http.StatusOK || response.Code != 0 || len(response.Data.List) != 2 || response.Data.List[0]["nickname"] != "张三" || response.Data.List[1]["nickname"] != "unknown" {
		t.Fatalf("login log response = %s; want nickname and username fallback", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
