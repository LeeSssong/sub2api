package service

import (
	"net/http"
	"regexp"
	"strings"
	"unicode"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

const (
	NativeUserCopyTooLarge     = "请求内容过大，请缩短内容后重试。"
	NativeUserCopyUpload       = "请求上传中断，请检查网络后重试。"
	NativeUserCopyBalance      = "余额不足，请充值后重试。"
	NativeUserCopyQuota        = "额度或订阅不可用，请检查当前套餐后重试。"
	NativeUserCopyAuth         = "认证失败，请检查 API Key 后重试。"
	NativeUserCopyRate         = "请求过于频繁，请稍后重试或降低并发。"
	NativeUserCopyPermission   = "当前模型或分组不可用，请调整后重试。"
	NativeUserCopyBadRequest   = "请求参数或格式不正确，请检查后重试。"
	NativeUserCopyBusy         = "服务暂时繁忙，请稍后重试。"
	NativeUserCopyAbnormal     = "服务暂时异常，请稍后重试。"
	NativeUserCopyFailed       = "请求处理失败，请检查后重试。"
	nativeUserErrorAdminHelp   = "如需协助请联系管理员"
	nativeTrustedUserCopyKey   = "native_trusted_user_copy"
	nativeRequestIDHeader      = "X-Request-ID"
)

// NativeUserErrorContactAdminSuggestion 是站内错误页的统一建议，Request ID 由详情字段单独给出。
const NativeUserErrorContactAdminSuggestion = nativeUserErrorAdminHelp + "并提供 Request ID"

type NativeUserErrorInput struct {
	Status          int
	Type            string
	Code            string
	Message         string
	Stage           string
	Ownership       string
	AccountSelected bool
	RequestID       string
	TrustedCopy     bool
}

type NativeUserErrorProjection struct {
	Type    string
	Code    string
	Message string
}

var nativeUserErrorLeakPattern = regexp.MustCompile(`(?i)(?:https?://|cloudflare|ray\s*id|\bupstream\b|上游|req_[a-z0-9_-]+)`)

func ProjectNativeUserError(input NativeUserErrorInput) NativeUserErrorProjection {
	result := ProjectNativeUserErrorCore(input)
	result.Message = AppendNativeUserErrorHelp(result.Message, input.RequestID)
	return result
}

func ProjectNativeUserErrorCore(input NativeUserErrorInput) NativeUserErrorProjection {
	errType := strings.TrimSpace(input.Type)
	if errType == "" {
		errType = "api_error"
	}
	result := NativeUserErrorProjection{Type: errType, Code: strings.TrimSpace(input.Code)}
	copy := strings.TrimSpace(input.Message)
	upstreamFacing := nativeUserErrorUpstreamFacing(input, errType)
	if copy != "" {
		if input.TrustedCopy && isSafeTrustedUserCopy(copy) {
			result.Message = copy
			return result
		}
		if !upstreamFacing && isSafeNativeUserMessage(copy) {
			result.Message = copy
			return result
		}
	}
	result.Message = classifyNativeUserErrorCopy(input, errType, upstreamFacing)
	if !isSafeNativeUserMessage(result.Message) {
		result.Message = NativeUserCopyAbnormal
	}
	return result
}

func nativeUserErrorUpstreamFacing(input NativeUserErrorInput, errType string) bool {
	typeLower := strings.ToLower(strings.TrimSpace(errType))
	stage := strings.ToLower(strings.TrimSpace(input.Stage))
	return input.AccountSelected ||
		typeLower == "upstream_error" ||
		stage == "upstream" || stage == "network" || stage == "account_auth"
}

func classifyNativeUserErrorCopy(input NativeUserErrorInput, errType string, upstreamFacing bool) string {
	code := strings.ToLower(strings.TrimSpace(input.Code))
	typeLower := strings.ToLower(strings.TrimSpace(errType))

	switch {
	case input.Status == http.StatusRequestEntityTooLarge || code == "context_length_exceeded":
		return NativeUserCopyTooLarge
	case input.Status == 499 || typeLower == "client_closed":
		return NativeUserCopyUpload
	case !upstreamFacing && (typeLower == "billing_error" || code == "insufficient_balance" || input.Status == http.StatusPaymentRequired):
		return NativeUserCopyBalance
	case !upstreamFacing && isNativeUserQuotaIdentity(typeLower, code):
		return NativeUserCopyQuota
	case !upstreamFacing && (input.Status == http.StatusUnauthorized || typeLower == "authentication_error" ||
		code == "invalid_api_key" || code == "api_key_required" || code == "api_key_disabled" ||
		code == "user_not_found" || code == "user_inactive"):
		return NativeUserCopyAuth
	case !upstreamFacing && (input.Status == http.StatusTooManyRequests || typeLower == "rate_limit_error"):
		return NativeUserCopyRate
	case typeLower == NativeErrorClassLocalCapacity || typeLower == "compact_not_supported" ||
		(!upstreamFacing && input.Status == http.StatusServiceUnavailable):
		return NativeUserCopyBusy
	case typeLower == "permission_error" || typeLower == "cyber_policy" || typeLower == "unsupported_model" ||
		(!upstreamFacing && input.Status == http.StatusForbidden):
		return NativeUserCopyPermission
	case typeLower == "invalid_request_error" || (!upstreamFacing && input.Status == http.StatusBadRequest):
		return NativeUserCopyBadRequest
	case upstreamFacing && (input.Status == http.StatusTooManyRequests || input.Status == 529 ||
		typeLower == "overloaded_error" || typeLower == "rate_limit_error" || code == "server_is_overloaded"):
		return NativeUserCopyBusy
	case upstreamFacing || input.Status >= 500:
		return NativeUserCopyAbnormal
	default:
		return NativeUserCopyFailed
	}
}

func isNativeUserQuotaIdentity(typeLower, code string) bool {
	switch typeLower {
	case "subscription_error":
		return true
	}
	switch code {
	case "subscription_not_found", "subscription_invalid", "api_key_expired", "api_key_quota_exhausted":
		return true
	}
	return strings.Contains(code, "usage_limit") || strings.Contains(code, "quota")
}

func AppendNativeUserErrorHelp(message, requestID string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		message = NativeUserCopyAbnormal
	}
	if strings.Contains(message, nativeUserErrorAdminHelp) {
		return message
	}
	if !strings.HasSuffix(message, "。") && !strings.HasSuffix(message, ".") {
		message += "。"
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return message + nativeUserErrorAdminHelp + "。"
	}
	return message + nativeUserErrorAdminHelp + "并提供 Request ID: " + requestID + "。"
}

func NativeUserErrorRequestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if c.Request != nil {
		if rid, ok := c.Request.Context().Value(ctxkey.RequestID).(string); ok {
			if rid = strings.TrimSpace(rid); rid != "" {
				return rid
			}
		}
	}
	if c.Writer != nil {
		return strings.TrimSpace(c.Writer.Header().Get(nativeRequestIDHeader))
	}
	return ""
}

func projectSelectedAccountUserError(c *gin.Context, status int, errType, code, message string) NativeUserErrorProjection {
	return ProjectNativeUserErrorFromGin(c, status, errType, code, message, true, "upstream", "provider")
}

func writeProjectedOpenAIUserError(c *gin.Context, status int, errType, message string) {
	writeProjectedOpenAIUserErrorClassified(c, status, status, errType, "", message)
}

func writeProjectedOpenAIUserErrorClassified(c *gin.Context, writeStatus, classifyStatus int, errType, code, message string) {
	projected := projectSelectedAccountUserError(c, classifyStatus, errType, code, message)
	c.JSON(writeStatus, gin.H{"error": gin.H{"type": projected.Type, "message": projected.Message}})
}

func writeProjectedAnthropicUserError(c *gin.Context, status int, errType, message string) {
	projected := projectSelectedAccountUserError(c, status, errType, "", message)
	c.JSON(status, gin.H{"type": "error", "error": gin.H{"type": projected.Type, "message": projected.Message}})
}

func ProjectNativeUserErrorFromGin(c *gin.Context, status int, errType, code, message string, accountSelected bool, stage, owner string) NativeUserErrorProjection {
	input := NativeUserErrorInput{
		Status:          status,
		Type:            errType,
		Code:            code,
		Message:         message,
		Stage:           stage,
		Ownership:       owner,
		AccountSelected: accountSelected,
		RequestID:       NativeUserErrorRequestID(c),
	}
	if trusted, ok := NativeTrustedUserCopy(c); ok {
		input.TrustedCopy = true
		input.Message = trusted
	}
	return ProjectNativeUserError(input)
}

func SetNativeTrustedUserCopy(c *gin.Context, message string) {
	if c == nil {
		return
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	c.Set(nativeTrustedUserCopyKey, message)
}

func NativeTrustedUserCopy(c *gin.Context) (string, bool) {
	if c == nil {
		return "", false
	}
	value, ok := c.Get(nativeTrustedUserCopyKey)
	if !ok {
		return "", false
	}
	message, ok := value.(string)
	if !ok {
		return "", false
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return "", false
	}
	return message, true
}

func isSafeTrustedUserCopy(message string) bool {
	return strings.TrimSpace(message) != "" && !nativeUserErrorLeakPattern.MatchString(message)
}

func isSafeNativeUserMessage(message string) bool {
	if !isSafeTrustedUserCopy(message) {
		return false
	}
	for _, r := range message {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
