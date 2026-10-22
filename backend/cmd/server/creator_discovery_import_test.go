package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

const discoveryReviewTestChannel = "UCabcdefghijklmnopqrstuv"

func discoveryReviewTestSnapshot() creatorDiscoverySnapshot {
	followers := int64(1234)
	return creatorDiscoverySnapshot{
		ChannelID: discoveryReviewTestChannel, Name: "Public channel", Handle: "@public-channel",
		ProfileURL: "https://www.youtube.com/channel/" + discoveryReviewTestChannel,
		Followers:  &followers, TotalViews: 20000, VideoCount: 12,
		RecentPosts: []platformPost{{PlatformPostID: "abcdefghijk", Title: "Phone review", PostURL: "https://www.youtube.com/watch?v=abcdefghijk", ViewCount: 1000}},
		Evidence:    []creatorDiscoveryEvidence{{URL: "https://www.youtube.com/watch?v=lmnopqrstuv"}},
		CollectedAt: "2026-10-21T12:30:00Z",
	}
}

func discoveryReviewTestInput() creatorDiscoveryReviewInput {
	return creatorDiscoveryReviewInput{ID: 12, Action: "approve", Country: "伊拉克", Language: "阿拉伯语", Note: "已核对近期手机评测作品与市场相关性", EvidenceURL: "https://www.youtube.com/watch?v=abcdefghijk", TopicConfirmed: true}
}

func expectDiscoveryCandidateLock(t *testing.T, mock sqlmock.Sqlmock, status, jobStatus string, resourceID any, snapshot creatorDiscoverySnapshot) {
	t.Helper()
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)select c.status,c.resource_id,c.channel_id,c.snapshot,j.status.*where c.id=\? and j.user_id=\? for update`).
		WithArgs(int64(12), 7).
		WillReturnRows(sqlmock.NewRows([]string{"status", "resource_id", "channel_id", "snapshot", "job_status"}).AddRow(status, resourceID, snapshot.ChannelID, data, jobStatus))
}

func requireDiscoveryReviewError(t *testing.T, err error, status int) {
	t.Helper()
	var reviewErr *creatorDiscoveryReviewError
	if !errors.As(err, &reviewErr) || reviewErr.Status != status {
		t.Fatalf("error=%v; want status %d", err, status)
	}
}

func TestCreatorDiscoveryReviewRequiresCollectedEvidence(t *testing.T) {
	snapshot := discoveryReviewTestSnapshot()
	for _, evidence := range []string{snapshot.ProfileURL, snapshot.RecentPosts[0].PostURL, snapshot.Evidence[0].URL} {
		input := discoveryReviewTestInput()
		input.EvidenceURL = evidence
		if err := validateCreatorDiscoveryReview(&input, snapshot); err != nil {
			t.Fatalf("collected evidence rejected: %v", err)
		}
	}
	for _, evidence := range []string{"", "https://example.com/invented", "https://www.youtube.com/watch?v=not-collected", "javascript:alert(1)", "https://user@www.youtube.com/watch?v=abcdefghijk"} {
		input := discoveryReviewTestInput()
		input.EvidenceURL = evidence
		requireDiscoveryReviewError(t, validateCreatorDiscoveryReview(&input, snapshot), http.StatusBadRequest)
	}
	for _, field := range []string{"country", "language", "note", "topic"} {
		input := discoveryReviewTestInput()
		switch field {
		case "country":
			input.Country = ""
		case "language":
			input.Language = strings.Repeat("语", 65)
		case "note":
			input.Note = " "
		case "topic":
			input.TopicConfirmed = false
		}
		requireDiscoveryReviewError(t, validateCreatorDiscoveryReview(&input, snapshot), http.StatusBadRequest)
	}
}

func TestCreatorDiscoveryReviewTenantAndStateBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, status, jobStatus, action string
		resourceID                      any
		wantStatus                      int
	}{
		{"running", "pending", "collecting", "approve", nil, http.StatusConflict},
		{"unknown job state", "pending", "unknown", "approve", nil, http.StatusConflict},
		{"rejected cannot approve", "rejected", "completed", "approve", nil, http.StatusConflict},
		{"approved cannot reject", "approved", "completed", "reject", int64(99), http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, _ := sqlmock.New()
			defer db.Close()
			expectDiscoveryCandidateLock(t, mock, tc.status, tc.jobStatus, tc.resourceID, discoveryReviewTestSnapshot())
			mock.ExpectRollback()
			input := discoveryReviewTestInput()
			input.Action = tc.action
			_, err := newApp(db, Config{}).reviewCreatorDiscoveryCandidate(context.Background(), 7, input)
			requireDiscoveryReviewError(t, err, tc.wantStatus)
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("other owner cannot review", func(t *testing.T) {
		db, mock, _ := sqlmock.New()
		defer db.Close()
		mock.ExpectBegin()
		mock.ExpectQuery(`(?s)from biz_creator_discovery_candidates c.*where c.id=\? and j.user_id=\? for update`).WithArgs(int64(12), 7).WillReturnError(sql.ErrNoRows)
		mock.ExpectRollback()
		_, err := newApp(db, Config{}).reviewCreatorDiscoveryCandidate(context.Background(), 7, discoveryReviewTestInput())
		requireDiscoveryReviewError(t, err, http.StatusNotFound)
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestCreatorDiscoveryRepeatedApprovalReturnsOriginalResource(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	expectDiscoveryCandidateLock(t, mock, "approved", "completed", int64(42), discoveryReviewTestSnapshot())
	mock.ExpectCommit()
	// A retried acknowledgement needs no second insert or resubmitted form fields.
	result, err := newApp(db, Config{}).reviewCreatorDiscoveryCandidate(context.Background(), 7, creatorDiscoveryReviewInput{ID: 12, Action: "approve"})
	if err != nil || result.ResourceID != 42 || !result.Reused || result.Status != "approved" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreatorDiscoveryApprovalRequiresResourceMenu(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	a := newApp(db, Config{})
	token := auditToken(mock, 7)
	mock.ExpectQuery(`select r.code from sys_roles r`).WithArgs(7).WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("operator"))
	mock.ExpectQuery(`(?s)select count\(\*\).*m.path = \?`).WithArgs(7, "/business/assistant").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`select r.code from sys_roles r`).WithArgs(7).WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("operator"))
	mock.ExpectQuery(`(?s)select count\(\*\).*m.path = \?`).WithArgs(7, "/business/resources").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	response := auditPost(t, a.creatorDiscoveryReview, token, map[string]any{"id": 12, "action": "approve"})
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreatorDiscoveryApprovalReusesIdentityAndPreservesRecommendation(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	snapshot := discoveryReviewTestSnapshot()
	snapshot.RecentPosts = nil
	input := discoveryReviewTestInput()
	input.EvidenceURL = snapshot.ProfileURL
	expectDiscoveryCandidateLock(t, mock, "pending", "partial", nil, snapshot)
	mock.ExpectExec(`insert into biz_creator_discovery_identities.*`).WithArgs(snapshot.ChannelID).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`(?s)select resource_id from biz_creator_discovery_identities.*for update`).WithArgs(snapshot.ChannelID).WillReturnRows(sqlmock.NewRows([]string{"resource_id"}).AddRow(72))
	mock.ExpectQuery(`(?s)select id from biz_resources where id=\?.*for update`).WithArgs(int64(72), snapshot.ChannelID, snapshot.ProfileURL, snapshot.ProfileURL+"/", strings.Replace(snapshot.ProfileURL, "www.", "", 1), strings.Replace(snapshot.ProfileURL, "www.", "", 1)+"/").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(72))
	mock.ExpectExec(`update biz_creator_discovery_identities`).WithArgs(int64(72), snapshot.ChannelID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)update biz_creator_discovery_candidates.*status='approved'`).WithArgs(int64(72), input.Country, input.Language, input.Note, input.EvidenceURL, 7, int64(12)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	result, err := newApp(db, Config{}).reviewCreatorDiscoveryCandidate(context.Background(), 7, input)
	if err != nil || result.ResourceID != 72 || !result.Reused {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreatorDiscoveryReviewRejectInvalidatesRecommendation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	input := creatorDiscoveryReviewInput{ID: 12, Action: "reject", Note: "主题不匹配"}
	expectDiscoveryCandidateLock(t, mock, "pending", "completed", nil, discoveryReviewTestSnapshot())
	mock.ExpectExec(`(?s)update biz_creator_discovery_candidates.*status='rejected'`).
		WithArgs(input.Note, 7, input.ID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)update biz_creator_discovery_jobs j.*set j.recommendation=NULL where c.id=\?`).
		WithArgs(input.ID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	result, err := newApp(db, Config{}).reviewCreatorDiscoveryCandidate(context.Background(), 7, input)
	if err != nil || result.Status != "rejected" || result.ResourceID != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreatorDiscoveryImportRollsBackResourceWhenPostsFail(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	snapshot := discoveryReviewTestSnapshot()
	snapshot.Warning = "近期作品只采集到部分结果"
	input := discoveryReviewTestInput()
	expectDiscoveryCandidateLock(t, mock, "pending", "completed", nil, snapshot)
	mock.ExpectExec(`insert into biz_creator_discovery_identities`).WithArgs(snapshot.ChannelID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)select resource_id from biz_creator_discovery_identities.*for update`).WithArgs(snapshot.ChannelID).WillReturnRows(sqlmock.NewRows([]string{"resource_id"}).AddRow(nil))
	mock.ExpectQuery(`(?s)select id from biz_resources.*platform='YouTube'.*for update`).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`(?s)insert into biz_resources.*'观察中'.*'YouTube Data API',0,\?,'待核实'`).
		WithArgs(snapshot.Name, input.Country, input.Language, *snapshot.Followers, *snapshot.Followers,
			snapshot.ChannelID, snapshot.Handle, snapshot.ProfileURL, "", snapshot.TotalViews, snapshot.VideoCount,
			calcLevel(0), sqlmock.AnyArg(), "部分成功", snapshot.Warning, parsePlatformTime(snapshot.CollectedAt)).
		WillReturnResult(sqlmock.NewResult(73, 1))
	mock.ExpectExec(`update biz_creator_discovery_identities`).WithArgs(int64(73), snapshot.ChannelID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`insert into biz_resource_platform_posts`).WillReturnError(errors.New("posts write failed"))
	mock.ExpectRollback()
	_, err := newApp(db, Config{}).reviewCreatorDiscoveryCandidate(context.Background(), 7, input)
	if err == nil || !strings.Contains(err.Error(), "posts write failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
