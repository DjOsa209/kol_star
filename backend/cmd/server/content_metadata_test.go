package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeContentTitleFitsCreativeName(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"empty", " \n\t ", ""},
		{"HTML and whitespace", "  Launch &amp;\n review\t ", "Launch & review"},
		{"preserve long title", strings.Repeat("界", 240), strings.Repeat("界", 240)},
		{"at limit", strings.Repeat("界", 512), strings.Repeat("界", 512)},
		{"over limit", strings.Repeat("a", 513), strings.Repeat("a", 512)},
		{"Unicode", strings.Repeat("新品📱", 200), strings.Repeat("新品📱", 170) + "新品"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeContentTitle(tc.input)
			if got != tc.want || !utf8.ValidString(got) || utf8.RuneCountInString(got) > 512 {
				t.Fatalf("sanitizeContentTitle() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestContentTitleFromURLFitsCreativeName(t *testing.T) {
	for _, rawURL := range []string{
		"https://example.com/" + strings.Repeat("new-product-", 30),
		"https://example.com/posts/" + strings.Repeat("1", 600),
		"https://example.com/" + strings.Repeat("新品📱", 200),
	} {
		got := contentTitleFromURL(rawURL)
		if got == "" || !utf8.ValidString(got) || utf8.RuneCountInString(got) > 512 {
			t.Errorf("contentTitleFromURL(%q) returned invalid or oversized title %q", rawURL, got)
		}
	}
}
