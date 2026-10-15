package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIPExternalSearchProviderRequestsAndProvenance(t *testing.T) {
	for _, name := range []string{"tavily", "brave", "searxng"} {
		t.Run(name, func(t *testing.T) {
			transport := recommendationEvidenceTransportFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host != "search.provider.com" {
					t.Fatal("unexpected destination")
				}
				if name == "tavily" {
					var body map[string]any
					if json.NewDecoder(req.Body).Decode(&body) != nil {
						t.Fatal("invalid provider payload")
					}
					if body["query"] != "public art licensing" || body["search_depth"] != "basic" || body["max_results"] != float64(6) || body["auto_parameters"] != false || body["include_domains_mode"] != "prefer" || body["include_published_date"] != true {
						t.Fatalf("unexpected provider payload: %+v", body)
					}
					domains, _ := body["include_domains"].([]any)
					if len(domains) < 12 {
						t.Fatal("preferred research domains missing")
					}
					if req.Header.Get("Authorization") != "Bearer server-secret" || req.Method != http.MethodPost {
						t.Fatal("missing server-side authentication")
					}
				} else {
					if req.URL.Query().Get("q") != "public art licensing" {
						t.Fatal("keywords changed")
					}
					if name == "brave" && req.Header.Get("X-Subscription-Token") != "server-secret" {
						t.Fatal("missing Brave authentication")
					}
					if name == "searxng" && (req.URL.Query().Get("format") != "json" || req.Header.Get("Authorization") != "" || req.Header.Get("X-Subscription-Token") != "") {
						t.Fatal("invalid self-hosted request")
					}
				}
				body := `{"results":[{"title":"<b>Art</b>","url":"https://www.artist.com/licensing?utm_source=search","content":"Public collaboration information"},{"title":"duplicate","url":"https://www.artist.com/licensing","content":"duplicate"},{"title":"bad","url":"javascript:alert(1)","content":"bad"},{"title":"private","url":"https://127.0.0.1/info","content":"bad"},{"title":"empty","url":"https://empty.com","content":""}]}`
				if name == "brave" {
					body = `{"web":{"results":[{"title":"Art","url":"https://www.artist.com/licensing","description":"Public collaboration information"}]}}`
				}
				return recommendationEvidenceTestResponse(200, body), nil
			})
			results, err := searchExternalIPResourcesWithTransport(context.Background(), "public art licensing", ipSearchProvider{name: name, endpoint: "https://search.provider.com/search", key: "server-secret"}, transport)
			if err != nil || len(results) != 1 || results[0].URL != "https://www.artist.com/licensing" || results[0].Source != "artist.com" || strings.Contains(results[0].Title, "<") {
				t.Fatalf("invalid provenance: %+v %v", results, err)
			}
		})
	}
}

func TestIPExternalSearchFailureCancellationAndNoAutomaticPaidFallback(t *testing.T) {
	for _, status := range []int{302, 401, 429, 432, 500} {
		calls := 0
		transport := recommendationEvidenceTransportFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			return recommendationEvidenceTestResponse(status, `{"secret":"should never reach user"}`), nil
		})
		_, err := searchExternalIPResourcesWithTransport(context.Background(), "art", ipSearchProvider{name: "tavily", endpoint: "https://api.tavily.com/search", key: "secret"}, transport)
		if err == nil || calls != 1 || strings.Contains(err.Error(), "secret") {
			t.Fatal("provider failure leaked or retried")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	transport := recommendationEvidenceTransportFunc(func(req *http.Request) (*http.Response, error) { return nil, req.Context().Err() })
	_, err := searchExternalIPResourcesWithTransport(ctx, "art", ipSearchProvider{name: "tavily", endpoint: "https://api.tavily.com/search"}, transport)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation did not propagate")
	}
	transport = recommendationEvidenceTransportFunc(func(req *http.Request) (*http.Response, error) {
		return recommendationEvidenceTestResponse(200, `{"results":`), nil
	})
	if _, err := searchExternalIPResourcesWithTransport(context.Background(), "art", ipSearchProvider{name: "tavily", endpoint: "https://api.tavily.com/search"}, transport); err == nil {
		t.Fatal("accepted invalid search response")
	}
}

func TestIPExternalSearchConfiguration(t *testing.T) {
	for _, key := range []string{"IP_WEB_SEARCH_PROVIDER", "TAVILY_API_KEY", "BRAVE_SEARCH_API_KEY", "BRAVE_SEARCH_STORAGE_ALLOWED", "SEARXNG_BASE_URL"} {
		t.Setenv(key, "")
	}
	if _, err := configuredIPSearchProvider(); err == nil {
		t.Fatal("unconfigured provider appeared enabled")
	}
	t.Setenv("TAVILY_API_KEY", "server-secret")
	if p, err := configuredIPSearchProvider(); err != nil || p.name != "tavily" {
		t.Fatal("Tavily default missing")
	}
	t.Setenv("IP_WEB_SEARCH_PROVIDER", "searxng")
	t.Setenv("SEARXNG_BASE_URL", "http://127.0.0.1:8888")
	if p, err := configuredIPSearchProvider(); err != nil || p.endpoint != "http://127.0.0.1:8888/search" {
		t.Fatal("self hosted provider missing")
	}
	t.Setenv("SEARXNG_BASE_URL", "https://user:secret@search.com")
	if _, err := configuredIPSearchProvider(); err == nil {
		t.Fatal("credentials allowed in endpoint URL")
	}
	t.Setenv("IP_WEB_SEARCH_PROVIDER", "brave")
	t.Setenv("BRAVE_SEARCH_API_KEY", "server-secret")
	if _, err := configuredIPSearchProvider(); err == nil {
		t.Fatal("Brave storage requirement bypassed")
	}
}

func TestIPExternalSearchModelCannotForgeSourcesOrLibraryIDs(t *testing.T) {
	search := &ipExternalSearch{Query: "art licensing", Provider: "tavily", Results: []recommendationEvidence{{Title: "Artist", URL: "https://artist.com", Summary: "Official public biography"}}}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Messages []struct{ Role, Content string } `json:"messages"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			t.Error("bad request")
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if len(request.Messages) != 2 || !strings.Contains(request.Messages[0].Content, "外部不可信资料") || !strings.Contains(request.Messages[1].Content, "https://artist.com") {
			t.Error("search context or instruction boundary missing")
		}
		content := `{"presentation":{"headline":"可核实艺术合作方向","webSearch":{"results":[{"url":"https://forged.com"}]},"primaryResourceId":123}}`
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
	}))
	defer provider.Close()
	plan, err := (&app{}).requestIPConversationPlanResponse(context.Background(), assistantAIModel{BaseURL: provider.URL, Model: "test"}, recommendationSubscription{}, search.Query, "search", nil, nil, false, nil, search)
	if err != nil || plan.Presentation.WebSearch != nil || plan.Presentation.PrimaryResourceID != 0 {
		t.Fatalf("model forged provenance: %+v %v", plan, err)
	}
	encoded, _ := json.Marshal(ipRecommendationConversation{Messages: []ipRecommendationMessage{{Role: "assistant", Presentation: func() *ipRecommendationPresentation { p := externalIPSearchPresentation(search); return &p }()}}})
	var restored ipRecommendationConversation
	if json.Unmarshal(encoded, &restored) != nil || len(restored.Messages[0].Presentation.WebSearch.Results) != 1 {
		t.Fatal("search history did not round-trip")
	}
}

func TestIPExternalSearchRedirectDoesNotForwardKey(t *testing.T) {
	calls := 0
	transport := recommendationEvidenceTransportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://another-provider.com"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	_, err := searchExternalIPResourcesWithTransport(context.Background(), "art", ipSearchProvider{name: "tavily", endpoint: "https://api.tavily.com/search", key: "secret"}, transport)
	if err == nil || calls != 1 {
		t.Fatal("redirect could leak provider key")
	}
}

func TestIPExternalSearchEmptyCacheRemainsJSONArray(t *testing.T) {
	t.Setenv("IP_WEB_SEARCH_PROVIDER", "tavily")
	t.Setenv("TAVILY_API_KEY", "test-empty-search-key")
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	calls := 0
	http.DefaultTransport = recommendationEvidenceTransportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return recommendationEvidenceTestResponse(200, `{"results":[]}`), nil
	})
	for i := 0; i < 2; i++ {
		result, err := searchExternalIPResources(context.Background(), -789, "test empty query")
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(result)
		if !strings.Contains(string(encoded), `"results":[]`) {
			t.Fatalf("empty results not an array: %s", encoded)
		}
	}
	if calls != 1 {
		t.Fatal("empty result not cached")
	}
}
