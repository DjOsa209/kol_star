package main

import (
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func TestIPMarketingNotificationFailureRollsBackCompletion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectBegin()
	mock.ExpectQuery("select status,project_name").WillReturnRows(sqlmock.NewRows([]string{"status", "project_name", "brief", "created_by"}).AddRow("ip_reviewed", "新品", "{}", 9))
	mock.ExpectQuery("select ip_id from biz_ip_request_candidates").WillReturnRows(sqlmock.NewRows([]string{"ip_id"}).AddRow(91))
	mock.ExpectExec("update biz_ip_requests set market_heat=").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("insert ignore into biz_ip_workflow_notifications").WithArgs(int64(81), "marketing_reviewed", sqlmock.AnyArg(), int64(9)).WillReturnError(errors.New("notification storage unavailable"))
	mock.ExpectRollback()
	opinion := map[string]any{}
	for _, key := range []string{"marketHeat", "fanAudience", "commercialValue", "marketingRisks", "recentTrend", "socialFollowers", "ugcForecast", "audienceMatch", "prOpinion"} {
		opinion[key] = "已评估"
	}
	code, _ := ipTestPost(t, a, a.ipRequestMarketing, map[string]any{"requestId": 81, "marketingComments": "齐全", "marketingProfile": map[string]any{"candidates": map[string]any{"91": opinion}}})
	if code != 500 {
		t.Fatalf("HTTP %d", code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPMarketingDraftDoesNotFinishOrNotify(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectBegin()
	mock.ExpectQuery("select status,project_name").WillReturnRows(sqlmock.NewRows([]string{"status", "project_name", "brief", "created_by"}).AddRow("ip_reviewed", "新品", "{}", 9))
	mock.ExpectExec("update biz_ip_requests set market_heat=").WithArgs("", "", "", "", "null", "", "null", "ip_reviewed", nil, int64(81)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	code, result := ipTestPost(t, a, a.ipRequestMarketing, map[string]any{"requestId": 81, "saveDraft": true})
	if code != 200 {
		t.Fatalf("HTTP %d: %v", code, result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPFeedbackPartialCandidatesCannotFinishStage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectBegin()
	mock.ExpectQuery("select status,project_name").WillReturnRows(sqlmock.NewRows([]string{"status", "project_name", "brief", "created_by"}).AddRow("submitted", "新品", "{}", 9))
	mock.ExpectExec("insert into biz_ip_request_candidates").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("select ip_id from biz_ip_request_candidates").WillReturnRows(sqlmock.NewRows([]string{"ip_id"}).AddRow(91).AddRow(92))
	mock.ExpectRollback()
	assessment := map[string]any{"licenseContent": "形象", "licenseTerritory": "中国", "ipResources": "素材包", "brandResources": "渠道", "businessRisk": "低", "publicRisk": "低", "ownershipClarity": "清晰", "scoreDimensions": map[string]any{"audienceMatch": 90, "marketCoverage": 90, "scheduleAvailability": 80, "licenseRisk": 90}}
	code, _ := ipTestPost(t, a, a.ipRequestFeedback, map[string]any{"requestId": 81, "candidates": []any{map[string]any{"ipId": 91, "priorityOrder": 0, "feasibility": "可行", "recommendation": "推荐", "reason": "契合", "assessment": assessment}}})
	if code != 400 {
		t.Fatalf("partial IP assessment HTTP %d", code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
