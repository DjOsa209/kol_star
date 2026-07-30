package main

import "testing"

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

func TestXHandleIdentifier(t *testing.T) {
	if got := xHandleIdentifier("https://twitter.com/OpenAI/status/1"); got != "OpenAI" {
		t.Fatalf("unexpected handle: %q", got)
	}
	if got := xHandleIdentifier("https://x.com/explore"); got != "" {
		t.Fatalf("expected reserved path to be rejected, got %q", got)
	}
}
