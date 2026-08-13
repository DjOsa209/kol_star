package main

import (
	"testing"
)

func TestNormalizeTikHubXUser(t *testing.T) {
	data := map[string]any{
		"data": map[string]any{
			"user": map[string]any{
				"result": map[string]any{
					"rest_id": "44196397",
					"legacy": map[string]any{
						"screen_name":             "OpenAI",
						"name":                    "OpenAI",
						"followers_count":         4200000,
						"friends_count":           3,
						"statuses_count":          900,
						"profile_image_url_https": "https://pbs.twimg.com/profile_images/openai.jpg",
					},
				},
			},
		},
	}
	user := normalizeTikHubXUser(data, "", "")
	if user.ID != "44196397" || user.Handle != "OpenAI" || user.DisplayName != "OpenAI" {
		t.Fatalf("unexpected user identity: %#v", user)
	}
	if user.FollowerCount != 4200000 || user.PostCount != 900 {
		t.Fatalf("unexpected user metrics: %#v", user)
	}
	if user.AvatarURL != "https://pbs.twimg.com/profile_images/openai.jpg" {
		t.Fatalf("unexpected avatar: %s", user.AvatarURL)
	}
}

func TestNormalizeTikHubXUserCurrentResponse(t *testing.T) {
	data := map[string]any{
		"rest_id":        "4398626122",
		"id":             "4398626122",
		"profile":        "OpenAI",
		"name":           "OpenAI",
		"avatar":         "https://pbs.twimg.com/profile_images/openai.jpg",
		"sub_count":      5095267,
		"friends":        4,
		"statuses_count": 2055,
	}
	user := normalizeTikHubXUser(data, "OpenAI", "")
	if user.ID != "4398626122" || user.Handle != "OpenAI" || user.DisplayName != "OpenAI" {
		t.Fatalf("unexpected user identity: %#v", user)
	}
	if user.FollowerCount != 5095267 || user.FollowingCount != 4 || user.PostCount != 2055 {
		t.Fatalf("unexpected current response metrics: %#v", user)
	}
}

func TestNormalizeTikHubXUserRejectsEmptyResponse(t *testing.T) {
	user := normalizeTikHubXUser(map[string]any{}, "missing_account", "")
	if user.ID != "" || user.Handle != "" {
		t.Fatalf("empty TikHub response must not inherit the requested handle: %#v", user)
	}
}

func TestNormalizeTikHubXPosts(t *testing.T) {
	data := map[string]any{
		"entries": []any{
			map[string]any{
				"content": map[string]any{
					"itemContent": map[string]any{
						"tweet_results": map[string]any{
							"result": map[string]any{
								"rest_id": "123456",
								"views":   map[string]any{"count": "12500000"},
								"legacy": map[string]any{
									"full_text":      "A test post",
									"created_at":     "Wed Jul 29 08:30:00 +0000 2026",
									"favorite_count": 1200,
									"reply_count":    30,
									"retweet_count":  90,
									"quote_count":    10,
									"extended_entities": map[string]any{
										"media": []any{map[string]any{
											"type":            "photo",
											"media_url_https": "https://pbs.twimg.com/media/test.jpg",
										}},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	posts := normalizeTikHubXPosts(data, "OpenAI")
	if len(posts) != 1 {
		t.Fatalf("expected one post, got %d: %#v", len(posts), posts)
	}
	post := posts[0]
	if post.PlatformPostID != "123456" || post.PostURL != "https://x.com/OpenAI/status/123456" {
		t.Fatalf("unexpected post identity: %#v", post)
	}
	if post.ViewCount != 12500000 || post.LikeCount != 1200 || post.ShareCount != 100 || post.CommentCount != 30 {
		t.Fatalf("unexpected post metrics: %#v", post)
	}
	if post.CoverURL != "https://pbs.twimg.com/media/test.jpg" || post.PublishedAt == nil {
		t.Fatalf("unexpected media/time: %#v", post)
	}
}

func TestNormalizeTikHubXPostsCurrentResponse(t *testing.T) {
	data := map[string]any{
		"timeline": []any{map[string]any{
			"tweet_id":   "2085434715675426889",
			"created_at": "Fri Aug 07 18:52:48 +0000 2026",
			"text":       "Current TikHub response",
			"views":      "2126028",
			"favorites":  9428,
			"replies":    790,
			"retweets":   820,
			"quotes":     458,
			"bookmarks":  1489,
			"media": map[string]any{
				"video": []any{map[string]any{
					"media_url_https": "https://pbs.twimg.com/amplify_video_thumb/test.jpg",
				}},
			},
		}},
	}
	posts := normalizeTikHubXPosts(data, "OpenAI")
	if len(posts) != 1 {
		t.Fatalf("expected one current response post, got %d: %#v", len(posts), posts)
	}
	post := posts[0]
	if post.PlatformPostID != "2085434715675426889" || post.ViewCount != 2126028 {
		t.Fatalf("unexpected current response post: %#v", post)
	}
	if post.LikeCount != 9428 || post.CommentCount != 790 || post.ShareCount != 1278 || post.SaveCount != 1489 {
		t.Fatalf("unexpected current response engagement: %#v", post)
	}
	if post.CoverURL == "" || post.MediaType != "VIDEO" || post.PublishedAt == nil {
		t.Fatalf("unexpected current response media/time: %#v", post)
	}
}

func TestXHandleIdentifier(t *testing.T) {
	if got := xHandleIdentifier("https://twitter.com/OpenAI/status/1"); got != "OpenAI" {
		t.Fatalf("unexpected handle: %q", got)
	}
	if got := xHandleIdentifier("https://x.com/explore"); got != "" {
		t.Fatalf("expected reserved path to be rejected, got %q", got)
	}
}
