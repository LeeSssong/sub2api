package admin

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type accountAdmissionValidator interface {
	ValidateAccountAdmission(context.Context, *service.CreateAccountInput) error
}

// Reject invalid groups before consuming an OAuth code or exchanging an SSO/PAT.
func validateAdmissionBeforeAuthorization(c *gin.Context, admin service.AdminService, input *service.CreateAccountInput) bool {
	if !input.Admission.IsEnabled() {
		return true
	}
	validator, ok := admin.(accountAdmissionValidator)
	if !ok {
		response.InternalError(c, "durable account admission is unavailable")
		return false
	}
	if err := validator.ValidateAccountAdmission(c.Request.Context(), input); err != nil {
		response.BadRequest(c, err.Error())
		return false
	}
	return true
}
