package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const uacDirectoryPageSize = 20

// UAC directory credentials are application credentials. User login tokens are
// not persisted or forwarded to this endpoint.
type uacDirectoryClient struct {
	cfg        SSOConfig
	httpClient *http.Client
}

type uacDirectoryPerson struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Department string `json:"department"`
	EmployeeNo string `json:"employeeNo"`
	Notifiable bool   `json:"notifiable"`
	email      string
	uid        string
}

type uacTokenEntry struct {
	token     string
	expiresAt time.Time
}

var uacApplicationTokens = struct {
	sync.Mutex
	entries map[string]uacTokenEntry
}{entries: map[string]uacTokenEntry{}}

func newUACDirectoryClient(cfg SSOConfig, client *http.Client) *uacDirectoryClient {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &uacDirectoryClient{cfg: cfg, httpClient: client}
}

func (client *uacDirectoryClient) configured() bool {
	return client.cfg.Enabled && strings.EqualFold(client.cfg.Provider, "uac") &&
		strings.TrimSpace(client.cfg.UACGateway) != "" && strings.TrimSpace(client.cfg.UACAppID) != "" &&
		strings.TrimSpace(client.cfg.UACAppSecret) != ""
}

func (client *uacDirectoryClient) search(ctx context.Context, keyword string, page int) ([]uacDirectoryPerson, bool, error) {
	if strings.TrimSpace(keyword) == "" || page < 1 {
		return nil, false, errors.New("UAC 查询词或页码无效")
	}
	return client.queryUsers(ctx, page, map[string]any{"likeNoOrName": strings.TrimSpace(keyword), "status": "OK"})
}

func (client *uacDirectoryClient) byEmployeeNo(ctx context.Context, employeeNo string) (uacDirectoryPerson, error) {
	employeeNo = strings.TrimSpace(employeeNo)
	if employeeNo == "" {
		return uacDirectoryPerson{}, errors.New("UAC 工号为空")
	}
	people, _, err := client.queryUsers(ctx, 1, map[string]any{"empNo": employeeNo, "status": "OK"})
	if err != nil {
		return uacDirectoryPerson{}, err
	}
	for _, person := range people {
		if person.EmployeeNo == employeeNo {
			return person, nil
		}
	}
	return uacDirectoryPerson{}, errors.New("UAC 用户不可用")
}

func (client *uacDirectoryClient) queryUsers(ctx context.Context, page int, param map[string]any) ([]uacDirectoryPerson, bool, error) {
	if !client.configured() {
		return nil, false, errors.New("UAC 目录查询未配置")
	}
	token, err := client.applicationToken(ctx)
	if err != nil {
		return nil, false, err
	}
	requestBody, _ := json.Marshal(map[string]any{
		"current": page, "size": uacDirectoryPageSize, "searchTotal": false, "param": param,
	})
	endpoint := strings.TrimRight(client.cfg.UACGateway, "/") + "/service-pf-open-gateway/open-apis/v2/api/uac-user/user/pages"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(requestBody)))
	if err != nil {
		return nil, false, errors.New("创建 UAC 目录请求失败")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, false, errors.New("UAC 目录请求失败")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("UAC 目录返回 HTTP %d", response.StatusCode)
	}
	var payload struct {
		Code json.RawMessage `json:"code"`
		Data struct {
			HasNext  bool `json:"hasNext"`
			DataList []struct {
				UID        json.RawMessage `json:"uid"`
				RealName   string          `json:"realName"`
				EmployeeNo string          `json:"employeeNo"`
				Email      string          `json:"email"`
				Status     string          `json:"status"`
				DeptName   string          `json:"deptName"`
				Depts      []struct {
					Name string `json:"name"`
				} `json:"depts"`
			} `json:"dataList"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2<<20))
	if err := decoder.Decode(&payload); err != nil {
		return nil, false, errors.New("UAC 目录响应无效")
	}
	if !uacSuccessCode(payload.Code) {
		return nil, false, fmt.Errorf("UAC 目录拒绝查询，错误码 %s；请检查应用接口与字段权限", uacRawString(payload.Code))
	}
	people := make([]uacDirectoryPerson, 0, len(payload.Data.DataList))
	for _, item := range payload.Data.DataList {
		uid := uacRawString(item.UID)
		name := strings.TrimSpace(item.RealName)
		employeeNo := strings.TrimSpace(item.EmployeeNo)
		if uid == "" || name == "" || employeeNo == "" || !strings.EqualFold(item.Status, "OK") {
			continue
		}
		department := strings.TrimSpace(item.DeptName)
		if department == "" && len(item.Depts) > 0 {
			department = strings.TrimSpace(item.Depts[0].Name)
		}
		email := strings.ToLower(strings.TrimSpace(item.Email))
		people = append(people, uacDirectoryPerson{
			ID: "uac:" + uid, Name: name, Department: department,
			EmployeeNo: employeeNo, Notifiable: email != "", email: email, uid: uid,
		})
	}
	return people, payload.Data.HasNext, nil
}

func (client *uacDirectoryClient) applicationToken(ctx context.Context) (string, error) {
	if !client.configured() {
		return "", errors.New("UAC 应用令牌未配置")
	}
	secretHash := sha256.Sum256([]byte(client.cfg.UACAppSecret))
	cacheKey := strings.TrimRight(client.cfg.UACGateway, "/") + "\x00" + client.cfg.UACAppID + "\x00" + hex.EncodeToString(secretHash[:])
	uacApplicationTokens.Lock()
	defer uacApplicationTokens.Unlock()
	if cached, ok := uacApplicationTokens.entries[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		return cached.token, nil
	}
	query := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {strings.TrimSpace(client.cfg.UACAppID)},
		"client_secret": {strings.TrimSpace(client.cfg.UACAppSecret)},
	}
	endpoint := strings.TrimRight(client.cfg.UACGateway, "/") + "/service-pf-open-gateway/oauth/token?" + query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", errors.New("创建 UAC 应用令牌请求失败")
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return "", errors.New("UAC 应用令牌请求失败")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("UAC 应用令牌返回 HTTP %d", response.StatusCode)
	}
	var payload struct {
		Code json.RawMessage `json:"code"`
		Data struct {
			AccessToken string          `json:"access_token"`
			ExpiresIn   json.RawMessage `json:"expires_in"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return "", errors.New("UAC 应用令牌响应无效")
	}
	if !uacSuccessCode(payload.Code) || strings.TrimSpace(payload.Data.AccessToken) == "" {
		return "", errors.New("UAC 应用令牌未获授权")
	}
	expiresIn, err := strconv.Atoi(uacRawString(payload.Data.ExpiresIn))
	if err != nil || expiresIn <= 0 {
		return "", errors.New("UAC 应用令牌有效期无效")
	}
	margin := 60
	if expiresIn < margin*2 {
		margin = expiresIn / 2
	}
	uacApplicationTokens.entries[cacheKey] = uacTokenEntry{
		token: payload.Data.AccessToken, expiresAt: time.Now().Add(time.Duration(expiresIn-margin) * time.Second),
	}
	return payload.Data.AccessToken, nil
}

func uacRawString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(string(raw))
}

func uacSuccessCode(raw json.RawMessage) bool {
	value := uacRawString(raw)
	return value == "0" || value == "200"
}
