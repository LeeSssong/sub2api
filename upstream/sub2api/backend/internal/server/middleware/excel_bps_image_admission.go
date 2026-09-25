package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	bpsImageMaxBodyBytes   = 64 << 20
	bpsImageBudgetBytes    = 512 << 20
	bpsImageBodyMultiplier = 8
	bpsImageMinBodyBytes   = 1 << 20
	bpsImageMaxRequests    = 32
)

type excelBPSImageSettingsReader interface {
	GetExcelBPSImageRelaySettings(context.Context) (service.ExcelBPSImageRelaySettings, error)
}

// A budget accounts for request bodies and their processing copies, not RSS.
// The independent preprocessing and retained-body pools are each capped at
// 512 MiB (1 GiB combined). Never queue large bodies in either pool.
type bpsImageAdmissionBudget struct {
	mu       sync.Mutex
	bytes    int64
	requests int
}

type bpsImageAdmissionLease struct {
	budget   *bpsImageAdmissionBudget
	weight   int64
	released bool
}

func (b *bpsImageAdmissionBudget) acquire(weight int64) (*bpsImageAdmissionLease, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if weight < 0 || b.requests >= bpsImageMaxRequests || weight > bpsImageBudgetBytes-b.bytes {
		return nil, false
	}
	b.bytes += weight
	b.requests++
	return &bpsImageAdmissionLease{budget: b, weight: weight}, true
}

func (l *bpsImageAdmissionLease) resize(weight int64) bool {
	b := l.budget
	b.mu.Lock()
	defer b.mu.Unlock()
	if l.released || weight < 0 || weight-l.weight > bpsImageBudgetBytes-b.bytes {
		return false
	}
	b.bytes += weight - l.weight
	l.weight = weight
	return true
}

func (l *bpsImageAdmissionLease) release() {
	b := l.budget
	b.mu.Lock()
	defer b.mu.Unlock()
	if !l.released {
		b.bytes -= l.weight
		b.requests--
		l.released = true
	}
}

func bpsImageBodyWeight(length int64) int64 {
	if length < bpsImageMinBodyBytes {
		length = bpsImageMinBodyBytes
	}
	return length * bpsImageBodyMultiplier
}

// ExcelBPSImageAdmission must be shared across the gateway route aliases.
// Account selection occurs after reading JSON, so enabling image relay applies
// this guard to OpenAI/Composite Responses, Chat and Messages HTTP requests,
// including text-only requests. Disabled relay leaves existing limits intact.
func ExcelBPSImageAdmission(settings excelBPSImageSettingsReader, configuredMax int64) gin.HandlerFunc {
	budget := &bpsImageAdmissionBudget{}
	preprocessing := &bpsImageAdmissionBudget{}
	maxBody := int64(bpsImageMaxBodyBytes)
	if configuredMax > 0 && configuredMax < maxBody {
		maxBody = configuredMax
	}
	return func(c *gin.Context) {
		if settings == nil || !bpsImageAdmissionRoute(c) {
			c.Next()
			return
		}
		key, ok := GetAPIKeyFromContext(c)
		if !ok || key == nil || (key.Group != nil && key.Group.Platform != service.PlatformOpenAI && key.Group.Platform != service.PlatformComposite) {
			c.Next()
			return
		}
		relay, err := settings.GetExcelBPSImageRelaySettings(c.Request.Context())
		if err != nil {
			bpsImageAdmissionError(c, http.StatusServiceUnavailable, "basispoints_image_settings_unavailable", "Image relay settings are unavailable")
			return
		}
		if !relay.Enabled {
			c.Next()
			return
		}
		length := c.Request.ContentLength
		if length > maxBody {
			bpsImageAdmissionError(c, http.StatusRequestEntityTooLarge, "basispoints_image_body_too_large", "Request body exceeds the image relay ingress limit")
			return
		}
		readLimit := maxBody
		if length > 0 {
			readLimit = length
		}
		encoding := strings.TrimSpace(c.GetHeader("Content-Encoding"))
		needsPreprocessing := length <= 0 || (encoding != "" && !strings.EqualFold(encoding, "identity"))
		accounted := length
		if needsPreprocessing {
			// Claim a request slot, but do not guess the retained decoded size.
			accounted = 0
		}
		lease, acquired := budget.acquire(bpsImageBodyWeight(accounted))
		if !acquired {
			bpsImageAdmissionError(c, http.StatusServiceUnavailable, "basispoints_image_request_busy", "Image relay request capacity is busy; retry later")
			return
		}
		defer lease.release()
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, readLimit)
		if needsPreprocessing && !bpsImagePreread(c, preprocessing, lease) {
			return
		}
		c.Next()
	}
}

// Preprocessing is bounded independently of retained requests: a gzip or
// unknown-length body can coexist with ordinary traffic, then releases its
// worst-case reservation before an upstream stream starts. The decoded body
// and its processing copies remain charged until the downstream handler exits.
func bpsImagePreread(c *gin.Context, preprocessing *bpsImageAdmissionBudget, retained *bpsImageAdmissionLease) bool {
	transient, acquired := preprocessing.acquire(bpsImageBudgetBytes)
	if !acquired {
		bpsImageAdmissionError(c, http.StatusServiceUnavailable, "basispoints_image_request_busy", "Request body preprocessing capacity is busy; retry later")
		return false
	}
	defer transient.release()
	wireBody := c.Request.Body
	defer func() { _ = wireBody.Close() }()
	body, err := httputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			bpsImageAdmissionError(c, http.StatusRequestEntityTooLarge, "basispoints_image_body_too_large", "Request body exceeds the image relay ingress limit")
		} else {
			bpsImageAdmissionError(c, http.StatusBadRequest, "invalid_request_body", "Unable to read or decode request body")
		}
		return false
	}
	if !retained.resize(bpsImageBodyWeight(int64(len(body)))) {
		bpsImageAdmissionError(c, http.StatusServiceUnavailable, "basispoints_image_request_busy", "Request body memory capacity is busy; retry later")
		return false
	}
	// Keep PrereadBody outermost: later middleware reuses its bytes directly.
	c.Request.Body = httputil.NewPrereadBody(body)
	c.Request.ContentLength = int64(len(body))
	c.Request.Header.Del("Content-Length")
	return true
}

func bpsImageAdmissionRoute(c *gin.Context) bool {
	if c.Request.Method != http.MethodPost {
		return false
	}
	switch c.FullPath() {
	case "/responses", "/responses/*subpath", "/v1/responses", "/v1/responses/*subpath",
		"/backend-api/codex/responses", "/backend-api/codex/responses/*subpath",
		"/chat/completions", "/v1/chat/completions", "/v1/messages":
		return true
	}
	return false
}

func bpsImageAdmissionError(c *gin.Context, status int, code, message string) {
	errorType := "server_error"
	if status == http.StatusRequestEntityTooLarge || status == http.StatusBadRequest {
		errorType = "invalid_request_error"
	}
	if status == http.StatusServiceUnavailable {
		c.Header("Retry-After", "1")
	}
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"type": errorType, "code": code, "message": message}})
}
