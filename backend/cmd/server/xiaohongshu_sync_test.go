package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestXiaohongshuUserIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantID  string
		wantURL string
	}{
		{
			name:    "profile url",
			value:   "https://www.xiaohongshu.com/user/profile/61b46d790000000010008153?xsec_token=test",
			wantID:  "61b46d790000000010008153",
			wantURL: "https://www.xiaohongshu.com/user/profile/61b46d790000000010008153",
		},
		{
			name:    "short share url",
			value:   "https://xhslink.com/m/3ZSCJZAMz0a",
			wantURL: "https://xhslink.com/m/3ZSCJZAMz0a",
		},
		{
			name:    "user id",
			value:   "61b46d790000000010008153",
			wantID:  "61b46d790000000010008153",
			wantURL: "https://www.xiaohongshu.com/user/profile/61b46d790000000010008153",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotID, gotURL := xiaohongshuUserIdentifier(test.value)
			if gotID != test.wantID || gotURL != test.wantURL {
				t.Fatalf("xiaohongshuUserIdentifier() = (%q, %q), want (%q, %q)", gotID, gotURL, test.wantID, test.wantURL)
			}
		})
	}
}

func TestNormalizeTikHubXiaohongshuUser(t *testing.T) {
	data := map[string]any{
		"data": map[string]any{
			"user_id":    "61b46d790000000010008153",
			"red_id":     "infinix_test",
			"nickname":   "Infinix Creator",
			"avatar":     "https://example.com/avatar.jpg",
			"fans_count": float64(123456),
			"follows":    float64(321),
			"note_count": float64(45),
		},
	}
	user := normalizeTikHubXiaohongshuUser(data, "", "")
	if user.ID != "61b46d790000000010008153" || user.Handle != "infinix_test" || user.DisplayName != "Infinix Creator" {
		t.Fatalf("unexpected user identity: %#v", user)
	}
	if user.FollowerCount != 123456 || user.FollowingCount != 321 || user.PostCount != 45 {
		t.Fatalf("unexpected user metrics: %#v", user)
	}
	if user.ProfileURL != "https://www.xiaohongshu.com/user/profile/61b46d790000000010008153" {
		t.Fatalf("profile url = %q", user.ProfileURL)
	}
}

func TestNormalizeTikHubXiaohongshuPosts(t *testing.T) {
	data := map[string]any{
		"data": map[string]any{
			"notes": []any{
				map[string]any{
					"note_id":         "abc123",
					"display_title":   "Test note",
					"type":            "video",
					"cover":           map[string]any{"url": "https://example.com/cover.jpg"},
					"liked_count":     "100",
					"comments_count":  float64(20),
					"shared_count":    float64(5),
					"collected_count": float64(30),
				},
			},
		},
	}
	posts := normalizeTikHubXiaohongshuPosts(data)
	if len(posts) != 1 {
		t.Fatalf("posts = %#v", posts)
	}
	post := posts[0]
	if post.PlatformPostID != "abc123" || post.MediaType != "VIDEO" || post.PostURL != "https://www.xiaohongshu.com/explore/abc123" {
		t.Fatalf("unexpected post: %#v", post)
	}
	if post.LikeCount != 100 || post.CommentCount != 20 || post.ShareCount != 5 || post.SaveCount != 30 {
		t.Fatalf("unexpected post metrics: %#v", post)
	}
}

func TestNormalizeTikHubXiaohongshuNestedNoteCard(t *testing.T) {
	data := map[string]any{
		"data": map[string]any{
			"items": []any{
				map[string]any{
					"id": "nested-note-123",
					"note_card": map[string]any{
						"display_title": "Nested note",
						"type":          "normal",
						"cover": map[string]any{
							"url_default": "https://example.com/nested-cover.jpg",
						},
						"interact_info": map[string]any{
							"liked_count":     "120",
							"comment_count":   "8",
							"collected_count": "35",
						},
					},
				},
			},
		},
	}
	posts := normalizeTikHubXiaohongshuPosts(data)
	if len(posts) != 1 {
		t.Fatalf("posts = %#v", posts)
	}
	post := posts[0]
	if post.PlatformPostID != "nested-note-123" || post.CoverURL != "https://example.com/nested-cover.jpg" {
		t.Fatalf("unexpected nested post: %#v", post)
	}
	if post.LikeCount != 120 || post.CommentCount != 8 || post.SaveCount != 35 {
		t.Fatalf("unexpected nested metrics: %#v", post)
	}
}

func TestNormalizeTikHubXiaohongshuVideoImagesListCover(t *testing.T) {
	data := map[string]any{
		"data": map[string]any{
			"data": map[string]any{
				"id":    "69fb38ae000000002003be8d",
				"title": "Video note",
				"type":  "video",
				"images_list": []any{
					map[string]any{"url": "https://example.com/video-cover.jpg"},
				},
			},
		},
	}
	posts := normalizeTikHubXiaohongshuPosts(data)
	if len(posts) != 1 || posts[0].CoverURL != "https://example.com/video-cover.jpg" {
		t.Fatalf("posts = %#v", posts)
	}
}

func TestXiaohongshuNextCursorAndServiceError(t *testing.T) {
	data := map[string]any{
		"data": map[string]any{
			"notes": []any{
				map[string]any{"note_id": "first", "cursor": "cursor-1"},
				map[string]any{"note_id": "second", "cursor": "cursor-2"},
			},
		},
	}
	if got := xiaohongshuNextCursor(data); got != "cursor-2" {
		t.Fatalf("cursor = %q, want cursor-2", got)
	}
	if got := xiaohongshuServiceError(map[string]any{"data": map[string]any{"msg": "服务异常，请稍后再试"}}); got == "" {
		t.Fatal("expected service error to be detected")
	}
	if got := xiaohongshuServiceError(map[string]any{"data": map[string]any{"nickname": "正常用户"}}); got != "" {
		t.Fatalf("unexpected service error: %q", got)
	}
}

func TestFetchXiaohongshuPostDetailFallsBackToShareText(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("note_id") != "" {
			_, _ = w.Write([]byte(`{"code":200,"data":{"msg":"服务异常，请稍后再试"}}`))
			return
		}
		if r.URL.Query().Get("share_text") == "" {
			http.Error(w, "share_text fallback was not sent", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{
			"code": 200,
			"data": {
				"items": [{
					"id": "fallback-note",
					"note_card": {
						"display_title": "Fallback note",
						"type": "normal",
						"cover": {"url_default": "https://example.com/fallback.jpg"}
					}
				}]
			}
		}`))
	}))
	defer server.Close()
	t.Setenv("TIKHUB_API_BASE_URL", server.URL)

	post, err := fetchXiaohongshuPostDetail(
		context.Background(), server.Client(), "test-key",
		url.Values{"note_id": []string{"bad-note-id"}},
		"bad-note-id", "https://xhslink.com/o/example",
	)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if post.PlatformPostID != "fallback-note" || !strings.Contains(post.CoverURL, "fallback.jpg") {
		t.Fatalf("unexpected fallback post: %#v", post)
	}
}

func TestResolveXiaohongshuShortLinkExtractsNoteID(t *testing.T) {
	const noteID = "69fb38ae000000002003be8d"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/o/") {
			http.Redirect(w, r, "/discovery/item/"+noteID+"?xsec_token=test", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("note"))
	}))
	defer server.Close()

	gotID, gotURL := resolveXiaohongshuPostReference(
		context.Background(), server.Client(), "", server.URL+"/o/7eJIkHmwAQo",
	)
	if gotID != noteID || !strings.Contains(gotURL, "/discovery/item/"+noteID) {
		t.Fatalf("resolved = (%q, %q), want note id %q", gotID, gotURL, noteID)
	}
}

func TestCanonicalXiaohongshuShareURLUpgradesHTTP(t *testing.T) {
	const input = "http://xhslink.com/o/7eJIkHmwAQo"
	const want = "https://xhslink.com/o/7eJIkHmwAQo"
	if got := canonicalXiaohongshuShareURL(input); got != want {
		t.Fatalf("canonicalXiaohongshuShareURL() = %q, want %q", got, want)
	}
}
