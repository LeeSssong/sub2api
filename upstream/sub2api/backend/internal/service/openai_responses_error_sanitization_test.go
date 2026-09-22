package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSanitizeOpenAIResponseFailedEventRemovesUpstreamIdentifiers(t *testing.T) {
	payload := []byte(`{"type":"response.failed","response":{"status":"failed","error":{"code":"server_error","message":"Service temporarily unavailable request id req_secret at https://internal.invalid/v1"}}}`)
	got, changed := sanitizeOpenAIResponseFailedEventForClient(nil, payload, "response.failed", true, &Account{Platform: PlatformOpenAI, Extra: map[string]any{"openai_passthrough": true}})

	require.True(t, changed)
	require.Contains(t, string(got), `"code":"upstream_unavailable"`)
	require.Contains(t, string(got), AppendNativeUserErrorHelp(NativeUserCopyAbnormal, ""))
	require.NotContains(t, strings.ToLower(string(got)), "req_secret")
	require.NotContains(t, strings.ToLower(string(got)), "internal.invalid")
}

func TestSanitizeOpenAIBareErrorRemovesUpstreamIdentifiers(t *testing.T) {
	payload := []byte(`{"type":"error","error":{"code":"server_error","message":"openai_error Ray ID abc-secret"}}`)
	got, changed := sanitizeOpenAIResponseFailedEventForClient(nil, payload, "error", false, &Account{Platform: PlatformOpenAI, Extra: map[string]any{"openai_passthrough": true}})

	require.True(t, changed)
	require.Contains(t, string(got), `"code":"upstream_unavailable"`)
	require.Contains(t, string(got), AppendNativeUserErrorHelp(NativeUserCopyAbnormal, ""))
	require.NotContains(t, strings.ToLower(string(got)), "abc-secret")
}

func TestSanitizeOpenAIPayloadTooLargeTellsUserToOpenNewWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/responses", nil)
	c.Set("ops_model", "gpt-5.6-sol")
	payload := []byte(`{"type":"response.failed","response":{"status":"failed","error":{"code":"upstream_error","message":"Request payload is too large request id req_secret https://internal.example/v1"}}}`)
	account := &Account{Platform: PlatformOpenAI, Extra: map[string]any{"openai_passthrough": true}}

	got, changed := sanitizeOpenAIResponseFailedEventForClient(c, payload, "response.failed", true, account)

	require.True(t, changed)
	require.Contains(t, string(got), `"code":"context_length_exceeded"`)
	require.Contains(t, string(got), `"type":"invalid_request_error"`)
	require.Contains(t, string(got), AppendNativeUserErrorHelp(NativeUserCopyTooLarge, ""))
	require.NotContains(t, strings.ToLower(string(got)), "req_secret")
	require.NotContains(t, strings.ToLower(string(got)), "internal.example")
	require.NotContains(t, string(got), "payload")
}

func TestSanitizeOpenAIModelUnavailableUsesRequestedModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/responses", nil)
	c.Set("ops_model", "gpt-5.6-luna")
	payload := []byte(`{"type":"response.failed","response":{"status":"failed","error":{"code":"upstream_error","message":"Model \"gpt-5.6-luna\" is not allowed for this API key"}}}`)
	account := &Account{Platform: PlatformOpenAI, Extra: map[string]any{"openai_passthrough": true}}

	got, changed := sanitizeOpenAIResponseFailedEventForClient(c, payload, "response.failed", false, account)

	require.True(t, changed)
	require.Equal(t, "group_model_unavailable", gjson.GetBytes(got, "response.error.code").String())
	require.Equal(t, AppendNativeUserErrorHelp(nativeUserModelUnavailableCopy("gpt-5.6-luna"), ""), gjson.GetBytes(got, "response.error.message").String())
	require.NotContains(t, string(got), "API key")
}

func TestSanitizeOpenAINativePassthroughPreservesCapacityError(t *testing.T) {
	payload := []byte(`{"type":"response.failed","response":{"status":"failed","error":{"type":"server_error","code":"server_is_overloaded","message":"The model is currently overloaded. Please try again later."}}}`)
	account := &Account{Platform: PlatformOpenAI, Extra: map[string]any{"openai_passthrough": true}}

	got, changed := sanitizeOpenAIResponseFailedEventForClient(nil, payload, "response.failed", false, account)

	require.True(t, changed)
	require.Contains(t, string(got), `"code":"server_is_overloaded"`)
	require.Contains(t, string(got), AppendNativeUserErrorHelp(NativeUserCopyBusy, ""))
	require.NotContains(t, string(got), "currently overloaded")
	require.NotContains(t, string(got), `"output"`)
}

func TestSanitizeOpenAINativePassthroughPreservesModelSelectionError(t *testing.T) {
	payload := []byte(`{"type":"error","error":{"type":"invalid_request_error","code":"model_not_found","message":"The model 'gpt-5.6-sol' does not exist or you do not have access to it."}}`)
	account := &Account{Platform: PlatformOpenAI, Extra: map[string]any{"openai_passthrough": true}}

	got, changed := sanitizeOpenAIResponseFailedEventForClient(nil, payload, "error", false, account)

	require.True(t, changed)
	require.Contains(t, string(got), `"code":"model_not_found"`)
	require.Contains(t, string(got), AppendNativeUserErrorHelp(NativeUserCopyBadRequest, ""))
	require.NotContains(t, string(got), "does not exist")
}

func TestSanitizeOpenAINativePassthroughPreservesRateLimitError(t *testing.T) {
	payload := []byte(`{"type":"response.failed","response":{"status":"failed","error":{"type":"rate_limit_error","code":"rate_limit_exceeded","message":"Rate limit reached for the model."}}}`)
	account := &Account{Platform: PlatformOpenAI, Extra: map[string]any{"openai_passthrough": true}}

	got, changed := sanitizeOpenAIResponseFailedEventForClient(nil, payload, "response.failed", false, account)

	require.True(t, changed)
	require.Contains(t, string(got), `"code":"rate_limit_exceeded"`)
	require.Contains(t, string(got), AppendNativeUserErrorHelp(NativeUserCopyBusy, ""))
	require.NotContains(t, string(got), "Rate limit reached")
}

func TestSanitizeOpenAINativePassthroughStillSanitizesUnknownProviderError(t *testing.T) {
	payload := []byte(`{"type":"error","error":{"type":"server_error","code":"vendor_internal_error","message":"The upstream vendor rejected this request."}}`)
	account := &Account{Platform: PlatformOpenAI, Extra: map[string]any{"openai_passthrough": true}}

	got, changed := sanitizeOpenAIResponseFailedEventForClient(nil, payload, "error", false, account)

	require.True(t, changed)
	require.Contains(t, string(got), `"code":"upstream_unavailable"`)
	require.Contains(t, string(got), AppendNativeUserErrorHelp(NativeUserCopyAbnormal, ""))
	require.NotContains(t, string(got), "upstream vendor")
}

func TestSanitizeOpenAINonPassthroughStillRewritesCapacityCode(t *testing.T) {
	payload := []byte(`{"type":"error","error":{"type":"server_error","code":"server_is_overloaded","message":"The model is currently overloaded."}}`)
	account := &Account{Platform: PlatformOpenAI, Extra: map[string]any{"openai_passthrough": false}}

	got, changed := sanitizeOpenAIResponseFailedEventForClient(nil, payload, "error", false, account)

	require.True(t, changed)
	require.Contains(t, string(got), `"code":"server_error"`)
	require.NotContains(t, string(got), `"code":"server_is_overloaded"`)
	require.Contains(t, string(got), AppendNativeUserErrorHelp(NativeUserCopyAbnormal, ""))
}

func TestSanitizeOpenAINativePassthroughRequiresOpenAIAccount(t *testing.T) {
	payload := []byte(`{"type":"error","error":{"type":"server_error","code":"server_is_overloaded","message":"The model is currently overloaded."}}`)
	account := &Account{Platform: PlatformAnthropic, Extra: map[string]any{"openai_passthrough": true}}

	got, changed := sanitizeOpenAIResponseFailedEventForClient(nil, payload, "error", false, account)

	require.True(t, changed)
	require.Contains(t, string(got), `"code":"server_error"`)
	require.NotContains(t, string(got), `"code":"server_is_overloaded"`)
	require.Contains(t, string(got), AppendNativeUserErrorHelp(NativeUserCopyAbnormal, ""))
}

func TestSanitizeOpenAINativePassthroughAcceptsLegacySwitch(t *testing.T) {
	payload := []byte(`{"type":"error","error":{"type":"rate_limit_error","code":"rate_limit_exceeded","message":"Rate limit reached."}}`)
	account := &Account{Platform: PlatformOpenAI, Extra: map[string]any{"openai_oauth_passthrough": true}}

	got, changed := sanitizeOpenAIResponseFailedEventForClient(nil, payload, "error", false, account)

	require.True(t, changed)
	require.Contains(t, string(got), `"code":"rate_limit_exceeded"`)
	require.Contains(t, string(got), AppendNativeUserErrorHelp(NativeUserCopyBusy, ""))
	require.NotContains(t, string(got), "Rate limit reached")
}
