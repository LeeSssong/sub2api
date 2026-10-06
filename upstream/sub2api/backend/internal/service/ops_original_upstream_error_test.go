package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// A client-safe projection must never replace the upstream evidence in admin details.
func TestOpenAIStreamOriginalErrorSurvivesAdminPersistence(t *testing.T) {
	const message = "Encrypted function output content could not be decrypted or decoded."
	const payload = `{"type":"response.failed", "response":{"error":{"type":"invalid_request_error","code":"thinking_signature_invalid","message":"Encrypted function output content could not be decrypted or decoded."},"metadata":{"access_token":"fixture-only"}}}`
	for _, passthrough := range []bool{false, true} {
		for _, partial := range []bool{false, true} {
			name := "native"
			if passthrough {
				name = "passthrough"
			}
			if partial {
				name += "_after_output"
			}
			t.Run(name, func(t *testing.T) {
				gin.SetMode(gin.TestMode)
				svc := &OpenAIGatewayService{cfg: &config.Config{}, toolCorrector: NewCodexToolCorrector()}
				account := &Account{ID: 339, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				body := ""
				if partial {
					body = "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n"
				}
				body += "data: " + payload + "\n\n"
				resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}
				if passthrough {
					_, err := svc.handleStreamingResponsePassthrough(context.Background(), resp, c, account, time.Now(), "gpt-5", "gpt-5")
					require.Error(t, err)
				} else {
					_, err := svc.handleStreamingResponse(context.Background(), resp, c, account, time.Now(), "gpt-5", "gpt-5")
					require.Error(t, err)
				}
				require.NotContains(t, recorder.Body.String(), message)
				require.NotContains(t, recorder.Body.String(), "fixture-only")
				require.Contains(t, recorder.Body.String(), NativeUserCopyBadRequest)
				events, ok := c.Get(OpsUpstreamErrorsKey)
				require.True(t, ok, "all terminal failures must retain upstream evidence before projecting client copy")
				entry := &OpsInsertErrorLogInput{ErrorPhase: "upstream", ErrorType: "upstream_error", AccountID: &account.ID, UpstreamErrors: events.([]*OpsUpstreamErrorEvent)}
				require.NoError(t, SanitizeOpsUpstreamErrorsForQueue(entry))
				var stored *OpsInsertErrorLogInput
				ops := NewOpsService(&opsRepoMock{InsertErrorLogFn: func(_ context.Context, input *OpsInsertErrorLogInput) (int64, error) { stored = input; return 1, nil }}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
				require.NoError(t, ops.RecordError(context.Background(), entry))
				require.NotNil(t, stored)
				require.NotNil(t, stored.UpstreamErrorsJSON)
				detail := AttachNativeErrorDiagnosis(&OpsErrorLogDetail{OpsErrorLog: OpsErrorLog{Phase: "upstream", Owner: "provider", AccountID: &account.ID}, UpstreamErrors: *stored.UpstreamErrorsJSON})
				require.NotNil(t, detail.Diagnosis)
				require.Equal(t, message, detail.Diagnosis.OriginalUpstreamMessage)
				require.Equal(t, payload, detail.Diagnosis.OriginalUpstreamDetail)
				require.Equal(t, payload, detail.UpstreamErrorDetail)
				userView, marshalErr := json.Marshal(ToUserErrorRequestDetail(detail))
				require.NoError(t, marshalErr)
				require.NotContains(t, string(userView), "fixture-only")
				require.NotContains(t, string(userView), "original_error")
				require.NotContains(t, string(userView), message)

			})
		}
	}
}

func TestOpenAIHTTPOriginalErrorPreservesWhitespaceAndClientProjection(t *testing.T) {
	const payload = " {\n  \"error\": {\"type\":\"invalid_request_error\",\"message\":\"bad schema https://provider.example?access_token=fixture-only\"}\n}\n"
	c, recorder := newOpenAIUpstreamErrorTestContext(t)
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	_, err := svc.handleErrorResponse(context.Background(), newOpenAIUpstreamErrorResponse(400, payload), c, newOpenAIUpstreamErrorTestAccount(), nil)
	require.Error(t, err)
	require.Contains(t, recorder.Body.String(), NativeUserCopyBadRequest)
	require.NotContains(t, recorder.Body.String(), "fixture-only")
	events, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	entry := &OpsInsertErrorLogInput{UpstreamErrors: events.([]*OpsUpstreamErrorEvent)}
	require.NoError(t, SanitizeOpsUpstreamErrorsForQueue(entry))
	detail := AttachNativeErrorDiagnosis(&OpsErrorLogDetail{OpsErrorLog: OpsErrorLog{Phase: "upstream", Owner: "provider", AccountID: int64Ptr(1)}, UpstreamErrors: *entry.UpstreamErrorsJSON})
	require.Equal(t, payload, detail.UpstreamErrorDetail)
	require.Equal(t, "bad schema https://provider.example?access_token=fixture-only", detail.UpstreamErrorMessage)
}

func TestOpsOriginalErrorStorageBoundsAreExplicitAndUTF8Safe(t *testing.T) {
	original := &OpsOriginalUpstreamError{Message: strings.Repeat("错", 1000), Body: strings.Repeat("错", 10000)}
	entry := &OpsInsertErrorLogInput{UpstreamErrors: []*OpsUpstreamErrorEvent{{UpstreamStatusCode: 502, Message: "failed", OriginalError: original}}}
	require.NoError(t, SanitizeOpsUpstreamErrorsForQueue(entry))
	detail := AttachNativeErrorDiagnosis(&OpsErrorLogDetail{OpsErrorLog: OpsErrorLog{Phase: "upstream", Owner: "provider", AccountID: int64Ptr(1)}, UpstreamErrors: *entry.UpstreamErrorsJSON})
	require.True(t, detail.Diagnosis.OriginalUpstreamTruncated)
	require.True(t, utf8.ValidString(detail.UpstreamErrorDetail))
	require.True(t, utf8.ValidString(detail.UpstreamErrorMessage))
	require.LessOrEqual(t, len(detail.UpstreamErrorDetail), opsMaxStoredErrorBodyBytes)
	require.True(t, strings.HasPrefix(original.Body, detail.UpstreamErrorDetail))
	require.False(t, original.Truncated, "queue preparation must not mutate a request snapshot")
}

func TestOpsOriginalErrorDoesNotReplaceLaterAttemptFailure(t *testing.T) {
	entry := &OpsInsertErrorLogInput{UpstreamErrors: []*OpsUpstreamErrorEvent{
		{UpstreamStatusCode: 429, Message: "earlier", OriginalError: newOpsOriginalUpstreamError([]byte(`{"error":{"message":"earlier raw failure"}}`))},
		{UpstreamStatusCode: 502, Message: "final transport failure"},
	}}
	require.NoError(t, SanitizeOpsUpstreamErrorsForQueue(entry))
	detail := AttachNativeErrorDiagnosis(&OpsErrorLogDetail{OpsErrorLog: OpsErrorLog{Phase: "upstream", Owner: "provider", AccountID: int64Ptr(1)}, UpstreamErrorMessage: "final transport failure", UpstreamErrors: *entry.UpstreamErrorsJSON})
	require.Equal(t, "final transport failure", detail.Diagnosis.OriginalUpstreamMessage)
}
