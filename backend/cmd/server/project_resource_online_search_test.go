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
		{name: "youtube ID", platform: "YouTube", query: "UC_x5XG1OV2P6uZZ5FSM9Ttw", wantPlatform: "YouTube", wantPlatformID: "UC_x5XG1OV2P6uZZ5FSM9Ttw"},
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
	if _, err := normalizeOnlineProjectResourceSeed("Website", "example.com", "媒体"); err == nil {
		t.Fatal("expected unsupported platform to be rejected")
	}
}
