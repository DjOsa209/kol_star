package main

import "testing"

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
