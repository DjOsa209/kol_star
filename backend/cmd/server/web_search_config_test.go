package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestWebSearchYAMLConfigAndLegacyEnvironmentFallback(t *testing.T) {
	t.Setenv("WEB_SEARCH_PROVIDER", "")
	t.Setenv("WEB_SEARCH_TAVILY_API_KEY", "")
	t.Setenv("IP_WEB_SEARCH_PROVIDER", "searxng")
	t.Setenv("TAVILY_API_KEY", "test-only-legacy-env-key")
	v := viper.New()
	v.SetConfigType("yaml")
	setConfigDefaults(v)
	if err := v.ReadConfig(strings.NewReader("web_search:\n  provider: tavily\n  tavily_api_key: test-only-yaml-key\n")); err != nil {
		t.Fatal(err)
	}
	cfg, err := decodeConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := configuredIPSearchProvider(cfg.WebSearch)
	if err != nil || provider.name != "tavily" || provider.key != "test-only-yaml-key" {
		t.Fatal("YAML search configuration was not used")
	}
	provider, err = configuredIPSearchProvider(WebSearchConfig{Provider: "tavily"})
	if err != nil || provider.key != "test-only-legacy-env-key" {
		t.Fatal("legacy environment fallback was lost")
	}
	t.Setenv("TAVILY_API_KEY", "")
	if _, err := configuredIPSearchProvider(WebSearchConfig{Provider: "tavily"}); err == nil {
		t.Fatal("empty YAML key appeared ready")
	}
}

func TestWebSearchReadsUpdatedYAMLAndUsesKeyServerSide(t *testing.T) {
	for _, key := range []string{"WEB_SEARCH_PROVIDER", "WEB_SEARCH_TAVILY_API_KEY", "IP_WEB_SEARCH_PROVIDER", "TAVILY_API_KEY"} {
		t.Setenv(key, "")
	}
	file := filepath.Join(t.TempDir(), "config.yaml")
	write := func(key string) {
		t.Helper()
		if err := os.WriteFile(file, []byte("web_search:\n  provider: tavily\n  tavily_api_key: "+key+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	v := viper.New()
	v.SetConfigFile(file)
	setConfigDefaults(v)
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	wantKey := ""
	a := &app{}
	transport := recommendationEvidenceTransportFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "Bearer "+wantKey {
			t.Error("updated YAML key did not reach provider")
		}
		return recommendationEvidenceTestResponse(200, `{"results":[]}`), nil
	})
	for index, key := range []string{"test-only-first-yaml-key", "test-only-second-yaml-key"} {
		write(key)
		if err := v.ReadInConfig(); err != nil {
			t.Fatal(err)
		}
		cfg, err := decodeConfig(v)
		if err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			a.config.Store(cfg)
		} else if err := a.reloadConfig(cfg); err != nil {
			t.Fatal(err)
		}
		wantKey = key
		http.DefaultTransport = transport
		result, err := searchExternalIPResources(context.Background(), -98765, "yaml config smoke", a.Config().WebSearch)
		http.DefaultTransport = original
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(result)
		if strings.Contains(string(encoded), key) {
			t.Fatal("server-side key appeared in search result")
		}
	}
}
