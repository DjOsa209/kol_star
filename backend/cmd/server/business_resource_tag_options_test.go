package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestResourceTagOptionsRouteReadOnlyPermission(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, menu string
		authenticated            bool
		allowed, wantStatus      int
	}{
		{"resource reader gets options", http.MethodGet, "/business/resources/tag-options", "/business/resources", true, 1, http.StatusOK},
		{"no resource access", http.MethodGet, "/business/resources/tag-options", "/business/resources", true, 0, http.StatusForbidden},
		{"anonymous denied", http.MethodGet, "/business/resources/tag-options", "", false, 0, http.StatusUnauthorized},
		{"resource reader cannot create tags", http.MethodPost, "/business/tags/create", "/business/tags", true, 0, http.StatusForbidden},
		{"resource reader cannot manage tags", http.MethodGet, "/business/tags", "/business/tags", true, 0, http.StatusForbidden},
		{"options has no write method", http.MethodPost, "/business/resources/tag-options", "", false, 0, http.StatusMethodNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			a := newApp(db, Config{})
			mux := http.NewServeMux()
			a.routes(mux)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"name":"attempted tag"}`))
			if tc.authenticated {
				token := auditToken(mock, 7)
				req.Header.Set("Authorization", "Bearer "+token)
				mock.ExpectQuery(`select r.code from sys_roles r`).WithArgs(7).
					WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("resource-reader"))
				mock.ExpectQuery(`(?s)select count\(\*\).*m.path = \?`).WithArgs(7, tc.menu).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tc.allowed))
			}
			if tc.wantStatus == http.StatusOK {
				// The query must exclude disabled tags on the server. Resource
				// readers receive usable options without access to tag management.
				mock.ExpectQuery(`select id,name,category,color,status from biz_tags where status='启用' order by category,id`).
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "category", "color", "status"}).
						AddRow(3, "数码评测", "内容领域", "#409EFF", "启用"))
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusOK {
				var response struct {
					Code int `json:"code"`
					Data []struct {
						ID     int    `json:"id"`
						Name   string `json:"name"`
						Status string `json:"status"`
					} `json:"data"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response.Code != 0 || len(response.Data) != 1 || response.Data[0].ID != 3 || response.Data[0].Name != "数码评测" || response.Data[0].Status != "启用" {
					t.Fatalf("unexpected tag options: %s", rec.Body.String())
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
