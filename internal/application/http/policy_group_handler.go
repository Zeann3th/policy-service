package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

type policyGroupHandler struct {
	service policy.GroupService
}

func RegisterPolicyGroupRoutes(routes *gin.RouterGroup, service policy.GroupService) {
	handler := &policyGroupHandler{service: service}
	routes.POST("/v1/policies/groups", handler.create)
	routes.GET("/v1/policies/groups/:groupId", handler.get)
	routes.DELETE("/v1/policies/groups/:groupId", handler.delete)
}

// create godoc
//
//	@Summary	Create a policy group
//	@Tags		Policy Groups
//	@Accept		json
//	@Produce	json
//	@Param		request		body		policy.CreatePolicyGroupRequest	true	"Policy group"
//	@Success	201			{object}	policy.PolicyGroupResponse
//	@Failure	400,409,500	{object}	core.AppError
//	@Router		/v1/policies/groups [post]
func (h *policyGroupHandler) create(c *gin.Context) {
	var request policy.CreatePolicyGroupRequest
	if !bindJSON(c, &request) {
		return
	}
	response, err := h.service.Create(c.Request.Context(), request)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, response)
}

// get godoc
//
//	@Summary	Get a policy group
//	@Tags		Policy Groups
//	@Produce	json
//	@Param		groupId		path		integer	true	"Policy group ID"	minimum(1)
//	@Success	200			{object}	policy.PolicyGroupResponse
//	@Failure	400,404,500	{object}	core.AppError
//	@Router		/v1/policies/groups/{groupId} [get]
func (h *policyGroupHandler) get(c *gin.Context) {
	id, ok := pathID(c, "groupId")
	if !ok {
		return
	}
	response, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// delete godoc
//
//	@Summary		Delete a policy group
//	@Description	Deletes the group and its configured policies.
//	@Tags			Policy Groups
//	@Accept			json
//	@Param			groupId	path	integer				true	"Policy group ID"	minimum(1)
//	@Param			request	body	core.ActorRequest	true	"Audit actor"
//	@Success		204
//	@Failure		400,404,409,500	{object}	core.AppError
//	@Router			/v1/policies/groups/{groupId} [delete]
func (h *policyGroupHandler) delete(c *gin.Context) {
	id, ok := pathID(c, "groupId")
	if !ok {
		return
	}
	var request core.ActorRequest
	if !bindJSON(c, &request) {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id, request); err != nil {
		handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
