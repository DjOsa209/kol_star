package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDiscoverProductLineConservative(t *testing.T) {
	lines := []string{"NOTE EDGE", "HOT 70 Series", "GT 50 Pro", "GT 50"}
	tests := []struct {
		name, project, creative, wantProduct, wantSource string
	}{
		{"project title", "总部_公关_NOTE EDGE_全球传播项目", "", "NOTE EDGE", "project_name"},
		{"creative title", "新品传播", "Review: GT 50 Pro - launch", "GT 50 Pro", "creative_name"},
		{"published text boundary", "HOT 700 系列", "", "", ""},
		{"conflicting titles", "NOTE EDGE 发布", "HOT 70 Series Review", "", ""},
		{"multiple products", "NOTE EDGE 与 HOT 70 Series 联合活动", "", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			product, source := discoverProductLine(tc.project, tc.creative, lines)
			if product != tc.wantProduct || source != tc.wantSource {
				t.Fatalf("got (%q, %q), want (%q, %q)", product, source, tc.wantProduct, tc.wantSource)
			}
		})
	}
}

func TestDiscoveryKey(t *testing.T) {
	if got := discoveryKey("  GT   50  Pro "); got != "gt 50 pro" {
		t.Fatalf("unexpected key %q", got)
	}
}

// Run explicitly against a local development database that has migration 044.
// Every mock project, account, cooperation, candidate and catalog row is removed.
func TestProductDiscoveryIntegration(t *testing.T) {
	configPath := os.Getenv("DISCOVERY_INTEGRATION_CONFIG")
	if configPath == "" {
		t.Skip("set DISCOVERY_INTEGRATION_CONFIG for MySQL integration test")
	}
	t.Setenv("CONFIG_FILE", configPath)
	cfg, _, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	db, err := openDB(cfg.MySQL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	a := newApp(db, cfg)
	stamp := fmt.Sprintf("codex-discovery-%d", time.Now().UnixNano())
	name := stamp + " creator"
	brandName := stamp + " brand"
	var projectID, resourceID, secondResourceID, publishedID, unpublishedID int64
	var productID, brandID int64
	var readerRoleID, readerUserID int64
	t.Cleanup(func() {
		_, _ = db.Exec(`delete from biz_cooperation_products where cooperation_id in (?, ?)`, publishedID, unpublishedID)
		_, _ = db.Exec(`delete from biz_discovery_products where id = ?`, productID)
		_, _ = db.Exec(`delete from biz_discovery_brands where id = ?`, brandID)
		_, _ = db.Exec(`delete from biz_cooperations where id in (?, ?)`, publishedID, unpublishedID)
		_, _ = db.Exec(`delete from biz_projects where id = ?`, projectID)
		_, _ = db.Exec(`delete from biz_resources where id in (?, ?)`, resourceID, secondResourceID)
		_, _ = db.Exec(`delete from sys_role_menus where role_id = ?`, readerRoleID)
		_, _ = db.Exec(`delete from sys_user_roles where user_id = ?`, readerUserID)
		_, _ = db.Exec(`delete from sys_users where id = ?`, readerUserID)
		_, _ = db.Exec(`delete from sys_roles where id = ?`, readerRoleID)
	})
	insert := func(query string, args ...any) int64 {
		t.Helper()
		result, err := db.Exec(query, args...)
		if err != nil {
			t.Fatal(err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	resourceID = insert(`insert into biz_resources (name, resource_type, platform) values (?, 'KOL', 'YouTube')`, name)
	secondResourceID = insert(`insert into biz_resources (name, resource_type, platform) values (?, 'KOL', 'TikTok')`, name)
	readerRoleID = insert(`insert into sys_roles (name, code) values (?, ?)`, stamp+" read-only", stamp+"-reader")
	readerUserID = insert(`insert into sys_users (username, password_hash) values (?, ?)`, stamp+"-reader", strings.Repeat("0", 64))
	if _, err := db.Exec(`insert into sys_user_roles (user_id, role_id) values (?, ?)`, readerUserID, readerRoleID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into sys_role_menus (role_id, menu_id)
		select ?, id from sys_menus where path = '/business/resources'`, readerRoleID); err != nil {
		t.Fatal(err)
	}
	projectID = insert(`insert into biz_projects (name) values (?)`, stamp+"_NOTE EDGE_全球传播项目")
	publishedURL := "https://example.test/mock/published/" + stamp
	publishedID = insert(`insert into biz_cooperations
		(project_id, resource_id, status, deliverable_status, final_link, views, impressions)
		values (?, ?, '已发布', '已完成', ?, 100, 900)`, projectID, resourceID, publishedURL)
	unpublishedID = insert(`insert into biz_cooperations
		(project_id, resource_id, status, deliverable_status, final_link, views, impressions)
		values (?, ?, '邀约中', '未开始', ?, 2000, 2000)`, projectID, resourceID, "https://example.test/mock/unpublished/"+stamp)

	call := func(handler http.HandlerFunc, body map[string]any, authenticated bool) map[string]any {
		t.Helper()
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+newLoginTokenResponse(1, time.Now(), time.Now())["accessToken"].(string))
		}
		rec := httptest.NewRecorder()
		handler(rec, req)
		var response map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v (%s)", err, rec.Body.String())
		}
		if response["code"] != float64(0) {
			t.Fatalf("API response: %s", rec.Body.String())
		}
		return response["data"].(map[string]any)
	}
	search := func(mode, keyword string) map[string]any {
		return call(a.productDiscoverySearch, map[string]any{"mode": mode, "keyword": keyword}, false)
	}
	if got := search("product", brandName)["total"].(float64); got != 0 {
		t.Fatalf("unconfirmed product is public: total=%v", got)
	}
	refresh := call(a.productDiscoveryCandidatesRefresh, map[string]any{}, true)
	if refresh["scanned"].(float64) < 1 || refresh["created"].(float64) < 1 {
		t.Fatalf("mock published cooperation was not scanned: %+v", refresh)
	}
	var candidateID int64
	if err := db.QueryRow(`select id from biz_cooperation_products where cooperation_id = ? and status = 'pending'`, publishedID).Scan(&candidateID); err != nil {
		t.Fatal(err)
	}
	candidates := call(a.productDiscoveryCandidatesList, map[string]any{"status": "pending", "pageSize": 100}, true)["list"].([]any)
	foundCandidate := false
	for _, raw := range candidates {
		row := raw.(map[string]any)
		if int64(row["id"].(float64)) == candidateID {
			foundCandidate = true
			if row["suggestedProduct"] != "NOTE EDGE" || row["sourceField"] != "project_name" || row["evidenceUrl"] != publishedURL {
				t.Fatalf("candidate API fields mismatch: %+v", row)
			}
		}
	}
	if !foundCandidate {
		t.Fatal("mock candidate missing from API list")
	}
	mux := http.NewServeMux()
	a.routes(mux)
	requestAsReader := func(path string, body string) int {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+newLoginTokenResponse(int(readerUserID), time.Now(), time.Now())["accessToken"].(string))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code
	}
	if status := requestAsReader("/business/discovery/search", `{"mode":"product"}`); status != http.StatusOK {
		t.Fatalf("reader cannot search public products: status=%d", status)
	}
	if status := requestAsReader("/business/discovery/candidates/list", `{"status":"pending"}`); status != http.StatusForbidden {
		t.Fatalf("reader listed review candidates: status=%d", status)
	}
	if status := requestAsReader("/business/discovery/candidates/refresh", `{}`); status != http.StatusForbidden {
		t.Fatalf("reader refreshed candidates: status=%d", status)
	}
	if status := requestAsReader("/business/discovery/candidates/decision", fmt.Sprintf(`{"candidateId":%d,"decision":"confirmed","brandName":"X","productName":"Y"}`, candidateID)); status != http.StatusForbidden {
		t.Fatalf("reader confirmed candidate: status=%d", status)
	}
	var unpublishedCandidates int
	if err := db.QueryRow(`select count(*) from biz_cooperation_products where cooperation_id = ?`, unpublishedID).Scan(&unpublishedCandidates); err != nil || unpublishedCandidates != 0 {
		t.Fatalf("unpublished cooperation candidate count=%d err=%v", unpublishedCandidates, err)
	}
	if got := search("product", brandName)["total"].(float64); got != 0 {
		t.Fatalf("pending candidate is public: total=%v", got)
	}
	call(a.productDiscoveryCandidateDecision, map[string]any{
		"candidateId": candidateID, "decision": "confirmed", "brandName": brandName, "productName": "NOTE EDGE",
	}, true)
	if err := db.QueryRow(`select p.id, b.id from biz_cooperation_products cp
		join biz_discovery_products p on p.id = cp.product_id
		join biz_discovery_brands b on b.id = p.brand_id where cp.id = ?`, candidateID).Scan(&productID, &brandID); err != nil {
		t.Fatal(err)
	}
	products := search("product", brandName)
	if products["total"].(float64) != 1 {
		t.Fatalf("confirmed product total=%v", products["total"])
	}
	product := products["list"].([]any)[0].(map[string]any)
	if product["contentCount"] != "1" || product["resourceCount"] != "1" || product["totalViews"] != "900" {
		t.Fatalf("published statistics do not exclude unpublished evidence: %+v", product)
	}
	brands := search("brand", brandName)
	if brands["total"].(float64) != 1 {
		t.Fatalf("confirmed brand total=%v", brands["total"])
	}
	brand := brands["list"].([]any)[0].(map[string]any)
	if brand["productCount"] != "1" || brand["contentCount"] != "1" || brand["totalViews"] != "900" {
		t.Fatalf("confirmed brand metrics mismatch: %+v", brand)
	}
	detail := call(a.productDiscoveryProductDetail, map[string]any{"productId": productID}, false)
	cooperations := detail["cooperations"].([]any)
	if len(cooperations) != 1 || cooperations[0].(map[string]any)["contentUrl"] != publishedURL {
		t.Fatalf("product evidence mismatch: %+v", detail)
	}
	evidence := cooperations[0].(map[string]any)
	if evidence["sourceKind"] != "internal_sync" || evidence["sourceField"] != "project_name" || evidence["reviewedEvidenceUrl"] != publishedURL ||
		evidence["sourceText"] != stamp+"_NOTE EDGE_全球传播项目" || evidence["confirmedBy"] != "1" ||
		evidence["confirmedByName"] == "" || evidence["confirmedAt"] == nil {
		t.Fatalf("product provenance or reviewer trace missing: %+v", evidence)
	}
	resources := search("resource", name)["list"].([]any)
	if len(resources) != 2 {
		t.Fatalf("same-name creator accounts merged: %+v", resources)
	}
	for _, raw := range resources {
		row := raw.(map[string]any)
		if int64(row["id"].(float64)) == resourceID && row["contentCount"] != "1" {
			t.Fatalf("published account missing content: %+v", row)
		}
		if int64(row["id"].(float64)) == secondResourceID && row["contentCount"] != "0" {
			t.Fatalf("same-name account inherited another account's content: %+v", row)
		}
	}
	unauthorized := httptest.NewRecorder()
	mux.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "/business/discovery/search", bytes.NewBufferString(`{"mode":"product"}`)))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("discovery route without auth status=%d", unauthorized.Code)
	}
	call(a.productDiscoveryCandidateDecision, map[string]any{"candidateId": candidateID, "decision": "rejected"}, true)
	if got := search("product", brandName)["total"].(float64); got != 0 {
		t.Fatalf("rejected product is public: total=%v", got)
	}
	detailBody, _ := json.Marshal(map[string]any{"productId": productID})
	detailRecorder := httptest.NewRecorder()
	a.productDiscoveryProductDetail(detailRecorder, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(detailBody)))
	var hiddenDetail map[string]any
	if err := json.Unmarshal(detailRecorder.Body.Bytes(), &hiddenDetail); err != nil {
		t.Fatal(err)
	}
	if hiddenDetail["code"] != float64(10004) {
		t.Fatalf("product without confirmed evidence remained accessible: %s", detailRecorder.Body.String())
	}
}
