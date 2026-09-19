package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSchedulerRetirementRejectsInactiveSettingsBeforeWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, key := range []string{"openai_advanced_scheduler_group_policies", "openai_advanced_scheduler_exploration_ratio"} {
		h := &SettingHandler{}
		r := gin.New()
		r.PUT("/settings", h.UpdateSettings)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/settings", strings.NewReader(`{"`+key+`":null}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "retired")
	}
}

func TestSchedulerRetirementOrdinarySavePreservesStoredPolicies(t *testing.T) {
	stored := map[string]string{
		service.SettingKeyOpenAIAdvancedSchedulerGroupOverrides:    `{"11":{"mode":"custom","extra_retry_count":2,"top_k":9}}`,
		service.SettingKeyOpenAIAdvancedSchedulerExplorationRatio:  "37",
		service.SettingKeyOpenAIAdvancedSchedulerCandidatePoolMode: "fair",
	}
	h, repo := newStepUpSwitchTestHandler(t, stored)
	before := make(map[string]string, len(stored))
	for key, value := range stored {
		before[key] = value
	}
	rec := doUpdateSettings(t, h, map[string]any{"openai_advanced_scheduler_lb_top_k": "8"}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "8", repo.values[service.SettingKeyOpenAIAdvancedSchedulerLBTopK])
	for key, value := range before {
		require.Equal(t, value, repo.values[key], key)
	}
}
