package main

import (
	"testing"
	"time"
)

func TestLinkedInProfileIdentifier(t *testing.T) {
	tests := []struct {
		value       string
		wantHandle  string
		wantURL     string
		wantCompany bool
	}{
		{
			value:      "https://www.linkedin.com/in/williamhgates/",
			wantHandle: "williamhgates",
			wantURL:    "https://www.linkedin.com/in/williamhgates/",
		},
		{
			value:       "https://linkedin.com/company/openai/",
			wantHandle:  "openai",
			wantURL:     "https://www.linkedin.com/company/openai/",
			wantCompany: true,
		},
	}
	for _, test := range tests {
		handle, profileURL, company := linkedinProfileIdentifier(test.value)
		if handle != test.wantHandle || profileURL != test.wantURL || company != test.wantCompany {
			t.Fatalf("linkedinProfileIdentifier(%q) = %q, %q, %v", test.value, handle, profileURL, company)
		}
	}
}

func TestRedditUsernameIdentifier(t *testing.T) {
	for input, want := range map[string]string{
		"https://www.reddit.com/user/spez/": "spez",
		"https://reddit.com/u/openai":       "openai",
		"u/example-user":                    "example-user",
	} {
		if got := redditUsernameIdentifier(input); got != want {
			t.Fatalf("redditUsernameIdentifier(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeTikHubLinkedInUserAndPosts(t *testing.T) {
	profile := map[string]any{
		"data": map[string]any{
			"id":                "person-1",
			"public_identifier": "creator",
			"full_name":         "Creator Name",
			"follower_count":    12345,
			"profile_picture":   "https://cdn.example.com/avatar.jpg",
		},
	}
	user := normalizeTikHubLinkedInUser(profile, "", "https://www.linkedin.com/in/creator/")
	if user.ID != "person-1" || user.Handle != "creator" || user.FollowerCount != 12345 {
		t.Fatalf("unexpected LinkedIn profile: %#v", user)
	}

	posts := normalizeTikHubLinkedInPosts(map[string]any{
		"items": []any{
			map[string]any{
				"id":            "urn:li:activity:123",
				"commentary":    "A LinkedIn update",
				"published_at":  "2026-07-30T12:00:00Z",
				"like_count":    10,
				"comment_count": 2,
				"repost_count":  1,
				"image":         "https://cdn.example.com/post.jpg",
			},
		},
	})
	if len(posts) != 1 {
		t.Fatalf("expected one LinkedIn post, got %d", len(posts))
	}
	if posts[0].LikeCount != 10 || posts[0].CommentCount != 2 || posts[0].ShareCount != 1 {
		t.Fatalf("unexpected LinkedIn metrics: %#v", posts[0])
	}
	if posts[0].PublishedAt == nil || !posts[0].PublishedAt.Equal(time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected LinkedIn publish time: %#v", posts[0].PublishedAt)
	}
}

func TestNormalizeTikHubRedditDoesNotUseKarmaAsFollowersOrViews(t *testing.T) {
	user := normalizeTikHubRedditUser(map[string]any{
		"name":            "creator",
		"id":              "reddit-user-1",
		"total_karma":     999999,
		"followers_count": 42,
		"icon_img":        "https://cdn.example.com/reddit-avatar.png?size=256",
	}, "")
	if user.FollowerCount != 42 {
		t.Fatalf("followers = %d, want 42", user.FollowerCount)
	}

	posts := normalizeTikHubRedditPosts(map[string]any{
		"data": map[string]any{
			"children": []any{
				map[string]any{
					"data": map[string]any{
						"id":           "post-1",
						"title":        "A Reddit post",
						"permalink":    "/r/example/comments/post-1/",
						"created_utc":  1753876800,
						"score":        800,
						"num_comments": 50,
						"thumbnail":    "https://cdn.example.com/post.png",
					},
				},
			},
		},
	})
	if len(posts) != 1 {
		t.Fatalf("expected one Reddit post, got %d", len(posts))
	}
	if posts[0].ViewCount != 0 || posts[0].LikeCount != 800 || posts[0].CommentCount != 50 {
		t.Fatalf("unexpected Reddit metrics: %#v", posts[0])
	}
}
