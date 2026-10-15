package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func marketingTestInput(key string) ipMarketingInput {
	function, _ := ipMarketingFunctionByKey(key)
	values := map[string]string{}
	for _, field := range function.Required {
		values[field] = "已评估"
	}
	return ipMarketingInput{FunctionKey: key, Comments: "本职能意见", Candidates: map[string]map[string]string{"91": values}}
}
func TestIPMarketingFunctionsValidateIndependently(t *testing.T) {
	profile := map[string]any{"legacyKey": "保留", "candidates": map[string]any{"91": map[string]any{"legacyField": "原始资料"}}}
	now := time.Now()
	for index, key := range []string{"pr", "media", "user", "brand", "imc"} {
		var message string
		profile, message = mergeIPMarketingOpinion(profile, marketingTestInput(key), []int64{91}, index+1, now)
		if message != "" {
			t.Fatalf("%s requires a colleague's content: %s", key, message)
		}
		if ipMarketingAllSubmitted(profile) != (index == 4) {
			t.Fatalf("premature completion after %s", key)
		}
	}
	candidates := profile["candidates"].(map[string]any)["91"].(map[string]any)
	if candidates["legacyField"] != "原始资料" || profile["legacyKey"] != "保留" {
		t.Fatal("legacy data lost")
	}
	for _, function := range ipMarketingFunctions {
		for _, field := range function.Required {
			if candidates[field] != "已评估" {
				t.Fatalf("lost %s's %s", function.Key, field)
			}
		}
	}
	// CPM remains optional; media still has to submit its own actual written opinion.
	if ipMarketingState(profile, "media").SubmittedAt == "" {
		t.Fatal("media could not submit without an optional CPM number")
	}
}
func TestIPMarketingRejectsForeignFieldsMissingOwnFieldsAndUnknownIPs(t *testing.T) {
	for _, mutate := range []func(*ipMarketingInput){
		func(i *ipMarketingInput) { i.FunctionKey = "unknown" },
		func(i *ipMarketingInput) { i.Comments = "  " },
		func(i *ipMarketingInput) { i.Candidates["91"]["prOpinion"] = "替别人填写" },
		func(i *ipMarketingInput) { i.Candidates["999"] = map[string]string{"ugcForecast": "未知IP"} },
		func(i *ipMarketingInput) { i.Candidates = nil },
	} {
		input := marketingTestInput("brand")
		mutate(&input)
		if _, message := mergeIPMarketingOpinion(map[string]any{}, input, []int64{91}, 1, time.Now()); message == "" {
			t.Fatalf("accepted invalid input: %+v", input)
		}
	}
	input := marketingTestInput("brand")
	if _, message := mergeIPMarketingOpinion(map[string]any{}, input, []int64{91, 92}, 1, time.Now()); message == "" {
		t.Fatal("accepted incomplete own opinion for second IP")
	}
}
func TestIPMarketingDraftVersionAndSubmissionProtectColleagues(t *testing.T) {
	profile := map[string]any{}
	input := marketingTestInput("brand")
	input.SaveDraft = true
	input.Candidates = nil
	input.Comments = ""
	profile, message := mergeIPMarketingOpinion(profile, input, []int64{91}, 1, time.Now())
	if message != "" {
		t.Fatal(message)
	}
	state := ipMarketingState(profile, "brand")
	if state.Version != 1 || state.SubmittedAt != "" {
		t.Fatalf("draft counted as submitted: %+v", state)
	}
	if _, message := mergeIPMarketingOpinion(profile, input, []int64{91}, 2, time.Now()); message == "" {
		t.Fatal("stale same-function page silently overwrote current draft")
	}
	input = marketingTestInput("brand")
	input.Version = 1
	profile, message = mergeIPMarketingOpinion(profile, input, []int64{91}, 1, time.Now())
	if message != "" {
		t.Fatal(message)
	}
	if _, message := mergeIPMarketingOpinion(profile, input, []int64{91}, 2, time.Now()); message == "" {
		t.Fatal("submitted function editable by stale page")
	}
}
func TestIPMarketingDatabaseFailureRollsBackFunctionAndCompletion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	profile := map[string]any{}
	for _, key := range []string{"imc", "brand", "user", "media"} {
		profile, _ = mergeIPMarketingOpinion(profile, marketingTestInput(key), []int64{91}, 2, time.Now())
	}
	raw, _ := json.Marshal(profile)
	expectIPTestSession(mock)
	mock.ExpectBegin()
	mock.ExpectQuery("select status,coalesce").WillReturnRows(sqlmock.NewRows([]string{"status", "profile", "comments", "channels", "heat", "audience", "value", "risks"}).AddRow("ip_reviewed", string(raw), "", "[]", "", "", "", ""))
	mock.ExpectQuery("select ip_id from biz_ip_request_candidates").WillReturnRows(sqlmock.NewRows([]string{"ip_id"}).AddRow(91))
	mock.ExpectExec("update biz_ip_requests set market_heat=").WillReturnError(errors.New("storage unavailable"))
	mock.ExpectRollback()
	input := marketingTestInput("pr")
	input.RequestID = 81
	code, _ := ipTestPost(t, a, a.ipRequestMarketing, input)
	if code != 500 {
		t.Fatalf("HTTP %d", code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestIPMarketingCompletionFeishuRecipientIsCreatorOnly(t *testing.T) {
	brief := map[string]any{"contactsByStage": map[string]any{"product": map[string]any{"id": 12}, "imc": map[string]any{"id": 13}}}
	ids := ipStageContactIDs(brief, 99, "marketing_reviewed")
	if len(ids) != 1 || ids[0] != 99 {
		t.Fatalf("completion recipients=%v want creator only", ids)
	}
	if recipients := ipStageDirectoryRecipients(brief, "marketing_reviewed"); len(recipients) != 0 {
		t.Fatal("completion unexpectedly notifies product contact")
	}
}

func TestIPMarketingCompletionUsesFeishuAndOnlyCreator(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := Config{Feishu: FeishuConfig{ApplicationEnabled: true, AppID: "mock-app", AppSecret: "mock-secret", FrontendURL: "https://xmp.example.com/"}}
	a := newApp(db, cfg)
	brief := ipJSON(map[string]any{"contactsByStage": map[string]any{"product": map[string]any{"id": 12}, "pr": map[string]any{"id": 13}}})
	mock.ExpectQuery("select project_name").WithArgs(int64(81)).WillReturnRows(sqlmock.NewRows([]string{"project_name", "brief", "created_by", "submitted_at", "ip_reviewed_at"}).AddRow("职能协作测试", brief, 99, "2026-10-15 10:00:00", "2026-10-15 11:00:00"))
	mock.ExpectQuery("select coalesce").WithArgs(int64(99)).WillReturnRows(sqlmock.NewRows([]string{"email", "status", "auth_provider"}).AddRow("creator@example.com", 1, "local"))
	sender := &mockIPStageSender{}
	result, err := a.sendIPStageNotifications(context.Background(), cfg, 81, "marketing_reviewed", sender)
	if err != nil || result.Sent != 1 || len(sender.emails) != 1 || sender.emails[0] != "creator@example.com" {
		t.Fatalf("Feishu completion result=%+v err=%v recipients=%v", result, err, sender.emails)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
