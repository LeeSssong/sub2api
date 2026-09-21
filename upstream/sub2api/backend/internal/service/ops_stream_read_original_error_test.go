//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type opsStreamReadFailureBody struct {
	payload []byte
	err     error
	read    bool
}

func (r *opsStreamReadFailureBody) Read(p []byte) (int, error) {
	if !r.read {
		r.read = true
		return copy(p, r.payload), nil
	}
	return 0, r.err
}

func (r *opsStreamReadFailureBody) Close() error { return nil }

func TestOpenAIStreamReadFailureAfterOutputPreservesOriginalAdminMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{
		cfg: &config.Config{Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			MaxLineSize:               defaultMaxLineSize,
		}},
		toolCorrector: NewCodexToolCorrector(),
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: &opsStreamReadFailureBody{
			payload: []byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n"),
			err:     io.ErrUnexpectedEOF,
		},
	}

	_, err := svc.handleStreamingResponse(
		context.Background(), resp, c,
		&Account{ID: 374, Platform: PlatformGrok, Type: AccountTypeAPIKey},
		time.Now(), "grok-4.6", "grok-4.6",
	)
	require.Error(t, err)
	require.NotContains(t, recorder.Body.String(), io.ErrUnexpectedEOF.Error())

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.NotEmpty(t, events)
	require.NotNil(t, events[len(events)-1].OriginalError)
	require.Equal(t, io.ErrUnexpectedEOF.Error(), events[len(events)-1].OriginalError.Message)
	require.Empty(t, events[len(events)-1].OriginalError.Body)

	accountID := int64(374)
	entry := &OpsInsertErrorLogInput{
		ErrorPhase:     "upstream",
		ErrorType:      "upstream_error",
		AccountID:      &accountID,
		UpstreamErrors: events,
	}
	require.NoError(t, SanitizeOpsUpstreamErrorsForQueue(entry))
	require.NotNil(t, entry.UpstreamErrorsJSON)
	detail := AttachNativeErrorDiagnosis(&OpsErrorLogDetail{
		OpsErrorLog:    OpsErrorLog{Phase: "upstream", Owner: "provider", AccountID: &accountID},
		UpstreamErrors: *entry.UpstreamErrorsJSON,
	})
	require.NotNil(t, detail.Diagnosis)
	require.Equal(t, io.ErrUnexpectedEOF.Error(), detail.Diagnosis.OriginalUpstreamMessage)
	require.Equal(t, io.ErrUnexpectedEOF.Error(), detail.UpstreamErrorMessage)
}
