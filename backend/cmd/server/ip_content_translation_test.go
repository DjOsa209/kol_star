package main

import (
	"context"
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestIPContentSourcesWhitelistAndBothDirections(t *testing.T) {
	row := map[string]any{"id": int64(9), "name": "Example IP", "summary": "适合新品联名", "contact": "张三 someone@example.com", "markets": []string{"东南亚"}, "profile": `{"cooperationTips":"Keep the original launch timeline","imageStyle":"https://example.com/image.png"}`}
	sources := ipContentSources("ip_resource", row)
	byPath := map[string]string{}
	for _, s := range sources {
		byPath[s.FieldKey] = ipTranslationTarget(s.Text)
	}
	if byPath["summary"] != "en" || byPath["profile.cooperationTips"] != "zh" {
		t.Fatalf("missing bilingual sources: %v", byPath)
	}
	for _, key := range []string{"contact", "markets", "profile.imageStyle"} {
		if _, ok := byPath[key]; ok {
			t.Fatalf("unexpected field %s", key)
		}
	}
	if row["summary"] != "适合新品联名" {
		t.Fatal("original mutated")
	}
	req := map[string]any{"id": int64(10), "description": "Keep notes", "brief": map[string]any{"owner": "张三", "candidateEvaluations": map[string]any{"9": map[string]any{"reason": "适合", "productEvaluation": "Premium product", "technicalFit": "可开发"}}}, "marketingProfile": map[string]any{"functions": map[string]any{"brand": map[string]any{"comments": "Brand feedback"}}, "candidates": map[string]any{"9": map[string]any{"ugcForecast": "用户创作潜力"}}}}
	paths := map[string]bool{}
	for _, s := range ipContentSources("ip_request", req) {
		paths[s.FieldKey] = true
	}
	for _, key := range []string{"description", "brief.candidateEvaluations.9.reason", "marketingProfile.functions.brand.comments", "marketingProfile.candidates.9.ugcForecast"} {
		if !paths[key] {
			t.Fatalf("missing %s", key)
		}
	}
	if paths["brief.owner"] {
		t.Fatal("person translated")
	}
}
func TestIPContentTranslationModelTargetsAndInvalidResponses(t *testing.T) {
	for _, target := range []string{"en", "zh"} {
		t.Run(target, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Messages []struct {
						Content string `json:"content"`
					} `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				want := "business English"
				if target == "zh" {
					want = "Simplified Chinese"
				}
				if !strings.Contains(body.Messages[0].Content, want) || !strings.Contains(body.Messages[0].Content, "untrusted data") {
					t.Error("missing language/data instruction")
				}
				json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": `{"translations":[{"id":0,"text":"Translated content"}]}`}}}})
			}))
			defer server.Close()
			got, err := requestAITranslations(context.Background(), assistantAIModel{BaseURL: server.URL, Model: "test", Timeout: time.Second}, []translationSource{{FieldKey: "summary", Text: "Example"}}, target)
			if err != nil || got[0] != "Translated content" {
				t.Fatalf("translation failed: %v", err)
			}
		})
	}
	for _, content := range []string{`{"translations":[]}`, `{"translations":[{"id":5,"text":"wrong id"}]}`, `{"translations":[{"id":0,"text":"a"},{"id":0,"text":"b"}]}`, `{"translations":[{"id":0,"text":""}]}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
		}))
		_, err := requestAITranslations(context.Background(), assistantAIModel{BaseURL: server.URL, Model: "test", Timeout: time.Second}, []translationSource{{Text: "原文"}}, "en")
		server.Close()
		if err == nil {
			t.Fatalf("accepted invalid response %s", content)
		}
	}
}
func TestIPContentMetadataIgnoresStaleTranslation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := Config{}
	cfg.AIModel.APIKey = "test"
	a := newApp(db, cfg)
	row := map[string]any{"id": int64(5), "summary": "新描述"}
	mock.ExpectQuery("select entity_id,field_key,target_language").WithArgs("ip_resource", 5).WillReturnRows(sqlmock.NewRows([]string{"entity_id", "field_key", "target_language", "source_hash", "translated_text", "translation_status"}).AddRow(5, "summary", "en", localizedSourceHash("旧描述"), "Old description", "completed"))
	if err := a.attachIPContentTranslations(context.Background(), "ip_resource", []map[string]any{row}); err != nil {
		t.Fatal(err)
	}
	meta := row["translation"].(*ipContentTranslation)
	if len(meta.Values["en"]) != 0 || meta.State != "failed" || row["summary"] != "新描述" {
		t.Fatalf("stale content applied: %#v", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestIPContentWorkerGuardsClaimAndCompletionHash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": `{"translations":[{"id":0,"text":"Product launch"}]}`}}}})
	}))
	defer server.Close()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := Config{}
	cfg.AIModel.APIKey = "test"
	a := newApp(db, cfg)
	hash := localizedSourceHash("新品发布")
	mock.ExpectQuery("select id,entity_type as entityType").WillReturnRows(sqlmock.NewRows([]string{"id", "entityType", "entityId", "fieldKey", "target", "text", "hash"}).AddRow(7, "ip_resource", 5, "summary", "en", "新品发布", hash))
	settings, _ := json.Marshal(map[string]any{"provider": "test", "model": "test", "baseUrl": server.URL})
	mock.ExpectQuery("select enabled, content").WillReturnRows(sqlmock.NewRows([]string{"enabled", "content"}).AddRow(1, string(settings)))
	mock.ExpectQuery("select id,name,rights_owner").WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id", "summary"}).AddRow(5, "新品发布"))
	mock.ExpectExec("update biz_localized_texts set translation_status='processing'.*where id=\\? and source_hash=\\?").WithArgs(int64(7), hash).WillReturnResult(sqlmock.NewResult(0, 1))
	// Simulate a new source hash being saved during the model call; an old completion affects zero rows.
	mock.ExpectExec("update biz_localized_texts set translated_text=.*where id=\\? and source_hash=\\? and translation_status='processing'").WithArgs("Product launch", "completed", "", int64(7), hash).WillReturnResult(sqlmock.NewResult(0, 0))
	if err := a.processIPContentTranslations(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestIPContentWorkerNeverSendsDeletedContent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := Config{}
	cfg.AIModel.APIKey = "test"
	a := newApp(db, cfg)
	hash := localizedSourceHash("Deleted original")
	mock.ExpectQuery("select id,entity_type as entityType").WillReturnRows(sqlmock.NewRows([]string{"id", "entityType", "entityId", "fieldKey", "target", "text", "hash"}).AddRow(7, "ip_resource", 5, "summary", "zh", "Deleted original", hash))
	mock.ExpectQuery("select enabled, content").WillReturnRows(sqlmock.NewRows([]string{"enabled", "content"}))
	mock.ExpectQuery("select id,name,rights_owner").WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id", "summary"}))
	mock.ExpectExec(regexp.QuoteMeta("delete from biz_localized_texts where id=? and source_hash=?")).WithArgs(int64(7), hash).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := a.processIPContentTranslations(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
