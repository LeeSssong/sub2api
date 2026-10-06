//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Historical #6171 draft tests arrived in 61fd6c626b without the proposed
// synthesized-error field or policy. Exercise the current upstream contract:
// unavailable responses cool image scope and still allow failover.

// countingModelRateLimitRepo 记录 SetModelRateLimit 调用，用于断言"没写账号状态"。
type countingModelRateLimitRepo struct {
	accountRepoStub
	calls  int
	scopes []string
}

func (r *countingModelRateLimitRepo) SetModelRateLimit(_ context.Context, _ int64, scope string, _ time.Time, _ ...string) error {
	r.calls++
	r.scopes = append(r.scopes, scope)
	return nil
}

func newImagesCooldownContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	return c, rec
}

func imagesCooldownAccount() *Account {
	return &Account{ID: 77, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Name: "img-oauth"}
}

// Official policy retries text-only responses without parking the account.
func TestHandleOpenAIImagesOAuthResponseError_TextFallbackDoesNotCoolImageScope(t *testing.T) {
	c, _ := newImagesCooldownContext(t)
	repo := &countingModelRateLimitRepo{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	account := imagesCooldownAccount()

	upstreamErr := openAIImagesTextFallbackErrorForText("Here's a polished image prompt for your request.")
	require.NotNil(t, upstreamErr)
	require.Equal(t, "image_generation_unavailable", upstreamErr.Code)

	err := svc.handleOpenAIImagesOAuthResponseError(
		context.Background(), c, account, "gpt-image-2", "https://upstream.example/v1/responses",
		&http.Response{StatusCode: http.StatusOK, Header: http.Header{}},
		OpenAIImagesJSONKeepaliveAdjustedWrittenSize(c), upstreamErr,
	)

	require.Zero(t, repo.calls)
	require.Empty(t, repo.scopes)

	// Cooldown still permits the handler to fail over to another account.
	var failover *UpstreamFailoverError
	require.True(t, errors.As(err, &failover), "仍应触发换号，got %T", err)
}

// 对照不变式：上游 error 帧点名该状态时仍然冷却，否则等于把功能整个废掉。
func TestHandleOpenAIImagesOAuthResponseError_StructuredUnavailableStillCoolsAccount(t *testing.T) {
	c, _ := newImagesCooldownContext(t)
	repo := &countingModelRateLimitRepo{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	account := imagesCooldownAccount()

	upstreamErr := &OpenAIImagesUpstreamError{
		StatusCode: http.StatusBadGateway,
		ErrorType:  "upstream_error",
		Code:       "image_generation_unavailable",
		Message:    "image generation tool is not available for this account",
	}

	_ = svc.handleOpenAIImagesOAuthResponseError(
		context.Background(), c, account, "gpt-image-2", "https://upstream.example/v1/responses",
		&http.Response{StatusCode: http.StatusOK, Header: http.Header{}},
		OpenAIImagesJSONKeepaliveAdjustedWrittenSize(c), upstreamErr,
	)

	require.Equal(t, 1, repo.calls, "结构化上游证据仍须写冷却")
	require.Equal(t, []string{openAIImageGenerationRateLimitKey}, repo.scopes)
}

// Both text entry points retain retryability; policy refusal stays a client error.
func TestOpenAIImagesTextFallback_ClassifiesResponseText(t *testing.T) {
	t.Run("plain_text_reply_is_unavailable", func(t *testing.T) {
		err := openAIImagesTextFallbackErrorForText("Here's a polished image prompt for your request.")
		require.NotNil(t, err)
		require.Equal(t, "image_generation_unavailable", err.Code)
		require.Equal(t, http.StatusBadGateway, err.StatusCode)
	})

	t.Run("body_entrypoint_is_unavailable", func(t *testing.T) {
		body := []byte("event: response.completed\n" +
			`data: {"type":"response.completed","response":{"id":"r","status":"completed",` +
			`"output":[{"type":"message","content":[{"type":"output_text","text":"I drafted a prompt for you."}]}]}}` +
			"\n\n")
		err := openAIImagesTextFallbackError(body)
		require.NotNil(t, err)
	})

	t.Run("content_policy_branch_unchanged", func(t *testing.T) {
		err := openAIImagesTextFallbackErrorForText("Blocked by our content policy.")
		require.NotNil(t, err)
		require.Equal(t, "content_policy_violation", err.Code)
		require.Equal(t, http.StatusBadRequest, err.StatusCode)
	})

	t.Run("empty_text_yields_no_error", func(t *testing.T) {
		require.Nil(t, openAIImagesTextFallbackErrorForText("   "))
	})
}

// 级联的前提条件：该错误确实是可重试的，所以会带着"已写冷却"的副作用换号。
// 这条用例把前提钉死，避免以后有人把 502 改成非重试后误以为本修复多余。
func TestOpenAIImagesTextFallback_RemainsRetryableAndThusCascades(t *testing.T) {
	err := openAIImagesTextFallbackErrorForText("Here's a polished image prompt for your request.")
	require.NotNil(t, err)
	require.True(t, IsOpenAIImagesRetryableUpstreamError(err),
		"文字兜底判据是可重试的——正因如此，写账号冷却会沿号池级联")
}
