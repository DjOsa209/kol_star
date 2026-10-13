package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestIPWeightedScore(t *testing.T) {
	assessment := map[string]any{"scoreDimensions": map[string]any{
		"audienceMatch":        float64(80),
		"marketCoverage":       float64(90),
		"scheduleAvailability": float64(60),
		"licenseRisk":          float64(100),
	}}
	got, valid := ipWeightedScore(assessment)
	if !valid || got != 82 {
		t.Fatalf("weighted score = %v, valid = %v; want 82, true", got, valid)
	}
	delete(assessment["scoreDimensions"].(map[string]any), "licenseRisk")
	if _, valid := ipWeightedScore(assessment); valid {
		t.Fatal("missing dimension must be invalid")
	}
	assessment["scoreDimensions"].(map[string]any)["licenseRisk"] = float64(101)
	if _, valid := ipWeightedScore(assessment); valid {
		t.Fatal("score above 100 must be invalid")
	}
}

func ipTestBrief() map[string]any {
	contacts := map[string]any{}
	for _, stage := range []string{"product", "ipMarketing", "imc", "brand", "user", "media", "pr"} {
		contacts[stage] = map[string]any{"id": 12, "name": "测试同事", "department": "品牌部"}
	}
	return map[string]any{
		"owner": "项目负责人", "projectPeriod": "2026年第四季度", "linkedProduct": "NOTE",
		"cooperationMode": "营销联动", "targetIPType": "动画", "targetIPCount": "1",
		"contactsByStage": contacts, "externalIPs": []any{map[string]any{
			"name": "库外测试IP", "rightsOwner": "版权方", "imageUrl": "https://example.com/ip-profile",
			"reason": "适合产品", "productEvaluation": "受众匹配", "technicalFit": "可落地",
		}},
	}
}

func TestIPBriefAndExternalOnlyValidation(t *testing.T) {
	brief := ipTestBrief()
	if !ipBriefValid(brief, nil) || !ipExternalValid(brief) {
		t.Fatal("complete external-only IP brief must be valid")
	}
	contacts := brief["contactsByStage"].(map[string]any)
	delete(contacts, "pr")
	if ipBriefValid(brief, nil) {
		t.Fatal("missing stage contact must be invalid")
	}
	delete(brief, "contactsByStage")
	brief["contacts"] = "历史草稿对接人"
	if !ipBriefValid(brief, nil) {
		t.Fatal("legacy contacts must remain editable")
	}
	external := brief["externalIPs"].([]any)[0].(map[string]any)
	external["imageUrl"] = "javascript:alert(1)"
	if ipExternalValid(brief) {
		t.Fatal("external IP must provide a valid http(s) display URL")
	}
	external["imageUrl"] = "https://example.com/profile"
	brief["candidateEvaluations"] = map[string]any{"91": map[string]any{"reason": "理由"}}
	if ipBriefValid(brief, []int64{91}) {
		t.Fatal("selected library IP must have all three evaluations")
	}
}

func TestIPImportOptionsAndDisplayFilename(t *testing.T) {
	row := ipResourceInput{IPType: "动画", Markets: []string{"南亚"}, CooperationStatus: "可合作"}
	if issue := ipImportOptionIssue(row); issue != "" {
		t.Fatalf("new options rejected: %s", issue)
	}
	row.IPType = "动画/影视"
	if issue := ipImportOptionIssue(row); issue != "" {
		t.Fatalf("historical type rejected: %s", issue)
	}
	row.IPType = "未知类型"
	if issue := ipImportOptionIssue(row); !strings.Contains(issue, "IP类型") {
		t.Fatalf("invalid type issue = %q", issue)
	}
	row.IPType, row.Markets = "动画", []string{"美国"}
	if issue := ipImportOptionIssue(row); issue != "" {
		t.Fatalf("custom market rejected: %q", issue)
	}
	row.Markets = []string{"   "}
	if issue := ipImportOptionIssue(row); !strings.Contains(issue, "覆盖市场") {
		t.Fatalf("blank market issue = %q", issue)
	}
	row.Markets, row.CooperationStatus = []string{"南亚"}, "未知状态"
	if issue := ipImportOptionIssue(row); !strings.Contains(issue, "合作状态") {
		t.Fatalf("invalid status issue = %q", issue)
	}
	for _, name := range []string{"版权介绍.pdf", "Brand Brief 2026.pdf"} {
		if !validIPDisplayFilename(name, ".pdf") {
			t.Fatalf("safe filename rejected: %q", name)
		}
	}
	for _, name := range []string{"../版权介绍.pdf", `..\版权介绍.pdf`, "版权介绍.png", "\n.pdf", ".pdf", " 版权介绍.pdf"} {
		if validIPDisplayFilename(name, ".pdf") {
			t.Fatalf("unsafe or type-changing filename accepted: %q", name)
		}
	}
}

func ipTestPost(t *testing.T, a *app, handler http.HandlerFunc, payload any) (int, map[string]any) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	startedAt := time.Now().Truncate(time.Second)
	req.Header.Set("Authorization", "Bearer "+newLoginTokenResponse(1, startedAt, startedAt, strings.Repeat("b", 64))["accessToken"].(string))
	rec := httptest.NewRecorder()
	handler(rec, req)
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return rec.Code, response
}

func expectIPTestSession(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`select s.last_seen_unix from sys_sessions`).
		WithArgs(sessionHash(strings.Repeat("b", 64)), 1, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"last_seen_unix"}).AddRow(time.Now().Unix()))
}

func TestIPExternalOnlySubmissionCanEnterFeedbackAndMarketingFlow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	expectIPTestSession(mock)
	mock.ExpectBegin()
	mock.ExpectExec("insert into biz_ip_requests").WillReturnResult(sqlmock.NewResult(81, 1))
	mock.ExpectCommit()
	status, response := ipTestPost(t, a, a.ipRequestSave, map[string]any{
		"projectName": "库外IP需求", "department": "营销", "markets": []string{"南亚"},
		"goal": "新品推广", "description": "寻找动画IP", "budgetCurrency": "USD",
		"budgetMin": 1000, "budgetMax": 10000, "candidateIds": []int{},
		"brief": ipTestBrief(), "submit": true,
	})
	if status != http.StatusOK || response["code"] != float64(0) || response["data"].(map[string]any)["status"] != "submitted" {
		t.Fatalf("external-only submission: status=%d response=%v", status, response)
	}

	// The IP group may associate a library candidate after reviewing the external suggestion.
	// It is not automatically imported as a resource when the demand is submitted.
	mock.ExpectBegin()
	mock.ExpectQuery("select status from biz_ip_requests").WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("submitted"))
	mock.ExpectExec("insert into biz_ip_request_candidates").WillReturnResult(sqlmock.NewResult(12, 1))
	mock.ExpectExec("update biz_ip_requests set status='ip_reviewed'").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	assessment := map[string]any{
		"licenseContent": "形象", "licenseTerritory": "南亚", "ipResources": "素材包", "brandResources": "渠道",
		"businessRisk": "低", "publicRisk": "低", "ownershipClarity": "清晰",
		"scoreDimensions": map[string]any{"audienceMatch": 90, "marketCoverage": 90, "scheduleAvailability": 80, "licenseRisk": 90},
	}
	status, response = ipTestPost(t, a, a.ipRequestFeedback, map[string]any{
		"requestId": 81, "candidates": []any{map[string]any{
			"ipId": 91, "priorityOrder": 1, "feasibility": "可行", "recommendation": "推荐",
			"reason": "匹配", "assessment": assessment,
		}},
	})
	if status != http.StatusOK || response["code"] != float64(0) {
		t.Fatalf("IP feedback: status=%d response=%v", status, response)
	}

	mock.ExpectQuery("select ip_id from biz_ip_request_candidates").WillReturnRows(sqlmock.NewRows([]string{"ip_id"}).AddRow(91))
	mock.ExpectExec("update biz_ip_requests set market_heat=").WillReturnResult(sqlmock.NewResult(0, 1))
	opinion := map[string]any{}
	for _, key := range []string{"marketHeat", "fanAudience", "commercialValue", "marketingRisks", "recentTrend", "socialFollowers", "ugcForecast", "audienceMatch", "prOpinion"} {
		opinion[key] = "已评估"
	}
	status, response = ipTestPost(t, a, a.ipRequestMarketing, map[string]any{
		"requestId": 81, "marketingComments": "营销同意", "marketingProfile": map[string]any{
			"candidates": map[string]any{"91": opinion},
		},
	})
	if status != http.StatusOK || response["code"] != float64(0) {
		t.Fatalf("marketing opinion: status=%d response=%v", status, response)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPDraftSubmissionRejectsLostStatusTransition(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	expectIPTestSession(mock)
	mock.ExpectBegin()
	mock.ExpectQuery("select count\\(\\*\\) from biz_ip_requests where id=\\? and status='draft'").
		WithArgs(int64(81), 1).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("update biz_ip_requests set project_name=").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	status, response := ipTestPost(t, a, a.ipRequestSave, map[string]any{
		"id": 81, "projectName": "库外IP需求", "department": "营销", "markets": []string{"南亚"},
		"goal": "新品推广", "description": "寻找动画IP", "budgetCurrency": "USD",
		"budgetMin": 1000, "budgetMax": 10000, "candidateIds": []int{},
		"brief": ipTestBrief(), "submit": true,
	})
	if status != http.StatusConflict || response["code"] != float64(409) {
		t.Fatalf("lost draft transition: status=%d response=%v", status, response)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPFeedbackAllowsMultipleIPsAtSameTier(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectBegin()
	mock.ExpectQuery("select status from biz_ip_requests").WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("submitted"))
	mock.ExpectExec("insert into biz_ip_request_candidates").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into biz_ip_request_candidates").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec("update biz_ip_requests set status='ip_reviewed'").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	assessment := map[string]any{
		"licenseContent": "形象", "licenseTerritory": "南亚", "ipResources": "素材包", "brandResources": "渠道",
		"businessRisk": "低", "publicRisk": "低", "ownershipClarity": "清晰",
		"scoreDimensions": map[string]any{"audienceMatch": 90, "marketCoverage": 90, "scheduleAvailability": 80, "licenseRisk": 90},
	}
	status, response := ipTestPost(t, a, a.ipRequestFeedback, map[string]any{
		"requestId": 81, "candidates": []any{
			map[string]any{"ipId": 91, "priorityOrder": 0, "feasibility": "可行", "recommendation": "推荐", "reason": "匹配", "assessment": assessment},
			map[string]any{"ipId": 92, "priorityOrder": 0, "feasibility": "可行", "recommendation": "推荐", "reason": "匹配", "assessment": assessment},
		},
	})
	if status != http.StatusOK || response["code"] != float64(0) {
		t.Fatalf("shared T0 feedback: status=%d response=%v", status, response)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPRequestListIncludesCompletedFeedback(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectQuery("select count\\(\\*\\) from biz_ip_requests.*status in").
		WithArgs("submitted", "ip_reviewed", "marketing_reviewed").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery("select id, project_name as projectName").
		WithArgs("submitted", "ip_reviewed", "marketing_reviewed", 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "projectName", "status"}).
			AddRow(1, "待反馈需求", "submitted").AddRow(2, "已反馈需求", "ip_reviewed"))
	status, response := ipTestPost(t, a, a.ipRequestsList, map[string]any{
		"statuses": []string{"submitted", "ip_reviewed", "marketing_reviewed"},
		"page":     1, "pageSize": 20,
	})
	if status != http.StatusOK || response["code"] != float64(0) {
		t.Fatalf("feedback list: status=%d response=%v", status, response)
	}
	data := response["data"].(map[string]any)
	if data["total"] != float64(2) || len(data["list"].([]any)) != 2 {
		t.Fatalf("completed feedback missing from list: %v", data)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPLegacyRecommendationAloneCannotSubmitNewDemand(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	brief := ipTestBrief()
	delete(brief, "externalIPs")
	status, response := ipTestPost(t, a, a.ipRequestSave, map[string]any{
		"projectName": "旧推荐文本", "department": "营销", "markets": []string{"南亚"},
		"goal": "新品推广", "description": "寻找动画IP", "budgetMin": 1000, "budgetMax": 10000,
		"externalRecommendation": "任意库外推荐名称", "candidateIds": []int{},
		"brief": brief, "submit": true,
	})
	if status != http.StatusBadRequest || !strings.Contains(response["message"].(string), "库外IP意向") {
		t.Fatalf("legacy text must not be sole intention: status=%d response=%v", status, response)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPFileRenameKeepsExtension(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectQuery("select original_name from biz_ip_files").WithArgs(int64(11)).WillReturnRows(sqlmock.NewRows([]string{"original_name"}).AddRow("旧文件.pdf"))
	mock.ExpectExec("update biz_ip_files set original_name=").WithArgs("新文件.pdf", int64(11)).WillReturnResult(sqlmock.NewResult(0, 1))
	status, response := ipTestPost(t, a, a.ipFileRename, map[string]any{"id": 11, "originalName": "新文件.pdf"})
	if status != http.StatusOK || response["data"].(map[string]any)["originalName"] != "新文件.pdf" {
		t.Fatalf("rename status=%d response=%v", status, response)
	}
	mock.ExpectQuery("select original_name from biz_ip_files").WithArgs(int64(11)).WillReturnRows(sqlmock.NewRows([]string{"original_name"}).AddRow("新文件.pdf"))
	status, _ = ipTestPost(t, a, a.ipFileRename, map[string]any{"id": 11, "originalName": "恶意文件.png"})
	if status != http.StatusBadRequest {
		t.Fatalf("extension change status=%d, want 400", status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPImportPreviewReportsInvalidOptionsByRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	status, response := ipTestPost(t, a, a.ipImportPreview, map[string]any{"rows": []any{
		map[string]any{"name": "测试IP", "ipType": "未知类型", "markets": []string{"南亚"}},
		map[string]any{"name": "测试IP2", "ipType": "动画", "markets": []string{"   "}},
		map[string]any{"name": "测试IP3", "ipType": "动画", "markets": []string{"南亚"}, "cooperationStatus": "未知状态"},
	}})
	if status != http.StatusOK {
		t.Fatalf("preview status=%d response=%v", status, response)
	}
	rows := response["data"].([]any)
	for index, row := range rows {
		result := row.(map[string]any)
		if result["row"] != float64(index+2) || result["status"] != "error" || result["message"] == "" {
			t.Fatalf("row %d validation=%v", index+2, result)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPColleaguesSearchUsesActiveSSOUsers(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectQuery("from sys_users u left join sys_departments d.*where u.status=1 and u.auth_provider='uac'").
		WithArgs("%张%", "%张%", "%张%", "%张%", 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "department", "employeeNo", "notifiable"}).AddRow(7, "张三", "IP营销", "1001", 1))
	req := httptest.NewRequest(http.MethodGet, "/business/ip/colleagues?q=张", nil)
	rec := httptest.NewRecorder()
	a.ipColleagues(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("colleague search status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	data := response["data"].([]any)
	if len(data) != 1 || data[0].(map[string]any)["name"] != "张三" {
		t.Fatalf("unexpected colleagues: %v", data)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
