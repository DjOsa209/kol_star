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
	_, creatorToken := addUser("_creator")
	reviewer, reviewerToken := addUser("_reviewer")
	partners := map[string]struct {
		id    int64
		token string
	}{}
	for _, function := range ipMarketingFunctions {
		id, token := addUser("_" + function.Key)
		partners[function.Key] = struct {
			id    int64
			token string
		}{id, token}
	}
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
	_, err = db.Exec("update biz_ip_requests set ip_reviewed_at='2026-10-15 15:00:00' where id=?", requestID)
	if err != nil {
		t.Fatal(err)
	}
	makeOpinion := func(key string, draft bool, version int64) ipMarketingInput {
		function, _ := ipMarketingFunctionByKey(key)
		fields := map[string]string{}
		for _, field := range function.Required {
			fields[field] = function.Label + "已评估"
		}
		return ipMarketingInput{RequestID: requestID, FunctionKey: key, Version: version, SaveDraft: draft, Comments: function.Label + "的独立意见", Candidates: map[string]map[string]string{strconv.FormatInt(ipID, 10): fields}}
	}
	imc := makeOpinion("imc", true, 0)
	code, result = request(a.ipRequestMarketing, "POST", "/", imc, partners["imc"].token)
	expectOK(code, result)
	var completed any
	if err := db.QueryRow("select marketing_reviewed_at from biz_ip_requests where id=?", requestID).Scan(&completed); err != nil || completed != nil {
		t.Fatalf("draft prematurely completed: %v %v", completed, err)
	}
	imc.SaveDraft = false
	imc.Version = 1
	code, result = request(a.ipRequestMarketing, "POST", "/", imc, partners["imc"].token)
	expectOK(code, result)
	if result["data"].(map[string]any)["status"] != "ip_reviewed" {
		t.Fatal("IMC alone completed marketing")
	}
	// Three colleagues submit different functions concurrently, from their own stale page snapshots.
	var wg sync.WaitGroup
	statuses := make(chan int, 3)
	for _, key := range []string{"brand", "user", "media"} {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			code, _ := request(a.ipRequestMarketing, "POST", "/", makeOpinion(key, false, 0), partners[key].token)
			statuses <- code
		}(key)
	}
	wg.Wait()
	close(statuses)
	for code := range statuses {
		if code != 200 {
			t.Fatalf("concurrent independent function HTTP %d", code)
		}
	}
	var raw, statusNow string
	if err := db.QueryRow("select marketing_profile,status,marketing_reviewed_at from biz_ip_requests where id=?", requestID).Scan(&raw, &statusNow, &completed); err != nil {
		t.Fatal(err)
	}
	var profile map[string]any
	if err := json.Unmarshal([]byte(raw), &profile); err != nil {
		t.Fatal(err)
	}
	if statusNow != "ip_reviewed" || completed != nil {
		t.Fatalf("completed without PR: %s %v", statusNow, completed)
	}
	for _, key := range []string{"imc", "brand", "user", "media"} {
		state := ipMarketingState(profile, key)
		if state.SubmittedBy != int(partners[key].id) || state.SubmittedAt == "" {
			t.Fatalf("lost independent %s state: %+v", key, state)
		}
	}
	values := profile["candidates"].(map[string]any)[strconv.FormatInt(ipID, 10)].(map[string]any)
	if values["marketHeat"] != "IMC已评估" || values["ugcForecast"] != "品牌运营已评估" || values["audienceMatch"] != "用户运营已评估" {
		t.Fatalf("concurrent writes clobbered colleagues: %v", values)
	}
	code, _ = request(a.ipRequestMarketing, "POST", "/", makeOpinion("imc", true, 0), partners["imc"].token)
	if code != 409 {
		t.Fatalf("stale submitted function HTTP %d", code)
	}
	missing := makeOpinion("pr", false, 0)
	missing.Candidates = nil
	code, _ = request(a.ipRequestMarketing, "POST", "/", missing, partners["pr"].token)
	if code != 400 {
		t.Fatalf("missing own PR opinion HTTP %d", code)
	}
	// Only the fifth completed function can transition the request, even if double-clicked.
	finalStatuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _ := request(a.ipRequestMarketing, "POST", "/", makeOpinion("pr", false, 0), partners["pr"].token)
			finalStatuses <- code
		}()
	}
	wg.Wait()
	close(finalStatuses)
	successes, conflicts := 0, 0
	for code := range finalStatuses {
		if code == 200 {
			successes++
		}
		if code == 409 {
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("completion success=%d conflict=%d", successes, conflicts)
	}
	if err := db.QueryRow("select status,marketing_reviewed_at,marketing_profile from biz_ip_requests where id=?", requestID).Scan(&statusNow, &completed, &raw); err != nil {
		t.Fatal(err)
	}
	json.Unmarshal([]byte(raw), &profile)
	if statusNow != "marketing_reviewed" || completed == nil || !ipMarketingAllSubmitted(profile) {
		t.Fatalf("final completion missing: %s %v", statusNow, completed)
	}
	var stationNotices int
	if err := db.QueryRow("select count(*) from biz_ip_workflow_notifications where request_id=?", requestID).Scan(&stationNotices); err != nil || stationNotices != 0 {
		t.Fatalf("unexpected station reminder: %d %v", stationNotices, err)
	}
	var after string
	if err := db.QueryRowContext(ctx, "select date_format(submitted_at,'%Y-%m-%d %H:%i:%s') from biz_ip_requests where id=?", requestID).Scan(&after); err != nil || after != before {
		t.Fatalf("submission clock changed: %s -> %s err=%v", before, after, err)
	}
	t.Log("PASS: working-day DDL, five independent colleagues, scoped validation, concurrent merge, stale-write rejection, final-only completion and no station popup")
}
