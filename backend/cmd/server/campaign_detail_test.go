package main

import "testing"

func TestNormalizeEditableContentPlatform(t *testing.T) {
	tests := map[string]string{
		"tiktok":   "TikTok",
		"INS":      "Instagram",
		"twitter":  "X",
		"LinkedIn": "LinkedIn",
		"网站":       "Website",
		"unknown":  "",
	}
	for input, want := range tests {
		if got := normalizeEditableContentPlatform(input); got != want {
			t.Fatalf("normalizeEditableContentPlatform(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestReplaceCooperationContentURL(t *testing.T) {
	raw := "https://example.com/one\nhttps://example.com/two"
	got := replaceCooperationContentURL(
		raw,
		"https://example.com/two",
		"https://example.com/updated",
	)
	want := "https://example.com/one\nhttps://example.com/updated"
	if got != want {
		t.Fatalf("replaceCooperationContentURL() = %q, want %q", got, want)
	}
}
