package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const ssoStateCookieName = "kol_sso_state"

type ssoIdentity struct {
	Provider     string
	Subject      string
	Email        string
	Name         string
	Avatar       string
	EmployeeNo   string
	Department   string
	RequestToken string
	RefreshToken string
}

type uacCallbackInput struct {
	State        string            `json:"state"`
	Token        string            `json:"token"`
	UToken       string            `json:"utoken"`
	RefreshToken string            `json:"refreshToken"`
	RToken       string            `json:"rtoken"`
	EmployeeNo   string            `json:"employeeNo"`
	Employee     string            `json:"employee"`
	UserID       string            `json:"userId"`
	Params       map[string]string `json:"params"`
}

type ssoUser struct {
	ID       int
	Avatar   string
	Username string
	Nickname string
	Status   int
}

func (cfg SSOConfig) ready() bool {
	return cfg.Enabled && strings.EqualFold(strings.TrimSpace(cfg.Provider), "uac") &&
		strings.TrimSpace(cfg.FrontendURL) != "" && strings.TrimSpace(cfg.RedirectURI) != "" &&
		strings.TrimSpace(cfg.UACGateway) != "" && strings.TrimSpace(cfg.UACAppID) != "" && validSSORedirectURI(cfg.RedirectURI)
}

func validSSORedirectURI(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" &&
		parsed.Fragment == "" && strings.TrimSuffix(parsed.Path, "/") == "/sso/callback"
}

func localPasswordLoginAllowed(cfg SSOConfig, roles []string) bool {
	return !cfg.Enabled || contains(roles, "admin")
}

func (a *app) authConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := a.Config().SSO
	writeOK(w, map[string]any{
		"ssoEnabled":  cfg.ready(),
		"ssoProvider": defaultString(strings.TrimSpace(cfg.Provider), "uac"),
		"ssoLoginUrl": "/api/auth/sso/login",
	})
}

func (a *app) ssoLogin(w http.ResponseWriter, r *http.Request) {
	cfg := a.Config().SSO
	if !cfg.ready() {
		writeError(w, http.StatusNotFound, 10004, "企业 SSO 未启用或配置不完整")
		return
	}
	state, err := newSSOState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, 10003, "无法创建 SSO 登录状态")
		return
	}
	loginURL, err := uacAuthorizationURL(cfg, state)
	if err != nil {
		writeError(w, http.StatusInternalServerError, 10003, "无法生成 SSO 登录地址")
		return
	}
	http.SetCookie(w, ssoStateCookie(r, state, time.Now().Add(10*time.Minute)))
	http.Redirect(w, r, loginURL, http.StatusFound)
}

func (a *app) ssoUACCallback(w http.ResponseWriter, r *http.Request) {
	cfg := a.Config().SSO
	if !cfg.ready() {
		writeError(w, http.StatusNotFound, 10004, "企业 SSO 未启用或配置不完整")
		return
	}
	var input uacCallbackInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, 10001, "SSO 回调参数无效")
		return
	}
	stateCookie, err := r.Cookie(ssoStateCookieName)
	if err != nil || !ssoStatesMatch(stateCookie.Value, input.State) {
		writeError(w, http.StatusBadRequest, 10002, "SSO 登录状态无效或已过期，请重新登录")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	identity, err := fetchUACIdentity(ctx, cfg, input, &http.Client{Timeout: 10 * time.Second})
	if err != nil {
		writeError(w, http.StatusBadGateway, 10002, err.Error())
		return
	}
	if strings.TrimSpace(identity.Subject) == "" || strings.TrimSpace(identity.Email) == "" {
		writeError(w, http.StatusForbidden, 10002, "企业身份缺少稳定用户标识或邮箱")
		return
	}
	identity = enrichSSOAvatar(ctx, identity, a.Config().Feishu, &http.Client{Timeout: 10 * time.Second})
	user, err := a.ensureSSOUser(r.Context(), identity)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if user.Status != 1 {
		writeError(w, http.StatusForbidden, 10002, "当前账号已停用")
		return
	}
	data, err := a.loginResponseData(r.Context(), user.ID, user.Avatar, user.Username, user.Nickname)
	if err != nil {
		writeDBError(w, err)
		return
	}
	http.SetCookie(w, ssoStateCookie(r, "", time.Unix(0, 0)))
	a.recordLoginLog(r, user.Username, 1, "企业 SSO 登录")
	writeOK(w, data)
}

func uacAuthorizationURL(cfg SSOConfig, state string) (string, error) {
	redirect, err := url.Parse(strings.TrimSpace(cfg.RedirectURI))
	if err != nil {
		return "", err
	}
	redirectQuery := redirect.Query()
	redirectQuery.Set("state", state)
	redirect.RawQuery = redirectQuery.Encode()
	query := url.Values{
		"appId":    {strings.TrimSpace(cfg.UACAppID)},
		"redirect": {redirect.String()},
		"lang":     {defaultString(strings.TrimSpace(cfg.UACLang), "zh_CN")},
		"type":     {"simple"},
	}
	if source := strings.TrimSpace(cfg.UACSource); source != "" {
		query.Set("source", source)
	}
	endpoint := strings.TrimRight(strings.TrimSpace(cfg.UACGateway), "/") + "/uac-auth-service/v2/api/uac-auth/login/redirect/web-login"
	return endpoint + "?" + query.Encode(), nil
}

func fetchUACIdentity(ctx context.Context, cfg SSOConfig, input uacCallbackInput, client *http.Client) (ssoIdentity, error) {
	requestToken := firstNonEmpty(input.Token, input.RefreshToken, input.param("token", "refreshToken", "refresh_token", "P-Auth", "p_auth", "pAuth", "accessToken", "access_token"))
	userToken := firstNonEmpty(input.RToken, input.UToken, input.param("rtoken", "rToken", "RToken", "utoken", "uToken", "UToken", "P-Rtoken", "p_rtoken", "pRtoken"))
	employeeNo := firstNonEmpty(input.EmployeeNo, input.Employee, input.UserID, input.param("employeeNo", "employee_no", "employee", "empNo", "staffNo", "jobNo", "workNo", "userId", "user_id"))
	if requestToken == "" || userToken == "" {
		return ssoIdentity{}, errors.New("UAC 回调缺少 token 或 rtoken")
	}
	body, _ := json.Marshal(map[string]string{"rtoken": requestToken, "utoken": userToken, "appId": strings.TrimSpace(cfg.UACAppID)})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(strings.TrimSpace(cfg.UACGateway), "/")+"/uac-auth-service/v2/api/uac-auth/utoken/getUserInfo", bytes.NewReader(body))
	if err != nil {
		return ssoIdentity{}, errors.New("创建 UAC 用户信息请求失败")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("P-Auth", requestToken)
	request.Header.Set("P-Rtoken", userToken)
	request.Header.Set("P-AppId", strings.TrimSpace(cfg.UACAppID))
	response, err := client.Do(request)
	if err != nil {
		return ssoIdentity{}, errors.New("UAC 用户信息请求失败")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ssoIdentity{}, uacSafeResponseError(response, requestToken, userToken)
	}
	var profile map[string]any
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&profile); err != nil {
		return ssoIdentity{}, errors.New("UAC 用户信息响应无效")
	}
	profile = nestedSSOProfile(profile, "data", "result", "user", "userInfo")
	identity := ssoIdentity{
		Provider:     "uac",
		Subject:      firstSSOString(profile, "uid", "userId", "id", "employeeNo"),
		Email:        firstSSOString(profile, "email", "mail"),
		Name:         firstSSOString(profile, "realName", "name", "displayName", "userName"),
		Avatar:       firstSSOAvatarURL(profile),
		EmployeeNo:   firstSSOString(profile, "employeeNo", "employeeNumber", "empNo", "jobNo", "workNo"),
		Department:   firstSSOString(profile, "deptName", "departmentName", "department", "dept"),
		RequestToken: requestToken,
		RefreshToken: userToken,
	}
	if identity.Subject == "" {
		identity.Subject = employeeNo
	}
	if identity.EmployeeNo == "" {
		identity.EmployeeNo = employeeNo
	}
	return identity, nil
}

func enrichSSOAvatar(ctx context.Context, identity ssoIdentity, cfg FeishuConfig, httpClient *http.Client) ssoIdentity {
	if strings.TrimSpace(identity.Avatar) != "" {
		return identity
	}
	avatar, err := newFeishuClient(cfg, httpClient).userAvatar(ctx, identity.Subject, identity.Email)
	if err != nil {
		log.Printf("SSO avatar lookup skipped: %v", err)
		return identity
	}
	identity.Avatar = strings.TrimSpace(avatar)
	return identity
}

func (a *app) ensureSSOUser(ctx context.Context, identity ssoIdentity) (ssoUser, error) {
	tx, err := a.DB().BeginTx(ctx, nil)
	if err != nil {
		return ssoUser{}, err
	}
	defer tx.Rollback()
	var user ssoUser
	queryUser := func(query string, args ...any) error {
		return tx.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.Avatar, &user.Username, &user.Nickname, &user.Status)
	}
	err = queryUser(`select id, avatar, username, nickname, status from sys_users where auth_provider = ? and external_subject = ? limit 1`, identity.Provider, identity.Subject)
	if errors.Is(err, sql.ErrNoRows) {
		err = queryUser(`select id, avatar, username, nickname, status from sys_users where lower(email) = lower(?) order by id limit 1`, identity.Email)
	}
	if errors.Is(err, sql.ErrNoRows) {
		username := ssoUsername(identity)
		nickname := firstNonEmpty(identity.Name, identity.Email)
		password, passwordErr := newSSOState()
		if passwordErr != nil {
			return ssoUser{}, passwordErr
		}
		result, insertErr := tx.ExecContext(ctx,
			`insert into sys_users
			  (avatar, username, nickname, password_hash, email, status, remark, auth_provider, external_subject, employee_no, department_name, last_login_at)
			 values (?, ?, ?, ?, ?, 1, '企业 SSO 自动创建', ?, ?, ?, ?, now())`,
			strings.TrimSpace(identity.Avatar), username, nickname, sha256Hex(password), strings.TrimSpace(identity.Email), identity.Provider, identity.Subject, identity.EmployeeNo, identity.Department,
		)
		if insertErr != nil {
			return ssoUser{}, insertErr
		}
		id, idErr := result.LastInsertId()
		if idErr != nil {
			return ssoUser{}, idErr
		}
		user = ssoUser{ID: int(id), Username: username, Nickname: nickname, Status: 1}
	} else if err != nil {
		return ssoUser{}, err
	} else {
		_, err = tx.ExecContext(ctx,
			`update sys_users set
			   avatar = if(? <> '', ?, avatar), email = ?, nickname = if(? <> '', ?, nickname), auth_provider = ?, external_subject = ?,
			   employee_no = ?, department_name = ?, last_login_at = now()
			 where id = ?`,
			strings.TrimSpace(identity.Avatar), strings.TrimSpace(identity.Avatar), strings.TrimSpace(identity.Email), strings.TrimSpace(identity.Name), strings.TrimSpace(identity.Name), identity.Provider, identity.Subject,
			strings.TrimSpace(identity.EmployeeNo), strings.TrimSpace(identity.Department), user.ID,
		)
		if err != nil {
			return ssoUser{}, err
		}
		if strings.TrimSpace(identity.Name) != "" {
			user.Nickname = strings.TrimSpace(identity.Name)
		}
		if strings.TrimSpace(identity.Avatar) != "" {
			user.Avatar = strings.TrimSpace(identity.Avatar)
		}
	}
	if err := assignDefaultSSORole(ctx, tx, user.ID, a.Config().SSO.DefaultRoleCode); err != nil {
		return ssoUser{}, err
	}
	if err := tx.Commit(); err != nil {
		return ssoUser{}, err
	}
	return user, nil
}

func assignDefaultSSORole(ctx context.Context, tx *sql.Tx, userID int, roleCode string) error {
	var activeRoleCount int
	if err := tx.QueryRowContext(ctx,
		`select count(*) from sys_user_roles ur
		  join sys_roles r on r.id = ur.role_id
		 where ur.user_id = ? and r.status = 1`,
		userID,
	).Scan(&activeRoleCount); err != nil {
		return err
	}
	if activeRoleCount > 0 {
		return nil
	}

	roleCode = strings.TrimSpace(roleCode)
	var roleID int64
	if err := tx.QueryRowContext(ctx,
		`select id from sys_roles where code = ? and status = 1 limit 1`,
		roleCode,
	).Scan(&roleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("SSO 默认角色 %q 不存在或已停用", roleCode)
		}
		return err
	}
	_, err := tx.ExecContext(ctx,
		`insert ignore into sys_user_roles (user_id, role_id) values (?, ?)`,
		userID, roleID,
	)
	return err
}

func (a *app) loginResponseData(ctx context.Context, userID int, avatar, username, nickname string) (map[string]any, error) {
	roles, err := a.roleCodes(ctx, userID)
	if err != nil {
		return nil, err
	}
	permissions, err := a.userPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}
	if contains(roles, "admin") {
		permissions = []string{"*:*:*"}
	}
	now := time.Now()
	return map[string]any{
		"avatar":       avatar,
		"username":     username,
		"nickname":     nickname,
		"roles":        roles,
		"permissions":  permissions,
		"accessToken":  fmt.Sprintf("kol.%d.%d", userID, now.Unix()),
		"refreshToken": fmt.Sprintf("kol.%d.refresh.%d", userID, now.Unix()),
		"expires":      now.Add(2 * time.Hour).Format("2006/01/02 15:04:05"),
	}, nil
}

func ssoUsername(identity ssoIdentity) string {
	sum := sha256.Sum256([]byte(strings.ToLower(identity.Provider) + ":" + identity.Subject))
	return "sso_" + hex.EncodeToString(sum[:12])
}

func newSSOState() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func ssoStatesMatch(expected, actual string) bool {
	return len(expected) == len(actual) && len(actual) >= 32 && subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

func ssoStateCookie(r *http.Request, value string, expires time.Time) *http.Cookie {
	maxAge := int(time.Until(expires).Seconds())
	if value == "" {
		maxAge = -1
	}
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	return &http.Cookie{Name: ssoStateCookieName, Value: value, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: maxAge}
}

func (input uacCallbackInput) param(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(input.Params[key]); value != "" {
			return value
		}
	}
	for paramKey, value := range input.Params {
		for _, key := range keys {
			if strings.EqualFold(paramKey, key) && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	for encodedQuery, value := range input.Params {
		if strings.TrimSpace(value) != "" || !strings.Contains(encodedQuery, "=") {
			continue
		}
		query, err := url.ParseQuery(strings.TrimPrefix(encodedQuery, "?"))
		if err != nil {
			continue
		}
		for _, key := range keys {
			if parsed := strings.TrimSpace(query.Get(key)); parsed != "" {
				return parsed
			}
		}
	}
	return ""
}

func nestedSSOProfile(profile map[string]any, keys ...string) map[string]any {
	for {
		found := false
		for _, key := range keys {
			if nested, ok := profile[key].(map[string]any); ok {
				profile = nested
				found = true
				break
			}
		}
		if !found {
			return profile
		}
	}
}

func firstSSOString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func firstSSOAvatarURL(profile map[string]any) string {
	for _, key := range []string{"avatarUrl", "avatarURL", "avatar_url", "avatar", "headImgUrl", "headImageUrl", "headUrl", "photoUrl", "profilePhoto"} {
		value, ok := profile[key]
		if !ok {
			continue
		}
		switch avatar := value.(type) {
		case string:
			if avatar = strings.TrimSpace(avatar); avatar != "" {
				return avatar
			}
		case map[string]any:
			if url := firstSSOString(avatar, "url", "href", "avatarUrl", "avatar_url"); url != "" {
				return url
			}
		}
	}
	return ""
}

func uacSafeResponseError(response *http.Response, tokens ...string) error {
	var detail struct {
		Code    any    `json:"code"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 16<<10)).Decode(&detail); err != nil {
		return fmt.Errorf("UAC 用户信息接口返回 HTTP %d", response.StatusCode)
	}
	message := firstNonEmpty(detail.Message, detail.Msg)
	for _, token := range tokens {
		if token != "" {
			message = strings.ReplaceAll(message, token, "[REDACTED]")
		}
	}
	if len(message) > 256 {
		message = message[:256]
	}
	if message == "" {
		return fmt.Errorf("UAC 用户信息接口返回 HTTP %d", response.StatusCode)
	}
	return fmt.Errorf("UAC 用户信息接口返回 HTTP %d：%s", response.StatusCode, message)
}
