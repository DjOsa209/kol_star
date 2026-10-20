package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// Run against an isolated local database with:
// KOL_RECOMMENDATION_DB_SMOKE=1 KOL_RECOMMENDATION_CONFIG=/abs/config.yaml go test ./cmd/server -run TestRecommendationDBSmoke -count=1
// The test owns and removes every row it creates, even when an assertion fails.
func TestRecommendationDBSmoke(t *testing.T) {
	if os.Getenv("KOL_RECOMMENDATION_DB_SMOKE") != "1" {
		t.Skip("local database smoke test is opt-in")
	}
	configPath := os.Getenv("KOL_RECOMMENDATION_CONFIG")
	if configPath == "" {
		t.Fatal("KOL_RECOMMENDATION_CONFIG is required")
	}
	v := viper.New()
	v.SetConfigFile(configPath)
	if err := v.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatal(err)
	}
	db, err := openDB(cfg.MySQL)
	if err != nil {
		t.Fatal(err)
	}
	base := db
	t.Cleanup(func() { _ = base.Close() })
	// Clone only the schema and authorization seeds; model tests never alter
	// the development database's configuration or recommendation history.
	if !regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(cfg.MySQL.Database) {
		t.Fatal("invalid schema")
	}
	temp := fmt.Sprintf("creator_recommendation_test_%d", time.Now().UnixNano())
	if _, err := base.Exec("create database `" + temp + "` character set utf8mb4"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec("drop database `" + temp + "`") })
	for _, table := range []string{"sys_users", "sys_roles", "sys_user_roles", "sys_sessions", "sys_menus", "sys_role_menus", "biz_resources", "biz_ip_resources", "biz_ip_files", "biz_cooperations", "biz_recommendation_subscriptions", "biz_recommendation_runs", "biz_recommendation_results", "biz_recommendation_notifications", "biz_governance_rules"} {
		if _, err := base.Exec("create table `" + temp + "`.`" + table + "` like `" + cfg.MySQL.Database + "`.`" + table + "`"); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{"sys_users", "sys_roles", "sys_user_roles"} {
		if _, err := base.Exec("insert into `" + temp + "`.`" + table + "` select * from `" + cfg.MySQL.Database + "`.`" + table + "`"); err != nil {
			t.Fatal(err)
		}
	}
	cfg.MySQL.Database = temp
	db, err = openDB(cfg.MySQL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var modelUnavailable atomic.Bool
	fakeModel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if modelUnavailable.Load() {
			http.Error(w, "test model outage", http.StatusServiceUnavailable)
			return
		}
		var request struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		var input struct {
			Domain     string `json:"domain"`
			Candidates []struct {
				ID int64 `json:"id"`
			} `json:"candidates"`
		}
		if len(request.Messages) < 2 {
			http.Error(w, "missing input", 400)
			return
		}
		_ = json.Unmarshal([]byte(request.Messages[1].Content), &input)
		// Keep the IP domain's existing fallback behavior under coverage.
		if input.Domain != "creator" {
			http.Error(w, "test model unavailable", 503)
			return
		}
		ranks := []recommendationAIRank{}
		for _, c := range input.Candidates {
			ranks = append(ranks, recommendationAIRank{ID: c.ID, Score: 83, Reason: "模型判断：科技视频适合产品体验", Advice: "适用场景：可考虑新品体验测评内容。匹配原因：候选资料显示为 YouTube 科技达人，可评估产品演示；报价与排期需确认。", Scenarios: []string{"可考虑产品体验视频"}})
		}
		content, _ := json.Marshal(map[string]any{"recommendations": ranks})
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(content)}}}})
	}))
	t.Cleanup(fakeModel.Close)
	config, _ := json.Marshal(map[string]any{"provider": "test", "model": "test", "baseUrl": fakeModel.URL})
	if _, err := db.Exec(`insert into biz_governance_rules(rule_type,name,enabled,content) values ('ai_model','test model',1,?)`, string(config)); err != nil {
		t.Fatal(err)
	}
	cfg.AIModel.APIKey = "test-key"
	a := newApp(db, cfg)
	ctx := context.Background()
	var adminID int
	err = db.QueryRowContext(ctx, `select u.id from sys_users u join sys_user_roles ur on ur.user_id=u.id join sys_roles r on r.id=ur.role_id where r.code='admin' and r.status=1 limit 1`).Scan(&adminID)
	if err != nil {
		t.Fatal(err)
	}
	sessionID, err := newSessionID()
	if err != nil {
		t.Fatal(err)
	}
	sessionStartedAt := time.Now()
	if err := a.createSession(httptest.NewRequest(http.MethodPost, "/login", nil), adminID, "admin", sessionID, sessionStartedAt); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`delete from sys_sessions where session_hash=?`, sessionHash(sessionID)) })

	marker := fmt.Sprintf("codex_rec_smoke_%d", time.Now().UnixNano())
	market := marker + "_market"
	creatorIDs := []int64{}
	ipIDs := []int64{}
	subscriptionIDs := []int64{}
	t.Cleanup(func() {
		for _, subID := range subscriptionIDs {
			_, _ = db.Exec(`delete n from biz_recommendation_notifications n join biz_recommendation_results r on r.id=n.result_id where r.subscription_id=?`, subID)
			_, _ = db.Exec(`delete from biz_recommendation_results where subscription_id=?`, subID)
			_, _ = db.Exec(`delete from biz_recommendation_runs where subscription_id=?`, subID)
			_, _ = db.Exec(`delete from biz_recommendation_subscriptions where id=?`, subID)
		}
		for _, id := range creatorIDs {
			_, _ = db.Exec(`delete from biz_resources where id=?`, id)
		}
		for _, id := range ipIDs {
			_, _ = db.Exec(`delete from biz_ip_resources where id=?`, id)
		}
	})
	addCreator := func(name string) int64 {
		t.Helper()
		result, err := db.ExecContext(ctx, `insert into biz_resources (name,resource_type,country,platform,status,score,level,risk_level) values (?,'KOL',?,'YouTube','可合作',95,'A','低')`, name, market)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := result.LastInsertId()
		creatorIDs = append(creatorIDs, id)
		return id
	}
	addIP := func(name string) int64 {
		t.Helper()
		result, err := db.ExecContext(ctx, `insert into biz_ip_resources (name,ip_type,rights_owner,markets,cooperation_status,currency,price_min,created_by) values (?,'动漫',?,?,'可合作','USD',1000,?)`, name, marker, `["`+market+`"]`, adminID)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := result.LastInsertId()
		ipIDs = append(ipIDs, id)
		return id
	}
	creatorOne := addCreator(marker + "_creator_one")
	ipOne := addIP(marker + "_ip_one")
	token := "Bearer " + newLoginTokenResponse(adminID, sessionStartedAt, sessionStartedAt, sessionID)["accessToken"].(string)
	var baselineUnread int
	if err := db.QueryRowContext(ctx, `select count(*) from biz_recommendation_notifications where user_id=? and read_at is null`, adminID).Scan(&baselineUnread); err != nil {
		t.Fatal(err)
	}
	post := func(handler http.HandlerFunc, body any, auth string) (int, map[string]any) {
		t.Helper()
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
		req.Header.Set("Authorization", auth)
		rec := httptest.NewRecorder()
		handler(rec, req)
		var response struct {
			Code int            `json:"code"`
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode %s: %v", rec.Body.String(), err)
		}
		return rec.Code, response.Data
	}
	for _, domain := range []string{"creator", "ip"} {
		payload := map[string]any{"domain": domain, "name": marker + "_" + domain, "query": "寻找合作资源", "markets": []string{market}, "budgetCurrency": "USD", "budgetMax": 100000}
		if domain == "ip" {
			payload["ipTypes"] = []string{"动漫"}
		}
		status, data := post(a.recommendationSubscriptionSave, payload, token)
		if status != http.StatusOK {
			t.Fatalf("save %s failed: %d", domain, status)
		}
		id := int64(data["id"].(float64))
		subscriptionIDs = append(subscriptionIDs, id)
		status, data = post(a.recommendationResultsList, map[string]any{"domain": domain, "subscriptionId": id, "view": "all"}, token)
		if status != http.StatusOK || data["total"].(float64) != 1 {
			t.Fatalf("immediate %s: status %d data %+v", domain, status, data)
		}
		results := data["list"].([]any)
		row := results[0].(map[string]any)
		want := creatorOne
		if domain == "ip" {
			want = ipOne
		}
		wantSource := "rule"
		if domain == "creator" {
			wantSource = "ai"
		}
		if int64(row["resourceId"].(float64)) != want || row["source"] != wantSource || row["isNew"] != false {
			t.Fatalf("unexpected immediate %s result: %+v", domain, row)
		}
		if domain == "creator" && (row["score"] != float64(83) || row["scoreSource"] != "ai" || row["adviceSource"] != "ai") {
			t.Fatalf("model scores and advice must survive reload: %+v", row)
		}
		if domain == "ip" {
			resultID := int64(row["id"].(float64))
			status, _ = post(a.recommendationResultFeedback, map[string]any{"domain": "ip", "id": resultID, "feedback": "interested"}, token)
			if status != http.StatusOK {
				t.Fatalf("IP feedback before refresh failed: %d", status)
			}
			status, _ = post(a.recommendationRun, map[string]any{"domain": "ip", "subscriptionId": id}, token)
			if status != http.StatusOK {
				t.Fatalf("IP refresh failed: %d", status)
			}
			status, data = post(a.recommendationResultsList, map[string]any{"domain": "ip", "subscriptionId": id, "view": "all"}, token)
			if status != http.StatusOK || data["total"].(float64) != 1 {
				t.Fatalf("IP refresh created duplicate result: %d %+v", status, data)
			}
			refreshed := data["list"].([]any)[0].(map[string]any)
			if refreshed["feedback"] != "interested" || refreshed["adviceSource"] != "rule" || refreshed["advice"] == "" {
				t.Fatalf("IP refresh must preserve interest and backfill advice: %+v", refreshed)
			}
		}
	}
	t.Run("creator failures never write rules or report a successful run", func(t *testing.T) {
		id := subscriptionIDs[0]
		for _, mode := range []string{"disabled", "outage"} {
			if mode == "disabled" {
				_, err = db.Exec(`update biz_governance_rules set enabled=0 where rule_type='ai_model'`)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				modelUnavailable.Store(true)
			}
			_, runErr := a.executeRecommendationRun(ctx, id, adminID, "creator", "failure-"+mode, false)
			modelUnavailable.Store(false)
			_, err = db.Exec(`update biz_governance_rules set enabled=1 where rule_type='ai_model'`)
			if err != nil {
				t.Fatal(err)
			}
			if !errors.Is(runErr, errCreatorRecommendationModel) {
				t.Fatalf("missing explicit model error: %v", runErr)
			}
		}
		var count int
		if err := db.QueryRow(`select count(*) from biz_recommendation_runs where subscription_id=?`, id).Scan(&count); err != nil || count != 1 {
			t.Fatalf("failed generation wrote run: count=%d err=%v", count, err)
		}
		if _, err := db.Exec(`update biz_recommendation_results set source='rule',advice_source='rule' where subscription_id=?`, id); err != nil {
			t.Fatal(err)
		}
		list, err := a.recommendationResultsData(ctx, adminID, "creator", id, "all")
		if err != nil || len(list) != 0 {
			t.Fatalf("historical rules remain visible: %+v %v", list, err)
		}
		if _, err := db.Exec(`update biz_recommendation_results set source='ai',advice_source='ai' where subscription_id=?`, id); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`update biz_resources set followers=12345 where id=?`, creatorOne); err != nil {
			t.Fatal(err)
		}
		list, err = a.recommendationResultsData(ctx, adminID, "creator", id, "all")
		if err != nil || len(list) != 0 {
			t.Fatalf("stale model assessment replaced with rules: %+v %v", list, err)
		}
		if _, err := db.Exec(`update biz_resources set followers=0 where id=?`, creatorOne); err != nil {
			t.Fatal(err)
		}
	})
	for i := 2; i <= 7; i++ {
		_ = addCreator(fmt.Sprintf("%s_creator_%d", marker, i))
	}
	ipTwo := addIP(marker + "_ip_two")
	for i, domain := range []string{"creator", "ip"} {
		key := marker + "_" + domain + "_weekly"
		if _, err := a.executeRecommendationRun(ctx, subscriptionIDs[i], adminID, domain, key, true); err != nil {
			t.Fatal(err)
		}
		if _, err := a.executeRecommendationRun(ctx, subscriptionIDs[i], adminID, domain, key, true); err != nil {
			t.Fatal(err)
		}
		var notices int
		err := db.QueryRowContext(ctx, `select count(*) from biz_recommendation_notifications n join biz_recommendation_results r on r.id=n.result_id where r.subscription_id=?`, subscriptionIDs[i]).Scan(&notices)
		wantNotices := 1
		wantNew := float64(1)
		if domain == "creator" {
			wantNotices = 5
			wantNew = 6
		}
		if err != nil || notices != wantNotices {
			t.Fatalf("weekly dedupe %s: notices=%d err=%v", domain, notices, err)
		}
		status, data := post(a.recommendationResultsList, map[string]any{"domain": domain, "subscriptionId": subscriptionIDs[i], "view": "new"}, token)
		if status != http.StatusOK || data["total"].(float64) != wantNew {
			t.Fatalf("new view %s: status=%d data=%+v", domain, status, data)
		}
	}
	getNotices := func() map[string]any {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", token)
		rec := httptest.NewRecorder()
		a.recommendationNotifications(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("notification get: %d %s", rec.Code, rec.Body.String())
		}
		var response struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response.Data
	}
	notices := getNotices()
	if notices["unread"].(float64) != float64(baselineUnread+6) {
		t.Fatalf("unread count before read: %+v", notices)
	}
	var ownNoticeID int64
	if err := db.QueryRowContext(ctx, `select n.id from biz_recommendation_notifications n join biz_recommendation_results r on r.id=n.result_id where r.subscription_id=? limit 1`, subscriptionIDs[0]).Scan(&ownNoticeID); err != nil {
		t.Fatal(err)
	}
	status, _ := post(a.recommendationNotificationsRead, map[string]any{"ids": []int64{ownNoticeID}}, token)
	if status != http.StatusOK {
		t.Fatalf("notification read: %d", status)
	}
	notices = getNotices()
	if notices["unread"].(float64) != float64(baselineUnread+5) {
		t.Fatalf("unread count after read: %+v", notices)
	}
	status, data := post(a.recommendationSubscriptionSave, map[string]any{"id": subscriptionIDs[0], "domain": "creator", "active": false}, token)
	if status != http.StatusOK || int64(data["id"].(float64)) != subscriptionIDs[0] {
		t.Fatalf("pause subscription: %d %+v", status, data)
	}
	var active int
	var query string
	if err := db.QueryRowContext(ctx, `select active,query_text from biz_recommendation_subscriptions where id=?`, subscriptionIDs[0]).Scan(&active, &query); err != nil || active != 0 || query != "寻找合作资源" {
		t.Fatalf("pause must preserve criteria: active=%d query=%q err=%v", active, query, err)
	}
	_ = addCreator(marker + "_creator_after_pause")
	if _, err := a.executeRecommendationRun(ctx, subscriptionIDs[0], adminID, "creator", marker+"_paused_weekly", true); err != nil {
		t.Fatal(err)
	}
	var totalResults int
	if err := db.QueryRowContext(ctx, `select count(*) from biz_recommendation_results where subscription_id=?`, subscriptionIDs[0]).Scan(&totalResults); err != nil || totalResults != 7 {
		t.Fatalf("paused search must not run: results=%d err=%v", totalResults, err)
	}
	// Stored matches and in-app reminders must respect current availability and
	// market facts, even when the next weekly job has not run yet.
	if _, err := db.ExecContext(ctx, `update biz_resources set status='已停用' where id=?`, creatorIDs[2]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `update biz_resources set country=? where id=?`, marker+"_other_market", creatorOne); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `update biz_ip_resources set cooperation_status='暂不可合作' where id=?`, ipTwo); err != nil {
		t.Fatal(err)
	}
	status, data = post(a.recommendationResultsList, map[string]any{"domain": "creator", "subscriptionId": subscriptionIDs[0], "view": "all"}, token)
	if status != http.StatusOK || data["total"].(float64) != 5 {
		t.Fatalf("stale creator results must be hidden: %d %+v", status, data)
	}
	status, data = post(a.recommendationResultsList, map[string]any{"domain": "ip", "subscriptionId": subscriptionIDs[1], "view": "all"}, token)
	if status != http.StatusOK || data["total"].(float64) != 1 {
		t.Fatalf("unavailable IP result must be hidden: %d %+v", status, data)
	}
	notices = getNotices()
	if notices["unread"].(float64) != float64(baselineUnread+3) {
		t.Fatalf("stale reminders must be hidden from the bell: %+v", notices)
	}
	deniedToken := fmt.Sprintf("Bearer kol.%d.%d.%d.%s", 9999999, time.Now().Unix(), time.Now().Unix(), sessionID)
	status, _ = post(a.recommendationResultsList, map[string]any{"domain": "creator", "view": "all"}, deniedToken)
	if status != http.StatusUnauthorized {
		t.Fatalf("expected unregistered session to return 401, got %d", status)
	}
	// A read-only check that each subscription is tied to the admin owner.
	for _, id := range subscriptionIDs {
		var owner int
		if err := db.QueryRowContext(ctx, `select user_id from biz_recommendation_subscriptions where id=?`, id).Scan(&owner); err != nil || owner != adminID {
			t.Fatalf("subscription owner: %d %v", owner, err)
		}
	}
	globalCreator := addCreator(marker + "_global_creator")
	if _, err := db.ExecContext(ctx, `update biz_resources set country='全球' where id=?`, globalCreator); err != nil {
		t.Fatal(err)
	}
	globalIP := addIP(marker + "_global_ip")
	if _, err := db.ExecContext(ctx, `update biz_ip_resources set markets='["全球"]' where id=?`, globalIP); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		domain string
		market string
		want   int64
	}{
		{"creator", market, globalCreator},
		{"creator", "全球", globalCreator},
		{"ip", market, globalIP},
		{"ip", "全球", globalIP},
	} {
		s := recommendationSubscription{Domain: tc.domain, Markets: []string{tc.market}}
		if tc.domain == "ip" {
			s.IPTypes = []string{"动漫"}
		}
		candidates, err := a.recommendationCandidates(ctx, s)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, candidate := range candidates {
			if candidate.ID == tc.want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("global %s resource missing for %q search: %+v", tc.domain, tc.market, candidates)
		}
	}
	t.Run("external search stays in owned conversation without altering library results", func(t *testing.T) {
		withoutModel := cfg
		withoutModel.AIModel.APIKey = ""
		withoutModel.WebSearch = WebSearchConfig{}
		a := newApp(db, withoutModel)
		id := subscriptionIDs[1]
		before, err := scanRecommendationSubscription(db.QueryRowContext(ctx, `select `+recommendationSubscriptionColumns+` from biz_recommendation_subscriptions where id=?`, id))
		if err != nil {
			t.Fatal(err)
		}
		beforeResults, err := a.recommendationResultsData(ctx, adminID, "ip", id, "all")
		if err != nil {
			t.Fatal(err)
		}
		originalTransport := http.DefaultTransport
		t.Cleanup(func() { http.DefaultTransport = originalTransport })
		t.Setenv("IP_WEB_SEARCH_PROVIDER", "tavily")
		t.Setenv("TAVILY_API_KEY", "test-only-secret")
		calls := 0
		http.DefaultTransport = recommendationEvidenceTransportFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			return recommendationEvidenceTestResponse(503, `{}`), nil
		})
		status, data := post(a.ipRecommendationConversation, map[string]any{"subscriptionId": id, "action": "search", "message": "推荐几个电竞IP", "documentText": "PRIVATE attachment, never search this"}, token)
		if status != 200 {
			t.Fatalf("clarification failed: %d", status)
		}
		p := data["presentation"].(map[string]any)
		search := p["webSearch"].(map[string]any)
		if search["phase"] != "clarify" || len(p["questions"].([]any)) == 0 || len(p["questions"].([]any)) > 2 || calls != 0 {
			t.Fatal("vague request searched instead of clarifying")
		}
		status, data = post(a.ipRecommendationConversation, map[string]any{"subscriptionId": id, "action": "search", "message": "东南亚，偏产品联名，预算还没定"}, token)
		if status != 200 {
			t.Fatalf("scoped research failed: %d", status)
		}
		search = data["presentation"].(map[string]any)["webSearch"].(map[string]any)
		if search["phase"] != "unavailable" || calls != 0 || len(data["results"].([]any)) != len(beforeResults) {
			t.Fatal("missing model charged search or altered library results")
		}
		after, err := scanRecommendationSubscription(db.QueryRowContext(ctx, `select `+recommendationSubscriptionColumns+` from biz_recommendation_subscriptions where id=?`, id))
		if err != nil || after.Query != before.Query || after.BudgetMax != before.BudgetMax || after.IPWorkspace == nil || len(after.IPWorkspace.Messages) != 4 || after.IPWorkspace.ExternalBrief.Topic != "电竞" || after.IPWorkspace.ExternalBrief.Goal != "产品联名" {
			t.Fatal("public research memory or private criteria persistence failed")
		}
		mock := &ipExternalSearch{Phase: "complete", Brief: after.IPWorkspace.ExternalBrief, Candidates: []ipExternalCandidate{{ID: "ext-test", Name: "Mock Nova", Kind: "战队", Identity: "Mock Nova esports team", Evidence: []recommendationEvidence{{Title: "Mock Nova esports", URL: "https://example.com/nova"}}}}, Results: []recommendationEvidence{}}
		after.IPWorkspace.Messages[3].Presentation.WebSearch = mock
		if _, err := a.persistIPConversation(ctx, after, after, adminID); err != nil {
			t.Fatal(err)
		}
		restored, err := scanRecommendationSubscription(db.QueryRowContext(ctx, `select `+recommendationSubscriptionColumns+` from biz_recommendation_subscriptions where id=?`, id))
		if err != nil || restored.IPWorkspace.Messages[3].Presentation.WebSearch.Candidates[0].ID != "ext-test" {
			t.Fatal("candidate evidence lost after reload")
		}
		t.Setenv("TAVILY_API_KEY", "")
		status, _ = post(a.ipRecommendationConversation, map[string]any{"subscriptionId": id, "action": "search", "message": "继续深入研究"}, token)
		if status != 503 {
			t.Fatalf("unconfigured search appeared successful: %d", status)
		}
		unchanged, err := scanRecommendationSubscription(db.QueryRowContext(ctx, `select `+recommendationSubscriptionColumns+` from biz_recommendation_subscriptions where id=?`, id))
		if err != nil || len(unchanged.IPWorkspace.Messages) != 4 {
			t.Fatal("failed search persisted a fake response")
		}

	})
}
