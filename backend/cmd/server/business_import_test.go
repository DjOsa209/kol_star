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

func TestImportCooperationRowKeyNormalizesTrackingParameters(t *testing.T) {
	base := map[string]any{
		"influencer":       "https://www.instagram.com/creator/?utm_source=sheet",
		"platform":         "Instagram",
		"deliverableLinks": "https://www.instagram.com/p/POST/?utm_campaign=launch",
		"cooperationType":  "付费合作",
		"quoteAmount":      float64(1200),
		"owner":            "Mia",
		"vendor":           "Vendor A",
	}
	same := map[string]any{}
	for key, value := range base {
		same[key] = value
	}
	same["influencer"] = "https://www.instagram.com/creator/"
	same["deliverableLinks"] = "https://www.instagram.com/p/POST/"

	first, err := importCooperationRowKey(base)
	if err != nil {
		t.Fatal(err)
	}
	second, err := importCooperationRowKey(same)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("equivalent rows must share a dedupe key:\n%s\n%s", first, second)
	}

	same["quoteAmount"] = float64(1500)
	changed, err := importCooperationRowKey(same)
	if err != nil {
		t.Fatal(err)
	}
	if changed == first {
		t.Fatal("different cooperation data must not be treated as the same row")
	}
}
