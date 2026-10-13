package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSessionValidationFailsClosedAndRevocationTakesEffect(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	started := time.Now().Truncate(time.Second)
	sessionID := strings.Repeat("a", 64)
	token := newLoginTokenResponse(7, started, started, sessionID)["accessToken"].(string)
	req := httptest.NewRequest(http.MethodGet, "/mine", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	mock.ExpectQuery(`select s.last_seen_unix from sys_sessions`).
		WithArgs(sessionHash(sessionID), 7, started.Unix(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"last_seen_unix"}).AddRow(started.Unix()))
	if userID, ok := a.currentUserID(req); !ok || userID != 7 {
		t.Fatalf("active session rejected: id=%d valid=%v", userID, ok)
	}
	mock.ExpectQuery(`select s.last_seen_unix from sys_sessions`).
		WillReturnRows(sqlmock.NewRows([]string{"last_seen_unix"}))
	if _, ok := a.currentUserID(req); ok {
		t.Fatal("revoked session retained access")
	}
	mock.ExpectQuery(`select s.last_seen_unix from sys_sessions`).
		WillReturnError(errors.New("database unavailable"))
	if _, ok := a.currentUserID(req); ok {
		t.Fatal("database outage silently authenticated a request")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoginResponseCreatesServerSideSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectQuery(`select r.code from sys_roles`).WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("admin"))
	mock.ExpectQuery(`select distinct m.auths`).WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{"auths"}))
	mock.ExpectExec(`insert into sys_sessions`).WillReturnResult(sqlmock.NewResult(31, 1))
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	data, err := a.loginResponseData(req, 7, "", "alice", "Alice")
	if err != nil {
		t.Fatal(err)
	}
	access, ok := data["accessToken"].(string)
	if !ok {
		t.Fatalf("login did not return an access token: %#v", data)
	}
	token, ok := parseAccessToken(access, time.Now())
	if !ok || token.SessionID == "" {
		t.Fatal("login returned a token without a random session identifier")
	}
	mock.ExpectQuery(`select s.last_seen_unix from sys_sessions`).
		WithArgs(sessionHash(token.SessionID), 7, token.SessionStartedAt.Unix(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"last_seen_unix"}).AddRow(time.Now().Unix()))
	req.Header.Set("Authorization", "Bearer "+access)
	if userID, valid := a.currentUserID(req); !valid || userID != 7 {
		t.Fatalf("login session could not be used: id=%d valid=%v", userID, valid)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestForcedOfflineRevokesExactlyOneSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectExec(`update sys_sessions set revoked_at_unix`).
		WithArgs(sqlmock.AnyArg(), 28, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	req := httptest.NewRequest(http.MethodPost, "/online-logs/offline", strings.NewReader(`{"id":28}`))
	rec := httptest.NewRecorder()
	a.forceSessionOffline(rec, req)
	var response apiResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != 0 {
		t.Fatalf("forced offline failed: %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreatedSessionWorksUntilForcedOffline(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	started := time.Now().Truncate(time.Second)
	sessionID := strings.Repeat("c", 64)
	mock.ExpectExec(`insert into sys_sessions`).
		WithArgs(sessionHash(sessionID), 7, "alice", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), started.Unix(), started.Unix(), started.Add(maximumSessionLifetime).Unix()).
		WillReturnResult(sqlmock.NewResult(31, 1))
	if err := a.createSession(httptest.NewRequest(http.MethodPost, "/login", nil), 7, "alice", sessionID, started); err != nil {
		t.Fatal(err)
	}
	tokens := newLoginTokenResponse(7, started, started, sessionID)
	req := httptest.NewRequest(http.MethodGet, "/mine", nil)
	req.Header.Set("Authorization", "Bearer "+tokens["accessToken"].(string))
	mock.ExpectQuery(`select s.last_seen_unix from sys_sessions`).
		WillReturnRows(sqlmock.NewRows([]string{"last_seen_unix"}).AddRow(started.Unix()))
	if id, ok := a.currentUserID(req); !ok || id != 7 {
		t.Fatalf("new session was not accepted: id=%d ok=%v", id, ok)
	}
	mock.ExpectExec(`update sys_sessions set revoked_at_unix`).
		WithArgs(sqlmock.AnyArg(), 31, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	offlineReq := httptest.NewRequest(http.MethodPost, "/online-logs/offline", strings.NewReader(`{"id":31}`))
	offlineRec := httptest.NewRecorder()
	a.forceSessionOffline(offlineRec, offlineReq)
	if offlineRec.Code != http.StatusOK {
		t.Fatalf("force offline failed: %s", offlineRec.Body.String())
	}
	mock.ExpectQuery(`select s.last_seen_unix from sys_sessions`).
		WillReturnRows(sqlmock.NewRows([]string{"last_seen_unix"}))
	if _, ok := a.currentUserID(req); ok {
		t.Fatal("forced offline did not invalidate access token")
	}
	mock.ExpectQuery(`select s.last_seen_unix from sys_sessions`).
		WillReturnRows(sqlmock.NewRows([]string{"last_seen_unix"}))
	refreshReq := httptest.NewRequest(http.MethodPost, "/refresh-token", strings.NewReader(fmt.Sprintf(`{"refreshToken":%q}`, tokens["refreshToken"])))
	refreshRec := httptest.NewRecorder()
	a.refreshToken(refreshRec, refreshReq)
	if refreshRec.Code != http.StatusUnauthorized {
		t.Fatalf("forced offline did not invalidate refresh token: %s", refreshRec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRevokedSessionCannotRefresh(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	started := time.Now().Truncate(time.Second)
	sessionID := strings.Repeat("b", 64)
	refresh := newLoginTokenResponse(7, started, started, sessionID)["refreshToken"].(string)
	mock.ExpectQuery(`select s.last_seen_unix from sys_sessions`).
		WithArgs(sessionHash(sessionID), 7, started.Unix(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"last_seen_unix"}))
	req := httptest.NewRequest(http.MethodPost, "/refresh-token", strings.NewReader(fmt.Sprintf(`{"refreshToken":%q}`, refresh)))
	rec := httptest.NewRecorder()
	a.refreshToken(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked refresh token status=%d response=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLogoutRevokesSessionAfterAccessTokenExpires(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	started := time.Now().Add(-3 * time.Hour).Truncate(time.Second)
	sessionID := strings.Repeat("d", 64)
	tokens := newLoginTokenResponse(7, started, started, sessionID)
	request := httptest.NewRequest(http.MethodPost, "/logout", strings.NewReader(fmt.Sprintf(`{"refreshToken":%q}`, tokens["refreshToken"])))
	request.Header.Set("Authorization", "Bearer "+tokens["accessToken"].(string))
	mock.ExpectQuery(`select s.last_seen_unix from sys_sessions`).
		WithArgs(sessionHash(sessionID), 7, started.Unix(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"last_seen_unix"}).AddRow(time.Now().Unix()))
	mock.ExpectExec(`update sys_sessions set revoked_at_unix`).
		WithArgs(sqlmock.AnyArg(), sessionHash(sessionID)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	response := httptest.NewRecorder()
	a.logout(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expired access token prevented explicit logout: %s", response.Body.String())
	}
	mock.ExpectQuery(`select s.last_seen_unix from sys_sessions`).
		WillReturnRows(sqlmock.NewRows([]string{"last_seen_unix"}))
	refreshReq := httptest.NewRequest(http.MethodPost, "/refresh-token", strings.NewReader(fmt.Sprintf(`{"refreshToken":%q}`, tokens["refreshToken"])))
	refreshRec := httptest.NewRecorder()
	a.refreshToken(refreshRec, refreshReq)
	if refreshRec.Code != http.StatusUnauthorized {
		t.Fatalf("logout did not revoke refresh token: %s", refreshRec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestOperationAuditClassifiesOnlyWritesAndChecksOutcome(t *testing.T) {
	for _, path := range []string{"/api/user", "/api/operation-logs", "/api/business/ip/resources/list", "/api/login"} {
		if action := operationAction(path); action != "" {
			t.Fatalf("read/auth route %s classified as operation: %s", path, action)
		}
	}
	if action := operationAction("/api/business/ip/resources/save"); action != "保存 IP 资源" {
		t.Fatalf("IP write route action = %q", action)
	}
	if apiResponseOK("not JSON") {
		t.Fatal("unparseable operation result classified as success")
	}
	if apiResponseOK(`{}`) || apiResponseOK(`{"code":null}`) {
		t.Fatal("response without an API result code classified as success")
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectExec(`insert into sys_system_logs`).WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectExec(`insert into sys_operation_logs`).
		WithArgs("alice", "系统管理", "新增用户", "POST", 0, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(12, 1))
	req := httptest.NewRequest(http.MethodPost, "/api/user/create", nil)
	a.recordPostRequestLog(req, "[credentials omitted]", `{"code":10001}`, time.Millisecond, http.StatusOK, "alice", false)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLargeSuccessfulResponseKeepsOperationSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	recorder := &loggingResponseWriter{ResponseWriter: httptest.NewRecorder()}
	writeOK(recorder, map[string]any{"large": strings.Repeat("x", 5000)})
	if recorder.body.Len() != 4000 || !recorder.apiResponseOK() {
		t.Fatalf("successful response over log capture limit lost result code: captured=%d success=%v", recorder.body.Len(), recorder.apiResponseOK())
	}
	mock.ExpectExec(`insert into sys_system_logs`).WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectExec(`insert into sys_operation_logs`).
		WithArgs("alice", "系统管理", "新增用户", "POST", 1, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(12, 1))
	request := httptest.NewRequest(http.MethodPost, "/api/user/create", nil)
	a.recordPostRequestLog(request, "[credentials omitted]", recorder.body.String(), time.Millisecond, http.StatusOK, "alice", recorder.apiResponseOK())
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCredentialRequestsAreOmittedFromSystemLog(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/mine/password", strings.NewReader(`{"oldPassword":"private-old","newPassword":"private-new"}`))
	if got := captureRequestBody(req); got != "[credentials omitted]" {
		t.Fatalf("credential body was logged: %s", got)
	}
	if body := readBody(req); stringField(body, "newPassword") != "private-new" {
		t.Fatal("redaction consumed the body needed by the handler")
	}
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/logout", strings.NewReader(`{"refreshToken":"private-refresh"}`))
	if got := captureRequestBody(logoutReq); got != "[credentials omitted]" {
		t.Fatalf("logout refresh token was logged: %s", got)
	}
	if body := readBody(logoutReq); stringField(body, "refreshToken") != "private-refresh" {
		t.Fatal("logout redaction consumed the body needed by the handler")
	}
}
