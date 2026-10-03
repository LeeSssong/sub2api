package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requesttiming"
	"github.com/gin-gonic/gin"
)

var diagnosticProcessKey = func() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return []byte("sub2api-diagnostic-process-key")
	}
	return key
}()

// RecordOpenAIRequestDiagnostics records bounded request metadata on the
// request timing trace. It deliberately never stores session IDs or bodies.
func RecordOpenAIRequestDiagnostics(c *gin.Context, body []byte, sessionHash string) {
	if c == nil {
		return
	}
	ctx := c.Request.Context()
	explicit := explicitOpenAIHeaderSessionID(c)
	view := openAIRequestPayloadView(body)
	promptKey := strings.TrimSpace(view.Get("prompt_cache_key").String())
	source := "missing"
	switch {
	case explicit != "":
		source = "explicit_header"
	case promptKey != "":
		source = "prompt_cache_key"
	case sessionHash != "":
		source = "content_fallback"
	}
	requesttiming.SetDiagnostic(ctx, "session_identity_source", source)
	requesttiming.SetDiagnostic(ctx, "session_id_present", strconv.FormatBool(explicit != ""))
	requesttiming.SetDiagnostic(ctx, "prompt_cache_key_present", strconv.FormatBool(promptKey != ""))
	requesttiming.SetDiagnostic(ctx, "scheduler_session_hash_present", strconv.FormatBool(sessionHash != ""))
	requesttiming.SetDiagnostic(ctx, "prefix_summary", openAIPrefixSummary(body))
}

func openAIPrefixSummary(body []byte) string {
	view := openAIRequestPayloadView(body)
	model := strings.TrimSpace(view.Get("model").String())
	instructions := view.Get("instructions").String()
	tools := view.Get("tools")
	functions := view.Get("functions")
	systemLen, developerLen, firstUserLen := 0, 0, 0
	systemText, developerText, firstUserText := "", "", ""
	messages := view.Get("messages")
	if messages.IsArray() {
		for _, message := range messages.Array() {
			role := strings.ToLower(strings.TrimSpace(message.Get("role").String()))
			textLen := len(message.Get("content").String())
			switch role {
			case "system":
				if systemLen == 0 {
					systemLen = textLen
					systemText = message.Get("content").String()
				}
			case "developer":
				if developerLen == 0 {
					developerLen = textLen
					developerText = message.Get("content").String()
				}
			case "user":
				if firstUserLen == 0 {
					firstUserLen = textLen
					firstUserText = message.Get("content").String()
				}
			}
		}
	}
	canonical := strings.Join([]string{
		model, instructions, boundedDigestInput(tools.Raw), boundedDigestInput(functions.Raw),
		boundedDigestInput(view.Get("input").Raw),
		boundedDigestInput(systemText), boundedDigestInput(developerText), boundedDigestInput(firstUserText),
		strconv.Itoa(systemLen), strconv.Itoa(developerLen), strconv.Itoa(firstUserLen),
	}, "|")
	return strings.Join([]string{
		"model=" + model,
		"tools=" + strconv.Itoa(len(tools.Array())),
		"functions=" + strconv.Itoa(len(functions.Array())),
		"system_len=" + strconv.Itoa(systemLen),
		"developer_len=" + strconv.Itoa(developerLen),
		"first_user_len=" + strconv.Itoa(firstUserLen),
		"prefix_hash=" + diagnosticDigest([]byte(canonical)),
	}, ";")
}

func boundedDigestInput(value string) string {
	if len(value) > 8192 {
		return value[:8192]
	}
	return value
}

func diagnosticDigest(value []byte) string {
	key := strings.TrimSpace(os.Getenv("SUB2API_DIAGNOSTIC_HASH_KEY"))
	if key != "" {
		mac := hmac.New(sha256.New, []byte(key))
		_, _ = mac.Write(value)
		return hex.EncodeToString(mac.Sum(nil)[:12])
	}
	mac := hmac.New(sha256.New, diagnosticProcessKey)
	_, _ = mac.Write(value)
	return hex.EncodeToString(mac.Sum(nil)[:12])
}
