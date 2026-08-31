package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNormalizeEditableContentPlatform(t *testing.T) {
	tests := map[string]string{
		"RedNote":  "小红书",
		"tiktok":   "TikTok",
		"INS":      "Instagram",
		"twitter":  "X",
		"LinkedIn": "LinkedIn",
		"网站":       "Website",
		"unknown":  "",
	}
	for input, want := range tests {
		if got := normalizeEditableContentPlatform(input); got != want {
			t.Fatalf("normalizeEditableContentPlatform(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestReplaceCooperationContentURL(t *testing.T) {
	raw := "https://example.com/one\nhttps://example.com/two"
	got := replaceCooperationContentURL(
		raw,
		"https://example.com/two",
		"https://example.com/updated",
	)
	want := "https://example.com/one\nhttps://example.com/updated"
	if got != want {
		t.Fatalf("replaceCooperationContentURL() = %q, want %q", got, want)
	}
}

func TestAggregateProjectResourcesByName(t *testing.T) {
	resources := []map[string]any{
		{"resourceId": 11, "resourceName": "Creator One", "platform": "YouTube"},
		{"resourceId": 12, "resourceName": " Creator   One ", "platform": "TikTok"},
		{"resourceId": 13, "resourceName": "Creator Two", "platform": "Instagram"},
	}
	cooperations := []map[string]any{
		{"resourceId": 11, "resourceName": "Creator One", "platform": "YouTube", "contentPlatform": "YouTube", "quoteAmount": 1200, "views": 300000},
		{"resourceId": 12, "resourceName": "creator one", "platform": "TikTok", "contentPlatform": "TikTok", "quoteAmount": 800, "views": 200000},
		{"resourceId": 13, "resourceName": "Creator Two", "platform": "Instagram", "quoteAmount": 500, "views": 0},
	}

	rows := aggregateProjectResourcesByName(resources, cooperations)
	if len(rows) != 2 {
		t.Fatalf("aggregate rows = %d, want 2: %#v", len(rows), rows)
	}
	if got := rows[0]["projectCost"]; got != float64(2000) {
		t.Fatalf("projectCost = %#v, want 2000", got)
	}
	if got := rows[0]["projectViews"]; got != float64(500000) {
		t.Fatalf("projectViews = %#v, want 500000", got)
	}
	if got := rows[0]["projectCpm"]; got != float64(4) {
		t.Fatalf("projectCpm = %#v, want 4", got)
	}
	platforms, ok := rows[0]["platforms"].([]string)
	if !ok || len(platforms) != 2 || platforms[0] != "YouTube" || platforms[1] != "TikTok" {
		t.Fatalf("platforms = %#v, want YouTube and TikTok", rows[0]["platforms"])
	}
	resourceIDs, ok := rows[0]["resourceIds"].([]int)
	if !ok || len(resourceIDs) != 2 || resourceIDs[0] != 11 || resourceIDs[1] != 12 {
		t.Fatalf("resourceIds = %#v, want 11 and 12", rows[0]["resourceIds"])
	}
}

func TestUpdateBusinessProjectContentReturnsAllLinkedFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	const postURL = "https://127.0.0.1/posts/new"
	mock.ExpectQuery("select c.project_id, c.resource_id, c.final_link").
		WithArgs(22).
		WillReturnRows(sqlmock.NewRows([]string{
			"project_id", "resource_id", "final_link", "deliverable_links", "content_platform",
		}).AddRow(11, 33, "https://127.0.0.1/posts/old", "", "Website"))
	mock.ExpectBegin()
	mock.ExpectExec("update biz_cooperations").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("update biz_resources").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("select coalesce\\(r.platform_url").
		WithArgs(22, 33).
		WillReturnRows(sqlmock.NewRows([]string{
			"platform_url", "avatar_remote_url", "final_link", "deliverable_links",
			"content_cover_url", "content_cover_remote_url",
		}).AddRow(
			"https://www.linkedin.com/in/new",
			"https://cdn.example.com/avatar.jpg",
			postURL,
			postURL,
			"/api/uploads/resource-images/cover.jpg",
			"https://cdn.example.com/cover.jpg",
		))

	body, err := json.Marshal(map[string]any{
		"projectId":     11,
		"cooperationId": 22,
		"resourceId":    33,
		"platform":      "LinkedIn",
		"postUrl":       postURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/business/projects/content/update", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	newApp(db, Config{}).updateBusinessProjectContent(recorder, request)

	var response struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != 0 {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
	expected := map[string]string{
		"platform":                "LinkedIn",
		"platformUrl":             "https://www.linkedin.com/in/new",
		"finalLink":               postURL,
		"deliverableLinks":        postURL,
		"resourceAvatarRemoteUrl": "https://cdn.example.com/avatar.jpg",
		"contentCoverLocalUrl":    "/api/uploads/resource-images/cover.jpg",
		"contentCoverRemoteUrl":   "https://cdn.example.com/cover.jpg",
	}
	for field, want := range expected {
		if got := response.Data[field]; got != want {
			t.Errorf("%s = %#v, want %q", field, got, want)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBusinessProjectDetailDisablesHTTPResponseCaching(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("select id, name, target_market").
		WithArgs(24).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	request := httptest.NewRequest("GET", "/business/projects/detail?id=24", nil)
	recorder := httptest.NewRecorder()
	newApp(db, Config{}).businessProjectDetail(recorder, request)

	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
