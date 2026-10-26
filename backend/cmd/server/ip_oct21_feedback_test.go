package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestIPCooperationFormsValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		profile map[string]any
		valid   bool
	}{
		{"legacy", nil, true},
		{"clear", map[string]any{"cooperationForms": []string{}}, true},
		{"multiple", map[string]any{"cooperationForms": []string{"产品定制", "功能联动", "联名周边礼盒", "线上营销", "线下物料及互动"}}, true},
		{"unknown", map[string]any{"cooperationForms": []string{"unlicensed"}}, false},
		{"duplicate", map[string]any{"cooperationForms": []string{"线上营销", "线上营销"}}, false},
		{"string", map[string]any{"cooperationForms": "线上营销"}, false},
		{"null", map[string]any{"cooperationForms": nil}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ipCooperationFormsIssue(tc.profile); (got == "") != tc.valid {
				t.Fatalf("issue=%q", got)
			}
			row := ipResourceInput{IPType: "动画", Markets: []string{"全球"}, Profile: tc.profile}
			if (ipImportOptionIssue(row) == "") != tc.valid {
				t.Fatal("import and manual validation differ")
			}
		})
	}
}

func TestIPCooperationFormsSaveAndClear(t *testing.T) {
	for _, forms := range [][]string{{"产品定制", "线上营销"}, {}} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		a := newApp(db, Config{})
		expectIPTestSession(mock)
		mock.ExpectQuery(`select markets from biz_ip_resources where id=\?`).WithArgs(int64(9)).WillReturnRows(sqlmock.NewRows([]string{"markets"}).AddRow(`["全球"]`))
		links := ""
		if len(forms) > 0 {
			links = "https://example.com/assets#hero\nhttps://example.com/poses?version=2"
		}
		profile := map[string]any{"cooperationForms": forms, "lifecycle": "常青", "assetLinks": links}
		mock.ExpectExec(`update biz_ip_resources set`).WithArgs("测试IP", "动画", "", "", `["全球"]`, nil, "", "", "待评估", "CNY", nil, nil, "", ipJSON(profile), int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
		status, response := ipTestPost(t, a, a.ipResourceSave, map[string]any{"id": 9, "name": "测试IP", "ipType": "动画", "markets": []string{"全球"}, "profile": profile})
		if status != http.StatusOK || response["code"] != float64(0) {
			t.Fatalf("status=%d response=%v", status, response)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}

func TestIPRequestOptionalContactAndTranslationExclusion(t *testing.T) {
	brief := ipTestBrief()
	if issue := ipRequestContactIssue(brief); issue != "" {
		t.Fatal(issue)
	}
	brief["requesterPIC"] = "测试发起人"
	external := brief["externalIPs"].([]any)[0].(map[string]any)
	external["contact"] = "测试联系人 @example"
	if !ipExternalValid(brief) || ipRequestContactIssue(brief) != "" {
		t.Fatal("optional contact rejected")
	}
	for _, source := range ipContentSources("ip_request", map[string]any{"brief": brief}) {
		if source.FieldKey == "brief.requesterPIC" || strings.HasSuffix(source.FieldKey, ".contact") {
			t.Fatal("contact sent for translation")
		}
	}
	external["contact"] = ""
	if !ipExternalValid(brief) || ipRequestContactIssue(brief) != "" {
		t.Fatal("empty optional contact rejected")
	}
	external["contact"] = strings.Repeat("联", 501)
	if ipRequestContactIssue(brief) == "" {
		t.Fatal("oversized contact accepted")
	}
	external["contact"] = ""
	brief["requesterPIC"] = 42
	if ipRequestContactIssue(brief) == "" {
		t.Fatal("non-text PIC accepted")
	}
}
