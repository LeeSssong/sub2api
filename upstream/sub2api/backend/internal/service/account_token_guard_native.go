package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func guardAccountEligible(a *Account) bool {
	return a != nil && a.Platform == PlatformOpenAI && a.IsOAuth() && !a.IsShadow() && !a.IsOpenAIAgentIdentity() && !a.IsOpenAIPersonalAccessToken()
}
func (s *AccountTokenGuardService) availableAccounts(ctx context.Context, cfg AccountTokenGuardConfig) ([]AccountTokenGuardAvailableAccount, error) {
	cfg.MaxProbePerCycle = 0
	accounts, err := s.listAccounts(ctx, cfg)
	if err != nil {
		return nil, err
	}
	result := make([]AccountTokenGuardAvailableAccount, 0, len(accounts))
	for _, a := range accounts {
		result = append(result, AccountTokenGuardAvailableAccount{AccountID: a.ID, AccountName: a.Name, Email: a.GetCredential("email")})
	}
	return result, nil
}

// Only saves migrate a legacy email binding; execution always uses a stable ID.
func (s *AccountTokenGuardService) bindReloginAccounts(ctx context.Context, cfg *AccountTokenGuardConfig) error {
	if len(cfg.ReloginAccounts) == 0 {
		return nil
	}
	if s.accounts == nil {
		return errors.New("账号仓储未就绪")
	}
	accounts, err := s.accounts.ListByPlatform(ctx, PlatformOpenAI)
	if err != nil {
		return errors.New("无法读取可绑定账号")
	}
	used := map[int64]bool{}
	for i := range cfg.ReloginAccounts {
		entry := &cfg.ReloginAccounts[i]
		if entry.AccountID < 0 {
			return errors.New("账号 ID 不合法")
		}
		if entry.Password == "" || entry.Email == "" || !strings.Contains(entry.Email, "@") {
			return errors.New("重登账号必须填写邮箱和密码")
		}
		secret := strings.ToUpper(strings.TrimSpace(entry.MFASecret))
		decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.TrimRight(secret, "="))
		if err != nil || len(decoded) == 0 {
			return errors.New("重登账号必须填写合法的 Base32 2FA 密钥")
		}
		entry.MFASecret = secret
		var matches []*Account
		for j := range accounts {
			a := &accounts[j]
			if !guardAccountEligible(a) {
				continue
			}
			if entry.AccountID > 0 {
				if a.ID == entry.AccountID {
					matches = append(matches, a)
				}
			} else if strings.EqualFold(a.GetCredential("email"), entry.Email) || strings.EqualFold(strings.TrimSpace(a.Name), entry.Email) {
				matches = append(matches, a)
			}
		}
		if len(matches) != 1 {
			return errors.New("重登凭据待绑定：请选择唯一的 OpenAI OAuth 账号")
		}
		target := matches[0]
		if email := strings.TrimSpace(target.GetCredential("email")); email != "" && !strings.EqualFold(email, entry.Email) {
			return errors.New("重登邮箱与所选账号凭据邮箱不一致")
		}
		entry.AccountID = target.ID
		if used[target.ID] {
			return errors.New("同一账号不能重复绑定重登凭据")
		}
		used[target.ID] = true
	}
	return nil
}
func findBoundGuardReloginAccount(cfg AccountTokenGuardConfig, id int64) (AccountTokenGuardReloginAccount, bool) {
	if id <= 0 {
		return AccountTokenGuardReloginAccount{}, false
	}
	for _, e := range cfg.ReloginAccounts {
		if e.AccountID == id {
			return e, true
		}
	}
	return AccountTokenGuardReloginAccount{}, false
}

// Existing OAuth parser validates structure/expiry; identity must match the selected
// account. A real access-token probe is required before restoring guard-owned state.
func validateGuardCredentialIdentity(credential map[string]any, entry AccountTokenGuardReloginAccount, account *Account) error {
	claims, err := openai.ParseIDToken(guardText(credential["id_token"]))
	if err != nil || claims.Exp <= time.Now().Unix() {
		return errors.New("重登返回的身份令牌无效或已过期")
	}
	info := claims.GetUserInfo()
	if info.Email == "" || !strings.EqualFold(info.Email, entry.Email) {
		return errors.New("重登返回的邮箱与绑定凭据不一致")
	}
	if expected := strings.TrimSpace(account.GetCredential("email")); expected != "" && !strings.EqualFold(expected, info.Email) {
		return errors.New("重登身份与目标账号不一致")
	}
	if expected := strings.TrimSpace(account.GetCredential("chatgpt_account_id")); expected != "" && expected != info.ChatGPTAccountID {
		return errors.New("重登返回的 ChatGPT 账号身份不一致")
	}
	expires := guardText(credential["expires_at"])
	if value, ok := credential["expires_at"].(float64); ok {
		if value != float64(int64(value)) {
			return errors.New("重登返回的凭据有效期无效")
		}
		expires = strconv.FormatInt(int64(value), 10)
	}
	expiry, err := time.Parse(time.RFC3339, expires)
	if err != nil {
		seconds, parseErr := strconv.ParseInt(expires, 10, 64)
		if parseErr != nil {
			return errors.New("重登返回的凭据有效期无效")
		}
		expiry = time.Unix(seconds, 0)
	}
	if !expiry.After(time.Now().Add(time.Minute)) {
		return errors.New("重登返回的凭据已过期或即将过期")
	}
	credential["expires_at"] = expiry.UTC().Format(time.RFC3339)
	return nil
}
func tokenGuardExecutorEndpoint() (string, string, error) {
	endpoint := strings.TrimSpace(os.Getenv("TOKEN_GUARD_RELOGIN_URL"))
	if endpoint == "" {
		endpoint = "http://127.0.0.1:8791/relogin"
	}
	key := strings.TrimSpace(os.Getenv("TOKEN_GUARD_RELOGIN_KEY"))
	if key == "" {
		return "", "", errors.New("原生重登执行器未配置认证密钥")
	}
	if err := validateGuardHTTPURL(endpoint, "TOKEN_GUARD_RELOGIN_URL"); err != nil {
		return "", "", errors.New("原生重登执行器地址无效")
	}
	return endpoint, key, nil
}
func (s *AccountTokenGuardService) nativeRelogin(ctx context.Context, entry AccountTokenGuardReloginAccount, account *Account) (map[string]any, error) {
	endpoint, key, err := tokenGuardExecutorEndpoint()
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"email": entry.Email, "password": entry.Password, "mfa_secret": entry.MFASecret, "proxy_url": "", "expected_account_id": account.GetCredential("chatgpt_account_id")}
	if account.ProxyID != nil && account.Proxy != nil {
		payload["proxy_url"] = account.Proxy.URL()
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.New("构造原生重登请求失败")
	}
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("构造原生重登请求失败")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, errors.New("原生重登执行器请求失败")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("原生重登执行器 HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, errors.New("原生重登响应读取失败")
	}
	result, err := parseGuardNDJSONResult(raw)
	if err != nil {
		return nil, errors.New("原生重登响应无效")
	}
	credential, ok := result["credential"].(map[string]any)
	if !ok {
		code := ""
		if failure, ok := result["error"].(map[string]any); ok {
			code = guardText(failure["code"])
		}
		switch code {
		case "invalid_credentials", "password_rejected", "mfa_rejected", "extra_verification", "registration_required", "callback_state", "identity_mismatch", "invalid_token", "network_error", "timeout", "busy", "unsupported_state":
			return nil, fmt.Errorf("原生重登失败（%s）", code)
		default:
			return nil, errors.New("原生重登失败；请检查执行器状态或凭据")
		}
	}
	for _, key := range []string{"access_token", "refresh_token", "id_token"} {
		if value, ok := credential[key].(string); !ok || strings.TrimSpace(value) == "" {
			return nil, errors.New("重登返回的凭据不完整")
		}
	}
	return credential, nil
}

// Reuses native Codex payload, models, headers, proxy, TLS and plugin routing
// without account repository writes (including SetError and rate-limit state).
func (s *AccountTestService) ProbeTokenGuardAccount(ctx context.Context, account *Account, model string) AccountTokenGuardProbeResult {
	started := time.Now()
	result := func(state, detail string) AccountTokenGuardProbeResult {
		return AccountTokenGuardProbeResult{State: state, Detail: detail, LatencyMS: int(time.Since(started).Milliseconds())}
	}
	if !guardAccountEligible(account) {
		return result(AccountTokenGuardProbeTransient, "账号不支持原生 OAuth 探活")
	}
	if strings.TrimSpace(account.GetOpenAIAccessToken()) == "" {
		return result(AccountTokenGuardProbeAuth, "账号没有 access_token")
	}
	model = normalizeOpenAIModelForUpstream(account, account.GetMappedModel(model))
	body, _ := json.Marshal(createOpenAITestPayload(model, true))
	req, err := http.NewRequestWithContext(WithHTTPUpstreamProfile(ctx, HTTPUpstreamProfileOpenAI), http.MethodPost, chatgptCodexAPIURL, bytes.NewReader(body))
	if err != nil {
		return result(AccountTokenGuardProbeTransient, "构造原生探活请求失败")
	}
	req.Host = "chatgpt.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+account.GetOpenAIAccessToken())
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	canonical := resolveCodexOutboundIdentity(account.GetOpenAIUserAgent())
	req.Header.Set("Originator", canonical.originator)
	req.Header.Set("User-Agent", canonical.userAgent)
	setOpenAIChatGPTAccountHeaders(req.Header, account)
	enforceCodexIdentityHeadersWithUA(req.Header, account.GetOpenAIUserAgent())
	account.ApplyHeaderOverrides(req.Header)
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.doOpenAIAccountTestUpstream(req, proxyURL, account, true)
	if err != nil {
		return result(AccountTokenGuardProbeTransient, "原生探活网络请求失败")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return result(AccountTokenGuardProbeAuth, "OpenAI 拒绝账号令牌（401）")
	}
	if resp.StatusCode != http.StatusOK {
		return result(AccountTokenGuardProbeTransient, fmt.Sprintf("OpenAI 探活 HTTP %d", resp.StatusCode))
	}
	scanner := bufio.NewScanner(io.LimitReader(resp.Body, 2<<20))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var event struct {
			Type  string `json:"type"`
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
			Response struct {
				Status string `json:"status"`
				Error  struct {
					Code string `json:"code"`
				} `json:"error"`
			} `json:"response"`
		}
		if json.Unmarshal([]byte(data), &event) != nil {
			continue
		}
		if event.Type == "response.completed" && (event.Response.Status == "" || event.Response.Status == "completed") {
			return result(AccountTokenGuardProbeOK, "原生探活成功")
		}
		code := firstNonEmptyGuard(event.Error.Code, event.Response.Error.Code)
		switch code {
		case "invalid_token", "token_expired", "token_revoked", "invalid_grant", "authentication_error":
			return result(AccountTokenGuardProbeAuth, "OpenAI 报告账号令牌失效")
		}
		if event.Type == "error" || event.Type == "response.failed" {
			return result(AccountTokenGuardProbeTransient, "OpenAI 探活返回临时或未知异常")
		}
	}
	return result(AccountTokenGuardProbeTransient, "原生探活未收到完整成功响应")
}

// ExecutorSource provides the exact source archive bundled with this release.
func (s *AccountTokenGuardService) ExecutorSource(ctx context.Context) ([]byte, error) {
	endpoint, key, err := tokenGuardExecutorEndpoint()
	if err != nil {
		return nil, err
	}
	location, err := url.Parse(endpoint)
	if err != nil {
		return nil, errors.New("执行器地址无效")
	}
	location.Path = "/source.tar.gz"
	requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, location.String(), nil)
	if err != nil {
		return nil, errors.New("构造源码请求失败")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, errors.New("执行器源码暂不可用")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("执行器源码暂不可用")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (32<<20)+1))
	if err != nil || len(data) > 32<<20 {
		return nil, errors.New("读取执行器源码失败")
	}
	return data, nil
}
