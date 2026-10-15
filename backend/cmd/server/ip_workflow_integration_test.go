package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// Opt in with KOL_IP_WORKFLOW_DB_SMOKE=1 and KOL_RECOMMENDATION_CONFIG=/abs/local/config.yaml.
// All created users, sessions, requests, resources and notifications are removed.
func TestIPWorkflowDBSmoke(t *testing.T) {
	if os.Getenv("KOL_IP_WORKFLOW_DB_SMOKE") != "1" {
		t.Skip("local DB smoke is opt-in")
	}
	v := viper.New()
	v.SetConfigFile(os.Getenv("KOL_RECOMMENDATION_CONFIG"))
	if err := v.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	cfg, err := decodeConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Feishu.ApplicationEnabled = false // Never send tenant messages in tests.
	db, err := openDB(cfg.MySQL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	a := newApp(db, cfg)
	ctx := context.Background()
	marker := fmt.Sprintf("ddl_smoke_%d", time.Now().UnixNano())
	var requestID, ipID int64
	users := []int64{}
	t.Cleanup(func() {
		exec := func(query string, args ...any) {
			t.Helper()
			if _, err := db.Exec(query, args...); err != nil {
				t.Errorf("clean owned workflow fixture: %v", err)
			}
		}
		exec("delete from biz_ip_workflow_notifications where request_id=?", requestID)
		exec("delete from biz_ip_request_candidates where request_id=?", requestID)
		exec("delete from biz_ip_requests where id=?", requestID)
		exec("delete from biz_ip_resources where id=?", ipID)
		for _, id := range users {
			exec("delete from sys_sessions where user_id=?", id)
			exec("delete from sys_users where id=?", id)
		}
	})
	addUser := func(suffix string) (int64, string) {
		t.Helper()
		res, err := db.Exec("insert into sys_users(username,password_hash) values (?,?)", marker+suffix, strings.Repeat("0", 64))
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		users = append(users, id)
		sid, err := newSessionID()
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now()
		if err := a.createSession(httptest.NewRequest("POST", "/login", nil), int(id), marker+suffix, sid, now); err != nil {
			t.Fatal(err)
		}
		return id, "Bearer " + newLoginTokenResponse(int(id), now, now, sid)["accessToken"].(string)
	}
	creator, creatorToken := addUser("_creator")
	reviewer, reviewerToken := addUser("_reviewer")
	request := func(handler http.HandlerFunc, method, path string, body any, token string) (int, map[string]any) {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Authorization", token)
		rec := httptest.NewRecorder()
		handler(rec, req)
		var result map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return rec.Code, result
	}
	expectOK := func(code int, result map[string]any) {
		t.Helper()
		if code != 200 {
			t.Fatalf("HTTP %d: %v", code, result)
		}
	}
	res, err := db.Exec("insert into biz_ip_resources(name,ip_type,rights_owner) values (?,'游戏',?)", marker, marker)
	if err != nil {
		t.Fatal(err)
	}
	ipID, _ = res.LastInsertId()
	brief := ipTestBrief()
	brief["candidateEvaluations"] = map[string]any{strconv.FormatInt(ipID, 10): map[string]any{"reason": "契合", "productEvaluation": "适配", "technicalFit": "可开发"}}
	// Real authenticated local recipients; repeated marketing roles must deduplicate.
	contacts := map[string]any{}
	for _, key := range []string{"product", "ipMarketing", "imc", "brand", "user", "media", "pr"} {
		contacts[key] = map[string]any{"id": reviewer, "name": "测试评审"}
	}
	brief["contactsByStage"] = contacts
	payload := map[string]any{"projectName": marker, "department": "测试", "markets": []string{"中国"}, "goal": "新品发布", "description": "工作日时效验收", "budgetMin": 1, "budgetMax": 10, "brief": brief, "candidateIds": []int64{ipID}, "submit": false}
	code, result := request(a.ipRequestSave, "POST", "/", payload, creatorToken)
	expectOK(code, result)
	requestID = int64(result["data"].(map[string]any)["id"].(float64))
	var submitted any
	if err := db.QueryRow("select submitted_at from biz_ip_requests where id=?", requestID).Scan(&submitted); err != nil || submitted != nil {
		t.Fatalf("draft started clock: %v %v", submitted, err)
	}
	payload["id"] = requestID
	payload["submit"] = true
	code, result = request(a.ipRequestSave, "POST", "/", payload, creatorToken)
	expectOK(code, result)
	// Pin only this test's submission to a known holiday boundary. Editing unrelated fields must not move it.
	_, err = db.Exec("update biz_ip_requests set submitted_at='2026-09-30 15:00:00',description='edited after submission' where id=?", requestID)
	if err != nil {
		t.Fatal(err)
	}
	code, result = request(a.ipRequestDetail, "GET", "/?id="+strconv.FormatInt(requestID, 10), nil, creatorToken)
	expectOK(code, result)
	deadlines := result["data"].(map[string]any)["request"].(map[string]any)["deadlines"].(map[string]any)
	if deadlines["ip"].(map[string]any)["dueAt"] != "2026-10-20T23:59:59+08:00" {
		t.Fatalf("wrong actual DB deadline: %v", deadlines)
	}
	var stageNoticeCount int
	if err := db.QueryRow("select count(*) from biz_ip_workflow_notifications where request_id=? and stage='submitted' and user_id=?", requestID, reviewer).Scan(&stageNoticeCount); err != nil || stageNoticeCount != 1 {
		t.Fatalf("IP recipient count=%d err=%v", stageNoticeCount, err)
	}
	var before string
	if err := db.QueryRow("select date_format(submitted_at,'%Y-%m-%d %H:%i:%s') from biz_ip_requests where id=?", requestID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	assessment := map[string]any{"licenseContent": "形象", "licenseTerritory": "中国", "ipResources": "素材包", "brandResources": "渠道", "businessRisk": "低", "publicRisk": "低", "ownershipClarity": "清晰", "scoreDimensions": map[string]any{"audienceMatch": 90, "marketCoverage": 90, "scheduleAvailability": 80, "licenseRisk": 90}}
	feedback := map[string]any{"requestId": requestID, "saveDraft": true, "candidates": []any{map[string]any{"ipId": ipID, "priorityOrder": 0, "feasibility": "可行", "recommendation": "推荐", "reason": "契合", "assessment": assessment}}}
	code, result = request(a.ipRequestFeedback, "POST", "/", feedback, reviewerToken)
	expectOK(code, result)
	var reviewed any
	if err := db.QueryRow("select ip_reviewed_at from biz_ip_requests where id=?", requestID).Scan(&reviewed); err != nil || reviewed != nil {
		t.Fatalf("IP draft completed stage: %v %v", reviewed, err)
	}
	feedback["saveDraft"] = false
	code, result = request(a.ipRequestFeedback, "POST", "/", feedback, reviewerToken)
	expectOK(code, result)
	if err := db.QueryRow("select count(*) from biz_ip_workflow_notifications where request_id=? and stage='ip_reviewed' and user_id=?", requestID, reviewer).Scan(&stageNoticeCount); err != nil || stageNoticeCount != 1 {
		t.Fatalf("repeated marketing roles were not deduplicated: %d err=%v", stageNoticeCount, err)
	}
	_, err = db.Exec("update biz_ip_requests set ip_reviewed_at='2026-10-15 15:00:00' where id=?", requestID)
	if err != nil {
		t.Fatal(err)
	}
	opinion := map[string]any{}
	for _, key := range []string{"marketHeat", "fanAudience", "commercialValue", "marketingRisks", "recentTrend", "socialFollowers", "ugcForecast", "audienceMatch", "prOpinion"} {
		opinion[key] = "已评估"
	}
	marketing := map[string]any{"requestId": requestID, "saveDraft": true, "marketingComments": "齐全", "marketingProfile": map[string]any{"candidates": map[string]any{strconv.FormatInt(ipID, 10): opinion}}}
	code, result = request(a.ipRequestMarketing, "POST", "/", marketing, reviewerToken)
	expectOK(code, result)
	var count int
	if err := db.QueryRow("select count(*) from biz_ip_workflow_notifications where request_id=? and stage='marketing_reviewed'", requestID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("draft completion notification: %d %v", count, err)
	}
	// Missing a required opinion cannot complete or notify.
	marketing["saveDraft"] = false
	delete(opinion, "prOpinion")
	code, _ = request(a.ipRequestMarketing, "POST", "/", marketing, reviewerToken)
	if code != 400 {
		t.Fatalf("incomplete marketing HTTP %d", code)
	}
	opinion["prOpinion"] = "已评估"
	// Concurrent submit: exactly one transition succeeds and only the creator gets one completion notice.
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _ := request(a.ipRequestMarketing, "POST", "/", marketing, reviewerToken)
			statuses <- code
		}()
	}
	wg.Wait()
	close(statuses)
	ok, conflict := 0, 0
	for code := range statuses {
		if code == 200 {
			ok++
		}
		if code == 409 {
			conflict++
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("concurrent outcomes success=%d conflict=%d", ok, conflict)
	}
	if err := db.QueryRow("select count(*) from biz_ip_workflow_notifications where request_id=? and stage='marketing_reviewed' and user_id=?", requestID, creator).Scan(&count); err != nil || count != 1 {
		t.Fatalf("creator completion count=%d err=%v", count, err)
	}
	code, result = request(a.ipWorkflowNotifications, "GET", "/", nil, creatorToken)
	expectOK(code, result)
	items := result["data"].(map[string]any)["list"].([]any)
	if len(items) != 1 {
		t.Fatalf("creator notices: %v", items)
	}
	noticeID := int64(items[0].(map[string]any)["id"].(float64))
	code, result = request(a.ipWorkflowNotificationsRead, "POST", "/", map[string]any{"ids": []int64{noticeID}}, reviewerToken)
	expectOK(code, result)
	if result["data"].(map[string]any)["updated"] != float64(0) {
		t.Fatal("another user acknowledged creator notice")
	}
	code, result = request(a.ipWorkflowNotificationsRead, "POST", "/", map[string]any{"ids": []int64{noticeID}}, creatorToken)
	expectOK(code, result)
	code, result = request(a.ipWorkflowNotifications, "GET", "/", nil, creatorToken)
	expectOK(code, result)
	if result["data"].(map[string]any)["unread"] != float64(0) {
		t.Fatalf("acknowledgement not persisted: %v", result)
	}
	var after string
	if err := db.QueryRowContext(ctx, "select date_format(submitted_at,'%Y-%m-%d %H:%i:%s') from biz_ip_requests where id=?", requestID).Scan(&after); err != nil || after != before {
		t.Fatalf("submission clock changed: %s -> %s err=%v", before, after, err)
	}
	t.Log("PASS: draft clocks, holiday DDL, sequential stages, incomplete rejection, concurrent completion, owner-only notification, durable acknowledgement")
}
