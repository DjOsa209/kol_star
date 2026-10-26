package main

import (
	"strings"
	"testing"
)

func TestIPAssetLinksValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		profile map[string]any
		valid   bool
	}{
		{"legacy", nil, true},
		{"empty", map[string]any{"assetLinks": ""}, true},
		{"multi link with blanks and anchor", map[string]any{"assetLinks": " https://example.com/hero#pose \r\n\nhttps://example.com/folder?x=1&y=2"}, true},
		{"uppercase", map[string]any{"assetLinks": "HTTPS://example.com/assets"}, true},
		{"script", map[string]any{"assetLinks": "javascript:alert(1)"}, false},
		{"data", map[string]any{"assetLinks": "data:text/html,hello"}, false},
		{"relative", map[string]any{"assetLinks": "//example.com/folder"}, false},
		{"credentials", map[string]any{"assetLinks": "https://user:secret@example.com"}, false},
		{"mixed valid and invalid", map[string]any{"assetLinks": "https://example.com/assets\nfile:///tmp/image.png"}, false},
		{"space", map[string]any{"assetLinks": "https://example.com/asset one"}, false},
		{"backslash", map[string]any{"assetLinks": `https://example.com\@other.com`}, false},
		{"long", map[string]any{"assetLinks": "https://example.com/" + strings.Repeat("x", 2048)}, false},
		{"too many", map[string]any{"assetLinks": strings.Repeat("https://example.com/\n", 21)}, false},
		{"wrong type", map[string]any{"assetLinks": []string{"https://example.com/"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issue := ipAssetLinksIssue(tc.profile)
			if (issue == "") != tc.valid {
				t.Fatalf("issue=%q", issue)
			}
			row := ipResourceInput{IPType: "动画", Markets: []string{"全球"}, Profile: tc.profile}
			if (ipImportOptionIssue(row) == "") != tc.valid {
				t.Fatal("import bypassed link validation")
			}
		})
	}
	for _, source := range ipContentSources("ip_resource", map[string]any{"profile": map[string]any{"assetLinks": "https://example.com/private-library"}}) {
		if source.FieldKey == "profile.assetLinks" {
			t.Fatal("asset URLs sent for translation")
		}
	}
}
