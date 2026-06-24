package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidPassword(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "letters and digits", value: "admin123", want: true},
		{name: "upper and lower letters", value: "Password", want: true},
		{name: "letters and symbols", value: "admin!!!", want: true},
		{name: "digits only", value: "12345678", want: false},
		{name: "too short", value: "a123456", want: false},
		{name: "too long", value: "Admin12345678901234", want: false},
		{name: "rejects non ascii", value: "密码Admin123", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validPassword(test.value); got != test.want {
				t.Fatalf("validPassword(%q) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}

func TestDetectSecurityClient(t *testing.T) {
	userAgent := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	if got := detectSystem(userAgent); got != "macOS" {
		t.Fatalf("detectSystem() = %q, want macOS", got)
	}
	if got := detectBrowser(userAgent); got != "Chrome" {
		t.Fatalf("detectBrowser() = %q, want Chrome", got)
	}
}

func TestTruncateLogTextKeepsValidUTF8(t *testing.T) {
	got := truncateLogText("开启同步", 2)
	if got != "...[truncated]" {
		t.Fatalf("truncateLogText() = %q, want truncated marker only", got)
	}
	got = truncateLogText("开启同步", 3)
	if got != "开...[truncated]" {
		t.Fatalf("truncateLogText() = %q, want valid first rune", got)
	}
}

func TestRedactSensitiveText(t *testing.T) {
	input := `https://www.googleapis.com/youtube/v3/channels?forHandle=%40demo&key=secret123&part=snippet`
	got := redactSensitiveText(input)
	if got != `https://www.googleapis.com/youtube/v3/channels?forHandle=%40demo&key=[REDACTED]&part=snippet` {
		t.Fatalf("redactSensitiveText() = %q", got)
	}
}

func TestAPIPrefixRoutesToBackendHandlers(t *testing.T) {
	a := &app{}
	mux := http.NewServeMux()
	a.routes(mux)

	healthRecorder := httptest.NewRecorder()
	mux.ServeHTTP(healthRecorder, httptest.NewRequest(http.MethodGet, "/api/healthz", nil))
	if healthRecorder.Code != http.StatusOK {
		t.Fatalf("GET /api/healthz status = %d, want %d", healthRecorder.Code, http.StatusOK)
	}

	passwordRecorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/mine/password", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(passwordRecorder, req)
	if passwordRecorder.Code == http.StatusNotFound {
		t.Fatalf("POST /api/mine/password returned 404; want routed handler response")
	}
	if passwordRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("POST /api/mine/password status = %d, want %d without token", passwordRecorder.Code, http.StatusUnauthorized)
	}
}
