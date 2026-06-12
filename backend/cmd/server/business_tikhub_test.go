package main

import (
	"testing"
	"time"
)

func TestNormalizeTikHubInstagramProfileAndPosts(t *testing.T) {
	profile := map[string]any{
		"pk":             "123",
		"username":       "creator",
		"full_name":      "Creator Name",
		"follower_count": 4567,
		"media_count":    88,
		"hd_profile_pic_url_info": map[string]any{
			"url": "https://example.com/avatar.jpg",
		},
	}
	user := normalizeTikHubInstagramUser(profile, "")
	if user.ID != "123" || user.Username != "creator" || user.FollowerCount != 4567 {
		t.Fatalf("unexpected profile: %#v", user)
	}
	if user.AvatarURL != "https://example.com/avatar.jpg" {
		t.Fatalf("unexpected avatar: %q", user.AvatarURL)
	}

	reels := map[string]any{
		"items": []any{
			map[string]any{
				"pk":             "reel-1",
				"code":           "ABC123",
				"taken_at":       time.Now().Unix(),
				"play_count":     12000,
				"like_count":     500,
				"comment_count":  30,
				"video_duration": 15,
				"caption": map[string]any{
					"text": "Recent reel",
				},
				"image_versions2": map[string]any{
					"candidates": []any{map[string]any{"url": "https://example.com/cover.jpg"}},
				},
			},
		},
	}
	posts := normalizeTikHubInstagramPostsData(reels, user.Username)
	if len(posts) != 1 {
		t.Fatalf("expected one post, got %d", len(posts))
	}
	if posts[0].ViewCount != 12000 || posts[0].CoverURL == "" || posts[0].Description != "Recent reel" {
		t.Fatalf("unexpected post: %#v", posts[0])
	}
	if averageViewedPostViews(posts) != 12000 {
		t.Fatalf("unexpected average views: %d", averageViewedPostViews(posts))
	}
}

func TestMergePlatformPostsPrefersMetrics(t *testing.T) {
	merged := mergePlatformPosts(
		[]platformPost{{PlatformPostID: "1", Description: "caption", LikeCount: 10}},
		[]platformPost{{PlatformPostID: "1", ViewCount: 900, CoverURL: "cover"}},
	)
	if len(merged) != 1 {
		t.Fatalf("expected one merged post, got %d", len(merged))
	}
	if merged[0].ViewCount != 900 || merged[0].LikeCount != 10 || merged[0].Description != "caption" {
		t.Fatalf("unexpected merged post: %#v", merged[0])
	}
}

func TestNormalizeTikHubTikTokAppV3Posts(t *testing.T) {
	data := map[string]any{
		"data": map[string]any{
			"aweme_list": []any{
				map[string]any{
					"aweme_id":    "7339393672959757570",
					"desc":        "App V3 post",
					"create_time": time.Now().Unix(),
					"author": map[string]any{
						"unique_id": "creator",
					},
					"statistics": map[string]any{
						"play_count":    12345,
						"digg_count":    678,
						"comment_count": 9,
						"share_count":   10,
					},
					"video": map[string]any{
						"duration": int64(15000),
						"cover": map[string]any{
							"url_list": []any{"https://example.com/cover.jpg"},
						},
					},
					"share_info": map[string]any{
						"share_url": "https://www.tiktok.com/@creator/video/7339393672959757570",
					},
				},
			},
		},
	}
	posts := normalizeTikHubTikTokPosts(data, "")
	if len(posts) != 1 {
		t.Fatalf("expected one post, got %d", len(posts))
	}
	post := posts[0]
	if post.PlatformPostID != "7339393672959757570" || post.ViewCount != 12345 || post.LikeCount != 678 {
		t.Fatalf("unexpected post metrics: %#v", post)
	}
	if post.Duration != 15 || post.CoverURL == "" || post.PostURL == "" {
		t.Fatalf("unexpected post fields: %#v", post)
	}
}
