package admin

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type admissionHandlerAdmin struct {
	service.AdminService
	input         *service.CreateAccountInput
	validationErr error
}

func (s *admissionHandlerAdmin) CreateAccount(_ context.Context, in *service.CreateAccountInput) (*service.Account, error) {
	s.input = in
	return &service.Account{ID: 8, Name: in.Name, Platform: in.Platform, Type: in.Type, Status: service.StatusActive, Schedulable: !in.Admission.IsEnabled()}, nil
}
func (s *admissionHandlerAdmin) ForceAntigravityPrivacy(context.Context, *service.Account) string {
	return ""
}
func (s *admissionHandlerAdmin) ForceOpenAIPrivacy(context.Context, *service.Account) string {
	return ""
}
func (s *admissionHandlerAdmin) ValidateAccountAdmission(_ context.Context, in *service.CreateAccountInput) error {
	s.input = in
	return s.validationErr
}
func TestAdmissionCreateHandlerForwardsExistingFormalGroups(t *testing.T) {
	old := service.DefaultIdempotencyCoordinator()
	service.SetDefaultIdempotencyCoordinator(nil)
	defer service.SetDefaultIdempotencyCoordinator(old)
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{true: "enabled", false: "legacy"}[enabled], func(t *testing.T) {
			svc := &admissionHandlerAdmin{}
			h := &AccountHandler{adminService: svc}
			router := gin.New()
			router.POST("/accounts", h.Create)
			addition := ""
			if enabled {
				addition = `,"admission":{"enabled":true,"test_group_id":1}`
			}
			req := httptest.NewRequest("POST", "/accounts", strings.NewReader(`{"name":"fixture","platform":"openai","type":"apikey","credentials":{"api_key":"fixture"},"group_ids":[2]`+addition+"}"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, 200, w.Code, w.Body.String())
			require.Equal(t, []int64{2}, svc.input.GroupIDs)
			require.Equal(t, enabled, svc.input.Admission.IsEnabled())
			require.False(t, svc.input.AdmissionAllowUngrouped)
		})
	}
}
func TestAdmissionAuthRejectsBeforeUsingExternalProvider(t *testing.T) {
	svc := &admissionHandlerAdmin{validationErr: errors.New("invalid admission groups")}
	for _, tt := range []struct {
		name, body string
		handler    gin.HandlerFunc
	}{
		{"openai", `{"session_id":"local","code":"unused","state":"local","group_ids":[2],"admission":{"enabled":true}}`, (&OpenAIOAuthHandler{adminService: svc}).CreateAccountFromOAuth},
		{"pat", `{"access_token":"unused","group_ids":[2],"admission":{"enabled":true}}`, (&OpenAIOAuthHandler{adminService: svc}).CreateAccountFromCodexPAT},
		{"grok", `{"session_id":"local","code":"unused","group_ids":[2],"admission":{"enabled":true}}`, (&GrokOAuthHandler{adminService: svc}).CreateAccountFromOAuth},
		{"sso", `{"sso_token":"unused","group_ids":[2],"admission":{"enabled":true}}`, (&GrokOAuthHandler{adminService: svc}).CreateAccountsFromSSO},
	} {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/auth", tt.handler)
			req := httptest.NewRequest("POST", "/auth", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, 400, w.Code)
			require.Contains(t, w.Body.String(), "invalid admission groups")
			require.Equal(t, []int64{2}, svc.input.GroupIDs)
		})
	}
}
