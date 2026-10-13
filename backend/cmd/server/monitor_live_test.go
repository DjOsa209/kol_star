package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestOnlineUsersOnlyListsLiveSessions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})

	// A new installation can contain the old demo row, but that is not a live session.
	mock.ExpectQuery(`select count\(\*\) from sys_sessions`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`(?s)from sys_sessions.*limit \? offset \?`).WillReturnRows(sqlmock.NewRows([]string{"id", "username", "nickname", "ip", "address", "system", "browser", "status", "loginTime", "lastActiveTime"}))
	request := httptest.NewRequest(http.MethodPost, "/online-logs", strings.NewReader(`{"currentPage":1,"pageSize":10}`))
	response := httptest.NewRecorder()
	a.onlineLogs(response, request)
	var got struct {
		Code int `json:"code"`
		Data struct {
			List []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Code != 0 || len(got.Data.List) != 0 {
		t.Fatalf("online users returned demo/non-live data: %s", response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestOperationLogsApplyFiltersInsteadOfShowingDemoRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})

	mock.ExpectQuery(`select count\(\*\) from sys_operation_logs where`).WithArgs("%业务管理%", "1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`(?s)from sys_operation_logs where.*limit \? offset \?`).WithArgs("%业务管理%", "1", 10, 0).WillReturnRows(sqlmock.NewRows([]string{"id", "username", "module", "summary", "status", "operationTime"}))
	request := httptest.NewRequest(http.MethodPost, "/operation-logs", strings.NewReader(`{"module":"业务管理","status":"1"}`))
	response := httptest.NewRecorder()
	a.operationLogs(response, request)
	var got struct {
		Code int `json:"code"`
		Data struct {
			List []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Code != 0 || len(got.Data.List) != 0 {
		t.Fatalf("operation logs ignored filters or returned demo data: %s", response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
