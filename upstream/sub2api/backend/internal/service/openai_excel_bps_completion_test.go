package service

import (
	"context"
	"encoding/json"
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

func bpsCompletionResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
func TestExcelBPSCorrectionUsesSameModelAndCountsBothAttempts(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			response := func(name, id string, tokens int) *http.Response {
				envelope, _ := json.Marshal(map[string]any{"name": name, "arguments": map[string]any{"cmd": "pwd"}})
				event, _ := json.Marshal(map[string]any{"type": "response.completed", "response": map[string]any{"id": id, "status": "completed", "usage": map[string]any{"input_tokens": tokens, "output_tokens": 1}, "output": []any{map[string]any{"id": "fc_" + id, "call_id": "call_" + id, "type": "function_call", "name": "run_officejs", "arguments": map[string]any{"code": string(envelope)}}}}})
				return bpsCompletionResponse(200, "data: "+string(event)+"\n\n")
			}
			upstream := &httpUpstreamRecorder{responses: []*http.Response{response("missing", "original", 5), response("shell", "fixed", 7)}}
			svc := openAIClientToolsTestService(upstream)
			body := []byte(fmt.Sprintf(`{"model":"gpt-6-astra","stream":%v,"input":"run pwd","tools":[{"type":"function","name":"shell","parameters":{"type":"object","required":["cmd"],"properties":{"cmd":{"type":"string"}}}}]}`, stream))
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
			result, err := svc.Forward(context.Background(), c, excelAccount(), body)
			require.NoError(t, err)
			require.Len(t, upstream.requests, 2)
			require.EqualValues(t, 12, result.Usage.InputTokens)
			require.EqualValues(t, 2, result.Usage.OutputTokens)
			require.Equal(t, upstream.requests[0].Header.Get("Authorization"), upstream.requests[1].Header.Get("Authorization"))
			require.Equal(t, gjson.GetBytes(upstream.bodies[0], "model").String(), gjson.GetBytes(upstream.bodies[1], "model").String())
			require.Contains(t, string(upstream.bodies[1]), "No client tool from that response was executed")
			require.Contains(t, rec.Body.String(), `"name":"shell"`)
			require.NotContains(t, rec.Body.String(), `"name":"missing"`)
		})
	}
}
