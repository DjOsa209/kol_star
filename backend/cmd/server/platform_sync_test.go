package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSelectedSyncPlatforms(t *testing.T) {
	tests := []struct {
		name      string
		body      map[string]any
		wantAll   bool
		wantCount int
		wantError bool
	}{
		{name: "missing means all", body: map[string]any{}, wantAll: true},
		{name: "empty means all", body: map[string]any{"platforms": []any{}}, wantAll: true},
		{name: "normalizes aliases", body: map[string]any{"platforms": []any{"youtube", "ins"}}, wantCount: 2},
		{name: "supports extended tikhub platforms", body: map[string]any{"platforms": []any{"LinkedIn", "Reddit"}}, wantCount: 2},
		{name: "normalizes xiaohongshu aliases", body: map[string]any{"platforms": []any{"RedNote", "xhs"}}, wantCount: 1},
		{name: "rejects unsupported platform", body: map[string]any{"platforms": []any{"Pinterest"}}, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := selectedSyncPlatforms(test.body)
			if (err != nil) != test.wantError {
				t.Fatalf("selectedSyncPlatforms() error = %v, wantError %v", err, test.wantError)
			}
			if test.wantAll && len(got) != 0 {
				t.Fatalf("selectedSyncPlatforms() = %v, want all platforms", got)
			}
			if len(got) != test.wantCount {
				t.Fatalf("selectedSyncPlatforms() count = %d, want %d", len(got), test.wantCount)
			}
		})
	}
}

func TestOptionalConfigValue(t *testing.T) {
	tests := []struct {
		value any
		want  string
	}{
		{value: nil, want: ""},
		{value: "<nil>", want: ""},
		{value: " http://127.0.0.1:7890 ", want: "http://127.0.0.1:7890"},
	}
	for _, test := range tests {
		if got := optionalConfigValue(test.value); got != test.want {
			t.Fatalf("optionalConfigValue(%v) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestTikHubAPIBaseURL(t *testing.T) {
	t.Setenv("TIKHUB_API_BASE_URL", "https://api.tikhub.io/")
	if got := tikHubAPIBaseURL(); got != "https://api.tikhub.io" {
		t.Fatalf("tikHubAPIBaseURL() = %q", got)
	}
}

func TestTikHubGETFallsBackFromIOToDevOnTimeout(t *testing.T) {
	t.Setenv("TIKHUB_API_BASE_URL", "https://api.tikhub.io")
	var hosts []string
	client := &http.Client{Transport: imageRoundTripper(func(req *http.Request) (*http.Response, error) {
		hosts = append(hosts, req.URL.Host)
		if req.URL.Host == "api.tikhub.io" {
			return nil, context.DeadlineExceeded
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"code":200,"data":{"id":"ok"}}`)),
		}, nil
	})}

	data, err := tikhubGET(context.Background(), client, "test-key", "/instagram/v1/fetch_post_by_url", nil)
	if err != nil {
		t.Fatal(err)
	}
	if data["id"] != "ok" || len(hosts) != 2 || hosts[1] != "api.tikhub.dev" {
		t.Fatalf("TikHub fallback was not used: data=%#v hosts=%v", data, hosts)
	}
}
