package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const lineCheckLimit = 30

// LineCheck caps active probes per authenticated user, including administrators.
// It is independent of configurable panel limits and fails closed on Redis errors.
func (p *PanelRateLimiter) LineCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := GetAuthSubjectFromContext(c)
		if !ok || subject.UserID <= 0 {
			AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录。")
			return
		}
		if p == nil || p.lineCheckLimiter == nil {
			AbortWithError(c, http.StatusServiceUnavailable, "LINE_CHECK_UNAVAILABLE", "线路检查暂不可用，请稍后重试。")
			return
		}
		result, err := p.lineCheckLimiter.AllowSlidingWindow(c.Request.Context(), "panel:line-check:user:"+strconv.FormatInt(subject.UserID, 10), lineCheckLimit, time.Minute)
		if err != nil {
			slog.Warn("line check rate limit unavailable", "error", err)
			AbortWithError(c, http.StatusServiceUnavailable, "LINE_CHECK_UNAVAILABLE", "线路检查暂不可用，请稍后重试。")
			return
		}
		if !result.Allowed {
			seconds := int64((result.RetryAfter + time.Second - 1) / time.Second)
			if seconds < 1 {
				seconds = 1
			}
			c.Header("Retry-After", strconv.FormatInt(seconds, 10))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":     "LINE_CHECK_RATE_LIMITED",
				"message":  fmt.Sprintf("每分钟最多检查 30 次，请在 %d 秒后重试。", seconds),
				"metadata": gin.H{"retry_after_seconds": seconds},
			})
			return
		}
		c.Next()
	}
}
