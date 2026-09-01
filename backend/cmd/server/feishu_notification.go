package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"
)

type feishuClient struct {
	appID        string
	appSecret    string
	baseURL      string
	httpClient   *http.Client
	tokenMu      sync.Mutex
	tenantToken  string
	tokenExpires time.Time
}

type projectImportNotification struct {
	ProjectID           int
	ProjectName         string
	UserID              int
	BatchID             string
	Imported            int
	ImportedContent     int
	ImportedProfiles    int
	CreatedResources    int
	MatchedResources    int
	CreatedCooperations int
	UpdatedCooperations int
	SkippedCooperations int
	Failed              int
}

type projectImportSyncResult struct {
	Status        string
	Message       string
	ProfileSynced int
	ContentSynced int
	Screenshots   int
	WarningCount  int
}

type projectImportNotificationDelivery struct {
	Status  string
	Message string
}

func newFeishuClient(cfg FeishuConfig, client *http.Client) *feishuClient {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &feishuClient{
		appID:      strings.TrimSpace(cfg.AppID),
		appSecret:  strings.TrimSpace(cfg.AppSecret),
		baseURL:    strings.TrimRight(defaultString(strings.TrimSpace(cfg.APIBaseURL), "https://open.feishu.cn"), "/"),
		httpClient: client,
	}
}

func (client *feishuClient) applicationConfigured() bool {
	return client.appID != "" && client.appSecret != "" && client.baseURL != ""
}

func (client *feishuClient) sendToEmail(ctx context.Context, email, message string) error {
	if !client.applicationConfigured() {
		return errors.New("飞书应用机器人配置不完整")
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return errors.New("导入用户未配置邮箱，无法发送飞书应用消息")
	}
	token, err := client.tenantAccessToken(ctx)
	if err != nil {
		return err
	}
	content, err := json.Marshal(map[string]string{"text": message})
	if err != nil {
		return err
	}
	payload := map[string]string{
		"receive_id": email,
		"msg_type":   "text",
		"content":    string(content),
	}
	return client.postJSON(ctx, client.baseURL+"/open-apis/im/v1/messages?receive_id_type=email", payload, token)
}

func (client *feishuClient) userAvatar(ctx context.Context, userID, email string) (string, error) {
	if !client.applicationConfigured() {
		return "", errors.New("飞书应用配置不完整")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", errors.New("飞书用户 ID 为空")
	}
	token, err := client.tenantAccessToken(ctx)
	if err != nil {
		return "", err
	}
	if resolvedUserID, resolveErr := client.userIDByEmail(ctx, token, email); resolveErr != nil {
		log.Printf("[SSO-DIAG] Feishu user ID lookup failed for email %q: %v", email, resolveErr)
	} else if resolvedUserID != "" {
		log.Printf("[SSO-DIAG] Feishu avatar user resolved: requested_user_id=%q resolved_user_id=%q", userID, resolvedUserID)
		userID = resolvedUserID
	}
	endpoint := client.baseURL + "/open-apis/contact/v3/users/" + url.PathEscape(userID) + "?user_id_type=user_id"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("查询飞书用户头像：%w", err)
	}
	defer response.Body.Close()
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"msg"`
		Data    struct {
			User struct {
				Avatar struct {
					AvatarOrigin string `json:"avatar_origin"`
					Avatar640    string `json:"avatar_640"`
					Avatar240    string `json:"avatar_240"`
					Avatar72     string `json:"avatar_72"`
				} `json:"avatar"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := decodeFeishuResponse(response, &result); err != nil {
		return "", fmt.Errorf("查询飞书用户头像：%w", err)
	}
	log.Printf("[SSO-DIAG] Feishu user profile: user_id=%q response=%s", userID, sanitizedSSOValueLog(result))
	if result.Code != 0 {
		return "", fmt.Errorf("查询飞书用户头像失败：code %d: %s", result.Code, result.Message)
	}
	avatar := firstNonEmpty(result.Data.User.Avatar.Avatar240, result.Data.User.Avatar.Avatar640, result.Data.User.Avatar.AvatarOrigin, result.Data.User.Avatar.Avatar72)
	if avatar == "" {
		return "", errors.New("飞书用户头像为空")
	}
	return avatar, nil
}

func (client *feishuClient) userIDByEmail(ctx context.Context, token, email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", nil
	}
	body, err := json.Marshal(map[string][]string{"emails": {email}})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/open-apis/contact/v3/users/batch_get_id?user_id_type=user_id", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("按邮箱查询飞书用户：%w", err)
	}
	defer response.Body.Close()
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"msg"`
		Data    struct {
			Users []struct {
				UserID string `json:"user_id"`
			} `json:"user_list"`
		} `json:"data"`
	}
	if err := decodeFeishuResponse(response, &result); err != nil {
		return "", err
	}
	log.Printf("[SSO-DIAG] Feishu user ID lookup: email=%q response=%s", email, sanitizedSSOValueLog(result))
	if result.Code != 0 {
		return "", fmt.Errorf("按邮箱查询飞书用户失败：code %d: %s", result.Code, result.Message)
	}
	if len(result.Data.Users) == 0 {
		return "", nil
	}
	return strings.TrimSpace(result.Data.Users[0].UserID), nil
}

func (client *feishuClient) sendWebhook(ctx context.Context, webhookURL, message string) error {
	if err := validateFeishuWebhookURL(webhookURL); err != nil {
		return err
	}
	payload := map[string]any{
		"msg_type": "text",
		"content":  map[string]string{"text": message},
	}
	return client.postJSON(ctx, strings.TrimSpace(webhookURL), payload, "")
}

func (client *feishuClient) tenantAccessToken(ctx context.Context) (string, error) {
	client.tokenMu.Lock()
	defer client.tokenMu.Unlock()
	if client.tenantToken != "" && time.Now().Before(client.tokenExpires) {
		return client.tenantToken, nil
	}
	payload := map[string]string{"app_id": client.appID, "app_secret": client.appSecret}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/open-apis/auth/v3/tenant_access_token/internal", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("获取飞书 tenant token：%w", err)
	}
	defer response.Body.Close()
	var result struct {
		Code              int    `json:"code"`
		Message           string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := decodeFeishuResponse(response, &result); err != nil {
		return "", fmt.Errorf("获取飞书 tenant token：%w", err)
	}
	if result.Code != 0 || result.TenantAccessToken == "" {
		return "", fmt.Errorf("获取飞书 tenant token 失败：code %d: %s", result.Code, result.Message)
	}
	client.tenantToken = result.TenantAccessToken
	expiresIn := time.Duration(result.Expire) * time.Second
	if expiresIn <= time.Minute {
		expiresIn = 2 * time.Hour
	}
	client.tokenExpires = time.Now().Add(expiresIn - time.Minute)
	return client.tenantToken, nil
}

func (client *feishuClient) postJSON(ctx context.Context, endpoint string, payload any, token string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("发送飞书消息：%w", err)
	}
	defer response.Body.Close()
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"msg"`
	}
	if err := decodeFeishuResponse(response, &result); err != nil {
		return fmt.Errorf("发送飞书消息：%w", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("发送飞书消息失败：code %d: %s", result.Code, result.Message)
	}
	return nil
}

func decodeFeishuResponse(response *http.Response, target any) error {
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4*1024))
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(target); err != nil {
		return fmt.Errorf("解析响应：%w", err)
	}
	return nil
}

func validateFeishuWebhookURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() != "open.feishu.cn" || !strings.HasPrefix(parsed.Path, "/open-apis/bot/v2/hook/") {
		return errors.New("飞书 Webhook 地址无效")
	}
	return nil
}

func (a *app) notifyProjectImportCompletion(notification projectImportNotification, result projectImportSyncResult) projectImportNotificationDelivery {
	cfg := a.Config().Feishu
	if !cfg.ApplicationEnabled && !cfg.WebhookEnabled {
		log.Printf("[project-import][batch=%s][feishu] skipped: notification disabled", notification.BatchID)
		return projectImportNotificationDelivery{Status: "未启用", Message: "飞书通知未启用，未发送"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client := newFeishuClient(cfg, nil)
	message := buildProjectImportFeishuMessage(cfg, notification, result)
	var channelResults []string
	attempted := 0
	succeeded := 0
	if cfg.ApplicationEnabled {
		attempted++
		log.Printf("[project-import][batch=%s][feishu][application] sending", notification.BatchID)
		var email string
		if err := a.DB().QueryRowContext(ctx, `select coalesce(email, '') from sys_users where id = ?`, notification.UserID).Scan(&email); err != nil {
			message := redactSensitiveText(fmt.Sprintf("读取导入用户邮箱：%v", err))
			channelResults = append(channelResults, "应用机器人失败："+message)
			log.Printf("[project-import][batch=%s][feishu][application] failed: %s", notification.BatchID, message)
		} else if err := client.sendToEmail(ctx, email, message); err != nil {
			failure := redactSensitiveText(err.Error())
			channelResults = append(channelResults, "应用机器人失败："+failure)
			log.Printf("[project-import][batch=%s][feishu][application] failed: %s", notification.BatchID, failure)
		} else {
			succeeded++
			channelResults = append(channelResults, "应用机器人发送成功")
			log.Printf("[project-import][batch=%s][feishu][application] sent", notification.BatchID)
		}
	}
	if cfg.WebhookEnabled {
		attempted++
		log.Printf("[project-import][batch=%s][feishu][webhook] sending", notification.BatchID)
		if err := client.sendWebhook(ctx, cfg.WebhookURL, message); err != nil {
			failure := redactSensitiveText(err.Error())
			channelResults = append(channelResults, "群机器人失败："+failure)
			log.Printf("[project-import][batch=%s][feishu][webhook] failed: %s", notification.BatchID, failure)
		} else {
			succeeded++
			channelResults = append(channelResults, "群机器人发送成功")
			log.Printf("[project-import][batch=%s][feishu][webhook] sent", notification.BatchID)
		}
	}
	status := "发送失败"
	if succeeded == attempted {
		status = "发送成功"
	} else if succeeded > 0 {
		status = "部分成功"
	}
	return projectImportNotificationDelivery{Status: status, Message: strings.Join(channelResults, "；")}
}

func buildProjectImportFeishuMessage(cfg FeishuConfig, notification projectImportNotification, result projectImportSyncResult) string {
	projectName := strings.TrimSpace(notification.ProjectName)
	if projectName == "" {
		projectName = fmt.Sprintf("项目 #%d", notification.ProjectID)
	}
	lines := []string{
		fmt.Sprintf("XMP 项目导入后台同步%s", result.Status),
		fmt.Sprintf("项目：%s", projectName),
		fmt.Sprintf("导入批次：%s", notification.BatchID),
		fmt.Sprintf("导入明细：有效 %d（资料 %d，内容 %d）", notification.Imported, notification.ImportedProfiles, notification.ImportedContent),
		fmt.Sprintf("导入结果：新增达人/媒体 %d，匹配已有 %d，新增合作 %d，更新合作 %d，跳过 %d，失败 %d", notification.CreatedResources, notification.MatchedResources, notification.CreatedCooperations, notification.UpdatedCooperations, notification.SkippedCooperations, notification.Failed),
		fmt.Sprintf("同步结果：账号 %d，内容 %d，截图 %d，提示 %d", result.ProfileSynced, result.ContentSynced, result.Screenshots, result.WarningCount),
	}
	if strings.TrimSpace(result.Message) != "" {
		lines = append(lines, "说明："+strings.TrimSpace(result.Message))
	}
	if frontendURL := strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/"); frontendURL != "" && notification.ProjectID > 0 {
		lines = append(lines, fmt.Sprintf("查看项目：%s/business/projects/detail?id=%d", frontendURL, notification.ProjectID))
	}
	return strings.Join(lines, "\n")
}

func (a *app) saveFeishuConfig(data map[string]any) error {
	cfg := a.Config().Feishu
	cfg.APIBaseURL = defaultString(strings.TrimSpace(cfg.APIBaseURL), "https://open.feishu.cn")
	cfg.ApplicationEnabled = boolField(data, "applicationEnabled")
	cfg.WebhookEnabled = boolField(data, "webhookEnabled")
	if value := optionalConfigValue(data["appId"]); value != "" {
		cfg.AppID = value
	}
	if value := optionalConfigValue(data["appSecret"]); value != "" {
		cfg.AppSecret = value
	}
	if value, ok := data["frontendUrl"]; ok {
		cfg.FrontendURL = optionalConfigValue(value)
	}
	if value := optionalConfigValue(data["webhookUrl"]); value != "" {
		cfg.WebhookURL = value
	}
	if cfg.ApplicationEnabled && (strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.AppSecret) == "") {
		return errors.New("启用飞书应用机器人前，请填写 App ID 和 App Secret")
	}
	if cfg.WebhookEnabled {
		if err := validateFeishuWebhookURL(cfg.WebhookURL); err != nil {
			return err
		}
	}
	if err := writeFeishuConfigToConfig(cfg); err != nil {
		return fmt.Errorf("写入 config.yaml 失败：%w", err)
	}
	current := a.Config()
	current.Feishu = cfg
	a.config.Store(current)
	return nil
}

func writeFeishuConfigToConfig(cfg FeishuConfig) error {
	path := configFilePath()
	raw := map[string]any{}
	data, err := os.ReadFile(path)
	if err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return err
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	feishu, ok := raw["feishu"].(map[string]any)
	if !ok {
		feishu = map[string]any{}
	}
	feishu["application_enabled"] = cfg.ApplicationEnabled
	feishu["app_id"] = strings.TrimSpace(cfg.AppID)
	feishu["app_secret"] = strings.TrimSpace(cfg.AppSecret)
	feishu["api_base_url"] = defaultString(strings.TrimSpace(cfg.APIBaseURL), "https://open.feishu.cn")
	feishu["webhook_enabled"] = cfg.WebhookEnabled
	feishu["webhook_url"] = strings.TrimSpace(cfg.WebhookURL)
	feishu["frontend_url"] = strings.TrimSpace(cfg.FrontendURL)
	raw["feishu"] = feishu
	output, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	mode := os.FileMode(0600)
	if stat, err := os.Stat(path); err == nil {
		mode = stat.Mode().Perm()
	}
	return os.WriteFile(path, output, mode)
}

func feishuConfigPublicContent(cfg FeishuConfig) map[string]any {
	return map[string]any{
		"applicationEnabled":  cfg.ApplicationEnabled,
		"appIdConfigured":     strings.TrimSpace(cfg.AppID) != "",
		"appIdLast4":          lastN(strings.TrimSpace(cfg.AppID), 4),
		"appSecretConfigured": strings.TrimSpace(cfg.AppSecret) != "",
		"frontendUrl":         strings.TrimSpace(cfg.FrontendURL),
		"webhookEnabled":      cfg.WebhookEnabled,
		"webhookConfigured":   strings.TrimSpace(cfg.WebhookURL) != "",
		"webhookLast4":        lastN(strings.TrimSpace(cfg.WebhookURL), 4),
	}
}

func feishuNotificationPublicStatus(cfg FeishuConfig) map[string]any {
	applicationEnabled := cfg.ApplicationEnabled && strings.TrimSpace(cfg.AppID) != "" && strings.TrimSpace(cfg.AppSecret) != ""
	webhookEnabled := cfg.WebhookEnabled && strings.TrimSpace(cfg.WebhookURL) != ""
	return map[string]any{
		"enabled":            applicationEnabled || webhookEnabled,
		"applicationEnabled": applicationEnabled,
		"webhookEnabled":     webhookEnabled,
	}
}

func (a *app) businessImportNotificationStatus(w http.ResponseWriter, _ *http.Request) {
	writeOK(w, feishuNotificationPublicStatus(a.Config().Feishu))
}
