package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestFeishuClientSendToEmail(t *testing.T) {
	var tokenRequests atomic.Int32
	var messageRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			tokenRequests.Add(1)
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload["app_id"] != "app-id" || payload["app_secret"] != "app-secret" {
				t.Fatalf("unexpected token payload: %#v", payload)
			}
			_, _ = w.Write([]byte(`{"code":0,"tenant_access_token":"tenant-token","expire":7200}`))
		case "/open-apis/im/v1/messages":
			messageRequests.Add(1)
			if r.URL.Query().Get("receive_id_type") != "email" {
				t.Fatalf("unexpected receive_id_type: %s", r.URL.RawQuery)
			}
			if r.Header.Get("Authorization") != "Bearer tenant-token" {
				t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
			}
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload["receive_id"] != "importer@example.com" || payload["msg_type"] != "text" {
				t.Fatalf("unexpected message payload: %#v", payload)
			}
			var content map[string]string
			if err := json.Unmarshal([]byte(payload["content"]), &content); err != nil {
				t.Fatal(err)
			}
			if content["text"] != "同步完成" {
				t.Fatalf("unexpected message content: %#v", content)
			}
			_, _ = w.Write([]byte(`{"code":0,"msg":"success"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newFeishuClient(FeishuConfig{
		AppID:      "app-id",
		AppSecret:  "app-secret",
		APIBaseURL: server.URL,
	}, server.Client())
	for range 2 {
		if err := client.sendToEmail(context.Background(), "importer@example.com", "同步完成"); err != nil {
			t.Fatal(err)
		}
	}
	if tokenRequests.Load() != 1 {
		t.Fatalf("token requests = %d, want 1", tokenRequests.Load())
	}
	if messageRequests.Load() != 2 {
		t.Fatalf("message requests = %d, want 2", messageRequests.Load())
	}
}

func TestValidateFeishuWebhookURL(t *testing.T) {
	if err := validateFeishuWebhookURL("https://open.feishu.cn/open-apis/bot/v2/hook/test-token"); err != nil {
		t.Fatalf("valid webhook rejected: %v", err)
	}
	for _, value := range []string{
		"http://open.feishu.cn/open-apis/bot/v2/hook/test-token",
		"https://example.com/open-apis/bot/v2/hook/test-token",
		"https://open.feishu.cn/open-apis/im/v1/messages",
	} {
		if err := validateFeishuWebhookURL(value); err == nil {
			t.Fatalf("invalid webhook accepted: %s", value)
		}
	}
}

func TestFeishuNotificationPublicStatusDoesNotExposeSecrets(t *testing.T) {
	status := feishuNotificationPublicStatus(FeishuConfig{
		ApplicationEnabled: true,
		AppID:              "app-id",
		AppSecret:          "application-secret",
		WebhookEnabled:     true,
		WebhookURL:         "https://open.feishu.cn/open-apis/bot/v2/hook/secret",
	})
	if status["enabled"] != true || status["applicationEnabled"] != true || status["webhookEnabled"] != true {
		t.Fatalf("unexpected public status: %#v", status)
	}
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "app-id") || strings.Contains(string(encoded), "secret") {
		t.Fatalf("public status exposed credentials: %s", encoded)
	}
}

func TestBuildProjectImportFeishuMessage(t *testing.T) {
	message := buildProjectImportFeishuMessage(
		FeishuConfig{FrontendURL: "https://xmp.example.com/"},
		projectImportNotification{
			ProjectID:           42,
			ProjectName:         "新品发布",
			BatchID:             "IMP123",
			CreatedResources:    3,
			MatchedResources:    2,
			CreatedCooperations: 4,
			UpdatedCooperations: 1,
			SkippedCooperations: 1,
			Failed:              2,
		},
		projectImportSyncResult{
			Status:        "部分失败",
			Message:       "同步完成，请检查提示项",
			ProfileSynced: 4,
			ContentSynced: 3,
			Screenshots:   2,
			WarningCount:  2,
		},
	)
	for _, expected := range []string{
		"XMP 项目导入后台同步部分失败",
		"项目：新品发布",
		"导入批次：IMP123",
		"新增达人/媒体 3",
		"同步结果：账号 4，内容 3，截图 2，提示 2",
		"https://xmp.example.com/business/projects/detail?id=42",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("message missing %q:\n%s", expected, message)
		}
	}
}

func TestFeishuConfigPublicContentDoesNotExposeSecrets(t *testing.T) {
	content := feishuConfigPublicContent(FeishuConfig{
		AppID:      "cli_123456",
		AppSecret:  "secret-value",
		WebhookURL: "https://open.feishu.cn/open-apis/bot/v2/hook/private-token",
	})
	encoded, err := json.Marshal(content)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"secret-value", "private-token", "cli_123456"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("public config exposed %q: %s", secret, encoded)
		}
	}
}
