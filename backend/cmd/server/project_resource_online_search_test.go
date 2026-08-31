package main

import "testing"

func TestNormalizeOnlineProjectResourceSeed(t *testing.T) {
	tests := []struct {
		name           string
		platform       string
		query          string
		wantPlatform   string
		wantHandle     string
		wantPlatformID string
	}{
		{name: "instagram URL", platform: "Instagram", query: "https://www.instagram.com/openai/", wantPlatform: "Instagram", wantHandle: "openai"},
		{name: "tiktok handle", platform: "TikTok", query: "@openai", wantPlatform: "TikTok", wantHandle: "openai"},
		{name: "xiaohongshu profile", platform: "RedNote", query: "https://www.xiaohongshu.com/user/profile/61b46d790000000010008153", wantPlatform: "小红书", wantPlatformID: "61b46d790000000010008153"},
		{name: "youtube ID", platform: "YouTube", query: "UC_x5XG1OV2P6uZZ5FSM9Ttw", wantPlatform: "YouTube", wantPlatformID: "UC_x5XG1OV2P6uZZ5FSM9Ttw"},
		{name: "x URL", platform: "X", query: "https://x.com/openai", wantPlatform: "X", wantHandle: "openai"},
		{name: "twitter handle", platform: "Twitter", query: "@OpenAI", wantPlatform: "X", wantHandle: "OpenAI"},
		{name: "linkedin URL", platform: "LinkedIn", query: "https://www.linkedin.com/in/openai/", wantPlatform: "LinkedIn", wantHandle: "openai"},
		{name: "reddit URL", platform: "Reddit", query: "https://www.reddit.com/user/openai/", wantPlatform: "Reddit", wantHandle: "openai"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seed, err := normalizeOnlineProjectResourceSeed(test.platform, test.query, "KOL")
			if err != nil {
				t.Fatal(err)
			}
			if seed.Platform != test.wantPlatform || seed.PlatformHandle != test.wantHandle || seed.PlatformUserID != test.wantPlatformID {
				t.Fatalf("unexpected seed: %#v", seed)
			}
		})
	}
}

func TestNormalizeOnlineProjectResourceSeedRejectsInvalidAccount(t *testing.T) {
	if _, err := normalizeOnlineProjectResourceSeed("Instagram", "https://www.instagram.com/p/POST/", "KOL"); err == nil {
		t.Fatal("expected Instagram post URL to be rejected")
	}
	website, err := normalizeOnlineProjectResourceSeed("Website", "https://www.example.com/news", "媒体")
	if err != nil || website.Platform != "Website" || website.Name != "example.com" || website.PlatformURL != "https://example.com" {
		t.Fatalf("unexpected Website seed: %#v, err=%v", website, err)
	}
	if _, err := normalizeOnlineProjectResourceSeed("Website", "example.com", "KOL"); err == nil {
		t.Fatal("expected Website KOL to be rejected")
	}
}
