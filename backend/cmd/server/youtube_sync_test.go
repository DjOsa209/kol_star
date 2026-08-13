package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestYouTubeSyncSavesFollowersBeforeFetchingPosts(t *testing.T) {
	t.Setenv("KOL_SKIP_RESOURCE_IMAGE_DOWNLOAD", "1")

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("select content from biz_governance_rules").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("update biz_resources set").
		WithArgs(
			"diag", "diag", "", "",
			true, int64(123456), int64(1000000), int64(10), int64(100000),
			"UCdiag", "@diag", "", "", "", 42,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("select count\\(\\*\\) from biz_resources where id = \\?").
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("update biz_resources r").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("select sync_posts, post_limit from biz_platform_sync_settings").
		WithArgs("YouTube").
		WillReturnRows(sqlmock.NewRows([]string{"sync_posts", "post_limit"}).AddRow(1, 25))

	fakeAPI := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/youtube/v3/channels":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"items":[{"id":"UCdiag","snippet":{"title":"Diagnostic Channel","customUrl":"@diag"},"statistics":{"subscriberCount":"123456","viewCount":"1000000","videoCount":"10"},"contentDetails":{"relatedPlaylists":{"uploads":"PLdiag"}}}]}`)
		case "/youtube/v3/playlistItems":
			http.Error(w, `{"error":{"message":"post sync failed"}}`, http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer fakeAPI.Close()

	fakeAddr := fakeAPI.Listener.Addr().String()
	originalTransport := http.DefaultTransport
	http.DefaultTransport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // test-only fake API
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, fakeAddr)
		},
	}
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	cfg := Config{PlatformAPIs: PlatformAPIConfig{YouTubeAPIKey: "diagnostic-key"}}
	app := newApp(db, cfg)
	_, syncErr := app.syncYouTubeResource(
		context.Background(),
		42,
		"Diagnostic Channel",
		"https://youtube.com/@diag",
	)
	if syncErr == nil {
		t.Fatal("expected post synchronization to fail")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestYouTubeChannelIdentifierSupportsLegacyUserURL(t *testing.T) {
	param, value, err := youtubeChannelIdentifier("", "https://www.youtube.com/user/legacy_creator/videos")
	if err != nil {
		t.Fatal(err)
	}
	if param != "forUsername" || value != "legacy_creator" {
		t.Fatalf("youtubeChannelIdentifier() = %q, %q; want forUsername, legacy_creator", param, value)
	}
}

func TestYouTubeSyncPreservesFollowersWhenSubscriberCountIsHidden(t *testing.T) {
	t.Setenv("KOL_SKIP_RESOURCE_IMAGE_DOWNLOAD", "1")

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("select content from biz_governance_rules").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("update biz_resources set").
		WithArgs(
			"hidden", "hidden", "", "",
			false, int64(0), int64(1000), int64(10), int64(100),
			"UChidden", "@hidden", "", "", "", 43,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("select count\\(\\*\\) from biz_resources where id = \\?").
		WithArgs(43).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("update biz_resources r").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("select sync_posts, post_limit from biz_platform_sync_settings").
		WithArgs("YouTube").
		WillReturnRows(sqlmock.NewRows([]string{"sync_posts", "post_limit"}).AddRow(0, 25))
	mock.ExpectExec("update biz_resources set").
		WithArgs(int64(100), float64(0), float64(0), "部分成功", "YouTube 频道已隐藏订阅数，已保留原粉丝数", 43).
		WillReturnResult(sqlmock.NewResult(0, 1))

	fakeAPI := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[{"id":"UChidden","snippet":{"title":"Hidden Channel","customUrl":"@hidden"},"statistics":{"viewCount":"1000","videoCount":"10"},"contentDetails":{"relatedPlaylists":{"uploads":"PLhidden"}}}]}`)
	}))
	defer fakeAPI.Close()

	fakeAddr := fakeAPI.Listener.Addr().String()
	originalTransport := http.DefaultTransport
	http.DefaultTransport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // test-only fake API
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, fakeAddr)
		},
	}
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	app := newApp(db, Config{PlatformAPIs: PlatformAPIConfig{YouTubeAPIKey: "diagnostic-key"}})
	result, err := app.syncYouTubeResource(context.Background(), 43, "Hidden Channel", "https://youtube.com/@hidden")
	if err != nil {
		t.Fatal(err)
	}
	if result["warning"] != "YouTube 频道已隐藏订阅数，已保留原粉丝数" {
		t.Fatalf("unexpected warning: %#v", result["warning"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
