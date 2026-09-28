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

func TestAPIKeyCacheCreationAsInputChatProtocols(t *testing.T) {
	for _, anthropic := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("anthropic=%t/stream=%t", anthropic, stream), func(t *testing.T) {
				payload := `{"id":"chatcmpl_cache","object":"chat.completion","model":"gpt-5.4","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1000,"completion_tokens":50,"total_tokens":1050,"prompt_tokens_details":{"cached_tokens":100,"cache_write_tokens":200}}}`
				contentType := "application/json"
				if stream {
					payload = "data: " + strings.ReplaceAll(payload, `"message":{"role":"assistant","content":"hi"}`, `"delta":{"content":"hi"}`) + "\n\ndata: [DONE]\n\n"
					contentType = "text/event-stream"
				}
				upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(payload))}}
				svc := openAIClientToolsTestService(upstream)
				account := rawChatCompletionsTestAccount()
				account.Credentials["base_url"] = "https://api.example.com"
				account.Extra = map[string]any{"openai_responses_supported": false, "openai_excel_bps_cache_creation_as_input": true}
				body := []byte(fmt.Sprintf(`{"model":"gpt-5.4","stream":%t,"max_tokens":100,"messages":[{"role":"user","content":"hi"}]}`, stream))
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
				var result *OpenAIForwardResult
				var err error
				if anthropic {
					result, err = svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")
				} else {
					result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
				}
				require.NoError(t, err)
				require.Equal(t, 200, result.Usage.CacheCreationInputTokens, "raw upstream measurement must survive output rewrite")
				var usages []gjson.Result
				if stream {
					for _, line := range strings.Split(rec.Body.String(), "\n") {
						if strings.HasPrefix(line, "data:") {
							value := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
							for _, path := range []string{"usage", "message.usage"} {
								if u := gjson.Get(value, path); u.IsObject() {
									usages = append(usages, u)
								}
							}
						}
					}
				} else {
					usages = append(usages, gjson.Get(rec.Body.String(), "usage"))
				}
				require.NotEmpty(t, usages, rec.Body.String())
				for _, u := range usages {
					require.Zero(t, u.Get("cache_creation_input_tokens").Int())
					require.Zero(t, u.Get("prompt_tokens_details.cache_write_tokens").Int())
				}
				if !stream {
					if anthropic {
						require.EqualValues(t, 900, usages[0].Get("input_tokens").Int())
						require.EqualValues(t, 100, usages[0].Get("cache_read_input_tokens").Int())
					} else {
						require.EqualValues(t, 1000, usages[0].Get("prompt_tokens").Int())
						require.EqualValues(t, 100, usages[0].Get("prompt_tokens_details.cached_tokens").Int())
					}
				}
			})
		}
	}
}
