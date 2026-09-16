package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCaptureRequestBodyPreservesLargeJSONForHandler(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"projectId": 43,
		"rows": []map[string]any{{
			"notes": strings.Repeat("x", (64<<10)+1),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/business/cooperations/import", strings.NewReader(string(payload)))
	request.Header.Set("Content-Type", "application/json")

	logged := captureRequestBody(request)
	if !strings.HasSuffix(logged, "...[truncated]") {
		t.Fatalf("large request log was not marked as truncated")
	}
	if projectID := intField(readBody(request), "projectId"); projectID != 43 {
		t.Fatalf("project id after request logging = %d, want 43", projectID)
	}
}
