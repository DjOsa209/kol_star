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

func TestNormalizeTikHubInstagramNestedProfileFollowers(t *testing.T) {
	profile := map[string]any{
		"data": map[string]any{
			"user": map[string]any{
				"pk":       "nested-123",
				"username": "nested_creator",
				"edge_followed_by": map[string]any{
					"count": "9876",
				},
			},
		},
	}
	user := normalizeTikHubInstagramUser(profile, "")
	if user.ID != "nested-123" || user.Username != "nested_creator" {
		t.Fatalf("nested Instagram profile was not parsed: %#v", user)
	}
	if user.FollowerCount != 9876 {
		t.Fatalf("nested Instagram followers = %d, want 9876", user.FollowerCount)
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

func TestNormalizeTikHubTikTokFlatMetrics(t *testing.T) {
	posts := normalizeTikHubTikTokPosts(map[string]any{
		"value": []any{
			map[string]any{
				"aweme_id":      "7350810998023949599",
				"play_count":    2140000,
				"digg_count":    128400,
				"comment_count": 3120,
				"share_count":   890,
				"collect_count": 15600,
			},
		},
	}, "creator")
	if len(posts) != 1 {
		t.Fatalf("expected one post, got %d", len(posts))
	}
	post := posts[0]
	if post.ViewCount != 2140000 || post.LikeCount != 128400 {
		t.Fatalf("unexpected exposure metrics: %#v", post)
	}
	if post.CommentCount != 3120 || post.ShareCount != 890 || post.SaveCount != 15600 {
		t.Fatalf("unexpected engagement metrics: %#v", post)
	}
}

func TestPlatformPostEngagementRateIncludesSaves(t *testing.T) {
	rate := platformPostEngagementRate([]platformPost{{
		LikeCount:    10,
		CommentCount: 5,
		ShareCount:   3,
		SaveCount:    2,
	}}, 100)
	if rate != 0.2 {
		t.Fatalf("platformPostEngagementRate() = %v, want 0.2", rate)
	}
}

func TestImageURLPrefersCompleteRemoteURLOverAssetKey(t *testing.T) {
	value := map[string]any{
		"uri":      "tos-useast8-p-0068-tx2/asset-key",
		"url_list": []any{"https://cdn.example.com/cover.jpg"},
	}
	if got := imageURL(value); got != "https://cdn.example.com/cover.jpg" {
		t.Fatalf("unexpected image URL: %q", got)
	}
	if got := imageURL("tos-useast8-p-0068-tx2/asset-key"); got != "" {
		t.Fatalf("expected platform asset key to be rejected, got %q", got)
	}
	if got := imageURL("//cdn.example.com/cover.jpg"); got != "https://cdn.example.com/cover.jpg" {
		t.Fatalf("unexpected protocol-relative image URL: %q", got)
	}
}
