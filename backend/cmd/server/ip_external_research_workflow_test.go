package main

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

func TestIPResearchPublicScopeAndMemory(t *testing.T) {
	s := recommendationSubscription{Query: "CONFIDENTIAL_PROJECT", Product: "SECRET_MODEL", BudgetMax: 123456}
	b := prepareIPResearchBrief(s, "推荐几个电竞IP")
	if len(ipResearchQuestions(b)) != 2 || len(b.Kinds) != 3 || slices.Contains(b.Kinds, "选手") {
		t.Fatalf("vague scope not gated: %+v", b)
	}
	if prepareIPResearchBrief(s, "推荐电竞IP，先探索").Explore {
		t.Fatal("must ask before exploration")
	}
	b.Asked = true
	s.IPWorkspace = &ipRecommendationConversation{ExternalBrief: &b}
	exploratory := prepareIPResearchBrief(s, "先探索，市场预算没定")
	if !exploratory.Explore || len(ipResearchQuestions(exploratory)) != 0 {
		t.Fatal("exploration rejected after asking")
	}
	b = prepareIPResearchBrief(s, "东南亚，产品联名，预算待定")
	if len(ipResearchQuestions(b)) != 0 || b.Goal != "产品联名" {
		t.Fatalf("minimum scope not accepted: %+v", b)
	}
	s.IPWorkspace.ExternalBrief = &b
	b = prepareIPResearchBrief(s, "只看战队，不要选手")
	if len(ipResearchQuestions(b)) != 0 || !slices.Equal(b.Kinds, []string{"战队"}) {
		t.Fatalf("memory or exclusions lost: %+v", b)
	}
	for round := 0; round < 3; round++ {
		for _, q := range researchQueries(b, round, nil) {
			if containsResearchTerm(q, "CONFIDENTIAL", "SECRET_MODEL", "123456", "phone", "smartphone") {
				t.Fatal("private or assumed product in public query")
			}
		}
	}
	b = prepareIPResearchBrief(s, "换成艺术设计")
	if b.Topic != "艺术设计" || len(b.ExcludedKinds) != 0 {
		t.Fatal("topic change retained esports restrictions")
	}
}

func researchMockCompletion(value any) *http.Response {
	content, _ := json.Marshal(value)
	payload, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
	return recommendationEvidenceTestResponse(200, string(payload))
}
func researchMockCandidate(name, kind, quote, reason string, ids []int) map[string]any {
	return map[string]any{"name": name, "kind": kind, "identityQuote": quote, "reason": reason, "entryPoints": []string{"评估内容共创"}, "evidenceIds": ids}
}
func TestIPResearchCandidateAdmission(t *testing.T) {
	pages := []recommendationEvidence{{Title: "Official esports profiles", URL: "https://example.com/teams", Summary: "T1 is a professional esports team. Nova is an esports team. Faker is an esports player. AgencyX is a licensing agency."}}
	raw := []any{
		researchMockCandidate("T1", "战队", "T1 is a professional esports team.", "目标市场适配待核实", []int{0}),
		researchMockCandidate("T1", "战队", "T1 is a professional esports team.", "duplicate", []int{0}),
		researchMockCandidate("Nova", "战队", "Nova is an esports team.", "available for licensing", []int{0}),
		researchMockCandidate("Faker", "选手", "Faker is an esports player.", "内容共创", []int{0}),
		researchMockCandidate("AgencyX", "品牌IP", "AgencyX is a licensing agency.", "内容共创", []int{0}),
		researchMockCandidate("Invented", "战队", "Invented is an esports team.", "内容共创", []int{0}),
		researchMockCandidate("Nova", "战队", "Nova is an esports team.", "内容共创", []int{99}),
	}
	transport := recommendationEvidenceTransportFunc(func(*http.Request) (*http.Response, error) {
		return researchMockCompletion(map[string]any{"candidates": raw}), nil
	})
	b := ipResearchBrief{Topic: "电竞", Kinds: []string{"战队", "赛事", "游戏IP"}}
	model := &assistantAIModel{BaseURL: "https://model.example.com"}
	items, err := extractIPResearchCandidates(context.Background(), model, b, pages, transport)
	if err != nil || len(items) != 1 || items[0].Name != "T1" || len(items[0].Pending) == 0 {
		t.Fatalf("admission failed: %+v %v", items, err)
	}
	b.Topic = "潮流品牌"
	items, err = extractIPResearchCandidates(context.Background(), model, b, pages, transport)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Name == "AgencyX" {
			t.Fatal("agency counted as IP")
		}
	}
}

func TestIPResearchBoundedRunAndFollowUp(t *testing.T) {
	var searches, extracts, models atomic.Int32
	transport := recommendationEvidenceTransportFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "model.example.com" {
			models.Add(1)
			return researchMockCompletion(map[string]any{"candidates": []any{researchMockCandidate("Nova", "战队", "Nova is an esports team.", "区域适配待核实", []int{0})}}), nil
		}
		if req.URL.Path == "/extract" {
			extracts.Add(1)
			return recommendationEvidenceTestResponse(200, `{"results":[{"url":"https://example.com/nova","raw_content":"Nova is an esports team. Official brand partners."}]}`), nil
		}
		searches.Add(1)
		var body map[string]any
		if json.NewDecoder(req.Body).Decode(&body) != nil {
			t.Error("invalid search payload")
		}
		q, _ := body["query"].(string)
		if containsResearchTerm(q, "SECRET", "123456") {
			t.Error("private query")
		}
		return recommendationEvidenceTestResponse(200, `{"results":[{"title":"Nova esports team","url":"https://example.com/nova","content":"Nova is an esports team.","score":0.95},{"title":"ISP代理IP","url":"https://proxy.example.com","content":"esports proxy"}]}`), nil
	})
	b := ipResearchBrief{Topic: "电竞", Markets: []string{"东南亚"}, Goal: "内容传播", Kinds: []string{"战队"}}
	result, err := runIPExternalResearch(context.Background(), b, ipSearchProvider{name: "tavily", endpoint: "https://api.tavily.com/search"}, &assistantAIModel{BaseURL: "https://model.example.com"}, transport, nil)
	if err != nil || result.Rounds > 3 || searches.Load() > 6 || extracts.Load() != 1 || models.Load() != 3 || len(result.Candidates) != 1 || len(result.Results) != 1 || len(result.Gaps) == 0 {
		t.Fatalf("limits/results failed: search=%d extract=%d model=%d result=%+v err=%v", searches.Load(), extracts.Load(), models.Load(), result, err)
	}
	if !slices.ContainsFunc(result.Queries, func(q string) bool { return strings.Contains(q, `"Nova"`) }) {
		t.Fatal("no named follow-up")
	}
	s := recommendationSubscription{IPWorkspace: &ipRecommendationConversation{ExternalBrief: &b, Messages: []ipRecommendationMessage{{Presentation: &ipRecommendationPresentation{WebSearch: result}}}}}
	focused := prepareIPResearchBrief(s, "继续补查「Nova」")
	if !slices.Equal(focused.FocusNames, []string{"Nova"}) {
		t.Fatal("focus not linked to verified candidate")
	}
	for round := 0; round < 3; round++ {
		for _, q := range researchQueries(focused, round, nil) {
			if !strings.Contains(q, `"Nova"`) {
				t.Fatal("focused query became broad")
			}
		}
	}
	encoded, _ := json.Marshal(s)
	var restored recommendationSubscription
	if json.Unmarshal(encoded, &restored) != nil || restored.IPWorkspace.ExternalBrief.Topic != "电竞" || restored.IPWorkspace.Messages[0].Presentation.WebSearch.Candidates[0].ID == "" {
		t.Fatal("research memory serialization failed")
	}
}

func TestIPResearchFailureBudgetAndPartialResults(t *testing.T) {
	provider := ipSearchProvider{name: "tavily", endpoint: "https://api.tavily.com/search"}
	b := ipResearchBrief{Topic: "电竞", Kinds: []string{"战队"}}
	model := &assistantAIModel{BaseURL: "https://model.example.com"}
	for _, failure := range []string{"unconfigured", "quota", "model"} {
		t.Run(failure, func(t *testing.T) {
			var calls atomic.Int32
			transport := recommendationEvidenceTransportFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host == "model.example.com" {
					return recommendationEvidenceTestResponse(503, `{}`), nil
				}
				if req.URL.Path == "/extract" {
					return recommendationEvidenceTestResponse(200, `{"results":[]}`), nil
				}
				calls.Add(1)
				if failure == "quota" {
					return recommendationEvidenceTestResponse(429, `{}`), nil
				}
				return recommendationEvidenceTestResponse(200, `{"results":[{"title":"Nova esports","url":"https://example.com/nova","content":"Nova is an esports team."}]}`), nil
			})
			m := model
			if failure == "unconfigured" {
				m = nil
			}
			result, err := runIPExternalResearch(context.Background(), b, provider, m, transport, nil)
			if err != nil || len(result.Candidates) != 0 || len(result.Results) != 0 || len(result.Gaps) == 0 || calls.Load() > 2 {
				t.Fatalf("failure retried or promoted pages: %+v %v", result, err)
			}
			if failure == "unconfigured" && calls.Load() != 0 {
				t.Fatal("charged without model")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	transport := recommendationEvidenceTransportFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "model.example.com" {
			return researchMockCompletion(map[string]any{"candidates": []any{researchMockCandidate("Nova", "战队", "Nova is an esports team.", "区域适配待核实", []int{0})}}), nil
		}
		if req.URL.Path == "/extract" {
			return recommendationEvidenceTestResponse(200, `{"results":[]}`), nil
		}
		return recommendationEvidenceTestResponse(200, `{"results":[{"title":"Nova esports","url":"https://example.com/nova","content":"Nova is an esports team."}]}`), nil
	})
	result, err := runIPExternalResearch(ctx, b, provider, model, transport, func(message string) error {
		if strings.HasPrefix(message, "第 2") {
			cancel()
		}
		return nil
	})
	if err != nil || result.Phase != "partial" || len(result.Candidates) != 1 {
		t.Fatalf("deadline lost prior verified results: %+v %v", result, err)
	}
}

func TestIPResearchPageCacheScopeAndRotation(t *testing.T) {
	var calls atomic.Int32
	transport := recommendationEvidenceTransportFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return recommendationEvidenceTestResponse(200, `{"results":[{"title":"Nova esports","url":"https://example.com/nova","content":"Nova is an esports team."}]}`), nil
	})
	cache := ipResearchCache{store: &recommendationEvidenceCacheStore{}, ownerID: 1}
	p := ipSearchProvider{name: "tavily", endpoint: "https://api.tavily.com/search", key: "test-first-key"}
	for run := 0; run < 2; run++ {
		pages, err := cachedIPResearchPages(context.Background(), "public esports team", p, nil, transport, cache)
		if err != nil || len(pages) != 1 || pages[0].Title == "mutated copy" {
			t.Fatal("cache request failed or leaked caller mutation")
		}
		pages[0].Title = "mutated copy"
	}
	if calls.Load() != 1 {
		t.Fatal("same public query did not reuse cached pages")
	}
	cache.ownerID = 2
	_, _ = cachedIPResearchPages(context.Background(), "public esports team", p, nil, transport, cache)
	p.key = "test-rotated-key"
	_, _ = cachedIPResearchPages(context.Background(), "public esports team", p, nil, transport, cache)
	if calls.Load() != 3 {
		t.Fatal("cache crossed user or key boundary")
	}
}

func TestIPResearchMarketDisambiguation(t *testing.T) {
	for _, tc := range []struct {
		message string
		want    string
	}{{"东南亚电竞产品联名", "东南亚"}, {"印度尼西亚电竞产品联名", "印度尼西亚"}, {"巴西电竞产品联名", "巴西"}} {
		b := prepareIPResearchBrief(recommendationSubscription{}, tc.message)
		if !slices.Equal(b.Markets, []string{tc.want}) {
			t.Fatalf("region substring collision for %s: %+v", tc.message, b.Markets)
		}
	}
}

func TestIPResearchExcludedTopicIsNotTheSearchDirection(t *testing.T) {
	for _, tc := range []struct{ message, want string }{{"优先艺术设计，排除游戏", "艺术设计"}, {"不要电竞，找体育IP", "体育"}} {
		b := prepareIPResearchBrief(recommendationSubscription{}, tc.message)
		if b.Topic != tc.want {
			t.Fatalf("excluded topic became active: %+v", b)
		}
	}
}
