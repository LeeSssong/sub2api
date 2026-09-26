//go:build unit

package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type pelicanAdminReadSettings struct {
	service.SettingRepository
	values map[string]string
}

func (r *pelicanAdminReadSettings) GetValue(_ context.Context, key string) (string, error) {
	return r.values[key], nil
}

func (r *pelicanAdminReadSettings) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = r.values[key]
	}
	return out, nil
}

type pelicanAdminReadUsers struct{ service.UserRepository }

func (*pelicanAdminReadUsers) GetFirstAdmin(context.Context) (*service.User, error) {
	return &service.User{ID: 42, Role: service.RoleAdmin, Status: service.StatusActive}, nil
}

type pelicanAdminReadRepo struct {
	service.PelicanShowcaseRepository
	t            *testing.T
	missingItem  bool
	noStatistics bool
	reads        int
}

func (r *pelicanAdminReadRepo) ReadReport(_ context.Context, groupID int64, model string, allowed []int64, _ int, _, _, _ time.Time) (*service.PelicanReportData, error) {
	r.reads++
	require.Equal(r.t, []int64{3, 9}, allowed)
	if groupID == 9 {
		return nil, nil
	}
	require.Equal(r.t, int64(3), groupID)
	rate := 1.25
	selected := model
	if selected == "" {
		selected = "public-model"
	}
	return &service.PelicanReportData{Group: service.PelicanReportGroup{ID: 3, Name: "Public gallery", Platform: "openai", RateMultiplier: &rate}, ModelID: selected, Facts: []service.PelicanReportFact{}}, nil
}

func (r *pelicanAdminReadRepo) ListGroups(_ context.Context, ids []int64) ([]*service.PelicanShowcaseGroup, error) {
	r.reads++
	require.Equal(r.t, []int64{3, 9}, ids)
	// Group 9 is configured but inactive/deleted, so the repository omits it.
	return []*service.PelicanShowcaseGroup{{ID: 3, Name: "Public gallery", Platform: "openai"}}, nil
}

func (r *pelicanAdminReadRepo) ListItems(_ context.Context, ids []int64, maxItems int, since time.Time) ([]*service.PelicanShowcaseItem, error) {
	r.reads++
	require.Equal(r.t, []int64{3}, ids)
	require.Equal(r.t, 2, maxItems)
	require.WithinDuration(r.t, time.Now().Add(-24*time.Hour), since, time.Minute)
	return []*service.PelicanShowcaseItem{{ID: 7, GroupID: 3, ModelID: "public-model"}}, nil
}

func (r *pelicanAdminReadRepo) GetItem(_ context.Context, id int64, ids []int64, maxItems int, since time.Time) (*service.PelicanShowcaseItem, error) {
	r.reads++
	require.Equal(r.t, int64(7), id)
	require.Equal(r.t, []int64{3, 9}, ids)
	require.Equal(r.t, 2, maxItems)
	require.WithinDuration(r.t, time.Now().Add(-24*time.Hour), since, time.Minute)
	if r.missingItem {
		// The repository returns nil for removed, expired, over-limit or hidden items.
		return nil, nil
	}
	return &service.PelicanShowcaseItem{ID: 7, GroupID: 3, ModelID: "public-model", ResponseText: "<svg></svg>"}, nil
}

func (r *pelicanAdminReadRepo) ReadStatistics(_ context.Context, ids []int64, from, to time.Time) (*service.PelicanShowcaseStatistics, error) {
	r.reads++
	require.Equal(r.t, []int64{3}, ids)
	require.Equal(r.t, 24*time.Hour, to.Sub(from))
	if r.noStatistics {
		return nil, nil
	}
	return &service.PelicanShowcaseStatistics{
		CoverageStartedAt: from.Add(time.Hour),
		Total:             service.PelicanShowcaseCounts{SuccessCount: 1, TotalCount: 4},
		Groups:            map[int64]service.PelicanShowcaseCounts{3: {SuccessCount: 1, TotalCount: 4}},
	}, nil
}

func newPelicanAdminReadRouter(t *testing.T) (*gin.Engine, *pelicanAdminReadRepo, *pelicanAdminReadSettings) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	settings := &pelicanAdminReadSettings{values: map[string]string{
		service.SettingKeyAdminAPIKey:            "test-only-admin-key",
		service.SettingKeyPelicanShowcaseEnabled: "true",
		service.SettingKeyPelicanShowcaseConfig:  `{"group_ids":[3,9],"max_items":2,"auto_cleanup":true,"retention_days":1}`,
	}}
	settingService := service.NewSettingService(settings, &config.Config{})
	ack, err := json.Marshal(service.AdminComplianceAcknowledgement{Version: service.AdminComplianceVersion, AdminUserID: 42})
	require.NoError(t, err)
	settings.values["admin_compliance_acknowledgement:42"] = string(ack)
	repo := &pelicanAdminReadRepo{t: t}
	h := &handler.Handlers{
		Admin:           &handler.AdminHandlers{},
		PelicanShowcase: handler.NewPelicanShowcaseHandler(service.NewPelicanShowcaseService(repo, settingService)),
	}
	userService := service.NewUserService(&pelicanAdminReadUsers{}, nil, nil, nil)
	adminAuth := middleware.NewAdminAuthMiddleware(nil, userService, settingService, nil)
	// Exercise the real admin route group and the real API-key middleware.
	audit := middleware.AuditLogMiddleware(func(c *gin.Context) {
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		require.True(t, ok)
		require.Equal(t, int64(42), subject.UserID)
		require.Equal(t, "admin_api_key", c.GetString("auth_method"))
		c.Next()
	})
	stepUp := middleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() })
	router := gin.New()
	RegisterAdminRoutes(router.Group("/api/v1"), h, adminAuth, audit, stepUp, settingService, nil)
	return router, repo, settings
}

func pelicanAdminReadRequest(router *gin.Engine, path, key string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if key != "" {
		request.Header.Set("X-API-Key", key)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestPelicanShowcaseAdminReadRequiresAdminAPIKey(t *testing.T) {
	router, repo, _ := newPelicanAdminReadRouter(t)
	for _, path := range []string{"/api/v1/admin/pelican-showcase", "/api/v1/admin/pelican-showcase/items/7"} {
		for _, key := range []string{"", "invalid-admin-key"} {
			w := pelicanAdminReadRequest(router, path, key)
			require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
		}
	}
	require.Zero(t, repo.reads, "unauthenticated reads must not reach gallery storage")
}

func TestPelicanShowcaseAdminReadUsesPublicGalleryAndStatistics(t *testing.T) {
	router, _, _ := newPelicanAdminReadRouter(t)
	// Request parameters cannot choose another user's identity or widen public groups.
	w := pelicanAdminReadRequest(router, "/api/v1/admin/pelican-showcase?user_id=99&group_ids=9", "test-only-admin-key")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var envelope struct {
		Data service.PelicanShowcaseView `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.True(t, envelope.Data.Enabled)
	require.Equal(t, 2, envelope.Data.MaxItems)
	require.Equal(t, 1, envelope.Data.RetentionDays)
	require.Len(t, envelope.Data.Groups, 1)
	group := envelope.Data.Groups[0]
	require.Equal(t, int64(3), group.ID)
	require.Len(t, group.Items, 1)
	require.Equal(t, int64(7), group.Items[0].ID)
	require.NotNil(t, envelope.Data.Stats)
	require.Equal(t, int64(1), envelope.Data.Stats.SuccessCount)
	require.Equal(t, int64(4), envelope.Data.Stats.TotalCount)
	require.Equal(t, 25.0, *envelope.Data.Stats.SuccessRate)
	require.Equal(t, envelope.Data.Stats, group.Stats)
	require.NotNil(t, envelope.Data.StatsWindow)
	require.False(t, envelope.Data.StatsWindow.Complete)
	for _, hidden := range []string{"account_id", "plan_id", "source_result_id", "response_text", "admin_api_key"} {
		require.NotContains(t, w.Body.String(), hidden)
	}
	w = pelicanAdminReadRequest(router, "/api/v1/admin/pelican-showcase/items/7", "test-only-admin-key")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.JSONEq(t, `{"code":0,"message":"success","data":{"id":7,"group_id":3,"model_id":"public-model","reasoning_effort":"","latency_ms":0,"generated_at":"0001-01-01T00:00:00Z","response_text":"<svg></svg>"}}`, w.Body.String())
}

func TestPelicanShowcaseAdminReadPreservesDisabledAndHiddenItems(t *testing.T) {
	router, repo, settings := newPelicanAdminReadRouter(t)
	settings.values[service.SettingKeyPelicanShowcaseEnabled] = "false"
	w := pelicanAdminReadRequest(router, "/api/v1/admin/pelican-showcase", "test-only-admin-key")
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"code":0,"message":"success","data":{"enabled":false,"max_items":2,"retention_days":1,"groups":[],"stats":null,"stats_window":null}}`, w.Body.String())
	w = pelicanAdminReadRequest(router, "/api/v1/admin/pelican-showcase/items/7", "test-only-admin-key")
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Zero(t, repo.reads, "disabled gallery must not load snapshots or statistics")
	settings.values[service.SettingKeyPelicanShowcaseEnabled] = "true"
	repo.missingItem = true
	w = pelicanAdminReadRequest(router, "/api/v1/admin/pelican-showcase/items/7", "test-only-admin-key")
	require.Equal(t, http.StatusNotFound, w.Code)
	w = pelicanAdminReadRequest(router, "/api/v1/admin/pelican-showcase/items/invalid", "test-only-admin-key")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPelicanShowcaseAdminReadPreservesUnavailableStatistics(t *testing.T) {
	router, repo, _ := newPelicanAdminReadRouter(t)
	repo.noStatistics = true
	w := pelicanAdminReadRequest(router, "/api/v1/admin/pelican-showcase", "test-only-admin-key")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"stats":null`)
	require.Contains(t, w.Body.String(), `"stats_window":null`)
	require.Contains(t, w.Body.String(), `"id":7`)
}

func TestPelicanShowcaseAdminReadPreservesAdminComplianceGuard(t *testing.T) {
	router, repo, settings := newPelicanAdminReadRouter(t)
	delete(settings.values, "admin_compliance_acknowledgement:42")
	for _, path := range []string{"/api/v1/admin/pelican-showcase", "/api/v1/admin/pelican-showcase/items/7"} {
		w := pelicanAdminReadRequest(router, path, "test-only-admin-key")
		require.Equal(t, http.StatusLocked, w.Code)
	}
	require.Zero(t, repo.reads)
}

func TestPelicanReportAdminReadRequiresAdminAPIKey(t *testing.T) {
	router, _, _ := newPelicanAdminReadRouter(t)
	w := pelicanAdminReadRequest(router, "/api/v1/admin/pelican-reports/groups/3?window=24h", "")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPelicanReportAdminReadUsesAuthenticatedPublicProjection(t *testing.T) {
	router, repo, _ := newPelicanAdminReadRouter(t)
	w := pelicanAdminReadRequest(router, "/api/v1/admin/pelican-reports/groups/3?window=24h&model_id=public-model", "test-only-admin-key")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"schema_version":2`)
	require.Contains(t, w.Body.String(), `"name":"Public gallery"`)
	require.Contains(t, w.Body.String(), `"rate_multiplier":1.25`)
	require.NotContains(t, w.Body.String(), "account_id")
	require.NotZero(t, repo.reads)
}

func TestPelicanReportAdminReadPreservesComplianceAndVisibility(t *testing.T) {
	t.Run("compliance", func(t *testing.T) {
		router, repo, settings := newPelicanAdminReadRouter(t)
		delete(settings.values, "admin_compliance_acknowledgement:42")
		w := pelicanAdminReadRequest(router, "/api/v1/admin/pelican-reports/groups/3", "test-only-admin-key")
		require.Equal(t, http.StatusLocked, w.Code)
		require.Zero(t, repo.reads)
	})
	t.Run("disabled", func(t *testing.T) {
		router, repo, settings := newPelicanAdminReadRouter(t)
		settings.values[service.SettingKeyPelicanShowcaseEnabled] = "false"
		w := pelicanAdminReadRequest(router, "/api/v1/admin/pelican-reports/groups/3", "test-only-admin-key")
		require.Equal(t, http.StatusNotFound, w.Code)
		require.Zero(t, repo.reads)
	})
	t.Run("not-public", func(t *testing.T) {
		router, repo, _ := newPelicanAdminReadRouter(t)
		w := pelicanAdminReadRequest(router, "/api/v1/admin/pelican-reports/groups/4?group_ids=4", "test-only-admin-key")
		require.Equal(t, http.StatusNotFound, w.Code)
		require.Zero(t, repo.reads)
	})
	t.Run("inactive", func(t *testing.T) {
		router, repo, _ := newPelicanAdminReadRouter(t)
		w := pelicanAdminReadRequest(router, "/api/v1/admin/pelican-reports/groups/9", "test-only-admin-key")
		require.Equal(t, http.StatusNotFound, w.Code)
		require.Equal(t, 1, repo.reads)
	})
}

func TestPelicanReportAdminReadRejectsBadInputs(t *testing.T) {
	router, repo, _ := newPelicanAdminReadRouter(t)
	for _, suffix := range []string{"invalid", "0", "-1", "3?window=48h", "3?window=", "3?model_id=" + strings.Repeat("x", 101), "3?model_id=" + url.QueryEscape(" model ")} {
		w := pelicanAdminReadRequest(router, "/api/v1/admin/pelican-reports/groups/"+suffix, "test-only-admin-key")
		require.Equal(t, http.StatusBadRequest, w.Code, suffix)
	}
	require.Zero(t, repo.reads)
}
