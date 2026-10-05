package handler

import (
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

func TestUserGroupModelsRequiresAuthentication(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/groups/available-models", nil)
	h := &GatewayHandler{}
	h.userGroupModels(c, nil)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestUserGroupModelsScopesAccountsAndFiltersAllowlist(t *testing.T) {
	repo := &gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		1: {{ID: 1, Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.4": "gpt-5.4", "gpt-5.2": "gpt-5.2"}}}},
		2: {{ID: 2, Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"private-model": "private-model"}}}},
	}}
	h := newGatewayModelsHandlerForTest(repo)
	authorizer := &channelMonitorV2GroupAuthorizerStub{groups: []service.Group{
		{ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-5.4"}}},
		{ID: 3, Platform: service.PlatformOpenAI, Status: "inactive"},
	}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/groups/available-models?group_id=2", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h.userGroupModels(c, authorizer)
	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Data []struct {
			GroupID int64    `json:"group_id"`
			Models  []string `json:"supported_models"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Len(t, body.Data, 2)
	require.Equal(t, int64(1), body.Data[0].GroupID)
	require.Equal(t, []string{"gpt-5.4"}, body.Data[0].Models)
	require.Empty(t, body.Data[1].Models)
	require.NotContains(t, recorder.Body.String(), "private-model")
	require.NotContains(t, recorder.Body.String(), "credentials")
	require.Equal(t, []int64{42}, authorizer.calls)
}

func TestUserGroupModelsAuthorizationFailureDoesNotReturnModels(t *testing.T) {
	h := &GatewayHandler{}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/groups/available-models", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h.userGroupModels(c, &channelMonitorV2GroupAuthorizerStub{err: errors.New("unavailable")})
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "supported_models")
}
