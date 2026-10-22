package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCreatorDiscoveryOnlyAcceptsBriefAndKeepsStrategyServerSide(t *testing.T) {
	in := creatorDiscoveryInput{Market: "美国", Query: " 伊拉克手机达人 ", Queries: []string{"client-injected keywords"}, MaxCandidates: 1000}
	if err := validateCreatorDiscoveryInput(&in); err != nil {
		t.Fatal(err)
	}
	if in.Query != "伊拉克手机达人" || in.Market != "" || len(in.Queries) != 0 || in.MaxCandidates != 12 {
		t.Fatalf("client controlled search strategy: %+v", in)
	}
	for _, q := range []string{" ", strings.Repeat("字", 2001)} {
		in.Query = q
		if validateCreatorDiscoveryInput(&in) == nil {
			t.Fatal("invalid demand accepted")
		}
	}
}

func TestCreatorDiscoveryOwnerScopeForEveryJobOperation(t *testing.T) {
	for _, name := range []string{"detail", "cancel", "retry", "recommend"} {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			a := newApp(db, Config{})
			expectIPTestSession(mock)
			mock.ExpectQuery("select r.code").WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("admin"))
			mock.ExpectQuery(`where j.id=\? and j.user_id=\?`).WithArgs(99, 1).WillReturnError(sql.ErrNoRows)
			handlers := map[string]http.HandlerFunc{"detail": a.creatorDiscoveryDetail, "cancel": a.creatorDiscoveryCancel, "retry": a.creatorDiscoveryRetry, "recommend": a.creatorDiscoveryRecommend}
			status, _ := ipTestPost(t, a, handlers[name], map[string]any{"id": 99})
			if status != http.StatusNotFound {
				t.Fatalf("foreign job access status=%d", status)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCreatorDiscoveryModelFactsPreserveUnknownsAndVerifiedEvidence(t *testing.T) {
	c := creatorDiscoveryCandidate{ID: 1, ResourceID: 23, Country: "伊拉克", Language: "阿拉伯语", Note: "频道国家IQ，作品为手机评测", EvidenceURL: "https://www.youtube.com/channel/UCactual",
		Snapshot: creatorDiscoverySnapshot{ChannelID: "UCactual", Country: "IQ", ProfileURL: "https://www.youtube.com/channel/UCactual", RecentPosts: []platformPost{{PlatformPostID: "video", Title: "Phone review"}}}}
	facts := creatorDiscoveryRecommendationFacts(c, map[string]any{"name": "public channel", "status": "观察中"})
	raw, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err = json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["followers"] != nil || payload["estimatedCost"] != nil || payload["quoteStatus"] != "unknown" {
		t.Fatalf("unknown data invented: %s", raw)
	}
	if payload["platformReportedCountry"] != "IQ" || payload["verificationEvidenceUrl"] != c.EvidenceURL || payload["country"] != "伊拉克" {
		t.Fatalf("evidence lost: %s", raw)
	}
	if _, present := payload["score"]; present {
		t.Fatal("unassessed resource acquired a fabricated internal score")
	}
	if len(payload["recentPosts"].([]any)) != 1 || !strings.Contains(payload["audienceCountry"].(string), "未知") {
		t.Fatalf("evidence scope changed: %s", raw)
	}
}
