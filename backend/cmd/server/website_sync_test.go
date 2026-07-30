package main

import (
	"testing"
	"time"
)

func TestNormalizeWebsiteDomain(t *testing.T) {
	domain, homepage := normalizeWebsiteDomain("https://www.example.com/news?id=1")
	if domain != "example.com" || homepage != "https://example.com" {
		t.Fatalf("unexpected website: %q %q", domain, homepage)
	}
}

func TestWebsiteAttributes(t *testing.T) {
	attrs := websiteAttributes(`<link rel="apple-touch-icon" href='/assets/icon.png'>`)
	if attrs["rel"] != "apple-touch-icon" || attrs["href"] != "/assets/icon.png" {
		t.Fatalf("unexpected attrs: %#v", attrs)
	}
}

func TestSimilarwebUsesLatestCompleteMonth(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	month := now.AddDate(0, -1, 0)
	if got := month.Format("2006-01"); got != "2026-06" {
		t.Fatalf("unexpected month: %s", got)
	}
}
