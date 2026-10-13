package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

type mockIPStageSender struct {
	emails []string
	fail   map[string]bool
}

func (sender *mockIPStageSender) sendToEmail(_ context.Context, email, message string) error {
	if !strings.Contains(message, "XMP IP 需求提醒") {
		return errors.New("missing IP notification title")
	}
	sender.emails = append(sender.emails, email)
	if sender.fail[email] {
		return errors.New("mock send failed")
	}
	return nil
}

func TestIPStageNotificationRecipientsDeduplicateAndSkipUnavailable(t *testing.T) {
	brief := map[string]any{"contactsByStage": map[string]any{
		"imc":   map[string]any{"id": 11, "name": "甲"},
		"brand": map[string]any{"id": 11, "name": "甲"},
		"user":  map[string]any{"id": 12, "name": "乙"},
		"media": map[string]any{"id": 13, "name": "丙"},
		"pr":    map[string]any{"id": 14, "name": "丁"},
	}}
	briefJSON, _ := json.Marshal(brief)
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	cfg := Config{Feishu: FeishuConfig{ApplicationEnabled: true, AppID: "app", AppSecret: "secret", FrontendURL: "https://xmp.example.com/"}}
	mock.ExpectQuery("select project_name, coalesce\\(brief,''\\), created_by from biz_ip_requests").WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"project_name", "brief", "created_by"}).AddRow("新品联名", string(briefJSON), 99))
	mock.ExpectQuery("select coalesce\\(email,''\\), status, auth_provider from sys_users").WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"email", "status", "auth_provider"}).AddRow("A@example.com", 1, "uac"))
	mock.ExpectQuery("select coalesce\\(email,''\\), status, auth_provider from sys_users").WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"email", "status", "auth_provider"}).AddRow("", 1, "uac"))
	mock.ExpectQuery("select coalesce\\(email,''\\), status, auth_provider from sys_users").WithArgs(int64(13)).
		WillReturnRows(sqlmock.NewRows([]string{"email", "status", "auth_provider"}).AddRow("inactive@example.com", 0, "uac"))
	mock.ExpectQuery("select coalesce\\(email,''\\), status, auth_provider from sys_users").WithArgs(int64(14)).
		WillReturnRows(sqlmock.NewRows([]string{"email", "status", "auth_provider"}).AddRow("b@example.com", 1, "uac"))
	sender := &mockIPStageSender{}
	result, err := a.sendIPStageNotifications(context.Background(), cfg, 42, "ip_reviewed", sender)
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent != 2 || result.Skipped != 2 || result.Failed != 0 {
		t.Fatalf("unexpected delivery result: %+v", result)
	}
	if strings.Join(sender.emails, ",") != "a@example.com,b@example.com" {
		t.Fatalf("unexpected recipients: %v", sender.emails)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPStageNotificationDisabledDoesNotQueryOrSend(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	sender := &mockIPStageSender{}
	result, err := a.sendIPStageNotifications(context.Background(), Config{}, 42, "submitted", sender)
	if err != nil || result != (ipStageNotificationResult{}) || len(sender.emails) != 0 {
		t.Fatalf("disabled notification: result=%+v err=%v emails=%v", result, err, sender.emails)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPStageNotificationCompletionIncludesCreatorOnce(t *testing.T) {
	brief := map[string]any{"contactsByStage": map[string]any{
		"product": map[string]any{"id": 9, "name": "发起人"},
	}}
	ids := ipStageContactIDs(brief, 9, "marketing_reviewed")
	if len(ids) != 1 || ids[0] != 9 {
		t.Fatalf("completion recipients=%v, want creator once", ids)
	}
	cfg := Config{SSO: SSOConfig{FrontendURL: "https://xmp.example.com/"}}
	message := ipStageFeishuMessage(cfg, 42, "新品联名", "ip_reviewed")
	if !strings.Contains(message, "https://xmp.example.com/#/business/ip/marketing?id=42") {
		t.Fatalf("stage-specific deep link missing: %s", message)
	}
}

func TestIPStageNotificationDeliveryFailureDoesNotStopNextRecipient(t *testing.T) {
	brief := map[string]any{"contactsByStage": map[string]any{
		"imc": map[string]any{"id": 11}, "brand": map[string]any{"id": 12},
	}}
	briefJSON, _ := json.Marshal(brief)
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	cfg := Config{Feishu: FeishuConfig{ApplicationEnabled: true, AppID: "app", AppSecret: "secret"}}
	mock.ExpectQuery("select project_name").WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"project_name", "brief", "created_by"}).AddRow("新品联名", string(briefJSON), 9))
	mock.ExpectQuery("select coalesce\\(email,''\\)").WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"email", "status", "auth_provider"}).AddRow("a@example.com", 1, "uac"))
	mock.ExpectQuery("select coalesce\\(email,''\\)").WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"email", "status", "auth_provider"}).AddRow("b@example.com", 1, "uac"))
	sender := &mockIPStageSender{fail: map[string]bool{"a@example.com": true}}
	result, err := a.sendIPStageNotifications(context.Background(), cfg, 42, "ip_reviewed", sender)
	if err != nil || result.Sent != 1 || result.Failed != 1 || len(sender.emails) != 2 {
		t.Fatalf("unexpected delivery outcome: result=%+v err=%v emails=%v", result, err, sender.emails)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPStageNotificationResolvesUnloggedSSOColleagueByEmployeeNo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/service-pf-open-gateway/oauth/token":
			_, _ = w.Write([]byte(`{"code":"0","data":{"access_token":"mock-token","expires_in":1800}}`))
		case "/service-pf-open-gateway/open-apis/v2/api/uac-user/user/pages":
			var body struct {
				Param map[string]any `json:"param"`
			}
			if r.Header.Get("Authorization") != "Bearer mock-token" || json.NewDecoder(r.Body).Decode(&body) != nil || body.Param["empNo"] != "1001" {
				t.Error("unexpected UAC employee lookup")
			}
			_, _ = w.Write([]byte(`{"code":"0","data":{"dataList":[{"uid":"15682833519993842","realName":"张三","employeeNo":"1001","email":"zhang@example.com","status":"OK"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	brief := map[string]any{"contactsByStage": map[string]any{
		"ipMarketing": map[string]any{"id": "uac:15682833519993842", "name": "张三", "employeeNo": "1001"},
	}}
	briefJSON, _ := json.Marshal(brief)
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectQuery("select project_name").WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"project_name", "brief", "created_by"}).AddRow("新品联名", string(briefJSON), 9))
	cfg := Config{
		Feishu: FeishuConfig{ApplicationEnabled: true, AppID: "feishu-app", AppSecret: "feishu-secret"},
		SSO:    SSOConfig{Enabled: true, Provider: "uac", UACGateway: server.URL, UACAppID: "uac-app", UACAppSecret: "uac-secret"},
	}
	sender := &mockIPStageSender{}
	result, err := a.sendIPStageNotifications(context.Background(), cfg, 42, "submitted", sender)
	if err != nil || result.Sent != 1 || strings.Join(sender.emails, ",") != "zhang@example.com" {
		t.Fatalf("unlogged SSO recipient: result=%+v err=%v emails=%v", result, err, sender.emails)
	}
	spoofed := map[string]any{"contactsByStage": map[string]any{
		"ipMarketing": map[string]any{"id": "uac:someone-else", "name": "张三", "employeeNo": "1001"},
	}}
	spoofedJSON, _ := json.Marshal(spoofed)
	mock.ExpectQuery("select project_name").WithArgs(int64(43)).
		WillReturnRows(sqlmock.NewRows([]string{"project_name", "brief", "created_by"}).AddRow("伪造联系人", string(spoofedJSON), 9))
	spoofedResult, err := a.sendIPStageNotifications(context.Background(), cfg, 43, "submitted", sender)
	if err != nil || spoofedResult.Sent != 0 || spoofedResult.Skipped != 1 || len(sender.emails) != 1 {
		t.Fatalf("mismatched directory identity was notified: result=%+v err=%v emails=%v", spoofedResult, err, sender.emails)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPStageNotificationRechecksCachedSSOStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/service-pf-open-gateway/oauth/token":
			_, _ = w.Write([]byte(`{"code":"0","data":{"access_token":"mock-token","expires_in":1800}}`))
		case "/service-pf-open-gateway/open-apis/v2/api/uac-user/user/pages":
			_, _ = w.Write([]byte(`{"code":"0","data":{"dataList":[{"uid":"501","realName":"已停用同事","employeeNo":"1001","email":"old@example.com","status":"FREEZE"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	briefJSON, _ := json.Marshal(map[string]any{"contactsByStage": map[string]any{
		"ipMarketing": map[string]any{"id": 11, "name": "已停用同事"},
	}})
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectQuery("select project_name").WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"project_name", "brief", "created_by"}).AddRow("新品联名", string(briefJSON), 9))
	mock.ExpectQuery("select coalesce\\(email,''\\)").WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"email", "status", "auth_provider"}).AddRow("old@example.com", 1, "uac"))
	mock.ExpectQuery("select employee_no").WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"employee_no", "external_subject"}).AddRow("1001", "501"))
	cfg := Config{
		Feishu: FeishuConfig{ApplicationEnabled: true, AppID: "feishu-app", AppSecret: "feishu-secret"},
		SSO:    SSOConfig{Enabled: true, Provider: "uac", UACGateway: server.URL, UACAppID: "uac-app", UACAppSecret: "uac-secret"},
	}
	sender := &mockIPStageSender{}
	result, err := a.sendIPStageNotifications(context.Background(), cfg, 42, "submitted", sender)
	if err != nil || result.Sent != 0 || result.Skipped != 1 || len(sender.emails) != 0 {
		t.Fatalf("inactive cached SSO recipient: result=%+v err=%v emails=%v", result, err, sender.emails)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
