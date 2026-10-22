package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCreatorDiscoveryYouTubeCollectsFactsAndDeduplicatesChannels(t *testing.T) {
	searches, profiles := 0, 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("key") != "test-only-key" {
			t.Error("API key missing")
		}
		switch r.URL.Path {
		case "/search":
			searches++
			if q.Get("type") != "video" || q.Get("maxResults") != "20" || q.Get("relevanceLanguage") != "" || q.Get("regionCode") != "" {
				t.Errorf("unexpected search constraints: %v", q)
			}
			fmt.Fprint(w, `{"items":[{"id":{"videoId":"evidence"},"snippet":{"channelId":"UCone","channelTitle":"Creator","title":"Phone review","description":"Public search evidence"}},{"id":{"videoId":"other"},"snippet":{"channelId":"UCone","channelTitle":"Creator"}}]}`)
		case "/channels":
			profiles++
			fmt.Fprint(w, `{"items":[{"id":"UCone","snippet":{"title":"Creator","customUrl":"@creator","description":"Public profile","country":"IQ","defaultLanguage":"ar","thumbnails":{"high":{"url":"https://example.org/avatar.jpg"}}},"statistics":{"subscriberCount":"123000","viewCount":"987654","videoCount":"42"},"contentDetails":{"relatedPlaylists":{"uploads":"UUone"}}}]}`)
		case "/playlistItems":
			if q.Get("maxResults") != "6" || q.Get("playlistId") != "UUone" {
				t.Errorf("unexpected playlist constraint")
			}
			fmt.Fprint(w, `{"items":[{"contentDetails":{"videoId":"newer"}},{"contentDetails":{"videoId":"older"}},{"contentDetails":{"videoId":"wrongOwner"}},{"contentDetails":{"videoId":"newer"}}]}`)
		case "/videos":
			fmt.Fprint(w, `{"items":[{"id":"older","snippet":{"channelId":"UCone","title":"Older phone","publishedAt":"2026-09-01T00:00:00Z"},"statistics":{"viewCount":"150"},"contentDetails":{"duration":"PT1M2S"}},{"id":"newer","snippet":{"channelId":"UCone","title":"New phone","publishedAt":"2026-10-01T00:00:00Z"},"statistics":{"viewCount":"999","likeCount":"100","commentCount":"5"},"contentDetails":{"duration":"PT2M"}},{"id":"wrongOwner","snippet":{"channelId":"UCother","title":"Another channel"}}]}`)
		default:
			t.Errorf("unexpected API path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	var found []creatorDiscoverySnapshot
	err := discoverYouTubeChannelsWithClient(context.Background(), api.Client(), api.URL, "test-only-key", []string{" Iraq phones ", "Iraq phones", "العراق هواتف", "Baghdad tech", "fourth query"}, 5, func(candidate creatorDiscoverySnapshot) error { found = append(found, candidate); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if searches != 3 || profiles != 1 || len(found) != 1 {
		t.Fatalf("searches=%d profiles=%d candidates=%d", searches, profiles, len(found))
	}
	c := found[0]
	if c.Country != "IQ" || c.Language != "ar" || c.Handle != "@creator" || c.Followers == nil || *c.Followers != 123000 || c.VideoCount != 42 {
		t.Fatalf("incorrect channel facts: %+v", c)
	}
	if c.CollectedAt == "" || len(c.Evidence) != 7 || c.Evidence[0].Query != "Iraq phones" || c.Evidence[0].URL != "https://www.youtube.com/watch?v=evidence" {
		t.Fatalf("missing search provenance: %+v", c.Evidence)
	}
	if len(c.RecentPosts) != 2 || c.RecentPosts[0].PlatformPostID != "newer" || c.RecentPosts[1].Duration != 62 || c.RecentPosts[0].ViewCount != 999 {
		t.Fatalf("wrong recent videos: %+v", c.RecentPosts)
	}
}

func TestCreatorDiscoveryYouTubeKeepsPartialResultsAndRedactsFailures(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			if r.URL.Query().Get("q") == "fails" {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, `{"error":{"message":"secret-key https://upstream?key=secret-key","errors":[{"reason":"quotaExceeded"}]}}`)
				return
			}
			fmt.Fprint(w, `{"items":[{"id":{"videoId":"one"},"snippet":{"channelId":"UCbad","channelTitle":"Keep search result"}},{"id":{"videoId":"two"},"snippet":{"channelId":"UChidden","channelTitle":"Hidden metrics"}}]}`)
		case "/channels":
			if r.URL.Query().Get("id") == "UCbad" {
				http.Error(w, "secret-key", 500)
				return
			}
			fmt.Fprint(w, `{"items":[{"id":"UChidden","snippet":{"title":"Hidden metrics"},"statistics":{"subscriberCount":"100","hiddenSubscriberCount":true},"contentDetails":{"relatedPlaylists":{"uploads":"UUhidden"}}}]}`)
		case "/playlistItems":
			http.Error(w, "secret-key", 503)
		default:
			t.Errorf("unexpected API path %s", r.URL.Path)
		}
	}))
	defer api.Close()
	var found []creatorDiscoverySnapshot
	err := discoverYouTubeChannelsWithClient(context.Background(), api.Client(), api.URL, "secret-key", []string{"fails", "Iraq phone review"}, 5, func(candidate creatorDiscoverySnapshot) error { found = append(found, candidate); return nil })
	if err == nil || !strings.Contains(err.Error(), "配额") || strings.Contains(err.Error(), "secret-key") || strings.Contains(err.Error(), "https://upstream") {
		t.Fatalf("unexpected partial failure: %v", err)
	}
	if len(found) != 2 || found[0].Warning == "" || found[1].Warning == "" {
		t.Fatalf("partial candidates were discarded: %+v", found)
	}
	if found[1].Followers != nil || found[1].Country != "" || found[1].Language != "" {
		t.Fatalf("invented hidden/unknown facts: %+v", found[1])
	}
	for _, c := range found {
		if strings.Contains(c.Warning, "secret-key") {
			t.Fatal("warning leaked API key")
		}
	}
}

func TestCreatorDiscoveryYouTubeCancellationAndCallbackFailureStopWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := discoverYouTubeChannelsWithClient(ctx, http.DefaultClient, "http://unused.invalid", "test-only-key", []string{"phones"}, 1, func(creatorDiscoverySnapshot) error {
		t.Fatal("cancelled context must not emit candidates")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	requests := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path == "/search" {
			fmt.Fprint(w, `{"items":[{"id":{"videoId":"one"},"snippet":{"channelId":"UCone"}},{"id":{"videoId":"two"},"snippet":{"channelId":"UCtwo"}}]}`)
			return
		}
		fmt.Fprint(w, `{"items":[]}`)
	}))
	defer api.Close()
	callbackErr := errors.New("storage unavailable")
	err = discoverYouTubeChannelsWithClient(context.Background(), api.Client(), api.URL, "test-only-key", []string{"phones"}, 2, func(creatorDiscoverySnapshot) error { return callbackErr })
	if !errors.Is(err, callbackErr) || requests != 2 {
		t.Fatalf("must stop on failed callback: requests=%d error=%v", requests, err)
	}
}

// Opt-in read-only acceptance with the configured service credentials. It never
// writes candidates or resources and does not print credentials or API URLs.
func TestCreatorDiscoveryYouTubeLive(t *testing.T) {
	if os.Getenv("KOL_LIVE_YOUTUBE_DISCOVERY") != "1" {
		t.Skip("set KOL_LIVE_YOUTUBE_DISCOVERY=1 for real YouTube acceptance")
	}
	cfg, _, err := loadConfig()
	if err != nil {
		t.Fatal("unable to load local service configuration")
	}
	db, err := openDB(cfg.MySQL)
	if err != nil {
		t.Fatal("unable to open local service database")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	effective := newApp(db, cfg).effectivePlatformAPIConfig(ctx)
	count := 0
	err = discoverYouTubeChannels(ctx, effective, []string{"مراجعة هواتف العراق", "تقنية هواتف بغداد", "Iraq smartphone review"}, 12, func(candidate creatorDiscoverySnapshot) error {
		count++
		data, marshalErr := json.Marshal(candidate)
		if marshalErr != nil {
			return marshalErr
		}
		t.Log(string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("real YouTube search returned no candidates")
	}
	t.Logf("collected %d real YouTube channel snapshots without database writes", count)
}
