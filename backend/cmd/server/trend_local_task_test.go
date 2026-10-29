package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exercise the server response -> downloaded task -> CLI boundary using only
// a disposable in-memory grant, never a real downloaded credential.
func TestTrendLocalDownloadedTask(t *testing.T) {
	for _, server := range []string{"http://127.0.0.1:8848/api", "http://localhost:8848/api", "https://admin.example.test/api"} {
		t.Run(server, func(t *testing.T) {
			a := localFixtureApp(t)
			r := requestForTrendOwner("/business/trends/local-jobs?tab=tag&region=US&period=30", 1)
			r.RemoteAddr = "127.0.0.1:1234"
			w := httptest.NewRecorder()
			a.trendLocalCreate(w, r)
			var response struct {
				Data map[string]any `json:"data"`
			}
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil {
				t.Fatal("could not create fixture grant")
			}
			data := response.Data
			raw, err := json.Marshal(map[string]any{
				"version": 1, "server": server, "job": data["id"], "token": data["token"], "expiresAt": data["expiresAt"],
				"query": map[string]string{"tab": "tag", "region": "US", "period": "30", "orderByMetric": "1"},
			})
			if err != nil {
				t.Fatal("could not encode fixture")
			}
			file := filepath.Join(t.TempDir(), "task.json")
			if os.WriteFile(file, raw, 0600) != nil {
				t.Fatal("could not write fixture")
			}
			input, output, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			_, _ = output.WriteString("no\n")
			_ = output.Close()
			original := os.Stdin
			os.Stdin = input
			defer func() { os.Stdin = original }()
			if err = runLocalTrendClient([]string{"--job", file}); err == nil || err.Error() != "已取消" {
				t.Fatalf("fresh downloaded task must reach destination confirmation, got: %v", err)
			}
		})
	}
}

func TestTrendLocalTaskValidationReasons(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	base := localTrendTask{Version: 1, Server: "https://admin.example.test/api", Job: strings.Repeat("a", 32), Token: strings.Repeat("b", 64), ExpiresAt: now.Add(30 * time.Minute)}
	tests := []struct {
		name   string
		change func(*localTrendTask)
		want   string
	}{
		{"valid", func(*localTrendTask) {}, ""},
		{"localhost", func(v *localTrendTask) { v.Server = "http://localhost:8848/api" }, ""},
		{"ipv6", func(v *localTrendTask) { v.Server = "http://[::1]:8848/api" }, ""},
		{"remote-http", func(v *localTrendTask) { v.Server = "http://admin.example.test/api" }, "服务器地址"},
		{"localhost-lookalike", func(v *localTrendTask) { v.Server = "http://localhost.example.test/api" }, "服务器地址"},
		{"credentials-in-url", func(v *localTrendTask) { v.Server = "https://user:secret@example.test/api" }, "服务器地址"},
		{"expired", func(v *localTrendTask) { v.ExpiresAt = now.Add(-time.Second) }, "授权已过期"},
		{"expiry-boundary", func(v *localTrendTask) { v.ExpiresAt = now }, "授权已过期"},
		{"clock-skew", func(v *localTrendTask) { v.ExpiresAt = now.Add(2 * time.Hour) }, "本机时间不一致"},
		{"no-expiry", func(v *localTrendTask) { v.ExpiresAt = time.Time{} }, "缺少有效期"},
		{"bad-version", func(v *localTrendTask) { v.Version = 3 }, "版本不支持"},
		{"missing-regions", func(v *localTrendTask) { v.Version = 2 }, "地区列表不正确"},
		{"bad-token", func(v *localTrendTask) { v.Token = strings.Repeat("z", 64) }, "缺少有效授权"},
		{"no-job", func(v *localTrendTask) { v.Job = "" }, "缺少有效授权"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := base
			tt.change(&v)
			raw, _ := json.Marshal(v)
			_, err := readLocalTrendTask(strings.NewReader(string(raw)), now)
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected reason %q, got %v", tt.want, err)
			}
			if err != nil && (strings.Contains(err.Error(), base.Token) || strings.Contains(err.Error(), "user:secret")) {
				t.Fatal("validation exposed credentials")
			}
		})
	}
	for _, raw := range []string{"{", `{ "unexpected": 1 }`, `{} {}`, strings.Repeat(" ", 32769)} {
		if _, err := readLocalTrendTask(strings.NewReader(raw), now); err == nil {
			t.Fatal("invalid or oversized task accepted")
		}
	}
}
