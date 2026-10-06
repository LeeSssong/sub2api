package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type userGroupModelAuthorizer interface {
	GetAvailableGroups(context.Context, int64) ([]service.Group, error)
	GetUserGroupDeniedModels(context.Context, int64) (map[int64][]string, error)
}

type userGroupModels struct {
	GroupID         int64    `json:"group_id"`
	SupportedModels []string `json:"supported_models"`
}

// UserGroupModels exposes only model names for groups the signed-in user can bind.
// It reuses ordinary /v1/models discovery (including pinned account catalogues),
// allowlists and user exclusions without exposing account or routing details.
func (h *GatewayHandler) UserGroupModels(c *gin.Context) {
	h.userGroupModels(c, h.apiKeyService)
}

func (h *GatewayHandler) userGroupModels(c *gin.Context, authorizer userGroupModelAuthorizer) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var requested map[int64]bool
	if values, scoped := c.Request.URL.Query()["group_ids"]; scoped {
		requested = make(map[int64]bool)
		if len(values) != 1 {
			response.BadRequest(c, "分组参数无效")
			return
		}
		for _, value := range strings.Split(values[0], ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err != nil || id <= 0 {
				response.BadRequest(c, "分组参数无效")
				return
			}
			requested[id] = true
		}
	}
	groups, err := authorizer.GetAvailableGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	denied, err := authorizer.GetUserGroupDeniedModels(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]userGroupModels, 0, len(groups))
	for i := range groups {
		g := &groups[i]
		// Requested IDs are intersected with the authorized native group list.
		if requested != nil && !requested[g.ID] {
			continue
		}
		models := make([]string, 0)
		if g.Status == service.StatusActive {
			if g.Platform == service.PlatformOpenAI && g.CodexModelsManifestConfig.Enabled {
				if h.openAIGatewayService == nil {
					response.Error(c, http.StatusServiceUnavailable, "模型目录暂不可用")
					return
				}
				catalog, _, err := h.openAIGatewayService.FetchPinnedOpenAIModelsList(c.Request.Context(), g, h.maxAccountSwitches, "")
				if err != nil {
					status := http.StatusBadGateway
					if errors.Is(err, service.ErrNoPinnedCodexModelsAccounts) {
						status = http.StatusServiceUnavailable
					}
					// Do not expose upstream errors or substitute static defaults.
					response.Error(c, status, "模型目录读取失败，请重试")
					return
				}
				var body struct {
					Data []struct {
						ID string `json:"id"`
					} `json:"data"`
				}
				if json.Unmarshal(catalog.Body, &body) != nil || body.Data == nil {
					response.Error(c, http.StatusBadGateway, "模型目录读取失败，请重试")
					return
				}
				for _, model := range body.Data {
					models = append(models, model.ID)
				}
			} else {
				models, _ = h.nativeModelIDsForListing(c.Request.Context(), &g.ID, g.Platform, g.ModelAllowlist)
			}
			models = service.FilterUserGroupDeniedModelIDs(models, denied[g.ID])
		}
		// Keep exactly the native catalogue IDs; applying a second, panel-only
		// mapping filter would make the user's two lists disagree again.
		if models == nil {
			models = []string{}
		}
		sort.Strings(models)
		out = append(out, userGroupModels{GroupID: g.ID, SupportedModels: models})
	}
	response.Success(c, out)
}
