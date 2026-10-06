package service

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requesttiming"
	"github.com/gin-gonic/gin"
)

func TestRecordOpenAIRequestDiagnosticsUsesExplicitHeaderAndBoundedPrefixSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	body := []byte(`{"model":"gpt-6.1-sol","messages":[{"role":"system","content":"stable"},{"role":"user","content":"hello"}],"tools":[{"type":"function"}]}`)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(string(body)))
	c.Request.Header.Set("session-id", "secret-session")
	collector := requesttiming.New(time.Now(), c.Request.ContentLength)
	c.Request = c.Request.WithContext(requesttiming.With(c.Request.Context(), collector))
	RecordOpenAIRequestDiagnostics(c, body, "hash")
	collector.Finish(200, false)
	var got requesttiming.Snapshot
	collector.WhenFinished(func(s requesttiming.Snapshot) { got = s })
	if got.Diagnostics["session_identity_source"] != "explicit_header" || !strings.Contains(got.Diagnostics["prefix_summary"], "prefix_hash=") {
		t.Fatalf("unexpected diagnostics: %#v", got.Diagnostics)
	}
	if strings.Contains(got.Diagnostics["prefix_summary"], "stable") || strings.Contains(got.Diagnostics["prefix_summary"], "hello") {
		t.Fatalf("raw prompt leaked into diagnostics: %#v", got.Diagnostics)
	}
}

func TestOpenAIPrefixSummaryChangesWhenContentChangesWithSameLength(t *testing.T) {
	a := openAIPrefixSummary([]byte(`{"model":"gpt-6.1-sol","messages":[{"role":"system","content":"abc"},{"role":"user","content":"hello"}]}`))
	b := openAIPrefixSummary([]byte(`{"model":"gpt-6.1-sol","messages":[{"role":"system","content":"xyz"},{"role":"user","content":"world"}]}`))
	if a == b || !strings.Contains(a, "prefix_hash=") || !strings.Contains(b, "prefix_hash=") {
		t.Fatalf("same-length prefix changes were not distinguished: %q / %q", a, b)
	}
}
