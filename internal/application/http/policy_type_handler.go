package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

type policyTypeHandler struct {
	service policy.TypeService
}

func RegisterPolicyTypeRoutes(routes *gin.RouterGroup, service policy.TypeService) {
	handler := &policyTypeHandler{service: service}
	routes.POST("/v1/policies/types", handler.create)
	routes.GET("/v1/policies/types", handler.list)
	routes.GET("/v1/policies/types/:typeId", handler.get)
	routes.PUT("/v1/policies/types/:typeId", handler.update)
	routes.DELETE("/v1/policies/types/:typeId", handler.delete)
}

// create godoc
//
//	@Summary	Create a policy type
//	@Tags		Policy Types
//	@Accept		json
//	@Produce	json
//	@Param		request			body		policy.CreatePolicyTypeRequest	true	"Policy type"
//	@Success	201				{object}	policy.PolicyTypeResponse
//	@Failure	400,404,409,500	{object}	core.AppError
//	@Router		/v1/policies/types [post]
func (h *policyTypeHandler) create(c *gin.Context) {
	var request policy.CreatePolicyTypeRequest
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
//	@Summary	Get a policy type
//	@Tags		Policy Types
//	@Produce	json
//	@Param		typeId		path		integer	true	"Policy type ID"	minimum(1)
//	@Success	200			{object}	policy.PolicyTypeResponse
//	@Failure	400,404,500	{object}	core.AppError
//	@Router		/v1/policies/types/{typeId} [get]
func (h *policyTypeHandler) get(c *gin.Context) {
	id, ok := pathID(c, "typeId")
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

// list godoc
//
//	@Summary	List policy types
//	@Tags		Policy Types
//	@Produce	json
//	@Param		page	query		integer	false	"Page number"	default(1)	minimum(1)
//	@Param		limit	query		integer	false	"Page size"		default(20)	minimum(1)	maximum(100)
//	@Success	200		{object}	policyTypePageResponse
//	@Failure	400,500	{object}	core.AppError
//	@Router		/v1/policies/types [get]
func (h *policyTypeHandler) list(c *gin.Context) {
	page, ok := bindPage(c)
	if !ok {
		return
	}
	response, err := h.service.List(c.Request.Context(), page)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// update godoc
//
//	@Summary		Update a policy type
//	@Description	Code, unit, kind, and window type are immutable.
//	@Tags			Policy Types
//	@Accept			json
//	@Produce		json
//	@Param			typeId			path		integer							true	"Policy type ID"	minimum(1)
//	@Param			request			body		policy.UpdatePolicyTypeRequest	true	"Mutable policy type fields"
//	@Success		200				{object}	policy.PolicyTypeResponse
//	@Failure		400,404,409,500	{object}	core.AppError
//	@Router			/v1/policies/types/{typeId} [put]
func (h *policyTypeHandler) update(c *gin.Context) {
	id, ok := pathID(c, "typeId")
	if !ok {
		return
	}
	var request policy.UpdatePolicyTypeRequest
	if !bindJSON(c, &request) {
		return
	}
	response, err := h.service.Update(c.Request.Context(), id, request)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// delete godoc
//
//	@Summary		Delete a policy type
//	@Description	Deletion is rejected when historical counters reference the type.
//	@Tags			Policy Types
//	@Accept			json
//	@Param			typeId	path	integer				true	"Policy type ID"	minimum(1)
//	@Param			request	body	core.ActorRequest	true	"Audit actor"
//	@Success		204
//	@Failure		400,404,409,500	{object}	core.AppError
//	@Router			/v1/policies/types/{typeId} [delete]
func (h *policyTypeHandler) delete(c *gin.Context) {
	id, ok := pathID(c, "typeId")
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
