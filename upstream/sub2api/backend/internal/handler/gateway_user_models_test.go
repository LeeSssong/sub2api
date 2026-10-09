package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userGroupModelAuthorizerStub struct {
	channelMonitorV2GroupAuthorizerStub
	denied    map[int64][]string
	deniedErr error
}

func (s *userGroupModelAuthorizerStub) GetUserGroupDeniedModels(context.Context, int64) (map[int64][]string, error) {
	return s.denied, s.deniedErr
}

func TestUserGroupModelsRequiresAuthentication(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/groups/available-models", nil)
	h := &GatewayHandler{}
	h.userGroupModels(c, nil)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestUserGroupModelsIncludesSameSourcePricingAfterAuthorization(t *testing.T) {
	h := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		1: {{ID: 1, Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{
			"gpt-5.4-2026-03-05": "gpt-5.4", "denied-model": "denied", "unknown-model": "unknown",
		}}}},
	}})
	billing := service.NewBillingService(&config.Config{}, nil)
	h.modelPlazaService = service.NewModelPlazaService(nil, nil, nil, billing, service.NewModelPricingResolver(nil, billing))
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/groups/available-models?group_ids=1,999&include_pricing=true", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h.userGroupModels(c, &userGroupModelAuthorizerStub{
		channelMonitorV2GroupAuthorizerStub: channelMonitorV2GroupAuthorizerStub{groups: []service.Group{{ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive}}},
		denied:                              map[int64][]string{1: {"denied-model"}},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Data []struct {
			GroupID int64                                 `json:"group_id"`
			Prices  map[string]*modelPlazaOfficialPricing `json:"official_pricing"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	require.Equal(t, int64(1), body.Data[0].GroupID)
	require.NotContains(t, body.Data[0].Prices, "denied-model")
	require.Contains(t, body.Data[0].Prices, "unknown-model")
	require.Nil(t, body.Data[0].Prices["unknown-model"])
	price := body.Data[0].Prices["gpt-5.4-2026-03-05"]
	require.NotNil(t, price)
	require.InDelta(t, 2.5e-6, *price.InputPrice, 1e-15)
	expected, err := h.modelPlazaService.OfficialPricesForModels(context.Background(), []string{"gpt-5.4-2026-03-05"})
	require.NoError(t, err)
	require.Equal(t, toModelPlazaOfficialPricing(expected["gpt-5.4-2026-03-05"]), price)
}

func TestUserGroupModelsPricingFailsClosedWithoutService(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/groups/available-models?include_pricing=true", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	(&GatewayHandler{}).userGroupModels(c, nil)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestUserGroupModelsRejectsInvalidPricingFlag(t *testing.T) {
	for _, query := range []string{"include_pricing=yes", "include_pricing=", "include_pricing=true&include_pricing=false"} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/groups/available-models?"+query, nil)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		(&GatewayHandler{}).userGroupModels(c, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	}
}

func TestUserGroupModelsScopesAccountsAndFiltersAllowlist(t *testing.T) {
	repo := &gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		1: {{ID: 1, Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.4": "gpt-5.4", "gpt-5.2": "gpt-5.2"}}}},
		2: {{ID: 2, Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"private-model": "private-model"}}}},
	}}
	h := newGatewayModelsHandlerForTest(repo)
	authorizer := &userGroupModelAuthorizerStub{channelMonitorV2GroupAuthorizerStub: channelMonitorV2GroupAuthorizerStub{groups: []service.Group{
		{ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-5.4"}}},
		{ID: 3, Platform: service.PlatformOpenAI, Status: "inactive"},
	}}}
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
	h.userGroupModels(c, &userGroupModelAuthorizerStub{channelMonitorV2GroupAuthorizerStub: channelMonitorV2GroupAuthorizerStub{err: errors.New("unavailable")}})
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "supported_models")
}

func userGroupModelIDsForTest(t *testing.T, h *GatewayHandler, group service.Group, denied []string) []string {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/groups/available-models", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h.userGroupModels(c, &userGroupModelAuthorizerStub{
		channelMonitorV2GroupAuthorizerStub: channelMonitorV2GroupAuthorizerStub{groups: []service.Group{group}},
		denied:                              map[int64][]string{group.ID: denied},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var response struct {
		Data []userGroupModels `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Len(t, response.Data, 1)
	return response.Data[0].SupportedModels
}

func nativeModelIDsForUserGroupTest(t *testing.T, h *GatewayHandler, group *service.Group, denied []string) []string {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{GroupID: &group.ID, Group: group, User: &service.User{UserGroupDeniedModels: denied}})
	h.Models(c)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var response gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	return modelIDsForTest(response.Data)
}

func TestUserGroupModelsMatchesNativeModelCatalog(t *testing.T) {
	for _, tc := range []struct {
		name      string
		platform  string
		accounts  []service.Account
		allowlist service.GroupModelAllowlist
		denied    []string
	}{
		{name: "OpenAI mapping", platform: service.PlatformOpenAI, accounts: []service.Account{{ID: 1, Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.4": "gpt-5.4", "alias": "gpt-5.4"}}}}},
		{name: "native wildcard mapping IDs", platform: service.PlatformOpenAI, accounts: []service.Account{{ID: 1, Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-*": "gpt-*", "alias": "gpt-5.4"}}}}},
		{name: "no accounts uses native fallback", platform: service.PlatformOpenAI},
		{name: "Anthropic mapping without allowlist", platform: service.PlatformAnthropic, accounts: []service.Account{{ID: 1, Platform: service.PlatformAnthropic, Credentials: map[string]any{"model_mapping": map[string]any{"custom-claude": "claude"}}}}},
		{name: "wildcard allowlist", platform: service.PlatformOpenAI, accounts: []service.Account{{ID: 1, Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.4": "gpt-5.4", "gpt-5.5": "gpt-5.5", "other": "other"}}}}, allowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-*"}}, denied: []string{"gpt-5.5"}},
		{name: "user denied default model", platform: service.PlatformOpenAI, denied: []string{"gpt-5.4"}},
		{name: "composite", platform: service.PlatformComposite, accounts: []service.Account{{ID: 1, Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.4": "gpt-5.4"}}}, {ID: 2, Platform: service.PlatformAnthropic, Credentials: map[string]any{"model_mapping": map[string]any{"claude-alias": "claude"}}}}},
		{name: "composite no accounts", platform: service.PlatformComposite},
		{name: "configured CN provider", platform: service.PlatformDeepseek, accounts: []service.Account{{ID: 1, Platform: service.PlatformDeepseek, Credentials: map[string]any{"model_mapping": map[string]any{"deepseek-alias": "deepseek-chat"}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := service.Group{ID: 31, Platform: tc.platform, Status: service.StatusActive, ModelAllowlist: tc.allowlist}
			h := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{group.ID: tc.accounts}})
			native := nativeModelIDsForUserGroupTest(t, h, &group, tc.denied)
			panel := userGroupModelIDsForTest(t, h, group, tc.denied)
			require.ElementsMatch(t, native, panel)
			for _, denied := range tc.denied {
				require.NotContains(t, panel, denied)
			}
		})
	}
}

func TestUserGroupModelsMatchesPinnedNativeCatalog(t *testing.T) {
	accounts := []service.Account{newPinnedCodexAccount(1, service.StatusActive, true, false)}
	upstream := &codexModelsPinnedHTTPUpstream{bodies: map[int64]string{1: `{"data":[{"id":"pinned-model"},{"id":"hidden-model"}]}`}}
	codex := newPinnedCodexTestHandler(accounts, upstream, 3)
	h := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{31: accounts}})
	h.openAIGatewayService = codex.gatewayService
	group := service.Group{ID: 31, Platform: service.PlatformOpenAI, Status: service.StatusActive, CodexModelsManifestConfig: service.GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: []int64{1}}}
	denied := []string{"hidden-model"}
	require.Equal(t, []string{"pinned-model"}, nativeModelIDsForUserGroupTest(t, h, &group, denied))
	require.Equal(t, []string{"pinned-model"}, userGroupModelIDsForTest(t, h, group, denied))
	require.Equal(t, []int64{1}, upstream.accountIDs(), "panel must reuse native account catalog cache")
}

func TestUserGroupModelsDeniedLookupFailureDoesNotExposeModels(t *testing.T) {
	h := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{})
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/groups/available-models", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h.userGroupModels(c, &userGroupModelAuthorizerStub{
		channelMonitorV2GroupAuthorizerStub: channelMonitorV2GroupAuthorizerStub{groups: []service.Group{{ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive}}},
		deniedErr:                           errors.New("unavailable"),
	})
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.NotContains(t, rec.Body.String(), "supported_models")
}

func TestUserGroupModelsPinnedCatalogFailureAndEmpty(t *testing.T) {
	for _, tc := range []struct {
		name, body                 string
		accountIDs                 []int64
		upstreamStatus, wantStatus int
	}{
		{name: "authoritative empty", body: `{"data":[]}`, accountIDs: []int64{1}, wantStatus: http.StatusOK},
		{name: "missing pinned account", accountIDs: []int64{99}, wantStatus: http.StatusServiceUnavailable},
		{name: "upstream failure", accountIDs: []int64{1}, upstreamStatus: http.StatusServiceUnavailable, wantStatus: http.StatusBadGateway},
	} {
		t.Run(tc.name, func(t *testing.T) {
			accounts := []service.Account{newPinnedCodexAccount(1, service.StatusActive, true, false)}
			upstream := &codexModelsPinnedHTTPUpstream{bodies: map[int64]string{1: tc.body}, statuses: map[int64]int{}}
			if tc.upstreamStatus != 0 {
				upstream.statuses[1] = tc.upstreamStatus
			}
			codex := newPinnedCodexTestHandler(accounts, upstream, 3)
			h := &GatewayHandler{openAIGatewayService: codex.gatewayService}
			group := service.Group{ID: 31, Platform: service.PlatformOpenAI, Status: service.StatusActive, CodexModelsManifestConfig: service.GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: tc.accountIDs}}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/groups/available-models", nil)
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
			h.userGroupModels(c, &userGroupModelAuthorizerStub{channelMonitorV2GroupAuthorizerStub: channelMonitorV2GroupAuthorizerStub{groups: []service.Group{group}}})
			require.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())
			if tc.wantStatus == http.StatusOK {
				require.JSONEq(t, `{"code":0,"message":"success","data":[{"group_id":31,"supported_models":[]}]}`, rec.Body.String())
			} else {
				require.NotContains(t, rec.Body.String(), "supported_models")
				require.NotContains(t, rec.Body.String(), "gpt-5.4")
				require.NotContains(t, rec.Body.String(), "account")
			}
		})
	}
}

func TestUserGroupModelsScopedRequestSkipsUnrelatedPinnedFailure(t *testing.T) {
	h := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		2: {{ID: 2, Platform: service.PlatformAnthropic, Credentials: map[string]any{"model_mapping": map[string]any{"claude-alias": "claude"}}}},
	}})
	groups := []service.Group{
		{ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive, CodexModelsManifestConfig: service.GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: []int64{99}}},
		{ID: 2, Platform: service.PlatformAnthropic, Status: service.StatusActive},
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/groups/available-models?group_ids=2,999", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h.userGroupModels(c, &userGroupModelAuthorizerStub{channelMonitorV2GroupAuthorizerStub: channelMonitorV2GroupAuthorizerStub{groups: groups}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.JSONEq(t, `{"code":0,"message":"success","data":[{"group_id":2,"supported_models":["claude-alias"]}]}`, rec.Body.String())
}

func TestUserGroupModelsRejectsInvalidScope(t *testing.T) {
	for _, query := range []string{"group_ids=", "group_ids=-1", "group_ids=oops", "group_ids=1&group_ids=2"} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/groups/available-models?"+query, nil)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		(&GatewayHandler{}).userGroupModels(c, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	}
}
