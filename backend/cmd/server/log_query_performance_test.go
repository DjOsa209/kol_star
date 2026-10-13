package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// The monitor must fetch only one page. Returning every stored row makes both
// database transfer and JSON serialization grow with log retention.
func TestSystemLogsFetchesOnlyRequestedPage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})

	mock.ExpectQuery(`select count\(\*\) from sys_system_logs`).WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(100000),
	)
	mock.ExpectQuery(`(?s)from sys_system_logs.*order by id desc limit \? offset \?`).
		WithArgs(10, 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "module", "requestTime"}).AddRow(99990, "业务管理", 1000))

	request := httptest.NewRequest(http.MethodPost, "/system-logs", strings.NewReader(`{"currentPage":2,"pageSize":10}`))
	response := httptest.NewRecorder()
	a.systemLogs(response, request)
	var got struct {
		Code int `json:"code"`
		Data struct {
			List        []map[string]any `json:"list"`
			Total       int              `json:"total"`
			CurrentPage int              `json:"currentPage"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Code != 0 || got.Data.Total != 100000 || got.Data.CurrentPage != 2 || len(got.Data.List) != 1 {
		t.Fatalf("system logs were not bounded to requested page: %s", response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSystemLogsAppliesModuleAndTimeFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})

	mock.ExpectQuery(`select count\(\*\) from sys_system_logs where module like \? and request_time between \? and \?`).
		WithArgs("%业务管理%", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`(?s)from sys_system_logs where module like \? and request_time between \? and \? order by id desc limit \? offset \?`).
		WithArgs("%业务管理%", sqlmock.AnyArg(), sqlmock.AnyArg(), 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "module", "requestTime"}).AddRow(3, "业务管理", 1000))

	request := httptest.NewRequest(http.MethodPost, "/system-logs", strings.NewReader(`{"module":"业务管理","requestTime":["2026-10-01T00:00:00+08:00","2026-10-13T23:59:59+08:00"],"currentPage":1,"pageSize":20}`))
	response := httptest.NewRecorder()
	a.systemLogs(response, request)
	var got struct {
		Code int `json:"code"`
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Code != 0 || got.Data.Total != 1 {
		t.Fatalf("system logs did not honor filters: %s", response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMonitorPageKeepsOffsetInIntRange(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	size, page := monitorPage(map[string]any{"pageSize": 100, "currentPage": maxInt})
	if size <= 0 || page <= 0 || page > maxInt/size {
		t.Fatalf("page offset would overflow int: size=%d page=%d", size, page)
	}
}
