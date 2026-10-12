package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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
	t.Cleanup(func() { _ = db.Close() })
	cfg.AIModel.APIKey = "" // verify the honest rule fallback, never call an external model
	a := newApp(db, cfg)
	ctx := context.Background()
	var adminID int
	err = db.QueryRowContext(ctx, `select u.id from sys_users u join sys_user_roles ur on ur.user_id=u.id join sys_roles r on r.id=ur.role_id where r.code='admin' and r.status=1 limit 1`).Scan(&adminID)
	if err != nil {
		t.Fatal(err)
	}

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
	token := fmt.Sprintf("Bearer kol.%d.%d.%d", adminID, time.Now().Unix(), time.Now().Unix())
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
		if int64(row["resourceId"].(float64)) != want || row["source"] != "rule" || row["isNew"] != false {
			t.Fatalf("unexpected immediate %s result: %+v", domain, row)
		}
	}
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
	deniedToken := fmt.Sprintf("Bearer kol.%d.%d.%d", 9999999, time.Now().Unix(), time.Now().Unix())
	status, _ = post(a.recommendationResultsList, map[string]any{"domain": "creator", "view": "all"}, deniedToken)
	if status != http.StatusForbidden {
		t.Fatalf("expected unauthorized menu to return 403, got %d", status)
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
}
