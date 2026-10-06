package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/mihomo"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Keys for the optional session-affinity proxy mode used by regular OpenAI
// traffic. The scheduler itself is shared with Excel/BPS; these keys only
// decide whether regular traffic opts into that scheduler.
const (
	OpenAISessionProxyEnabledKey = "openai_session_proxy"
	OpenAISessionProxySourceKey  = "openai_session_proxy_source"
)

func (a *Account) IsOpenAISessionProxyEnabled() bool {
	return a != nil && a.Platform == PlatformOpenAI && a.Type == AccountTypeOAuth && a.Extra != nil && a.Extra[OpenAISessionProxyEnabledKey] == true
}

func (a *Account) OpenAISessionProxySource() string {
	if a != nil && a.Extra != nil {
		if source, ok := a.Extra[OpenAISessionProxySourceKey].(string); ok && source == string(ExcelBPSProxySourceIPPool) {
			return ExcelBPSProxySourceIPPool
		}
	}
	return ExcelBPSProxySourceMihomo
}

type regularProxyContextKey struct{}

func regularProxyScope(c *gin.Context, a *Account, body []byte) string {
	identity, transient := resolveExcelBPSIdentity(c, body, getAPIKeyIDFromContext(c), true)
	scope := fmt.Sprintf("regular:account:%d/key:%d/thread:%s", a.ID, getAPIKeyIDFromContext(c), identity)
	if transient {
		scope = "transient:" + scope
	}
	return scope
}

// Capture the original client identity before fingerprint and account rewrites.
func withRegularProxyScope(ctx context.Context, c *gin.Context, a *Account, body []byte) context.Context {
	if !a.IsOpenAISessionProxyEnabled() {
		return ctx
	}
	if _, ok := ctx.Value(regularProxyContextKey{}).(string); ok {
		return ctx
	}
	return context.WithValue(ctx, regularProxyContextKey{}, regularProxyScope(c, a, body))
}

func acquireRegularProxy(ctx context.Context, a *Account, excluded ...string) (*mihomo.BPSLease, error) {
	scope, _ := ctx.Value(regularProxyContextKey{}).(string)
	if scope == "" {
		return nil, errors.New("session proxy identity unavailable")
	}
	acquire := mihomo.AcquireBPSLease
	if a.OpenAISessionProxySource() == ExcelBPSProxySourceIPPool {
		acquire = mihomo.AcquireBPSStaticLease
		if strings.HasPrefix(scope, "transient:") {
			acquire = mihomo.AcquireBPSStaticTransientLease
		}
	} else if strings.HasPrefix(scope, "transient:") {
		acquire = mihomo.AcquireBPSTransientLease
	}
	return acquire(ctx, scope, excluded...)
}

// A lease stays live until the caller closes the full response, including SSE.
// Never treat HTTP 200 alone as a completed model response.
type regularProxyBody struct {
	io.ReadCloser
	lease *mihomo.BPSLease
	ctx   context.Context
	once  sync.Once
}

func (b *regularProxyBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil && err != io.EOF && b.ctx.Err() == nil {
		b.lease.ReportStreamFailure()
	}
	return n, err
}
func (b *regularProxyBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.lease.Release)
	return err
}

func (s *OpenAIGatewayService) doRegularProxyUpstream(req *http.Request, a *Account) (*http.Response, error) {
	var excluded []string
	for attempt := 0; attempt < 2; attempt++ {
		lease, err := acquireRegularProxy(req.Context(), a, excluded...)
		if err != nil {
			return nil, errors.New("session proxy unavailable")
		}
		resp, err, unsent := s.doOpenAIProxyAttemptWithTrace(req, a, runtimeProxyEgress{url: lease.ProxyURL, proxyID: -1})
		if err != nil {
			if req.Context().Err() == nil {
				lease.ReportFailure()
			}
			lease.Release()
			var pluginErr *PluginTransportError
			pluginSent := errors.As(err, &pluginErr) && pluginErr.RequestSent
			if attempt == 0 && resp == nil && unsent && !pluginSent && req.Context().Err() == nil && req.GetBody != nil {
				retry, cloneErr := cloneUpstreamRequestForRetry(req.Context(), req)
				if cloneErr == nil {
					excluded = append(excluded, lease.ProxyURL)
					req = retry
					continue
				}
			}
			return resp, err
		}
		if resp == nil || resp.Body == nil {
			lease.Release()
			return resp, nil
		}
		if resp.StatusCode >= 500 {
			lease.ReportUpstreamFailure()
		}
		resp.Body = &regularProxyBody{ReadCloser: resp.Body, lease: lease, ctx: req.Context()}
		return resp, nil
	}
	return nil, errors.New("session proxy unavailable")
}
