package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRecommendationSaveActiveFieldCanPauseWithoutOverwritingCriteria(t *testing.T) {
	var input recommendationSaveInput
	if err := json.Unmarshal([]byte(`{"id":7,"domain":"ip","active":false}`), &input); err != nil {
		t.Fatal(err)
	}
	if input.ID != 7 || input.Domain != "ip" || input.ActiveInput == nil || *input.ActiveInput {
		t.Fatalf("pause payload not decoded: %+v", input)
	}
}

func TestRecommendationWeeklyRunKeyUsesShanghaiMondayMorning(t *testing.T) {
	tests := []struct {
		utc  string
		want string
	}{
		{"2026-10-04T23:59:00Z", ""},
		{"2026-10-05T00:59:00Z", ""},
		{"2026-10-05T01:00:00Z", "weekly-20261005"},
		{"2026-10-05T15:59:00Z", "weekly-20261005"},
		{"2026-10-05T16:00:00Z", ""},
	}
	for _, tc := range tests {
		now, err := time.Parse(time.RFC3339, tc.utc)
		if err != nil {
			t.Fatal(err)
		}
		got, due := recommendationWeeklyRunKey(now)
		if got != tc.want || due != (tc.want != "") {
			t.Errorf("%s: key=%q due=%v, want %q", tc.utc, got, due, tc.want)
		}
	}
}

func TestFilterRecommendationAIRanksOnlyAcceptsRealCandidateIDs(t *testing.T) {
	candidates := []recommendationCandidate{
		{ID: 11, Name: "A", Score: 70, Reason: "规则 A", Source: "rule"},
		{ID: 22, Name: "B", Score: 60, Reason: "规则 B", Source: "rule"},
	}
	ranked := filterRecommendationAIRanks(candidates, []recommendationAIRank{
		{ID: 999, Score: 100, Reason: "杜撰资源"},
		{ID: 22, Score: 82, Reason: "真实资源匹配"},
		{ID: 22, Score: 99, Reason: "重复"},
	})
	if len(ranked) != 2 || ranked[0].ID != 22 || ranked[0].Score != 82 || ranked[0].Source != "ai" {
		t.Fatalf("unexpected ranked candidates: %+v", ranked)
	}
	if ranked[1].ID != 11 || ranked[1].Source != "rule" {
		t.Fatalf("rule fallback must remain after verified AI selections: %+v", ranked)
	}
}

func TestCreatorRecommendationFreeTextMarketIsHardFilter(t *testing.T) {
	parsed := parseAssistantDemand(map[string]any{}, "寻找美国市场适合手机新品推广的科技达人")
	if str(parsed, "targetMarket") != "美国" || str(parsed, "resourceType") != "KOL" {
		t.Fatalf("unexpected parsed demand: %+v", parsed)
	}
	markets := []string{str(parsed, "targetMarket")}
	resourceTypes := recommendationAllResourceTypeAliases([]string{str(parsed, "resourceType")})
	us := map[string]any{"country": "United States", "platform": "YouTube", "resourceType": "KOL"}
	uk := map[string]any{"country": "英国", "platform": "YouTube", "resourceType": "KOL"}
	de := map[string]any{"country": "德国", "platform": "YouTube", "resourceType": "KOL"}
	if !recommendationCreatorFilters(us, markets, nil, resourceTypes) ||
		recommendationCreatorFilters(uk, markets, nil, resourceTypes) ||
		recommendationCreatorFilters(de, markets, nil, resourceTypes) {
		t.Fatal("explicit US intent must exclude UK and German creators")
	}
	clause, args := recommendationSQLAny("r.country", recommendationAllMarketAliases(markets), false)
	if !strings.Contains(clause, "lower(r.country)") || len(args) < 2 {
		t.Fatalf("market must be filtered in SQL before LIMIT: %q %+v", clause, args)
	}
}

func TestIPMarketAndTypePrefilterBeforeLimit(t *testing.T) {
	marketClause, marketArgs := recommendationSQLAny("r.markets", recommendationAllMarketAliases([]string{"美国"}), true)
	typeClause, typeArgs := recommendationSQLAny("r.ip_type", recommendationIPTypeAliases([]string{"动画/影视"}), false)
	if marketClause == "" || typeClause == "" || len(marketArgs) == 0 || len(typeArgs) < 3 {
		t.Fatalf("IP market/type SQL prefilters missing: %q %+v / %q %+v", marketClause, marketArgs, typeClause, typeArgs)
	}
}

func TestRecommendationGlobalMarketMatchesBothDirections(t *testing.T) {
	for _, tc := range []struct {
		available string
		requested []string
		want      bool
	}{
		{"美国", []string{"全球"}, true},
		{"全球", []string{"美国"}, true},
		{`["全球"]`, []string{"北美"}, true},
		{"英国", []string{"美国"}, false},
		{"德国", []string{"北美"}, false},
	} {
		if got := recommendationMarketMatchesAny(tc.available, tc.requested); got != tc.want {
			t.Errorf("available %q requested %+v: got %v, want %v", tc.available, tc.requested, got, tc.want)
		}
	}
	if got := recommendationAllMarketAliases([]string{"全球"}); len(got) != 0 {
		t.Fatalf("global request must not have a SQL market restriction: %+v", got)
	}
	aliases := recommendationAllMarketAliases([]string{"美国"})
	if !strings.Contains(strings.Join(aliases, ","), "全球") {
		t.Fatalf("specific-market SQL prefilter must retain global entries: %+v", aliases)
	}
	if !recommendationIPMarketsMatch(`["全球"]`, []string{"美国"}) ||
		!recommendationIPMarketsMatch(`["英国"]`, []string{"全球"}) {
		t.Fatal("IP market filtering must use global coverage in both directions")
	}
}

func TestRecommendationOnlyUsesAvailableStatuses(t *testing.T) {
	if !recommendationCreatorActiveStatus("可合作") || recommendationCreatorActiveStatus("已停用") || recommendationCreatorActiveStatus("待补全") {
		t.Fatal("inactive creator status was treated as recommendable")
	}
	for _, status := range []string{"可合作", "洽谈中", "待评估"} {
		if !recommendationIPActiveStatus(status) {
			t.Errorf("expected %q to remain eligible", status)
		}
	}
	for _, status := range []string{"暂不可合作", "已下架", "已归档", "已停用", "停用"} {
		if recommendationIPActiveStatus(status) {
			t.Errorf("invalid IP status %q was treated as eligible", status)
		}
	}
}

func TestCreatorRecommendationRecomputesConfirmedProductClaim(t *testing.T) {
	row := map[string]any{
		"id": 7, "name": "科技达人", "status": "可合作", "country": "美国",
		"resourceType": "KOL", "platform": "YouTube", "score": 80,
		"confirmedProducts": "Infinix HOT 70",
	}
	s := recommendationSubscription{Domain: "creator", Query: "寻找美国科技达人", Markets: []string{"美国"}}
	criteria := deriveCreatorRecommendationCriteria(s)
	confirmed, ok := scoreCreatorRecommendation(cloneRecommendationTestMap(row), s, criteria)
	if !ok || !strings.Contains(confirmed.Reason, "已确认且已发布") {
		t.Fatalf("confirmed cooperation missing from reason: %+v %v", confirmed, ok)
	}
	row["confirmedProducts"] = []string{}
	current, ok := scoreCreatorRecommendation(cloneRecommendationTestMap(row), s, criteria)
	if !ok || strings.Contains(current.Reason, "已确认且已发布") || current.Score >= confirmed.Score {
		t.Fatalf("revoked cooperation claim remained in current recommendation: %+v %v", current, ok)
	}
	row["country"] = "英国"
	if _, ok := scoreCreatorRecommendation(cloneRecommendationTestMap(row), s, criteria); ok {
		t.Fatal("creator that moved outside requested market remains recommended")
	}
}

func TestIPTypeAliasesMatchHistoricalNames(t *testing.T) {
	for _, tc := range []struct{ chosen, stored string }{
		{"动画", "动漫"},
		{"动画", "动画/影视"},
		{"影视", "动画/影视"},
		{"体育", "体育IP"},
		{"游戏", "电竞"},
		{"线下活动", "音乐/活动"},
		{"动画/影视", "动漫"},
		{"动画/影视", "影视"},
		{"体育IP", "体育"},
		{"音乐/活动", "音乐"},
		{"文化艺术", "艺术"},
	} {
		if !recommendationMatchesAny(tc.stored, recommendationIPTypeAliases([]string{tc.chosen})) {
			t.Errorf("%q should match historical type %q", tc.chosen, tc.stored)
		}
	}
}

func TestIPRecommendationExcludesUnavailableAndOverBudget(t *testing.T) {
	base := map[string]any{
		"id": 3, "name": "测试 IP", "ipType": "动漫", "markets": `["美国"]`,
		"audience": "年轻用户", "summary": "新品联名", "cooperationStatus": "可合作",
		"currency": "USD", "priceMin": "12000", "priceMax": "20000",
	}
	s := recommendationSubscription{
		Domain: "ip", Markets: []string{"美国"}, IPTypes: []string{"动漫"},
		Audience: "年轻", BudgetCurrency: "USD", BudgetMax: 15000,
	}
	candidate, ok := scoreIPRecommendation(cloneRecommendationTestMap(base), s)
	if !ok || candidate.ID != 3 || candidate.Score < 70 {
		t.Fatalf("expected eligible IP candidate, got %+v %v", candidate, ok)
	}
	overBudget := s
	overBudget.BudgetMax = 10000
	if _, ok := scoreIPRecommendation(cloneRecommendationTestMap(base), overBudget); ok {
		t.Fatal("IP above a same-currency budget must be excluded")
	}
	otherCurrency := s
	otherCurrency.BudgetCurrency = "CNY"
	candidate, ok = scoreIPRecommendation(cloneRecommendationTestMap(base), otherCurrency)
	if !ok || !strings.Contains(candidate.Risks, "币种不同") {
		t.Fatalf("currency mismatch should keep candidate with price warning: %+v %v", candidate, ok)
	}
	unavailable := cloneRecommendationTestMap(base)
	unavailable["cooperationStatus"] = "暂不可合作"
	if _, ok := scoreIPRecommendation(unavailable, s); ok {
		t.Fatal("unavailable IP must not be recommended")
	}
	wrongMarket := s
	wrongMarket.Markets = []string{"日本"}
	if _, ok := scoreIPRecommendation(cloneRecommendationTestMap(base), wrongMarket); ok {
		t.Fatal("wrong market must not be recommended")
	}
}

func cloneRecommendationTestMap(source map[string]any) map[string]any {
	target := make(map[string]any, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}
