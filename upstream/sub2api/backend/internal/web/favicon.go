//go:build embed

package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Safari requires an HTTP URL for uploaded tab icons. The original image stays
// in settings; its content determines the URL so a new upload refreshes caches.
func faviconURL(logo string) string {
	if !strings.HasPrefix(strings.ToLower(logo), "data:image/") {
		return logo
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(logo))
	return fmt.Sprintf("/branding/favicon/%08x", hash.Sum32())
}

func (s *FrontendServer) serveFavicon(c *gin.Context) {
	c.Header("Cache-Control", "no-cache")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	var cfg struct {
		SiteLogo string `json:"site_logo"`
	}
	settings, err := s.settings.GetPublicSettingsForInjection(ctx)
	if err == nil {
		if data, marshalErr := json.Marshal(settings); marshalErr == nil {
			_ = json.Unmarshal(data, &cfg)
		}
	}
	logo := safeImageURL(cfg.SiteLogo)
	if strings.HasPrefix(strings.ToLower(logo), "data:image/") {
		if mime, data, ok := decodeFavicon(logo); ok {
			hash := fnv.New32a()
			_, _ = hash.Write(data)
			etag := fmt.Sprintf(`"%08x"`, hash.Sum32())
			c.Header("ETag", etag)
			c.Header("X-Content-Type-Options", "nosniff")
			if c.GetHeader("If-None-Match") == etag {
				c.Status(http.StatusNotModified)
			} else {
				c.Data(http.StatusOK, mime, data)
			}
			c.Abort()
			return
		}
		logo = ""
	}
	if logo == "" || strings.HasPrefix(logo, "/branding/favicon/") || logo == "/favicon.ico" || logo == "/apple-touch-icon.png" {
		logo = "/xingqiao/logo-brand.png"
	}
	c.Redirect(http.StatusFound, logo)
	c.Abort()
}

func decodeFavicon(logo string) (string, []byte, bool) {
	if len(logo) > 5*1024*1024 {
		return "", nil, false
	}
	header, encoded, found := strings.Cut(logo, ",")
	if !found {
		return "", nil, false
	}
	mime := strings.ToLower(strings.Split(strings.TrimPrefix(strings.ToLower(header), "data:"), ";")[0])
	var data []byte
	var err error
	if strings.HasSuffix(strings.ToLower(header), ";base64") {
		data, err = base64.StdEncoding.DecodeString(encoded)
	} else {
		var decoded string
		decoded, err = url.PathUnescape(encoded)
		data = []byte(decoded)
	}
	if err != nil || len(data) == 0 {
		return "", nil, false
	}
	switch mime {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/x-icon", "image/vnd.microsoft.icon":
		detected := http.DetectContentType(data)
		if detected != mime && !(mime == "image/x-icon" && detected == "image/vnd.microsoft.icon") {
			return "", nil, false
		}
	case "image/svg+xml":
		if !strings.Contains(string(data), "<svg") {
			return "", nil, false
		}
	default:
		return "", nil, false
	}
	return mime, data, true
}
