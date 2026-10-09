package service

import (
	"time"

	"github.com/gin-gonic/gin"
)

const qualityTrafficAttemptStartKey = "quality_traffic_attempt_start"

func beginQualityTrafficAttempt(c *gin.Context) time.Time {
	started := time.Now()
	if c != nil {
		c.Set(qualityTrafficAttemptStartKey, started)
	}
	return started
}

func QualityTrafficAttemptStartedAt(c *gin.Context) time.Time {
	if c != nil {
		if value, ok := c.Get(qualityTrafficAttemptStartKey); ok {
			if started, ok := value.(time.Time); ok {
				return started
			}
		}
	}
	return time.Time{}
}

// Traffic follows explicit quality evidence, independent of quarantine/BPS actions.
func qualityTrafficVerdict(cfg *PelicanTestConfig, results []*ScheduledTestResult) string {
	if cfg == nil {
		return ""
	}
	switch qualityOutcome(results) {
	case "failed":
		return "degraded"
	case "passed":
		models := len(cfg.ModelIDs)
		if models == 0 {
			models = 1
		}
		if cfg.ParallelCount > 0 && len(results) == models*cfg.ParallelCount {
			return "healthy"
		}
	}
	return ""
}

func stampQualityTrafficStart(result *OpenAIForwardResult, started time.Time) {
	if result != nil && result.QualityRequestStartedAt.IsZero() {
		result.QualityRequestStartedAt = started
	}
}

func stampGatewayQualityTrafficStart(result *ForwardResult, started time.Time) {
	if result != nil && result.QualityRequestStartedAt.IsZero() {
		result.QualityRequestStartedAt = started
	}
}

func qualityTrafficStartedAt(started time.Time) *time.Time {
	if started.IsZero() {
		return nil
	}
	return &started
}
