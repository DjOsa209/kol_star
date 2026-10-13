package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func auditToken(mock sqlmock.Sqlmock, userID int) string {
	startedAt := time.Now().Truncate(time.Second)
	sessionID := strings.Repeat("a", 64)
	mock.ExpectQuery(`select s.last_seen_unix from sys_sessions`).
		WithArgs(sessionHash(sessionID), userID, startedAt.Unix(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"last_seen_unix"}).AddRow(startedAt.Unix()))
	return newLoginTokenResponse(userID, startedAt, startedAt, sessionID)["accessToken"].(string)
}

func auditPost(t *testing.T, handler http.HandlerFunc, token string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func TestProjectCreateRecordsAuthenticatedUploader(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	token := auditToken(mock, 27)
	mock.ExpectExec(`insert into biz_projects\s*\([^)]*created_by`).
		WithArgs("Launch", "美国", "", "", "", float64(0), "USD", "未开始", "", "", nil, nil, nil, 27).
		WillReturnResult(sqlmock.NewResult(45, 1))
	rec := auditPost(t, a.createBusinessProject, token, map[string]any{
		"name": "Launch", "targetMarket": "美国", "createdBy": 999, "uploader": "spoofed",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectImportRecordsAuthenticatedUploader(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	token := auditToken(mock, 27)
	mock.ExpectBegin()
	mock.ExpectQuery(`select id from biz_projects`).WithArgs("Launch", "美国").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec(`insert into biz_projects\s*\([^)]*created_by`).
		WithArgs("Launch", "美国", "", "", "", float64(0), "USD", "未开始", "", "", nil, nil, 27).
		WillReturnResult(sqlmock.NewResult(45, 1))
	mock.ExpectCommit()
	rec := auditPost(t, a.importBusinessProjects, token, map[string]any{
		"rows": []map[string]any{{"name": "Launch", "targetMarket": "美国", "createdBy": 999, "uploader": "spoofed"}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("import status=%d body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectListIncludesReadOnlyAuditMetadata(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectQuery(`select.*created_by as uploadedBy.*from biz_projects`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "uploadedBy", "uploaderName"}).AddRow(45, 27, "Ada"))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	a.businessProjects(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("project list status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Data struct {
			List []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data.List) != 1 || response.Data.List[0]["uploadedBy"] != "27" || response.Data.List[0]["uploaderName"] != "Ada" {
		t.Fatalf("project audit metadata absent: %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPResourceCreateRecordsAuthenticatedUploader(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	token := auditToken(mock, 27)
	mock.ExpectExec(`insert into biz_ip_resources\s*\([^)]*created_by`).
		WithArgs("Sonic", "游戏", "Rights Owner", "", `["美国"]`, "", "", "待评估", "CNY", nil, nil, "", "null", 27).
		WillReturnResult(sqlmock.NewResult(45, 1))
	rec := auditPost(t, a.ipResourceSave, token, map[string]any{
		"name": "Sonic", "ipType": "游戏", "rightsOwner": "Rights Owner", "markets": []string{"美国"},
		"createdBy": 999, "uploadedBy": 999,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("IP create status=%d body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPResourceImportRecordsAuthenticatedUploader(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	token := auditToken(mock, 27)
	mock.ExpectQuery(`select count\(\*\) from biz_ip_resources where name=\? and rights_owner=\?`).
		WithArgs("Sonic", "Rights Owner").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectExec(`insert ignore into biz_ip_resources\s*\([^)]*created_by`).
		WithArgs("Sonic", "游戏", "Rights Owner", "", `["美国"]`, "", "", "待评估", "CNY", nil, nil, "", "null", 27).
		WillReturnResult(sqlmock.NewResult(45, 1))
	mock.ExpectCommit()
	rec := auditPost(t, a.ipImport, token, map[string]any{
		"rows": []map[string]any{{
			"name": "Sonic", "ipType": "游戏", "rightsOwner": "Rights Owner", "markets": []string{"美国"},
			"createdBy": 999, "uploadedBy": 999,
		}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("IP import status=%d body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPFileUploadRecordsAuthenticatedUploader(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	token := auditToken(mock, 27)
	previousRoot := ipFileRoot
	ipFileRoot = t.TempDir()
	defer func() { ipFileRoot = previousRoot }()

	mock.ExpectQuery(`select count\(\*\) from biz_ip_resources`).WithArgs(int64(45)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(`insert into biz_ip_files\s*\([^)]*uploaded_by`).
		WithArgs(int64(45), nil, "copyright", "rights.pdf", sqlmock.AnyArg(), len("%PDF-audit"), 27).
		WillReturnResult(sqlmock.NewResult(61, 1))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("ipId", "45")
	_ = writer.WriteField("fileKind", "copyright")
	_ = writer.WriteField("uploadedBy", "999")
	file, err := writer.CreateFormFile("file", "rights.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("%PDF-audit")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	a.ipFileUpload(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("IP file upload status=%d body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUploaderWritesRejectMissingSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	for name, handler := range map[string]http.HandlerFunc{
		"project create":     a.createBusinessProject,
		"project import":     a.importBusinessProjects,
		"IP resource create": a.ipResourceSave,
		"IP resource import": a.ipImport,
		"IP file upload":     a.ipFileUpload,
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler(rec, httptest.NewRequest(http.MethodPost, "/", nil))
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPResourceDetailIncludesReadOnlyAuditMetadata(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectQuery(`select.*created_by as uploadedBy.*from biz_ip_resources`).WithArgs(int64(45)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "uploadedBy", "uploaderName"}).AddRow(45, 27, "Ada"))
	mock.ExpectQuery(`select.*from biz_ip_cases`).WithArgs(int64(45)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`select.*uploaded_by as uploadedBy.*from biz_ip_files`).WithArgs(int64(45)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "uploadedBy", "uploaderName"}).AddRow(61, 27, "Ada"))
	req := httptest.NewRequest(http.MethodGet, "/?id=45", nil)
	rec := httptest.NewRecorder()
	a.ipResourceDetail(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("IP detail status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Data struct {
			Resource map[string]any   `json:"resource"`
			Files    []map[string]any `json:"files"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data.Files) != 1 || response.Data.Resource["uploadedBy"] != "27" || response.Data.Files[0]["uploadedBy"] != "27" || response.Data.Resource["uploaderName"] != "Ada" {
		t.Fatalf("audit metadata absent: %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
