package main

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestIPRequestManagementCreatorAtEveryStage(t *testing.T) {
	for _, status := range []string{"draft", "submitted", "ip_reviewed", "marketing_reviewed"} {
		for _, userID := range []int{0, 1, 2} {
			row := map[string]any{"status": status, "createdBy": "1", "feedbackStarted": "0"}
			ipRequestManagementActions(row, userID)
			if row["canEdit"] != (userID == 1) || row["canDelete"] != (userID == 1) {
				t.Fatalf("management mismatch stage=%s user=%d", status, userID)
			}
		}
	}
}

func TestIPRequestEditPreservesCompletedWorkflow(t *testing.T) {
	for _, stage := range []string{"submitted", "ip_reviewed", "marketing_reviewed"} {
		t.Run(stage, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			a := newApp(db, Config{})
			brief := ipTestBrief()
			brief["notificationLanguage"] = "zh"
			brief["candidateEvaluations"] = map[string]any{"91": map[string]any{"reason": "原选择理由"}}
			expectIPTestSession(mock)
			mock.ExpectBegin()
			mock.ExpectQuery("select status,created_by,date_format").WithArgs(int64(81)).WillReturnRows(sqlmock.NewRows([]string{"status", "created_by", "submitted_at", "brief", "feedbackStarted"}).AddRow(stage, 1, "2026-09-30 15:00:00", ipJSON(brief), true))
			mock.ExpectQuery("select ip_id from biz_ip_request_candidates").WithArgs(int64(81)).WillReturnRows(sqlmock.NewRows([]string{"ip_id"}).AddRow(91))
			mock.ExpectExec("update biz_ip_requests set project_name=").WithArgs("修改后的项目", "营销", ipJSON([]string{"中国"}), nil, "新品发布", "更新说明", "CNY", 1.0, 10.0, "", ipJSON(brief), stage, "2026-09-30 15:00:00", int64(81), 1).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			payload := map[string]any{"id": 81, "projectName": "修改后的项目", "department": "营销", "markets": []string{"中国"}, "goal": "新品发布", "description": "更新说明", "budgetMin": 1, "budgetMax": 10, "candidateIds": []int{91}, "brief": brief, "submit": true}
			code, response := ipTestPost(t, a, a.ipRequestSave, payload)
			if code != 200 || response["data"].(map[string]any)["status"] != stage {
				t.Fatalf("edit HTTP %d: %v", code, response)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIPRequestEditCannotOverwriteReviewersCandidates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, Config{})
	expectIPTestSession(mock)
	mock.ExpectBegin()
	mock.ExpectQuery("select status,created_by,date_format").WillReturnRows(sqlmock.NewRows([]string{"status", "created_by", "submitted_at", "brief", "feedbackStarted"}).AddRow("ip_reviewed", 1, nil, "{}", true))
	mock.ExpectQuery("select ip_id from biz_ip_request_candidates").WillReturnRows(sqlmock.NewRows([]string{"ip_id"}).AddRow(91))
	mock.ExpectRollback()
	code, _ := ipTestPost(t, a, a.ipRequestSave, map[string]any{"id": 81, "projectName": "项目", "department": "营销", "markets": []string{"中国"}, "goal": "新品发布", "description": "说明", "budgetMin": 1, "budgetMax": 10, "brief": ipTestBrief(), "candidateIds": []int{92}, "submit": true})
	if code != 409 {
		t.Fatalf("candidate removal HTTP %d", code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIPRequestDeletionAuthorizationAndAtomicity(t *testing.T) {
	for _, scenario := range []string{"success", "other_creator", "missing", "cleanup_failure", "commit_failure"} {
		t.Run(scenario, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			a := newApp(db, Config{})
			expectIPTestSession(mock)
			mock.ExpectBegin()
			query := mock.ExpectQuery("select created_by from biz_ip_requests where id=\\? for update").WithArgs(int64(81))
			want := 200
			switch scenario {
			case "missing":
				query.WillReturnError(sql.ErrNoRows)
				mock.ExpectRollback()
				want = 404
			case "other_creator":
				query.WillReturnRows(sqlmock.NewRows([]string{"created_by"}).AddRow(2))
				mock.ExpectRollback()
				want = 403
			default:
				query.WillReturnRows(sqlmock.NewRows([]string{"created_by"}).AddRow(1))
				mock.ExpectExec("update biz_recommendation_subscriptions set request_id=null,active=0").WithArgs(int64(81)).WillReturnResult(sqlmock.NewResult(0, 2))
				if scenario == "cleanup_failure" {
					mock.ExpectExec("delete from biz_ip_workflow_notifications").WillReturnError(errors.New("storage unavailable"))
					mock.ExpectRollback()
					want = 500
				} else {
					for _, query := range []string{"delete from biz_ip_workflow_notifications", "delete from biz_ip_request_candidates", "delete from biz_ip_requests"} {
						mock.ExpectExec(query).WithArgs(int64(81)).WillReturnResult(sqlmock.NewResult(0, 1))
					}
					if scenario == "commit_failure" {
						mock.ExpectCommit().WillReturnError(errors.New("commit failed"))
						want = 500
					} else {
						mock.ExpectCommit()
					}
				}
			}
			code, _ := ipTestPost(t, a, a.ipRequestDelete, map[string]any{"id": 81})
			if code != want {
				t.Fatalf("delete HTTP %d want %d", code, want)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
