package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAPIKeyCacheCreationAsInputResponses(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, passthrough := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				for _, path := range []string{"/v1/responses", "/v1/responses/compact"} {
					t.Run(fmt.Sprintf("stream=%t/pass=%t/enabled=%t/%s", stream, passthrough, enabled, path), func(t *testing.T) {
						payload := `{"id":"resp_cache","status":"completed","model":"gpt-6-astra","output":[],"usage":` + excelBPSUsageWithCreation + `}`
						contentType := "application/json"
						if stream {
							payload = "data: " + `{"type":"response.completed","response":` + payload + "}\n\n"
							contentType = "text/event-stream"
						}
						upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(payload))}}
						svc := openAIClientToolsTestService(upstream)
						account := &Account{ID: 51, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
							Credentials: map[string]any{"api_key": "test-api-key", "base_url": "https://api.example.com"},
							Extra:       map[string]any{"openai_excel_bps_cache_creation_as_input": enabled, "openai_passthrough": passthrough}}
						require.False(t, account.IsExcelBPSEnabled(), "billing opt-in must not change API key routing")
						body := []byte(fmt.Sprintf(`{"model":"gpt-6-astra","stream":%t,"input":"cache billing regression"}`, stream))
						rec := httptest.NewRecorder()
						c, _ := gin.CreateTestContext(rec)
						c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
						result, err := svc.Forward(context.Background(), c, account, body)
						require.NoError(t, err)
						require.Equal(t, 200, result.Usage.CacheCreationInputTokens, "retain raw upstream measurement")
						downstream := rec.Body.Bytes()
						if strings.Contains(rec.Header().Get("Content-Type"), "text/event-stream") {
							for _, line := range strings.Split(rec.Body.String(), "\n") {
								if strings.HasPrefix(line, "data: ") && gjson.Get(strings.TrimPrefix(line, "data: "), "response.usage").Exists() {
									downstream = []byte(gjson.Get(strings.TrimPrefix(line, "data: "), "response").Raw)
								}
							}
						}
						usage, ok := extractOpenAIUsageFromJSONBytes(downstream)
						require.True(t, ok, string(downstream))
						want := 200
						if enabled {
							want = 0
						}
						require.Equal(t, want, usage.CacheCreationInputTokens)
						require.Equal(t, 1000, usage.InputTokens)
						require.Equal(t, 100, usage.CacheReadInputTokens)
						require.Equal(t, "9007199254740993", gjson.GetBytes(downstream, "usage.extension").Raw)
					})
				}
			}
		}
	}
}
