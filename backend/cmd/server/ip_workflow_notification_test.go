package main

import (
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func TestIPFeedbackPartialCandidatesCannotFinishStage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	mock.ExpectBegin()
	mock.ExpectQuery("select status from biz_ip_requests").WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("submitted"))
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
