package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRecommendationAdviceExplainsScenarioAndDemandWithoutLicensingClaim(t *testing.T) {
	s := recommendationSubscription{Domain: "ip", Query: "NOTE70 新品发布", Goal: "新品发布", Markets: []string{"中国"}, IPTypes: []string{"动画"}}
	row := map[string]any{
		"id": 7, "name": "测试动画", "ipType": "动画", "markets": `["中国"]`,
		"cooperationStatus": "待评估", "audience": "年轻用户", "summary": "角色故事",
	}
	candidate, ok := scoreIPRecommendation(row, s)
	if !ok {
		t.Fatal("valid IP should be eligible")
	}
	candidates := []recommendationCandidate{candidate}
	prepareRecommendationAdvice(s, candidates)
	got := candidates[0]
	for _, required := range []string{"适用场景：", "匹配原因：", "新品发布", "中国", "动画", "待评估", "版权方确认"} {
		if !strings.Contains(got.Advice, required) {
			t.Fatalf("advice omits %q: %s", required, got.Advice)
		}
	}
	if strings.Contains(got.Advice, "已获授权") || got.AdviceSource != "rule" || len(got.Scenarios) == 0 || got.EvidenceStatus != "not_searched" {
		t.Fatalf("fallback advice overstates rights or lacks scenario: %+v", got)
	}
}

func TestRecommendationAIRequestCarriesGroundedAdviceContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected model request: %s %q", r.URL, r.Header.Get("Authorization"))
		}
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode model request: %v", err)
		}
		if len(body.Messages) != 2 || !strings.Contains(body.Messages[0].Content, "evidenceIds") || !strings.Contains(body.Messages[1].Content, "publicEvidence") || strings.Contains(body.Messages[1].Content, "https://www.wikidata.org/wiki/Q1") {
			t.Errorf("model prompt omitted citations contract or leaked source URL: %+v", body.Messages)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"recommendations\":[{\"id\":7,\"advice\":\"适用场景：评估发布会互动。匹配原因：与新品发布目标及所选类型一致。\",\"scenarios\":[\"发布会互动\"],\"evidenceIds\":[1]}]}"}}]}`))
	}))
	defer server.Close()
	candidate := recommendationCandidate{ID: 7, Name: "真实 IP", Score: 70, Resource: map[string]any{"ipType": "动画"}, Evidence: []recommendationEvidence{{Title: "公开描述", URL: "https://www.wikidata.org/wiki/Q1", Source: "Wikidata", Summary: "动画系列"}}}
	ranks, err := (&app{}).requestRecommendationAIRanks(context.Background(), assistantAIModel{BaseURL: server.URL, APIKey: "test-key", Model: "mock", Timeout: time.Second}, recommendationSubscription{Domain: "ip", Goal: "新品发布"}, []recommendationCandidate{candidate})
	if err != nil || len(ranks) != 1 || ranks[0].ID != 7 || len(ranks[0].EvidenceIDs) != 1 {
		t.Fatalf("model advice response not parsed: %+v %v", ranks, err)
	}
}

func TestRecommendationAIAdviceCanOnlyCiteRetrievedEvidence(t *testing.T) {
	c := recommendationCandidate{
		ID: 7, Name: "真实 IP", Score: 70, Reason: "规则匹配", Source: "rule",
		Advice: "适用场景：规则场景。匹配原因：规则事实。", AdviceSource: "rule",
		Scenarios: []string{"规则场景"},
		Evidence: []recommendationEvidence{
			{Title: "实际资料", URL: "https://www.wikidata.org/wiki/Q1", Source: "Wikidata", Summary: "动画作品"},
		},
	}
	ranked := filterRecommendationAIRanks([]recommendationCandidate{c}, []recommendationAIRank{
		{ID: 999, Advice: "杜撰 IP"},
		{ID: 7, Advice: "适用场景：新品发布互动。匹配原因：与动画 IP 的目标类型一致。", Scenarios: []string{"主题视觉"}, ExternalInsight: "外部资料记录为动画作品。", EvidenceIDs: []int{1, 99, 1}},
	})
	if len(ranked) != 1 || ranked[0].ID != 7 || ranked[0].AdviceSource != "ai" || len(ranked[0].Evidence) != 1 || !strings.Contains(ranked[0].Advice, "外部背景：") {
		t.Fatalf("valid citation was not retained or unknown id was accepted: %+v", ranked)
	}
	withoutSource := filterRecommendationAIRanks([]recommendationCandidate{c}, []recommendationAIRank{
		{ID: 7, Advice: "适用场景：新品发布互动。匹配原因：与动画 IP 的目标类型一致。", ExternalInsight: "虚构的外部结论", EvidenceIDs: []int{88}},
	})
	if strings.Contains(withoutSource[0].Advice, "虚构的外部结论") {
		t.Fatalf("uncited external claim survived: %+v", withoutSource[0])
	}
	unsupported := filterRecommendationAIRanks([]recommendationCandidate{c}, []recommendationAIRank{
		{ID: 7, Advice: "适用场景：立即上线联名。匹配原因：已经获得授权，保证销量增长。", EvidenceIDs: []int{1}},
	})
	if unsupported[0].AdviceSource != "rule" || strings.Contains(unsupported[0].Advice, "保证销量") {
		t.Fatalf("unsupported rights claim must fall back to rule advice: %+v", unsupported[0])
	}
}

func TestRecommendationResourceHashTracksFactsButIgnoresPresentation(t *testing.T) {
	initial := map[string]any{"name": "IP", "ipType": "动画", "markets": `["中国"]`, "cooperationStatus": "待评估", "visualFileId": 5}
	live := map[string]any{"name": "IP", "ipType": "动画", "markets": []string{"中国"}, "cooperationStatus": "待评估", "visualFileId": 9}
	if recommendationResourceHash(initial) != recommendationResourceHash(live) {
		t.Fatal("equivalent current facts should retain saved advice")
	}
	live["cooperationStatus"] = "可合作"
	if recommendationResourceHash(initial) == recommendationResourceHash(live) {
		t.Fatal("changed cooperation status must invalidate saved advice")
	}
}
