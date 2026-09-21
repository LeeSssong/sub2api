package service

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// OpsOriginalUpstreamError preserves upstream response content for administrator
// diagnostics. Never use this evidence as a client response or an ordinary log.
// It lives in the existing attempt JSON, so no schema migration is required.
type OpsOriginalUpstreamError struct {
	Message   string `json:"message,omitempty"`
	Body      string `json:"body,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

func newOpsOriginalUpstreamError(payload []byte) *OpsOriginalUpstreamError {
	if len(payload) == 0 {
		return nil
	}
	evidence := &OpsOriginalUpstreamError{Body: string(payload)}
	for _, path := range []string{"response.error.message", "error.message", "message"} {
		value := gjson.GetBytes(payload, path)
		if value.Type == gjson.String {
			evidence.Message = value.String()
			break
		}
	}
	return boundOpsOriginalUpstreamError(evidence)
}

func boundOpsOriginalUpstreamError(original *OpsOriginalUpstreamError) *OpsOriginalUpstreamError {
	if original == nil {
		return nil
	}
	out := *original
	// Keep the same bounded-storage policy as existing operations diagnostics.
	// Unlike legacy evidence, retained bytes are never rewritten or redacted.
	if len(out.Body) > opsMaxStoredErrorBodyBytes {
		out.Body = originalErrorPrefix(out.Body, opsMaxStoredErrorBodyBytes)
		out.Truncated = true
	}
	if len(out.Message) > 2048 {
		out.Message = originalErrorPrefix(out.Message, 2048)
		out.Truncated = true
	}
	return &out
}

func originalOpsUpstreamError(detail *OpsErrorLogDetail) *OpsOriginalUpstreamError {
	if detail == nil {
		return nil
	}
	events, err := ParseOpsUpstreamErrors(detail.UpstreamErrors)
	if err != nil {
		return nil
	}
	// Do not attribute a recovered attempt's raw error to a later local/transport
	// failure. Only the final attempt may supply the administrator's root cause.
	for i := len(events) - 1; i >= 0; i-- {
		if events[i] != nil {
			return events[i].OriginalError
		}
	}
	return nil
}

// captureOpsOriginalHTTPError snapshots bytes before credential/client projection.
// The returned closure attaches them to this response's existing attempt event,
// including paths that return early for a passthrough rule.
func captureOpsOriginalHTTPError(c *gin.Context, account *Account, status int, requestID string, payload []byte) func() {
	original := newOpsOriginalUpstreamError(payload)
	if c == nil || original == nil {
		return func() {}
	}
	before := 0
	if value, ok := c.Get(OpsUpstreamErrorsKey); ok {
		if events, ok := value.([]*OpsUpstreamErrorEvent); ok {
			before = len(events)
		}
	}
	return func() {
		if value, ok := c.Get(OpsUpstreamErrorsKey); ok {
			if events, ok := value.([]*OpsUpstreamErrorEvent); ok {
				for i := len(events) - 1; i >= before; i-- {
					if events[i] != nil {
						events[i].OriginalError = original
						return
					}
				}
			}
		}
		event := OpsUpstreamErrorEvent{UpstreamStatusCode: status, UpstreamRequestID: requestID, Kind: "http_error", Message: original.Message, OriginalError: original, ProxyID: opsUpstreamProxyID(account), ProxyName: opsUpstreamProxyName(account)}
		if account != nil {
			event.AccountID = account.ID
			event.AccountName = account.Name
			event.Platform = account.Platform
		}
		appendOpsUpstreamError(c, event)
	}
}

func recordOpsOriginalStreamTransportError(c *gin.Context, account *Account, passthrough bool, requestID, kind string, err error) {
	if c == nil || err == nil {
		return
	}
	originalMessage := strings.TrimSpace(err.Error())
	if originalMessage == "" {
		return
	}
	safeMessage := "OpenAI upstream stream transport failed"
	setOpsUpstreamError(c, http.StatusBadGateway, safeMessage, "")
	event := OpsUpstreamErrorEvent{
		ProxyID:            opsUpstreamProxyID(account),
		ProxyName:          opsUpstreamProxyName(account),
		Platform:           PlatformOpenAI,
		UpstreamStatusCode: http.StatusBadGateway,
		UpstreamRequestID:  strings.TrimSpace(requestID),
		Passthrough:        passthrough,
		Kind:               kind,
		Message:            safeMessage,
		OriginalError:      boundOpsOriginalUpstreamError(&OpsOriginalUpstreamError{Message: originalMessage}),
	}
	if account != nil {
		event.Platform = account.Platform
		event.AccountID = account.ID
		event.AccountName = account.Name
	}
	appendOpsUpstreamError(c, event)
}

func originalErrorPrefix(value string, limit int) string {
	for limit > 0 && limit < len(value) && !utf8.RuneStart(value[limit]) {
		limit--
	}
	return strings.Clone(value[:limit])
}
