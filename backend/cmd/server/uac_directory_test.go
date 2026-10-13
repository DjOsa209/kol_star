package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestUACDirectorySearchUsesApplicationTokenAndPagination(t *testing.T) {
	var tokens, searches atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/service-pf-open-gateway/oauth/token":
			tokens.Add(1)
			query := r.URL.Query()
			if r.Method != http.MethodGet || query.Get("grant_type") != "client_credentials" || query.Get("client_id") != "test-app" || query.Get("client_secret") != "test-secret" {
				t.Error("unexpected OAuth token request")
			}
			_, _ = w.Write([]byte(`{"code":"0","data":{"access_token":"mock-token","expires_in":1800}}`))
		case "/service-pf-open-gateway/open-apis/v2/api/uac-user/user/pages":
			searches.Add(1)
			if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer mock-token" {
				t.Error("UAC directory request missing application Bearer token")
			}
			var body struct {
				Current     int            `json:"current"`
				Size        int            `json:"size"`
				SearchTotal bool           `json:"searchTotal"`
				Param       map[string]any `json:"param"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Size != uacDirectoryPageSize || body.SearchTotal || body.Param["status"] != "OK" {
				t.Errorf("invalid directory page parameters: %+v", body)
			}
			if body.Param["likeNoOrName"] == "张" && body.Current != 2 {
				t.Errorf("search page = %d, want 2", body.Current)
			}
			if body.Param["empNo"] == "1001" && body.Current != 1 {
				t.Errorf("employee lookup page = %d, want 1", body.Current)
			}
			_, _ = w.Write([]byte(`{"code":"0","data":{"hasNext":true,"dataList":[{"uid":15682833519993842,"realName":"张三","employeeNo":"1001","email":"ZHANG@example.com","status":"OK","depts":[{"name":"IP营销"}]},{"uid":"99","realName":"离职同事","employeeNo":"1002","status":"FREEZE"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	cfg := SSOConfig{Enabled: true, Provider: "uac", UACGateway: server.URL, UACAppID: "test-app", UACAppSecret: "test-secret"}
	client := newUACDirectoryClient(cfg, server.Client())
	people, hasNext, err := client.search(context.Background(), "张", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNext || len(people) != 1 || people[0].ID != "uac:15682833519993842" || people[0].EmployeeNo != "1001" || !people[0].Notifiable || people[0].Department != "IP营销" {
		t.Fatalf("unexpected directory results: %+v, hasNext=%t", people, hasNext)
	}
	person, err := client.byEmployeeNo(context.Background(), "1001")
	if err != nil || person.email != "zhang@example.com" {
		t.Fatalf("lookup by employee number failed: %+v, %v", person, err)
	}
	if tokens.Load() != 1 || searches.Load() != 2 {
		t.Fatalf("token requests=%d searches=%d, want 1 and 2", tokens.Load(), searches.Load())
	}
	uacApplicationTokens.Lock()
	for key, cached := range uacApplicationTokens.entries {
		if strings.HasPrefix(key, server.URL+"\x00") {
			cached.expiresAt = time.Now().Add(-time.Second)
			uacApplicationTokens.entries[key] = cached
		}
	}
	uacApplicationTokens.Unlock()
	if _, _, err := client.search(context.Background(), "张", 2); err != nil {
		t.Fatal(err)
	}
	if tokens.Load() != 2 || searches.Load() != 3 {
		t.Fatalf("expired token was reused: token requests=%d searches=%d", tokens.Load(), searches.Load())
	}
}

func TestUACDirectoryDoesNotExposeCredentialsInErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "secret in response", http.StatusForbidden)
	}))
	defer server.Close()
	client := newUACDirectoryClient(SSOConfig{
		Enabled: true, Provider: "uac", UACGateway: server.URL,
		UACAppID: "test-app", UACAppSecret: "test-secret",
	}, server.Client())
	_, _, err := client.search(context.Background(), "张", 1)
	if err == nil || strings.Contains(err.Error(), "test-secret") || strings.Contains(err.Error(), "secret in response") {
		t.Fatalf("unsafe directory error: %v", err)
	}
}

func TestIPColleaguesUsesUACDirectoryWithoutCreatingAccounts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/service-pf-open-gateway/oauth/token":
			_, _ = w.Write([]byte(`{"code":"0","data":{"access_token":"mock-token","expires_in":1800}}`))
		case "/service-pf-open-gateway/open-apis/v2/api/uac-user/user/pages":
			_, _ = w.Write([]byte(`{"code":"0","data":{"hasNext":true,"dataList":[{"uid":"123456789012345678","realName":"王五","employeeNo":"2002","email":"wang@example.com","status":"OK","deptName":"品牌部"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{SSO: SSOConfig{
		Enabled: true, Provider: "uac", UACGateway: server.URL, UACAppID: "test-app", UACAppSecret: "test-secret",
	}})
	req := httptest.NewRequest(http.MethodGet, "/business/ip/colleagues?q=王&page=2", nil)
	rec := httptest.NewRecorder()
	a.ipColleagues(rec, req)
	var response struct {
		Data            []uacDirectoryPerson `json:"data"`
		DirectorySource string               `json:"directorySource"`
		HasNext         bool                 `json:"hasNext"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || response.DirectorySource != "uac" || !response.HasNext || len(response.Data) != 1 || response.Data[0].EmployeeNo != "2002" || response.Data[0].Department != "品牌部" {
		t.Fatalf("unexpected endpoint response: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "wang@example.com") {
		t.Fatal("directory response exposed colleague email")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPColleaguesFallsBackToCachedSSOUsersWhenDirectoryUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "mock upstream failure", http.StatusForbidden)
	}))
	defer server.Close()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{SSO: SSOConfig{
		Enabled: true, Provider: "uac", UACGateway: server.URL, UACAppID: "test-app", UACAppSecret: "test-secret",
	}})
	mock.ExpectQuery("from sys_users u left join sys_departments d.*u.auth_provider='uac'").
		WithArgs("%张%", "%张%", "%张%", "%张%", 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "department", "employeeNo", "notifiable"}).AddRow(7, "张三", "IP营销", "1001", 1))
	req := httptest.NewRequest(http.MethodGet, "/business/ip/colleagues?q=张", nil)
	rec := httptest.NewRecorder()
	a.ipColleagues(rec, req)
	var response struct {
		Data             []map[string]any `json:"data"`
		DirectorySource  string           `json:"directorySource"`
		DirectoryWarning string           `json:"directoryWarning"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || response.DirectorySource != "cache" || len(response.Data) != 1 || response.DirectoryWarning == "" {
		t.Fatalf("directory fallback failed: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "mock upstream failure") || strings.Contains(rec.Body.String(), "test-secret") {
		t.Fatal("directory fallback exposed upstream response or credential")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
