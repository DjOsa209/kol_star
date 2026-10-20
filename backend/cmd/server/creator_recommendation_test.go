package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCreatorRecommendationsNeverBackfillRuleCandidates(t *testing.T) {
	local := []map[string]any{
		{"id": 1, "matchScore": 99, "reason": "rule reason", "hitRules": []string{"rule hint"}, "downReasons": []string{"rule downgrade"}},
		{"id": 2, "matchScore": 98, "reason": "rule reason"},
	}
	for _, tc := range []struct {
		name string
		rows []map[string]any
		want int
	}{
		{"one selected", []map[string]any{{"id": 1, "matchScore": 72, "reason": "model reason"}}, 1},
		{"unknown id", []map[string]any{{"id": 999, "matchScore": 90, "reason": "model reason"}}, 0},
		{"missing score", []map[string]any{{"id": 1, "reason": "model reason"}}, 0},
		{"invalid score", []map[string]any{{"id": 1, "matchScore": 101, "reason": "model reason"}}, 0},
		{"missing reason", []map[string]any{{"id": 1, "matchScore": 90}}, 0},
		{"no selections", nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeAIRecommendations(local, tc.rows)
			if len(got) != tc.want {
				t.Fatalf("got %+v, want %d model selections", got, tc.want)
			}
			if len(got) > 0 && (got[0]["source"] != "ai" || got[0]["reason"] != "model reason" || len(got[0]["hitRules"].([]string)) != 0 || len(got[0]["downReasons"].([]string)) != 0) {
				t.Fatalf("rule explanation leaked: %+v", got)
			}
		})
	}
	if local[0]["reason"] != "rule reason" {
		t.Fatal("candidate pool mutated")
	}
}

func TestCreatorAIRanksRequireModelAdvice(t *testing.T) {
	candidates := []recommendationCandidate{
		{ID: 1, Source: "rule", Score: 99, Reason: "rule", Advice: "rule advice", AdviceSource: "rule", Scenarios: []string{"rule scenario"}},
		{ID: 2, Source: "rule", Score: 98, Reason: "rule", Advice: "rule advice", AdviceSource: "rule"},
	}
	advice := "适用场景：可考虑产品测评视频。匹配原因：库内平台与本次投放需求一致，内容排期、报价和交付效果需要确认。"
	got := filterCreatorAIRanks(candidates, []recommendationAIRank{
		{ID: 999, Score: 99, Reason: "invented", Advice: advice},
		{ID: 1, Score: 75, Reason: "model reason", Advice: advice},
		{ID: 2, Score: 80, Reason: "model reason"},
	})
	if len(got) != 1 || got[0].ID != 1 || got[0].Score != 75 || got[0].Source != "ai" || got[0].AdviceSource != "ai" || len(got[0].Scenarios) != 0 {
		t.Fatalf("saved results must contain only validated model output: %+v", got)
	}
}

func TestCreatorRecommendEndpointRequiresSuccessfulModel(t *testing.T) {
	for _, state := range []string{"missing", "disabled", "unavailable", "malformed", "unknown_ids", "success"} {
		t.Run(state, func(t *testing.T) {
			fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if state == "unavailable" {
					http.Error(w, "temporarily unavailable", 503)
					return
				}
				var input struct{ Messages []struct{ Content string } }
				_ = json.NewDecoder(r.Body).Decode(&input)
				if len(input.Messages) < 2 || strings.Contains(input.Messages[1].Content, "matchScore") || strings.Contains(input.Messages[1].Content, "hitRules") {
					t.Error("model must receive resource facts without rule recommendations")
				}
				content := `{"recommendations":[{"id":1,"matchScore":77,"reason":"模型认为科技测评与项目匹配"}]}`
				if state == "malformed" {
					content = "invalid JSON"
				} else if state == "unknown_ids" {
					content = `{"recommendations":[{"id":9999,"matchScore":90,"reason":"unknown resource"}]}`
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}}}})
			}))
			defer fake.Close()
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			query := mock.ExpectQuery("select enabled, content")
			if state == "missing" {
				query.WillReturnError(sql.ErrNoRows)
			} else {
				config, _ := json.Marshal(map[string]any{"provider": "test", "model": "test", "baseUrl": fake.URL, "enableRecommendationReason": state != "disabled"})
				query.WillReturnRows(sqlmock.NewRows([]string{"enabled", "content"}).AddRow(1, string(config)))
			}
			if state != "missing" && state != "disabled" {
				mock.ExpectQuery("select id, name, avatar_url").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "resourceType", "status", "score", "level", "riskLevel", "followers"}).
					AddRow(1, "Creator One", "KOL", "可合作", 95, "A", "低", 10000).
					AddRow(2, "Creator Two", "KOL", "可合作", 90, "A", "低", 10000))
			}
			a := newApp(db, Config{AIModel: AIModelConfig{APIKey: "test-key"}})
			recorder := httptest.NewRecorder()
			a.businessAssistantRecommend(recorder, httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"demandText":"寻找科技测评达人"}`)))
			var response struct {
				Code int `json:"code"`
				Data struct {
					Recommendations []map[string]any `json:"recommendations"`
					Fallback        bool             `json:"fallback"`
				} `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if state == "success" {
				if response.Code != 0 || response.Data.Fallback || len(response.Data.Recommendations) != 1 || response.Data.Recommendations[0]["source"] != "ai" {
					t.Fatalf("unexpected successful response: %s", recorder.Body.String())
				}
			} else if response.Code == 0 || len(response.Data.Recommendations) != 0 {
				t.Fatalf("model failure returned recommendations: %s", recorder.Body.String())
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
