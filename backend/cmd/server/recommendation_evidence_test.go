package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

type recommendationEvidenceTransportFunc func(*http.Request) (*http.Response, error)

func (fn recommendationEvidenceTransportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func recommendationEvidenceTestResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestRecommendationEvidenceWikidataDefaultAndExactMatch(t *testing.T) {
	calls := 0
	transport := recommendationEvidenceTransportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.Scheme != "https" || req.URL.Host != "www.wikidata.org" || req.URL.Path != "/w/api.php" {
			t.Fatalf("unexpected network destination: %s", req.URL)
		}
		if req.Header.Get("X-Subscription-Token") != "" {
			t.Fatal("Brave key leaked to Wikidata")
		}
		if req.URL.Query().Get("action") != "wbsearchentities" || req.URL.Query().Get("maxlag") != "5" {
			t.Fatalf("unexpected Wikidata request: %s", req.URL)
		}
		switch req.URL.Query().Get("search") {
		case "索尼克Sonic the Hedgehog":
			return recommendationEvidenceTestResponse(http.StatusOK, `{"search":[{"id":"Q1","label":"Sonic the Hedgehog","description":"fictional character","match":{"text":"Sonic the Hedgehog"}}]}`), nil
		case "Sonic the Hedgehog":
			return recommendationEvidenceTestResponse(http.StatusOK, `{"search":[{"id":"Q2","label":"Sonic the Hedgehog","description":"video game character and mascot","match":{"text":"Sonic the Hedgehog"}}]}`), nil
		default:
			t.Fatalf("unexpected public search term: %q", req.URL.Query().Get("search"))
			return nil, nil
		}
	})
	evidence, status := fetchIPRecommendationEvidenceWithTransport(context.Background(), "test-索尼克Sonic the Hedgehog", "游戏", "unused-key", false, transport)
	if status != "available" || len(evidence) != 1 || calls != 2 {
		t.Fatalf("status=%q evidence=%#v calls=%d", status, evidence, calls)
	}
	if evidence[0].URL != "https://www.wikidata.org/wiki/Q2" || evidence[0].Source != "Wikidata" || evidence[0].Summary != "video game character and mascot" {
		t.Fatalf("unexpected external evidence: %#v", evidence[0])
	}
}

func TestRecommendationEvidenceBraveRequiresStorageRight(t *testing.T) {
	braveCalls := 0
	transport := recommendationEvidenceTransportFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "api.search.brave.com" {
			braveCalls++
			return recommendationEvidenceTestResponse(http.StatusOK, `{"web":{"results":[]}}`), nil
		}
		return recommendationEvidenceTestResponse(http.StatusOK, `{"search":[]}`), nil
	})
	_, status := fetchIPRecommendationEvidenceWithTransport(context.Background(), "Sonic", "游戏", "present-key", false, transport)
	if status != "none" || braveCalls != 0 {
		t.Fatalf("Brave used without storage rights: status=%q calls=%d", status, braveCalls)
	}
}

func TestRecommendationEvidenceBraveUsesPublicIPNameVariant(t *testing.T) {
	transport := recommendationEvidenceTransportFunc(func(req *http.Request) (*http.Response, error) {
		query := req.URL.Query().Get("q")
		if !strings.Contains(query, `"Sonic the Hedgehog"`) || strings.Contains(query, "test-") {
			t.Fatalf("query should use public IP name only: %q", query)
		}
		return recommendationEvidenceTestResponse(http.StatusOK, `{"web":{"results":[]}}`), nil
	})
	_, status := fetchBraveRecommendationEvidence(context.Background(), "test-索尼克Sonic the Hedgehog", "key", transport)
	if status != "none" {
		t.Fatalf("status=%q", status)
	}
}

func TestRecommendationEvidenceBraveCitationsAreBoundedAndSafe(t *testing.T) {
	transport := recommendationEvidenceTransportFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "www.wikidata.org":
			return recommendationEvidenceTestResponse(http.StatusOK, `{"search":[{"id":"Q2","label":"Sonic","description":"video game franchise","match":{"text":"Sonic"}}]}`), nil
		case "api.search.brave.com":
			if req.URL.Scheme != "https" || req.URL.Path != "/res/v1/web/search" || req.Header.Get("X-Subscription-Token") != "key" {
				t.Fatalf("bad Brave request: %s", req.URL)
			}
			if !strings.Contains(req.URL.Query().Get("q"), `"Sonic"`) {
				t.Fatalf("IP name missing from Brave query: %q", req.URL.Query().Get("q"))
			}
			return recommendationEvidenceTestResponse(http.StatusOK, `{"web":{"results":[
				{"title":"<b>Brand</b> collaboration","url":"javascript:alert(1)","description":"unsafe"},
				{"title":"Launch &amp; campaign","url":"https://WWW.Example.ORG/campaign?utm_source=feed&id=7#top","description":"<b>Sonic</b> appeared in a launch campaign.","page_age":"2025-03-04T12:00:00Z"},
				{"title":"Duplicate","url":"https://www.example.org/campaign?id=7","description":"same page"},
				{"title":"Official licensing","url":"https://licensing.example.org/news","description":"Sonic licensing announcement"},
				{"title":"Third article","url":"https://news.example.org/article","description":"Another article"}
			]}}`), nil
		default:
			t.Fatalf("unexpected network destination: %s", req.URL)
			return nil, nil
		}
	})
	evidence, status := fetchIPRecommendationEvidenceWithTransport(context.Background(), "Sonic", "游戏", "key", true, transport)
	if status != "available" || len(evidence) != 3 {
		t.Fatalf("status=%q evidence=%#v", status, evidence)
	}
	if evidence[1].URL != "https://www.example.org/campaign?id=7" || evidence[1].Source != "example.org" || evidence[1].Title != "Launch & campaign" || evidence[1].Summary != "Sonic appeared in a launch campaign." || evidence[1].PublishedAt != "" {
		t.Fatalf("unexpected canonical citation: %#v", evidence[1])
	}
	if evidence[2].URL != "https://licensing.example.org/news" {
		t.Fatalf("duplicate or unsafe citation survived: %#v", evidence[2])
	}
}

func TestRecommendationEvidenceWikidataDisambiguation(t *testing.T) {
	cases := []struct {
		name        string
		ipType      string
		body        string
		wantStatus  string
		wantItemURL string
	}{
		{
			name:   "game character outranks individual releases",
			ipType: "游戏",
			body: `{"search":[
				{"id":"Q1","label":"Sonic","description":"1991 video game","match":{"text":"Sonic"}},
				{"id":"Q2","label":"Sonic","description":"video game character and franchise mascot","match":{"text":"Sonic"}},
				{"id":"Q3","label":"Sonic","description":"2006 video game","match":{"text":"Sonic"}}
			]}`,
			wantStatus:  "available",
			wantItemURL: "https://www.wikidata.org/wiki/Q2",
		},
		{
			name:   "film is selected for film IP",
			ipType: "影视",
			body: `{"search":[
				{"id":"Q2","label":"Sonic","description":"video game character and franchise mascot","match":{"text":"Sonic"}},
				{"id":"Q4","label":"Sonic","description":"2020 film","match":{"text":"Sonic"}}
			]}`,
			wantStatus:  "available",
			wantItemURL: "https://www.wikidata.org/wiki/Q4",
		},
		{
			name:   "same score remains ambiguous",
			ipType: "游戏",
			body: `{"search":[
				{"id":"Q1","label":"Sonic","description":"1991 video game","match":{"text":"Sonic"}},
				{"id":"Q3","label":"Sonic","description":"2006 video game","match":{"text":"Sonic"}}
			]}`,
			wantStatus: "none",
		},
		{
			name:       "wrong IP type excluded",
			ipType:     "游戏",
			body:       `{"search":[{"id":"Q5","label":"Sonic","description":"American singer","match":{"text":"Sonic"}}]}`,
			wantStatus: "none",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transport := recommendationEvidenceTransportFunc(func(*http.Request) (*http.Response, error) {
				return recommendationEvidenceTestResponse(http.StatusOK, tc.body), nil
			})
			evidence, status := searchWikidataRecommendationTerm(context.Background(), "Sonic", tc.ipType, transport)
			if status != tc.wantStatus {
				t.Fatalf("status=%q evidence=%#v", status, evidence)
			}
			if tc.wantItemURL != "" && (len(evidence) != 1 || evidence[0].URL != tc.wantItemURL) {
				t.Fatalf("unexpected Wikidata item: %#v", evidence)
			}
		})
	}
}

func TestRecommendationEvidenceProviderErrorsAndUnsafeURLs(t *testing.T) {
	transport := recommendationEvidenceTransportFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network unavailable")
	})
	evidence, status := fetchIPRecommendationEvidenceWithTransport(context.Background(), "Sonic", "游戏", "", false, transport)
	if evidence != nil || status != "unavailable" {
		t.Fatalf("status=%q evidence=%#v", status, evidence)
	}
	for _, raw := range []string{"http://example.org/x", "https://127.0.0.1/x", "https://localhost/x", "https://user:pass@example.org/x", "https://example.org:8080/x", "https://example.local/x"} {
		if _, _, ok := canonicalRecommendationEvidenceURL(raw); ok {
			t.Fatalf("unsafe URL accepted: %s", raw)
		}
	}
}

func TestRecommendationEvidenceCacheBoundAndExpiry(t *testing.T) {
	cache := &recommendationEvidenceCacheStore{}
	now := time.Unix(1000, 0)
	cache.put("one", []recommendationEvidence{{Title: "Sonic"}}, "available", now, time.Hour)
	items, status, ok := cache.get("one", now.Add(59*time.Minute))
	if !ok || status != "available" || len(items) != 1 {
		t.Fatalf("cache hit failed: %#v %q %v", items, status, ok)
	}
	items[0].Title = "mutated"
	stored, _, ok := cache.get("one", now.Add(59*time.Minute))
	if !ok || stored[0].Title != "Sonic" {
		t.Fatal("cache returned mutable storage")
	}
	if _, _, ok := cache.get("one", now.Add(time.Hour)); ok {
		t.Fatal("expired evidence remained cached")
	}
	for index := range 513 {
		cache.put(strconv.Itoa(index), nil, "none", now, time.Hour)
	}
	if len(cache.entries) != 512 {
		t.Fatalf("cache grew beyond bound: %d", len(cache.entries))
	}
}

func TestRecommendationEvidenceAnimationAcceptsFranchiseAndComic(t *testing.T) {
	cases := []struct {
		name, body, id string
	}{
		{"宝可梦", `{"search":[{"id":"Q864","label":"Pokémon","description":"Japanese media franchise","match":{"text":"宝可梦"}}]}`, "Q864"},
		{"加菲猫", `{"search":[{"id":"Q72533","label":"Garfield","description":"comic strip created by Jim Davis","match":{"text":"加菲猫"}}]}`, "Q72533"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transport := recommendationEvidenceTransportFunc(func(*http.Request) (*http.Response, error) {
				return recommendationEvidenceTestResponse(http.StatusOK, tc.body), nil
			})
			items, status := searchWikidataRecommendationTerm(context.Background(), tc.name, "动画", transport)
			if status != "available" || len(items) != 1 || items[0].URL != "https://www.wikidata.org/wiki/"+tc.id {
				t.Fatalf("status=%q evidence=%#v", status, items)
			}
		})
	}
}
