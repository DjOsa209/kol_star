package main

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

const discoveryRecommendationResourceQuery = `(?s)from biz_resources where platform='YouTube' and \(platform_user_id=\? or \(platform_user_id='' and platform_url=\?\)\) order by id`

func discoveryRecommendationTestCandidate(id, resourceID int64, status, channelID string) creatorDiscoveryCandidate {
	profile := "https://www.youtube.com/channel/" + channelID
	candidate := creatorDiscoveryCandidate{
		ID: id, JobID: 11, ChannelID: channelID, Status: status, ResourceID: resourceID,
		Country: "伊拉克", Language: "阿拉伯语", Note: "人工核实频道公开资料及手机评测作品", EvidenceURL: profile,
		Snapshot: creatorDiscoverySnapshot{
			ChannelID: channelID, Name: "Collected channel", ProfileURL: profile,
			Country: "IQ", Followers: nil, CollectedAt: "2026-10-20T08:00:00Z",
			Evidence:    []creatorDiscoveryEvidence{{Title: "Collected profile", URL: profile, Query: "Iraq phones"}},
			RecentPosts: []platformPost{{PlatformPostID: "test-video", Title: "Collected phone review", PostURL: "https://www.youtube.com/watch?v=test-video", ViewCount: 1200}},
		},
	}
	if status == "pending" {
		candidate.Country, candidate.Language, candidate.Note, candidate.EvidenceURL = "", "", "", ""
	}
	return candidate
}

func discoveryRecommendationJobRows(status, message string, candidateCount, reviewedCount int) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "market", "query_text", "queries", "max_candidates", "status", "message", "discovered_count", "reviewed_count", "imported_count", "created_at", "updated_at", "recommendation"}).
		AddRow(11, "伊拉克", "伊拉克手机科技达人", `["Iraq phones"]`, 12, status, message, candidateCount, reviewedCount, 1, time.Now(), time.Now(), nil)
}

func expectDiscoveryRecommendationInputs(t *testing.T, mock sqlmock.Sqlmock, modelURL string, candidates []creatorDiscoveryCandidate, reviewedCount int, priorState ...string) {
	t.Helper()
	status, message := "completed", "已完成采集"
	if len(priorState) >= 2 {
		status, message = priorState[0], priorState[1]
	}
	expectIPTestSession(mock)
	mock.ExpectQuery("select r.code").WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("admin"))
	mock.ExpectQuery(`where j.id=\? and j.user_id=\?`).WithArgs(11, 1).
		WillReturnRows(discoveryRecommendationJobRows(status, message, len(candidates), reviewedCount))
	content, err := json.Marshal(map[string]any{"provider": "test-provider", "model": "test-model", "baseUrl": modelURL, "enableRecommendationReason": true, "timeoutSeconds": 2})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(`(?s)select enabled, content.*where rule_type = 'ai_model'`).
		WillReturnRows(sqlmock.NewRows([]string{"enabled", "content"}).AddRow(1, string(content)))
	rows := sqlmock.NewRows([]string{"id", "job_id", "channel_id", "status", "resource_id", "country", "language", "note", "evidence_url", "snapshot"})
	for _, candidate := range candidates {
		raw, err := json.Marshal(candidate.Snapshot)
		if err != nil {
			t.Fatal(err)
		}
		rows.AddRow(candidate.ID, candidate.JobID, candidate.ChannelID, candidate.Status, candidate.ResourceID, candidate.Country, candidate.Language, candidate.Note, candidate.EvidenceURL, string(raw))
	}
	mock.ExpectQuery(`from biz_creator_discovery_candidates where job_id=\? order by id`).WithArgs(11).WillReturnRows(rows)
	for _, candidate := range candidates {
		if candidate.Status == "rejected" {
			continue
		}
		resources := sqlmock.NewRows([]string{"id", "name", "resourceType", "status", "industry", "category", "notes", "riskLevel"})
		if candidate.ResourceID == 101 {
			resources.AddRow(101, "Verified public channel", "KOL", "观察中", "", "", "Existing curated note", "待核实")
		}
		if candidate.ChannelID == "UCblacklisted" {
			// Rediscovery cannot evade a restriction by lacking resourceId or by
			// encountering an unrestricted duplicate before a blacklisted row.
			resources.AddRow(205, "Unrestricted duplicate", "KOL", "可合作", "", "", "", "低")
			resources.AddRow(305, "Blacklisted channel", "KOL", "黑名单", "", "", "", "高")
		}
		// UCchanged has no matching current resource, while UCpending is a
		// genuinely new account that can be evaluated without being imported.
		mock.ExpectQuery(discoveryRecommendationResourceQuery).
			WithArgs(candidate.ChannelID, "https://www.youtube.com/channel/"+candidate.ChannelID).
			WillReturnRows(resources)
	}
}

type discoveryRecommendationJSONCapture struct{ target *map[string]any }

func (capture discoveryRecommendationJSONCapture) Match(value driver.Value) bool {
	text, ok := value.(string)
	return ok && json.Unmarshal([]byte(text), capture.target) == nil
}

func TestCreatorDiscoveryRecommendEvaluatesPendingFactsWithoutImportAndPersistsAI(t *testing.T) {
	requests := make(chan map[string]any, 4)
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Method != http.MethodPost {
			t.Errorf("unexpected model request %s %s", r.Method, r.URL.Path)
		}
		var request struct {
			Messages []struct{ Role, Content string } `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, "invalid test request", 400)
			return
		}
		for _, message := range request.Messages {
			if message.Role == "user" {
				var input map[string]any
				if err := json.Unmarshal([]byte(message.Content), &input); err != nil {
					t.Error(err)
				}
				requests <- input
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": `{"summary":"按真实作品相关性推荐并保留待核实状态","recommendations":[{"id":1,"matchScore":93,"priority":"高","reason":"已核实伊拉克市场且近期作品涉及手机评测"},{"id":2,"matchScore":88,"reason":"频道公开作品相关，建议核实后考虑合作"},{"id":999,"matchScore":100,"reason":"模型编造的候选"}]}`}}}})
	}))
	defer model.Close()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{AIModel: AIModelConfig{APIKey: "test-only-model-key"}})
	valid := discoveryRecommendationTestCandidate(1, 101, "approved", "UCabcdefghijklmnopqrstuv")
	candidates := []creatorDiscoveryCandidate{
		valid,
		discoveryRecommendationTestCandidate(2, 0, "pending", "UCpending"),
		discoveryRecommendationTestCandidate(3, 103, "rejected", "UCrejected"),
		discoveryRecommendationTestCandidate(4, 104, "approved", "UCchanged"),
		discoveryRecommendationTestCandidate(5, 0, "pending", "UCblacklisted"),
	}
	expectDiscoveryRecommendationInputs(t, mock, model.URL, candidates, 3)
	var saved map[string]any
	// The only write is the recommendation snapshot. Pending candidates are
	// never inserted into formal resources or marked approved by the model.
	mock.ExpectExec(`(?s)update biz_creator_discovery_jobs set recommendation=\?,status=\?,message=\? where id=\? and user_id=\? and status=\?.*job_id=\? and status<>'pending'\)=\?`).
		WithArgs(discoveryRecommendationJSONCapture{target: &saved}, "completed", "已根据 2 位候选的真实资料生成推荐，可查看依据并核实入库", int64(11), 1, "completed", int64(11), 3).
		WillReturnResult(sqlmock.NewResult(0, 1))
	status, response := ipTestPost(t, a, a.creatorDiscoveryRecommend, map[string]any{"id": 11, "demandText": "寻找伊拉克手机评测合作达人"})
	if status != http.StatusOK || response["code"] != float64(0) {
		t.Fatalf("recommendation failed: status=%d response=%v", status, response)
	}
	if len(requests) != 1 {
		t.Fatalf("expected exactly one model invocation, got %d", len(requests))
	}
	input := <-requests
	facts, ok := input["candidateKOLs"].([]any)
	if !ok || len(facts) != 2 {
		t.Fatalf("rejected, blacklisted or mismatched resource sent to model: %v", input)
	}
	fact := facts[0].(map[string]any)
	if fact["id"] != float64(1) || fact["candidateId"] != float64(1) || fact["resourceId"] != float64(101) || fact["verificationStatus"] != "approved" || fact["country"] != "伊拉克" || fact["platformReportedCountry"] != "IQ" || fact["verificationEvidenceUrl"] != valid.EvidenceURL || fact["collectedAt"] != valid.Snapshot.CollectedAt {
		t.Fatalf("verified source facts lost: %v", fact)
	}
	if value, exists := fact["followers"]; !exists || value != nil {
		t.Fatalf("unknown followers must remain null: %v", fact)
	}
	if value, exists := fact["estimatedCost"]; !exists || value != nil || fact["quoteStatus"] != "unknown" {
		t.Fatalf("unknown quotation must remain unknown: %v", fact)
	}
	if _, exists := fact["matchScore"]; exists {
		t.Fatal("rule ranking leaked into model facts")
	}
	posts, ok := fact["recentPosts"].([]any)
	if !ok || len(posts) != 1 || posts[0].(map[string]any)["title"] != "Collected phone review" {
		t.Fatalf("collected video evidence lost: %v", fact)
	}
	pending := facts[1].(map[string]any)
	if pending["id"] != float64(2) || pending["candidateId"] != float64(2) || pending["verificationStatus"] != "pending" || pending["verificationNote"] != "" || pending["verificationEvidenceUrl"] != "" || pending["status"] != "待核实" {
		t.Fatalf("new candidate was presented as verified: %v", pending)
	}
	if _, exists := pending["resourceId"]; exists {
		t.Fatalf("new candidate acquired an invented resource identity: %v", pending)
	}
	if pending["followers"] != nil || pending["estimatedCost"] != nil || pending["quoteStatus"] != "unknown" || pending["platformReportedCountry"] != "IQ" || !strings.Contains(pending["audienceCountry"].(string), "未知") {
		t.Fatalf("new candidate acquired invented metrics or audience claims: %v", pending)
	}
	if saved["source"] != "ai" || saved["modelInvoked"] != true || saved["fallback"] != false || saved["candidateCount"] != float64(2) {
		t.Fatalf("saved output must identify the actual model execution: %v", saved)
	}
	recommendations, ok := saved["recommendations"].([]any)
	if !ok || len(recommendations) != 2 || recommendations[0].(map[string]any)["id"] != float64(1) || recommendations[1].(map[string]any)["id"] != float64(2) || recommendations[0].(map[string]any)["source"] != "ai" || recommendations[1].(map[string]any)["verificationStatus"] != "pending" {
		t.Fatalf("model invented ID was accepted: %v", saved)
	}
	if !reflect.DeepEqual(response["data"], saved) {
		t.Fatalf("returned result differs from persisted result: response=%v saved=%v", response["data"], saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreatorDiscoveryRecommendModelFailureNeverFallsBackOrPersists(t *testing.T) {
	for _, scenario := range []string{"model unavailable", "model returns only unknown ID", "missing array", "null array", "unexplained empty array"} {
		t.Run(scenario, func(t *testing.T) {
			calls := make(chan struct{}, 4)
			model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls <- struct{}{}
				if scenario == "model unavailable" {
					http.Error(w, "test model unavailable", http.StatusServiceUnavailable)
					return
				}
				content := `{"summary":"已有说明仍不能使编造ID有效","recommendations":[{"id":999,"matchScore":99,"reason":"不存在的候选"}]}`
				switch scenario {
				case "missing array":
					content = `{"summary":"不能省略候选列表"}`
				case "null array":
					content = `{"summary":"不能将null当空结果","recommendations":null}`
				case "unexplained empty array":
					content = `{"summary":" ","recommendations":[]}`
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}}}})
			}))
			defer model.Close()
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			a := newApp(db, Config{AIModel: AIModelConfig{APIKey: "test-only-model-key"}})
			expectDiscoveryRecommendationInputs(t, mock, model.URL, []creatorDiscoveryCandidate{discoveryRecommendationTestCandidate(1, 101, "approved", "UCabcdefghijklmnopqrstuv")}, 1)
			// No write is expected: a failed or invalid model response must not
			// replace an earlier result with a rule-generated recommendation.
			status, response := ipTestPost(t, a, a.creatorDiscoveryRecommend, map[string]any{"id": 11})
			if status != http.StatusBadGateway || response["code"] != float64(50201) || response["data"] != nil {
				t.Fatalf("model failure returned fallback data: status=%d response=%v", status, response)
			}
			if len(calls) != 1 {
				t.Fatalf("expected one actual model request, got %d", len(calls))
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCreatorDiscoveryRecommendRefreshesFailureStateAndAcceptsExplainedEmpty(t *testing.T) {
	for _, scenario := range []struct {
		name, warning, wantStatus, wantMessage string
		empty                                  bool
	}{
		{name: "successful retry clears model error", wantStatus: "completed", wantMessage: "已根据 1 位候选的真实资料生成推荐，可查看依据并核实入库"},
		{name: "successful retry retains collection warning", warning: "最近作品采集失败", wantStatus: "partial", wantMessage: "已根据 1 位候选的真实资料生成推荐，可查看依据并核实入库；1 个候选资料采集不完整，请查看候选提示"},
		{name: "explained empty result is persisted", empty: true, wantStatus: "completed", wantMessage: "大模型未找到合适候选：候选内容与伊拉克手机测评需求不匹配"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				content := `{"summary":"已根据实际作品完成评估","recommendations":[{"id":1,"matchScore":90,"reason":"手机评测主题相关"}]}`
				if scenario.empty {
					content = `{"summary":"候选内容与伊拉克手机测评需求不匹配","recommendations":[]}`
				}
				writeCreatorDiscoveryPlannerTestCompletion(w, content)
			}))
			defer model.Close()
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			a := newApp(db, Config{AIModel: AIModelConfig{APIKey: "test-only-key"}})
			candidate := discoveryRecommendationTestCandidate(1, 0, "pending", "UCpending")
			candidate.Snapshot.Warning = scenario.warning
			expectDiscoveryRecommendationInputs(t, mock, model.URL, []creatorDiscoveryCandidate{candidate}, 0, "partial", "旧模型调用失败")
			var saved map[string]any
			mock.ExpectExec(`(?s)update biz_creator_discovery_jobs set recommendation=\?,status=\?,message=\?.*status=\?.*status<>'pending'\)=\?`).
				WithArgs(discoveryRecommendationJSONCapture{target: &saved}, scenario.wantStatus, scenario.wantMessage, int64(11), 1, "partial", int64(11), 0).
				WillReturnResult(sqlmock.NewResult(0, 1))
			status, response := ipTestPost(t, a, a.creatorDiscoveryRecommend, map[string]any{"id": 11})
			if status != http.StatusOK || saved["source"] != "ai" || saved["modelInvoked"] != true || saved["fallback"] != false {
				t.Fatalf("successful model assessment rejected: status=%d response=%v saved=%v", status, response, saved)
			}
			rows, ok := saved["recommendations"].([]any)
			if !ok || (scenario.empty && (len(rows) != 0 || saved["message"] != "候选内容与伊拉克手机测评需求不匹配")) || (!scenario.empty && len(rows) != 1) {
				t.Fatalf("model assessment or empty-result explanation lost: %v", saved)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCreatorDiscoveryBackgroundRecommendationStopsAfterCancellation(t *testing.T) {
	for _, scenario := range []string{"job cancelled before reread", "context already cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			a := newApp(db, Config{})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "context already cancelled" {
				cancel()
			} else {
				mock.ExpectQuery(`where j.id=\? and j.user_id=\?`).WithArgs(11, 1).
					WillReturnRows(discoveryRecommendationJobRows("cancelled", "已取消", 1, 0))
			}
			// No candidates query, model request or save is allowed after the
			// transition-to-recommending/reread cancellation race.
			response, err := a.generateActiveCreatorDiscoveryRecommendation(ctx, 11, 1, "手机评测", assistantAIModel{})
			if !errors.Is(err, context.Canceled) || response != nil {
				t.Fatalf("cancelled job entered recommendation: response=%v error=%v", response, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCreatorDiscoveryRecommendDoesNotReportSuccessWhenSaveFails(t *testing.T) {
	for _, scenario := range []string{"database failure", "state changed during model call"} {
		t.Run(scenario, func(t *testing.T) {
			model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeCreatorDiscoveryPlannerTestCompletion(w, `{"summary":"评估完成","recommendations":[{"id":1,"matchScore":90,"reason":"手机测评主题相关"}]}`)
			}))
			defer model.Close()
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			a := newApp(db, Config{AIModel: AIModelConfig{APIKey: "test-only-key"}})
			expectDiscoveryRecommendationInputs(t, mock, model.URL, []creatorDiscoveryCandidate{discoveryRecommendationTestCandidate(1, 0, "pending", "UCpending")}, 0)
			save := mock.ExpectExec(`(?s)update biz_creator_discovery_jobs set recommendation=\?,status=\?,message=\?.*where id=\? and user_id=\? and status=\?.*status<>'pending'\)=\?`).
				WithArgs(sqlmock.AnyArg(), "completed", "已根据 1 位候选的真实资料生成推荐，可查看依据并核实入库", int64(11), 1, "completed", int64(11), 0)
			if scenario == "database failure" {
				save.WillReturnError(errors.New("database unavailable"))
			} else {
				save.WillReturnResult(sqlmock.NewResult(0, 0))
			}
			status, response := ipTestPost(t, a, a.creatorDiscoveryRecommend, map[string]any{"id": 11})
			if status != http.StatusBadGateway || response["data"] != nil {
				t.Fatalf("unsaved assessment was reported as success: status=%d response=%v", status, response)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
