package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func englishIPContext() context.Context {
	return context.WithValue(context.Background(), ipLanguageKey{}, "en")
}

func TestIPLocaleHTTPPreservesBusinessAndHistory(t *testing.T) {
	data := map[string]any{"projectName": "中文项目", "description": "保留原始需求", "ipWorkspace": map[string]any{"messages": []any{map[string]string{"role": "assistant", "content": "历史回答"}}}}
	data["request"] = map[string]any{"projectName": "原始名称", "deadlines": map[string]ipStageDeadline{"ip": {Label: "IP意见", Warning: "2027 年法定节假日及调休日历尚未配置，截止时间待核定"}}}
	original, _ := json.Marshal(data)
	rec := httptest.NewRecorder()
	logger := &loggingResponseWriter{ResponseWriter: rec}
	handler := (&app{}).withIPLocale(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ipLanguage(r.Context()) != "en" {
			t.Fatal("language context missing")
		}
		writeOK(w, data)
	}))
	req := httptest.NewRequest("GET", "/business/ip/requests/detail", nil)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	handler.ServeHTTP(logger, req)
	var response apiResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if containsHan(response.Message) || !logger.hasAPICode || logger.apiCode != 0 {
		t.Fatalf("localization/logging failed: %s", rec.Body.String())
	}
	warning := response.Data.(map[string]any)["request"].(map[string]any)["deadlines"].(map[string]any)["ip"].(map[string]any)["warning"].(string)
	if containsHan(warning) || !strings.Contains(warning, "2027") {
		t.Fatal("calendar warning did not localize")
	}
	after, _ := json.Marshal(data)
	if string(original) != string(after) {
		t.Fatal("localization mutated source data")
	}
	if !strings.Contains(rec.Body.String(), "中文项目") || !strings.Contains(rec.Body.String(), "历史回答") {
		t.Fatal("business content or history was rewritten")
	}
}
func TestIPLocaleWriterSupportsStreaming(t *testing.T) {
	rec := httptest.NewRecorder()
	logger := &loggingResponseWriter{ResponseWriter: rec}
	writer := &ipLocalizedWriter{logger, "en"}
	stream := newIPConversationStream(englishIPContext(), writer)
	if err := stream.send("status", map[string]string{"message": "正在筛选候选并核对合作资料…"}); err != nil {
		t.Fatal(err)
	}
	if err := stream.send("error", map[string]string{"message": "研究会话保存失败，请重试"}); err != nil {
		t.Fatal(err)
	}
	stream.close()
	if containsHan(rec.Body.String()) || !strings.Contains(rec.Body.String(), "event: status") || !logger.hasAPICode || logger.apiCode != 500 {
		t.Fatalf("invalid localized stream: %s", rec.Body.String())
	}
}
func TestIPLocaleEnglishClarificationAndParsing(t *testing.T) {
	first := prepareIPResearchBrief(recommendationSubscription{}, "Recommend some esports IPs")
	if first.Topic != "电竞" || len(ipResearchQuestions(first)) != 2 {
		t.Fatalf("missing clarification: %#v", first)
	}
	p := localizeIPPresentation(englishIPContext(), ipResearchPresentation(&ipExternalSearch{Phase: "clarify", Brief: &first}))
	if containsHan(p.Headline) || containsHan(p.Explanation) || len(p.Questions) != 2 || containsHan(strings.Join(p.Questions, " ")) {
		t.Fatalf("clarification not English: %#v", p)
	}
	first.Asked = true
	before := first
	next := prepareIPResearchBrief(recommendationSubscription{IPWorkspace: &ipRecommendationConversation{ExternalBrief: &first}}, "Southeast Asia, product collaboration, exclude teams")
	if !reflect.DeepEqual(next.Markets, []string{"东南亚"}) || next.Goal != "产品联名" || !reflect.DeepEqual(next.ExcludedKinds, []string{"战队"}) {
		t.Fatalf("English intent mismatch: %#v", next)
	}
	if !reflect.DeepEqual(before, first) {
		t.Fatal("previous brief was mutated")
	}
	criteria := ruleIPConversationCriteria(recommendationSubscription{IPTypes: []string{"游戏"}}, "prefer art, exclude games", "en")
	if len(criteria.IPTypes) != 1 || criteria.IPTypes[0] != "文化艺术" || len(criteria.ExcludedIPTypes) != 1 || criteria.ExcludedIPTypes[0] != "游戏" {
		t.Fatalf("English exclusion mismatch: %#v", criteria)
	}
	if containsHan(criteria.Query) {
		t.Fatal("English rule summary introduced Chinese scaffolding")
	}
}
func TestIPLocaleEnglishPromptKeepsCanonicalCriteria(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(body.Messages[0].Content, "LANGUAGE RULE") || !strings.Contains(body.Messages[0].Content, "canonical filter values") {
			t.Error("English instruction missing")
		}
		answer := map[string]any{"criteria": map[string]any{"ipTypes": []string{"游戏"}, "markets": []string{"东南亚"}}, "presentation": map[string]any{"headline": "Confirm the collaboration scope", "explanation": "Which market should this project prioritize?", "basis": []map[string]string{{"label": "Regional conditions", "value": "Rights require verification"}}, "questions": []string{"Which market?"}, "suggestedPrompts": []string{}, "adjustments": []string{}}}
		content, _ := json.Marshal(answer)
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
	}))
	defer server.Close()
	plan, err := (&app{}).requestIPConversationPlanResponse(englishIPContext(), assistantAIModel{BaseURL: server.URL, Model: "test", Timeout: time.Second}, recommendationSubscription{}, "Recommend gaming IPs", "clarify", nil, nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Criteria.IPTypes[0] != "游戏" || plan.Criteria.Markets[0] != "东南亚" || plan.Presentation.Basis[0].Label != "Regional conditions" {
		t.Fatalf("canonical IDs or English label damaged: %#v", plan)
	}
}
func TestIPLocaleRuleCandidateAndFeishuEnglish(t *testing.T) {
	c := recommendationCandidate{Name: "Original IP Name", Reason: "覆盖目标市场；IP 类型匹配", Risks: "合作状态需进一步确认", AdviceSource: "rule", Scenarios: []string{"新品发布：评估产品功能与游戏情境的内容共创"}, Resource: map[string]any{"ipType": "游戏", "cooperationStatus": "待评估"}, Assessment: &ipRecommendationAssessment{}}
	localized := ipEnglishCandidate(recommendationSubscription{Markets: []string{"东南亚"}}, c)
	if containsHan(localized.Advice) || containsHan(localized.Assessment.Fit) || containsHan(strings.Join(localized.Scenarios, " ")) || localized.Name != c.Name || c.Resource["ipType"] != "游戏" {
		t.Fatalf("invalid English rule advice: %#v", localized)
	}
	message := ipStageFeishuMessage(Config{}, 12, "原项目名", "marketing_reviewed", "en")
	if !strings.Contains(message, "原项目名") || !strings.Contains(message, "Project:") || strings.Contains(message, "意见") {
		t.Fatalf("invalid English notification: %s", message)
	}
}
