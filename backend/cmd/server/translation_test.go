package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestContainsHan(t *testing.T) {
	if !containsHan("新品合作 notes") {
		t.Fatal("expected Chinese text to be detected")
	}
	if containsHan("Creator notes 2026") {
		t.Fatal("did not expect English text to be detected as Chinese")
	}
}

func TestLocalizedSourceHashIgnoresOuterWhitespace(t *testing.T) {
	if localizedSourceHash(" 新品合作 ") != localizedSourceHash("新品合作") {
		t.Fatal("outer whitespace should not invalidate a translation")
	}
	if localizedSourceHash("新品合作") == localizedSourceHash("长期合作") {
		t.Fatal("different source text must invalidate a translation")
	}
}

func TestRequestAITranslations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"content": `{"translations":[{"id":0,"text":"New product collaboration"}]}`,
				},
			}},
		})
	}))
	defer server.Close()

	translations, err := requestAITranslations(context.Background(), assistantAIModel{
		Model:   "test-model",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
		Timeout: time.Second,
	}, []translationSource{{FieldKey: "notes", Text: "新品合作"}}, englishLocale)
	if err != nil {
		t.Fatal(err)
	}
	if got := translations[0]; got != "New product collaboration" {
		t.Fatalf("translation = %q", got)
	}
}

func TestSortedUniqueInts(t *testing.T) {
	got := sortedUniqueInts([]int{3, 0, 2, 3, -1, 1})
	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
