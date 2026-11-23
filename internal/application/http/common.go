package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
	"github.com/zeann3th/policy-service/internal/core/unit"
)

type commonHandler struct {
}

type healthResponse struct {
	Status string `json:"status" example:"ok"`
}

type unitPageResponse struct {
	Items []unit.Response `json:"items"`
	Page  int32           `json:"page" example:"1"`
	Limit int32           `json:"limit" example:"20"`
	Total int64           `json:"total" example:"1"`
}

type policyGroupPageResponse struct {
	Items []policy.PolicyGroupResponse `json:"items"`
	Page  int32                        `json:"page" example:"1"`
	Limit int32                        `json:"limit" example:"20"`
	Total int64                        `json:"total" example:"1"`
}

type policyTypePageResponse struct {
	Items []policy.PolicyTypeResponse `json:"items"`
	Page  int32                       `json:"page" example:"1"`
	Limit int32                       `json:"limit" example:"20"`
	Total int64                       `json:"total" example:"1"`
}

type policyPageResponse struct {
	Items []policy.PolicyResponse `json:"items"`
	Page  int32                   `json:"page" example:"1"`
	Limit int32                   `json:"limit" example:"20"`
	Total int64                   `json:"total" example:"1"`
}

type customPolicyPageResponse struct {
	Items []policy.CustomPolicyResponse `json:"items"`
	Page  int32                         `json:"page" example:"1"`
	Limit int32                         `json:"limit" example:"20"`
	Total int64                         `json:"total" example:"1"`
}

// checkHealth godoc
//
//	@Summary	Check service health
//	@Tags		Utility
//	@Produce	json
//	@Success	200	{object}	healthResponse
//	@Router		/healthz [get]
func (h *commonHandler) checkHealth(c *gin.Context) {
	c.JSON(http.StatusOK, healthResponse{Status: "ok"})
}

func RegisterUtilityRoutes(rg *gin.RouterGroup) {
	handler := &commonHandler{}

	rg.GET("/healthz", handler.checkHealth)
}

func pathID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id < 1 {
		handleError(c, core.ErrBadRequest.WithMessage(name+" must be a positive integer"))
		return 0, false
	}
	return id, true
}

func bindJSON(c *gin.Context, value any) bool {
	if err := c.ShouldBindJSON(value); err != nil {
		handleError(c, badRequest(err, value))
		return false
	}
	return true
}

func bindPage(c *gin.Context) (core.PageRequest, bool) {
	var page core.PageRequest
	if err := c.ShouldBindQuery(&page); err != nil {
		handleError(c, badRequest(err, page))
		return page, false
	}
	return page, true
}
