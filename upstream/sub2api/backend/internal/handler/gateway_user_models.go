package handler

import (
	"context"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type userGroupModelAuthorizer interface {
	GetAvailableGroups(context.Context, int64) ([]service.Group, error)
}

type userGroupModels struct {
	GroupID         int64    `json:"group_id"`
	SupportedModels []string `json:"supported_models"`
}

// UserGroupModels exposes only model names for groups the signed-in user can bind.
// It reuses native group account discovery and allowlist rules without an API key
// or an upstream request. Account identities and routing configuration stay private.
func (h *GatewayHandler) UserGroupModels(c *gin.Context) {
	h.userGroupModels(c, h.apiKeyService)
}

func (h *GatewayHandler) userGroupModels(c *gin.Context, authorizer userGroupModelAuthorizer) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	groups, err := authorizer.GetAvailableGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]userGroupModels, 0, len(groups))
	for i := range groups {
		g := &groups[i]
		models := make([]string, 0)
		if g.Status == service.StatusActive {
			if g.Platform == service.PlatformComposite {
				models = h.compositeAvailableModels(c.Request.Context(), &g.ID, true)
			} else if _, exists := h.gatewayService.GetSchedulablePlatforms(c.Request.Context(), &g.ID)[g.Platform]; exists {
				configured := h.gatewayService.GetAvailableModels(c.Request.Context(), &g.ID, g.Platform)
				// Same fallback as the native model listing, only for a group with accounts.
				// CN providers have no static native catalog: keep their configured names.
				if service.IsCNProvider(g.Platform) {
					models = configured
				} else {
					models = modelListingSource(g.Platform, configured, defaultModelIDsForPlatform(g.Platform))
				}
			}
			models = g.ModelAllowlist.FilterForListing(models)
		}
		clean := make([]string, 0, len(models))
		seen := make(map[string]bool, len(models))
		for _, model := range models {
			model = strings.TrimSpace(model)
			// Mapping wildcards are routing patterns, not callable model IDs.
			if model != "" && !strings.Contains(model, "*") && !seen[model] {
				seen[model] = true
				clean = append(clean, model)
			}
		}
		sort.Strings(clean)
		out = append(out, userGroupModels{GroupID: g.ID, SupportedModels: clean})
	}
	response.Success(c, out)
}
