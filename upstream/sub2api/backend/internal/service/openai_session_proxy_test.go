package service

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestRegularSessionProxyIdentity(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	a := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"openai_session_proxy": true}}
	x := regularProxyScope(c, a, []byte(`{"prompt_cache_key":"one"}`))
	require.Equal(t, x, regularProxyScope(c, a, []byte(`{"prompt_cache_key":"one","input":"next"}`)))
	require.NotEqual(t, x, regularProxyScope(c, a, []byte(`{"prompt_cache_key":"two"}`)))
	a.ID = 2
	require.NotEqual(t, x, regularProxyScope(c, a, []byte(`{"prompt_cache_key":"one"}`)))
	c.Request.Header.Set("OpenAI-Beta", "responses=v1")
	anonymous := regularProxyScope(c, a, []byte(`{}`))
	require.Contains(t, anonymous, "transient:")
	d, _ := gin.CreateTestContext(httptest.NewRecorder())
	d.Request = c.Request.Clone(context.Background())
	require.NotEqual(t, anonymous, regularProxyScope(d, a, []byte(`{}`)))
}
func TestRegularSessionProxyWarmTargets(t *testing.T) {
	a := Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 4, Extra: map[string]any{"openai_session_proxy": true, "openai_session_proxy_source": "ip_pool"}}
	m, s := excelBPSWarmTargets([]Account{a})
	require.Equal(t, 0, m)
	require.Equal(t, 4, s)
}

func TestRegularProxyUnavailableNeverUsesAccountFallback(t *testing.T) {
	a := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"openai_session_proxy": true, "openai_session_proxy_source": "ip_pool"}, Proxy: &Proxy{FallbackMode: FallbackModeDirect}}
	req := httptest.NewRequest("POST", "https://example.invalid/v1/responses", nil)
	svc := &OpenAIGatewayService{}
	_, err := svc.doOpenAIUpstream(req, "", a)
	require.ErrorContains(t, err, "session proxy unavailable") // A direct send would dereference the deliberately absent transport.
}

func TestRegularProxyChangeInvalidatesRoute(t *testing.T) {
	a := &Account{ID: 1, Extra: map[string]any{}}
	before := openAITurnRouteFingerprint(a)
	a.Extra[OpenAISessionProxyEnabledKey] = true
	require.NotEqual(t, before, openAITurnRouteFingerprint(a))
	before = openAITurnRouteFingerprint(a)
	a.Extra[OpenAISessionProxySourceKey] = "ip_pool"
	require.NotEqual(t, before, openAITurnRouteFingerprint(a))
}
