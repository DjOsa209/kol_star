package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestUACAuthorizationURL(t *testing.T) {
	loginURL, err := uacAuthorizationURL(SSOConfig{
		RedirectURI: "https://xmp.example.com/sso/callback",
		UACGateway:  "https://uac.example.com/",
		UACAppID:    "xmp-app",
		UACLang:     "zh_CN",
		UACSource:   "xmp",
	}, "state-value")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(loginURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Path != "/uac-auth-service/v2/api/uac-auth/login/redirect/web-login" {
		t.Fatalf("unexpected login path: %s", parsed.Path)
	}
	if parsed.Query().Get("appId") != "xmp-app" || parsed.Query().Get("source") != "xmp" {
		t.Fatalf("unexpected login query: %s", parsed.RawQuery)
	}
	redirect, err := url.Parse(parsed.Query().Get("redirect"))
	if err != nil {
		t.Fatal(err)
	}
	if redirect.Query().Get("state") != "state-value" || redirect.Path != "/sso/callback" || redirect.Fragment != "" {
		t.Fatalf("unexpected redirect URL: %s", redirect)
	}
}

func TestFetchUACIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/uac-auth-service/v2/api/uac-auth/utoken/getUserInfo" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("P-Auth") != "request-token" || r.Header.Get("P-Rtoken") != "user-token" || r.Header.Get("P-AppId") != "xmp-app" {
			t.Errorf("unexpected UAC headers: %#v", r.Header)
		}
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload["rtoken"] != "request-token" || payload["utoken"] != "user-token" {
			t.Errorf("unexpected UAC payload: %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"userInfo":{"uid":"u-1001","email":"user@example.com","realName":"张三","employeeNo":"1001","deptName":"市场部","sex":1,"avatarUrl":"https://cdn.example.com/u-1001.png"}}}`))
	}))
	defer server.Close()

	identity, err := fetchUACIdentity(context.Background(), SSOConfig{
		UACGateway: server.URL,
		UACAppID:   "xmp-app",
	}, uacCallbackInput{Token: "request-token", RToken: "user-token"}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if identity.Subject != "u-1001" || identity.Email != "user@example.com" || identity.Name != "张三" || identity.Department != "市场部" || identity.Avatar != "https://cdn.example.com/u-1001.png" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
	if identity.Sex == nil || *identity.Sex != 1 {
		t.Fatalf("unexpected identity sex: %#v", identity.Sex)
	}
}

func TestFirstSSOSex(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  *int
	}{
		{name: "male number", value: float64(0), want: intPointer(0)},
		{name: "female number", value: float64(1), want: intPointer(1)},
		{name: "female string", value: "1", want: intPointer(1)},
		{name: "unsupported value", value: float64(2)},
		{name: "missing value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile := map[string]any{}
			if test.value != nil {
				profile["sex"] = test.value
			}
			got := firstSSOSex(profile, "sex")
			if test.want == nil {
				if got != nil {
					t.Fatalf("sex = %d, want nil", *got)
				}
				return
			}
			if got == nil || *got != *test.want {
				t.Fatalf("sex = %v, want %d", got, *test.want)
			}
		})
	}
}

func TestEnsureSSOUserUpdatesSexOnlyWhenProvided(t *testing.T) {
	for _, test := range []struct {
		name       string
		sex        *int
		wantHasSex bool
		wantSex    int
	}{
		{name: "female", sex: intPointer(1), wantHasSex: true, wantSex: 1},
		{name: "missing", wantHasSex: false, wantSex: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()

			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(`select id, avatar, username, nickname, status from sys_users where auth_provider = ? and external_subject = ? limit 1`)).
				WithArgs("uac", "u-1001").
				WillReturnRows(sqlmock.NewRows([]string{"id", "avatar", "username", "nickname", "status"}).AddRow(3, "", "sso-user", "旧昵称", 1))
			mock.ExpectExec(`update sys_users set`).
				WithArgs("", "", "user@example.com", "张三", "张三", test.wantHasSex, test.wantSex, "uac", "u-1001", "1001", "市场部", 3).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery(`select count\(\*\) from sys_user_roles`).
				WithArgs(3).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			mock.ExpectCommit()

			a := newApp(db, Config{}.withDefaults())
			if _, err := a.ensureSSOUser(context.Background(), ssoIdentity{
				Provider: "uac", Subject: "u-1001", Email: "user@example.com", Name: "张三",
				EmployeeNo: "1001", Department: "市场部", Sex: test.sex,
			}); err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func intPointer(value int) *int {
	return &value
}

func TestEnrichSSOAvatarFromFeishuUserID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_, _ = w.Write([]byte(`{"code":0,"tenant_access_token":"tenant-token","expire":7200}`))
		case "/open-apis/contact/v3/users/batch_get_id":
			if r.URL.Query().Get("user_id_type") != "user_id" || r.Header.Get("Authorization") != "Bearer tenant-token" {
				t.Fatalf("unexpected Feishu lookup request: %s %#v", r.URL.String(), r.Header)
			}
			var payload map[string][]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if len(payload["emails"]) != 1 || payload["emails"][0] != "user@example.com" {
				t.Fatalf("email was not normalized: %#v", payload)
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"user_list":[{"user_id":"1001"}]}}`))
		case "/open-apis/contact/v3/users/1001":
			if r.URL.Query().Get("user_id_type") != "user_id" || r.Header.Get("Authorization") != "Bearer tenant-token" {
				t.Fatalf("unexpected Feishu user request: %s %#v", r.URL.String(), r.Header)
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"user":{"avatar":{"avatar_240":"https://cdn.example.com/u-1001.png"}}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	identity := enrichSSOAvatar(context.Background(), ssoIdentity{Subject: "u-1001", Email: "USER@EXAMPLE.COM"}, FeishuConfig{
		AppID:      "app-id",
		AppSecret:  "app-secret",
		APIBaseURL: server.URL,
	}, server.Client())
	if identity.Avatar != "https://cdn.example.com/u-1001.png" {
		t.Fatalf("avatar = %q", identity.Avatar)
	}
}

func TestEnrichSSOAvatarSurvivesCanceledCallbackContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_, _ = w.Write([]byte(`{"code":0,"tenant_access_token":"tenant-token","expire":7200}`))
		case "/open-apis/contact/v3/users/batch_get_id":
			_, _ = w.Write([]byte(`{"code":0,"data":{"user_list":[{"user_id":"1001"}]}}`))
		case "/open-apis/contact/v3/users/1001":
			_, _ = w.Write([]byte(`{"code":0,"data":{"user":{"avatar":{"avatar_240":"https://cdn.example.com/u-1001.png"}}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	callbackContext, cancelCallback := context.WithCancel(context.Background())
	cancelCallback()

	identity := enrichSSOAvatar(callbackContext, ssoIdentity{Subject: "u-1001", Email: "user@example.com"}, FeishuConfig{
		AppID:      "app-id",
		AppSecret:  "app-secret",
		APIBaseURL: server.URL,
	}, server.Client())
	if identity.Avatar != "https://cdn.example.com/u-1001.png" {
		t.Fatalf("avatar = %q after callback context cancellation", identity.Avatar)
	}
}

func TestFeishuUserAvatarRejectsSuccessfulResponseWithoutAvatar(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_, _ = w.Write([]byte(`{"code":0,"tenant_access_token":"tenant-token","expire":7200}`))
		case "/open-apis/contact/v3/users/batch_get_id":
			_, _ = w.Write([]byte(`{"code":0,"data":{"user_list":[{"user_id":"1001"}]}}`))
		case "/open-apis/contact/v3/users/1001":
			_, _ = w.Write([]byte(`{"code":0,"data":{"user":{"avatar":{}}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	_, err := newFeishuClient(FeishuConfig{
		AppID:      "app-id",
		AppSecret:  "app-secret",
		APIBaseURL: server.URL,
	}, server.Client()).userAvatar(context.Background(), "uac-subject", "user@example.com")
	if err == nil || !strings.Contains(err.Error(), "头像为空") {
		t.Fatalf("expected empty-avatar error, got %v", err)
	}
}

func TestSSOStateValidation(t *testing.T) {
	state, err := newSSOState()
	if err != nil {
		t.Fatal(err)
	}
	if !ssoStatesMatch(state, state) {
		t.Fatal("matching state was rejected")
	}
	if ssoStatesMatch(state, state+"x") || ssoStatesMatch("short", "short") {
		t.Fatal("invalid state was accepted")
	}
}

func TestLocalPasswordLoginIsAdminOnlyWhenSSOEnabled(t *testing.T) {
	enabled := SSOConfig{
		Enabled:     true,
		Provider:    "uac",
		FrontendURL: "https://xmp.example.com/#/",
		RedirectURI: "https://xmp.example.com/#/sso/callback",
		UACGateway:  "https://uac.example.com",
		UACAppID:    "xmp-app",
	}
	if localPasswordLoginAllowed(enabled, []string{"common"}) {
		t.Fatal("regular local account was allowed while SSO is enabled")
	}
	if !localPasswordLoginAllowed(enabled, []string{"admin"}) {
		t.Fatal("administrator fallback login was rejected")
	}
	enabled.Enabled = false
	if !localPasswordLoginAllowed(enabled, []string{"common"}) {
		t.Fatal("regular local account was rejected while SSO is disabled")
	}
}

func TestSSOConfigRejectsHashRouteCallback(t *testing.T) {
	cfg := SSOConfig{
		Enabled:     true,
		Provider:    "uac",
		FrontendURL: "http://localhost:8848/",
		RedirectURI: "http://localhost:8848/#/sso/callback",
		UACGateway:  "https://uac.example.com",
		UACAppID:    "xmp-app",
	}
	if cfg.ready() {
		t.Fatal("hash-route callback was accepted even though UAC drops URL fragments")
	}
}

func TestSSODefaultRoleIsOperation(t *testing.T) {
	cfg := Config{}.withDefaults()
	if cfg.SSO.DefaultRoleCode != "operation" {
		t.Fatalf("default SSO role = %q, want operation", cfg.SSO.DefaultRoleCode)
	}
}

func TestAssignDefaultSSORoleRepairsRolelessUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("select count\\(\\*\\) from sys_user_roles").
		WithArgs(3).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("select id from sys_roles").
		WithArgs("operation").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))
	mock.ExpectExec("insert ignore into sys_user_roles").
		WithArgs(3, int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := assignDefaultSSORole(context.Background(), tx, 3, "operation"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureSSOCallbackBodyOmitsCredentialsWithoutConsumingBody(t *testing.T) {
	body := `{"token":"secret-token","rtoken":"secret-rtoken"}`
	request := httptest.NewRequest(http.MethodPost, "/api/auth/sso/uac/callback", strings.NewReader(body))
	if got := captureRequestBody(request); got != "[SSO credentials omitted]" {
		t.Fatalf("captureRequestBody() = %q", got)
	}
	var decoded map[string]string
	if err := json.NewDecoder(request.Body).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["token"] != "secret-token" {
		t.Fatalf("request body was consumed: %#v", decoded)
	}
}
