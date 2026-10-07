//go:build embed

package web

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUploadedFaviconHTTP(t *testing.T) {
	var pngBytes bytes.Buffer
	require.NoError(t, png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	logo := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes.Bytes())
	provider := &mockSettingsProvider{settings: map[string]string{"site_logo": logo}}
	server, err := NewFrontendServer(provider)
	require.NoError(t, err)
	router := gin.New()
	router.Use(server.Middleware())
	for _, path := range []string{faviconURL(logo), "/favicon.ico", "/apple-touch-icon.png"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		require.Equal(t, 200, w.Code, path)
		require.Equal(t, "image/png", w.Header().Get("Content-Type"), path)
		require.Equal(t, pngBytes.Bytes(), w.Body.Bytes(), path)
		require.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("If-None-Match", w.Header().Get("ETag"))
		cached := httptest.NewRecorder()
		router.ServeHTTP(cached, req)
		require.Equal(t, 304, cached.Code)
		require.Empty(t, cached.Body.String())
	}
	// Changes must be visible immediately, independent of the HTML cache.
	provider.settings = map[string]string{"site_logo": "/uploads/new.png"}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/favicon.ico", nil))
	require.Equal(t, 302, w.Code)
	require.Equal(t, "/uploads/new.png", w.Header().Get("Location"))
	for _, unsafe := range []string{"", "javascript:alert(1)", "data:text/html;base64,PHNjcmlwdD4=", "data:image/png;base64,invalid", "data:image/png;base64,PHNjcmlwdD4="} {
		provider.settings = map[string]string{"site_logo": unsafe}
		w = httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/favicon.ico", nil))
		require.Equal(t, 302, w.Code)
		require.Equal(t, "/xingqiao/logo-brand.png", w.Header().Get("Location"))
	}
}

func TestInjectedUploadedFaviconUsesHTTP(t *testing.T) {
	html := []byte(`<link rel="icon" href="/old.png" />`)
	result := injectSiteFavicon(html, []byte(`{"site_logo":"data:image/png;base64,AA=="}`))
	require.Contains(t, string(result), `href="/branding/favicon/02039e14"`)
	require.False(t, strings.Contains(string(result), "data:image"))
}
