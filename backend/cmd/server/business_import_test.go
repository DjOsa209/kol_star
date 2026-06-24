package main

import "testing"

func TestNormalizeImportedCooperationLink(t *testing.T) {
	got, err := normalizeImportedCooperationLink("https://example.com/review?utm_source=x&keep=1#section")
	if err != nil {
		t.Fatalf("normalize link: %v", err)
	}
	if want := "https://example.com/review?keep=1"; got != want {
		t.Fatalf("normalized link = %q, want %q", got, want)
	}

	if _, err := normalizeImportedCooperationLink("not-a-link"); err == nil {
		t.Fatal("expected invalid link to be rejected")
	}
}

func TestImportedLinkWebsite(t *testing.T) {
	got := importedLinkWebsite("https://www.youtube.com/watch?v=abc&utm_source=sheet")
	if want := "https://www.youtube.com"; got != want {
		t.Fatalf("website = %q, want %q", got, want)
	}
}

func TestImportedReleaseDate(t *testing.T) {
	cases := map[string]any{
		"2025-08-13": "2025-08-13",
		"07/31/25":   "2025-07-31",
		"08/13/25":   "2025-08-13",
		"10-25-09":   "2009-10-25",
		"not a date": nil,
	}
	for input, want := range cases {
		if got := importedReleaseDate(input); got != want {
			t.Fatalf("importedReleaseDate(%q) = %#v, want %#v", input, got, want)
		}
	}
}

func TestNormalizePlatformResourceName(t *testing.T) {
	if got := normalizePlatformResourceName("Chigz Tech · Official"); got != "chigztechofficial" {
		t.Fatalf("normalized name = %q", got)
	}
}
