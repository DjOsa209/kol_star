package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestParseCooperationPostLink(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		platform string
		postID   string
	}{
		{name: "youtube watch", value: "https://www.youtube.com/watch?v=51dNGqLoM00", platform: "YouTube", postID: "51dNGqLoM00"},
		{name: "youtube short", value: "https://youtu.be/51dNGqLoM00?t=2", platform: "YouTube", postID: "51dNGqLoM00"},
		{name: "youtube shorts", value: "https://youtube.com/shorts/51dNGqLoM00", platform: "YouTube", postID: "51dNGqLoM00"},
		{name: "tiktok", value: "https://www.tiktok.com/@creator/video/7350810998023949599", platform: "TikTok", postID: "7350810998023949599"},
		{name: "instagram", value: "https://www.instagram.com/reel/DPwhVB-jo9k/", platform: "Instagram", postID: "DPwhVB-jo9k"},
		{name: "finds url in text", value: "发布链接：https://www.instagram.com/p/DPwhVB-jo9k/ 备注", platform: "Instagram", postID: "DPwhVB-jo9k"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseCooperationPostLink(test.value)
			if err != nil {
				t.Fatal(err)
			}
			if got.Platform != test.platform || got.PostID != test.postID {
				t.Fatalf("parseCooperationPostLink() = %#v, want platform=%q postID=%q", got, test.platform, test.postID)
			}
		})
	}
}

func TestParseCooperationPostLinkRejectsUnsupportedLink(t *testing.T) {
	if _, err := parseCooperationPostLink("https://example.com/post/123"); err == nil {
		t.Fatal("expected unsupported link error")
	}
}

func TestCooperationPostSourceUsesFinalLinkWhenDeliverableLinksAreEmpty(t *testing.T) {
	finalLink := "https://www.tiktok.com/@creator/video/7350810998023949599"
	if got := cooperationPostSource(finalLink, ""); got != finalLink {
		t.Fatalf("cooperationPostSource() = %q, want %q", got, finalLink)
	}
}

func TestCooperationPostSourcePrefersFinalLink(t *testing.T) {
	finalLink := "https://www.instagram.com/reel/DPwhVB-jo9k/"
	deliverableLinks := "https://www.instagram.com/reel/older/"
	if got := cooperationPostSource(finalLink, deliverableLinks); got != finalLink {
		t.Fatalf("cooperationPostSource() = %q, want %q", got, finalLink)
	}
}

func TestSyncCooperationPostUsesFinalLinkAndStoredCover(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	finalLink := "https://www.youtube.com/watch?v=51dNGqLoM00"
	mock.ExpectQuery("select resource_id, coalesce\\(final_link, ''\\), coalesce\\(deliverable_links, ''\\)").
		WithArgs(99).
		WillReturnRows(sqlmock.NewRows([]string{"resource_id", "final_link", "deliverable_links"}).
			AddRow(7, finalLink, ""))
	mock.ExpectQuery("select platform_post_id, title, description, post_url,").
		WithArgs(7, "YouTube").
		WillReturnRows(sqlmock.NewRows([]string{
			"platform_post_id", "title", "description", "post_url", "cover_url", "media_type",
			"published_at", "duration_seconds", "view_count", "like_count", "comment_count", "share_count", "save_count",
		}).AddRow(
			"51dNGqLoM00", "Video", "", finalLink, "https://i.ytimg.com/vi/51dNGqLoM00/hqdefault.jpg", "VIDEO",
			nil, 60, 1000, 100, 12, 10, 20,
		))
	mock.ExpectExec("update biz_cooperations set").
		WithArgs(int64(1000), int64(130), int64(12), nil, 99).
		WillReturnResult(sqlmock.NewResult(0, 1))

	app := newApp(db, Config{})
	result, err := app.syncCooperationPost(context.Background(), 99, false)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Synced || result.Source != "作品库" {
		t.Fatalf("syncCooperationPost() = %#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyPlatformPostToCooperationUsesFetchedRemoteInstagramMedia(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	const remoteAvatar = "https://cdn.example.com/avatar.jpg"
	const remoteCover = "https://cdn.example.com/cover.jpg"
	mock.ExpectExec("update biz_cooperations set").
		WithArgs(int64(100), int64(15), int64(2), nil, 99).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("update biz_cooperations set content_platform").
		WithArgs(
			"Instagram",
			remoteCover,
			remoteCover,
			99,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("update biz_resources set platform").
		WithArgs(
			"Instagram",
			"https://www.instagram.com/imparkerburton/",
			"https://www.instagram.com/imparkerburton/",
			"author-1", "author-1",
			"imparkerburton", "imparkerburton",
			remoteAvatar, remoteAvatar,
			remoteAvatar, remoteAvatar,
			7,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	app := newApp(db, Config{})
	err = app.applyPlatformPostToCooperation(
		context.Background(),
		99,
		7,
		cooperationPostLink{
			Platform: "Instagram",
			PostID:   "post-1",
			URL:      "https://www.instagram.com/reels/post-1/",
		},
		platformPost{
			PlatformPostID: "post-1",
			CoverURL:       remoteCover,
			ViewCount:      100,
			LikeCount:      10,
			CommentCount:   2,
			ShareCount:     3,
			SaveCount:      2,
			Raw: map[string]any{
				"author": map[string]any{
					"id":              "author-1",
					"username":        "imparkerburton",
					"profile_pic_url": remoteAvatar,
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClearCooperationPostSyncFieldsRemovesPreviousIdentityAndCover(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("update biz_cooperations").
		WithArgs("Instagram", 99, 7).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("update biz_resources").
		WithArgs("Instagram", 7).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("update biz_resource_platform_posts").
		WithArgs(7, "Instagram", "post-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	app := newApp(db, Config{})
	if err := app.clearCooperationPostSyncFields(
		context.Background(),
		99,
		7,
		cooperationPostLink{Platform: "Instagram", PostID: "post-1"},
	); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPopulateCooperationPostSyncResultReturnsFetchedCoverFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("select coalesce\\(r.platform_url").
		WithArgs(99, 7).
		WillReturnRows(sqlmock.NewRows([]string{
			"platform_url", "final_link", "deliverable_links",
			"avatar_url", "avatar_remote_url", "content_cover_url",
			"content_cover_remote_url",
		}).AddRow(
			"https://www.instagram.com/imparkerburton/",
			"https://www.instagram.com/reels/post-1/",
			"https://www.instagram.com/reels/post-1/",
			"https://cdn.example.com/avatar.jpg",
			"https://cdn.example.com/avatar.jpg",
			"https://cdn.example.com/cover.jpg",
			"https://cdn.example.com/cover.jpg",
		))

	result := cooperationPostSyncResult{
		FetchedCoverURL: "https://cdn.example.com/cover.jpg",
	}
	app := newApp(db, Config{})
	if err := app.populateCooperationPostSyncResult(
		context.Background(),
		99,
		7,
		&result,
	); err != nil {
		t.Fatal(err)
	}
	if result.CoverURL != "https://cdn.example.com/cover.jpg" ||
		result.ContentCoverRemoteURL != "https://cdn.example.com/cover.jpg" ||
		result.FetchedCoverURL != "https://cdn.example.com/cover.jpg" {
		t.Fatalf("unexpected cover fields: %#v", result)
	}
	if result.PlatformURL != "https://www.instagram.com/imparkerburton/" ||
		result.ResourceAvatarRemoteURL != "https://cdn.example.com/avatar.jpg" {
		t.Fatalf("unexpected resource fields: %#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCooperationPostSyncResultJSONIncludesFetchedCoverURL(t *testing.T) {
	const coverURL = "https://cdn.example.com/fetched-cover.jpg"
	payload, err := json.Marshal(cooperationPostSyncResult{
		Synced:          true,
		FetchedCoverURL: coverURL,
		CoverURL:        coverURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatal(err)
	}
	if result["fetchedCoverUrl"] != coverURL || result["coverUrl"] != coverURL {
		t.Fatalf("sync response does not expose fetched cover URL: %s", payload)
	}
	for _, field := range []string{
		"platform",
		"platformUrl",
		"postId",
		"finalLink",
		"deliverableLinks",
		"resourceAvatarUrl",
		"resourceAvatarRemoteUrl",
		"contentCoverLocalUrl",
		"contentCoverRemoteUrl",
		"previousFieldsCleared",
	} {
		if _, exists := result[field]; !exists {
			t.Fatalf("sync response omits %s: %s", field, payload)
		}
	}
}

func TestCooperationPostSyncFailure(t *testing.T) {
	if err := cooperationPostSyncFailure(cooperationPostSyncResult{
		Platform: "TikTok",
		Message:  "API 获取作品数据失败",
	}); err == nil {
		t.Fatal("expected unsuccessful API synchronization to become an error")
	}
	if err := cooperationPostSyncFailure(cooperationPostSyncResult{Synced: true}); err != nil {
		t.Fatalf("successful synchronization returned error: %v", err)
	}
}

func TestPlatformPostMatchesLink(t *testing.T) {
	link, err := parseCooperationPostLink("https://youtu.be/51dNGqLoM00")
	if err != nil {
		t.Fatal(err)
	}
	if !platformPostMatchesLink(platformPost{PlatformPostID: "51dNGqLoM00"}, link) {
		t.Fatal("expected platform post id match")
	}
	if !platformPostMatchesLink(platformPost{PostURL: "https://www.youtube.com/watch?v=51dNGqLoM00"}, link) {
		t.Fatal("expected normalized URL match")
	}
}

func TestFindSinglePlatformItem(t *testing.T) {
	got := findSinglePlatformItem(map[string]any{
		"value": []any{
			map[string]any{"item_info": map[string]any{"item_struct": map[string]any{"aweme_id": "123"}}},
		},
	})
	if anyString(got["aweme_id"]) != "123" {
		t.Fatalf("findSinglePlatformItem() = %#v", got)
	}
}
