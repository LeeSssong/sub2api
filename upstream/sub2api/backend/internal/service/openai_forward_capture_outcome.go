package service

import (
	"github.com/Wei-Shaw/sub2api/internal/requestcapture"
	"github.com/gin-gonic/gin"
)

// Record only a forwarding result confirmed by the route after terminal
// validation and downstream writes. Capture's parser cannot decide this alone.
func recordOpenAIForwardingOutcome(c *gin.Context, outcome requestcapture.ForwardingOutcome) {
	if c == nil || c.Request == nil {
		return
	}
	session := requestcapture.FromContext(c.Request.Context())
	session.RecordForwardingOutcome(session.LastAttempt(), outcome)
}
