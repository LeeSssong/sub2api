package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type availableGroupsToolsUserRepo struct{ service.UserRepository }

func (*availableGroupsToolsUserRepo) GetByID(_ context.Context, id int64) (*service.User, error) {
	return &service.User{ID: id}, nil
}

type availableGroupsToolsSubRepo struct {
	service.UserSubscriptionRepository
}

func (*availableGroupsToolsSubRepo) ListActiveByUserID(context.Context, int64) ([]service.UserSubscription, error) {
	return nil, nil
}

type availableGroupsToolsRepo struct {
	service.GroupRepository
	requested  []int64
	mappingErr error
}

func (*availableGroupsToolsRepo) ListActive(context.Context) ([]service.Group, error) {
	return []service.Group{{ID: 1, Name: "跨平台授权组", Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: .2}, {ID: 2, Name: "默认组", Platform: service.PlatformOpenAI, Status: service.StatusActive}, {ID: 99, Name: "私有组", IsExclusive: true}}, nil
}
func (s *availableGroupsToolsRepo) ReadGroupToolMappings(_ context.Context, ids []int64) (map[int64]service.GroupToolMapping, error) {
	s.requested = append([]int64(nil), ids...)
	return map[int64]service.GroupToolMapping{1: {GroupID: 1, ToolIDs: []string{"claude"}}, 99: {GroupID: 99, ToolIDs: []string{"grok"}}}, s.mappingErr
}
func (*availableGroupsToolsRepo) ReplaceGroupToolMapping(context.Context, int64, []string, int64) (service.GroupToolMapping, error) {
	panic("read only")
}

func TestAvailableGroupsExposesOnlyAuthorizedNativeToolMappings(t *testing.T) {
	for _, lookupFails := range []bool{false, true} {
		repo := &availableGroupsToolsRepo{}
		if lookupFails {
			repo.mappingErr = errors.New("mapping lookup failed")
		}
		h := NewAPIKeyHandler(service.NewAPIKeyService(nil, &availableGroupsToolsUserRepo{}, repo, &availableGroupsToolsSubRepo{}, nil, nil, nil))
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/groups/available", nil)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		h.GetAvailableGroups(c)
		require.Equal(t, []int64{1, 2}, repo.requested)
		if lookupFails {
			require.Equal(t, http.StatusInternalServerError, rec.Code)
			continue
		}
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var body struct {
			Data []availableKeyGroupWithTools `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Len(t, body.Data, 2)
		require.Equal(t, []string{"claude"}, body.Data[0].ToolIDs)
		require.Equal(t, .2, body.Data[0].RateMultiplier)
		require.NotNil(t, body.Data[1].ToolIDs)
		require.Empty(t, body.Data[1].ToolIDs)
		require.NotContains(t, rec.Body.String(), "私有组")
		require.NotContains(t, rec.Body.String(), "grok")
	}
}
