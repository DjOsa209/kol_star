package main

import "testing"

func TestParseCooperationPostLink(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		platform string
		postID   string
	}{
		{name: "youtube watch", value: "https://www.youtube.com/watch?v=51dNGqLoM00", platform: "YouTube", postID: "51dNGqLoM00"},
		{name: "youtube short", value: "https://youtu.be/51dNGqLoM00?t=2", platform: "YouTube", postID: "51dNGqLoM00"},
		{name: "youtube shorts", value: "https://youtube.com/shorts/51dNGqLoM00", platform: "YouTube", postID: "51dNGqLoM00"},
		{name: "tiktok", value: "https://www.tiktok.com/@creator/video/7350810998023949599", platform: "TikTok", postID: "7350810998023949599"},
		{name: "instagram", value: "https://www.instagram.com/reel/DPwhVB-jo9k/", platform: "Instagram", postID: "DPwhVB-jo9k"},
		{name: "finds url in text", value: "发布链接：https://www.instagram.com/p/DPwhVB-jo9k/ 备注", platform: "Instagram", postID: "DPwhVB-jo9k"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseCooperationPostLink(test.value)
			if err != nil {
				t.Fatal(err)
			}
			if got.Platform != test.platform || got.PostID != test.postID {
				t.Fatalf("parseCooperationPostLink() = %#v, want platform=%q postID=%q", got, test.platform, test.postID)
			}
		})
	}
}

func TestParseCooperationPostLinkRejectsUnsupportedLink(t *testing.T) {
	if _, err := parseCooperationPostLink("https://example.com/post/123"); err == nil {
		t.Fatal("expected unsupported link error")
	}
}

func TestPlatformPostMatchesLink(t *testing.T) {
	link, err := parseCooperationPostLink("https://youtu.be/51dNGqLoM00")
	if err != nil {
		t.Fatal(err)
	}
	if !platformPostMatchesLink(platformPost{PlatformPostID: "51dNGqLoM00"}, link) {
		t.Fatal("expected platform post id match")
	}
	if !platformPostMatchesLink(platformPost{PostURL: "https://www.youtube.com/watch?v=51dNGqLoM00"}, link) {
		t.Fatal("expected normalized URL match")
	}
}

func TestFindSinglePlatformItem(t *testing.T) {
	got := findSinglePlatformItem(map[string]any{
		"value": []any{
			map[string]any{"item_info": map[string]any{"item_struct": map[string]any{"aweme_id": "123"}}},
		},
	})
	if anyString(got["aweme_id"]) != "123" {
		t.Fatalf("findSinglePlatformItem() = %#v", got)
	}
}
