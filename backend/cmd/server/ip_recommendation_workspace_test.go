package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestIPConversationExclusionsSurviveInferenceAndLiveScoring(t *testing.T) {
	before := recommendationSubscription{Domain: "ip", Query: "为新品寻找合作IP", Markets: []string{"东南亚"}, BudgetCurrency: "USD", BudgetMax: 9000}
	next := ruleIPConversationCriteria(before, "优先艺术，排除游戏")
	next = deriveIPRecommendationCriteria(next)
	if !slices.Contains(next.ExcludedIPTypes, "游戏") || !slices.Contains(next.IPTypes, "文化艺术") || next.BudgetMax != 9000 || !slices.Equal(next.Markets, before.Markets) {
		t.Fatalf("refinement lost confirmed conditions: %+v", next)
	}
	row := map[string]any{"id": 11, "name": "游戏候选", "ipType": "游戏", "markets": []string{"东南亚"}, "cooperationStatus": "可合作"}
	if _, ok := scoreIPRecommendation(row, next); ok {
		t.Fatal("excluded game entered the live candidate set")
	}
	onlyExclude := deriveIPRecommendationCriteria(ruleIPConversationCriteria(before, "不要游戏"))
	if slices.Contains(onlyExclude.IPTypes, "游戏") {
		t.Fatal("negative mention was inferred as a requested type")
	}
}
func TestIPWorkspaceAdviceEnvelopeAndLegacyCompatibility(t *testing.T) {
	candidate := recommendationCandidate{ID: 7, Name: "真实资源", Advice: "适用场景：设计共创。匹配原因：符合产品方向。", Resource: map[string]any{"ipType": "文化艺术"}}
	candidate.Assessment = defaultIPRecommendationAssessment(recommendationSubscription{Domain: "ip"}, candidate)
	advice, assessment := decodedRecommendationAdvice(storedRecommendationAdvice(candidate))
	if advice != candidate.Advice || assessment == nil || assessment.Verdict == "" || len(assessment.Basis) == 0 {
		t.Fatal("structured fields were lost on persistence round trip")
	}
	legacy, oldAssessment := decodedRecommendationAdvice("历史合作建议")
	if legacy != "历史合作建议" || oldAssessment != nil {
		t.Fatal("legacy advice must remain plain text")
	}
	unsafe := cleanIPRecommendationAssessment(&ipRecommendationAssessment{Verdict: "已获授权，保证销量", Pending: []string{"已签约"}}, candidate.Assessment)
	if strings.Contains(unsafe.Verdict, "已获授权") || len(unsafe.Pending) == 0 || !strings.Contains(unsafe.Pending[len(unsafe.Pending)-1], "版权方确认") {
		t.Fatal("unverified rights assertion survived or uncertainty vanished")
	}
}
func TestIPConversationModelContractAndCandidateWhitelist(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if len(request.Messages) != 2 || !strings.Contains(request.Messages[0].Content, "不能返回HTML") || !strings.Contains(request.Messages[1].Content, "已确认预算") || !strings.Contains(request.Messages[1].Content, "compareIds") {
			t.Error("model did not receive the workspace contract and conversation context")
		}
		payload := `{"presentation":{"headline":"先核对当前候选的设计适配","explanation":"结合已确认的产品方向比较。","basis":[{"label":"产品适配","value":"资料支持设计表达"},{"label":"授权","value":"已获授权"}],"primaryResourceId":999,"alternativeResourceId":7,"questions":[],"suggestedPrompts":["核对版权资料"]}}`
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": payload}}}})
	}))
	defer provider.Close()
	subscription := recommendationSubscription{Domain: "ip", IPWorkspace: &ipRecommendationConversation{Messages: []ipRecommendationMessage{{Role: "user", Content: "已确认预算是9000美元"}}}}
	result, err := (&app{}).requestIPConversationPlan(context.Background(), assistantAIModel{BaseURL: provider.URL, APIKey: "test", Model: "mock", Timeout: time.Second}, subscription, "比较候选", "compare", []map[string]any{{"resourceId": int64(7), "resourceName": "真实资源"}}, []int64{7})
	if err != nil {
		t.Fatal(err)
	}
	if result.Presentation.PrimaryResourceID != 0 || result.Presentation.AlternativeResourceID != 7 || len(result.Presentation.Basis) != 1 || result.Presentation.Source != "ai" {
		t.Fatalf("unknown IDs or unsupported claims escaped validation: %+v", result)
	}
}
func TestIPConversationRejectsUnauthenticatedRequests(t *testing.T) {
	response := httptest.NewRecorder()
	(&app{}).ipRecommendationConversation(response, httptest.NewRequest(http.MethodPost, "/business/recommendations/ip/conversation", strings.NewReader(`{"action":"recommend","message":"找IP"}`)))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated conversation returned %d", response.Code)
	}
}
