package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

type policyHandler struct {
	service policy.Service
}

func RegisterPolicyRoutes(routes *gin.RouterGroup, service policy.Service) {
	handler := &policyHandler{service: service}
	routes.GET("/v1/policies", handler.getConfig)
	routes.POST("/v1/policies/groups/:groupId/policies", handler.create)
	routes.GET("/v1/policies/groups/:groupId/policies", handler.list)
	routes.GET("/v1/policies/groups/:groupId/policies/:policyId", handler.get)
	routes.PUT("/v1/policies/groups/:groupId/policies/:policyId", handler.update)
	routes.DELETE("/v1/policies/groups/:groupId/policies/:policyId", handler.delete)
	routes.POST("/v1/policies/custom/:walletId", handler.createCustom)
	routes.GET("/v1/policies/custom/:walletId", handler.listCustom)
	routes.GET("/v1/policies/custom/:walletId/:customPolicyId", handler.getCustom)
	routes.PUT("/v1/policies/custom/:walletId/:customPolicyId", handler.updateCustom)
	routes.DELETE("/v1/policies/custom/:walletId/:customPolicyId", handler.deleteCustom)
}

// getConfig godoc
//
//	@Summary		Resolve effective policies
//	@Description	Returns active system policies merged with active wallet-specific overrides. An override replaces only an exact policy type, transaction type, channel, direction, and window type match.
//	@Tags			Policy Resolution
//	@Produce		json
//	@Param			walletType		query		string	true	"Wallet type"
//	@Param			currency		query		string	true	"ISO 4217 currency"	minlength(3)	maxlength(3)
//	@Param			tier			query		string	true	"Tier"
//	@Param			walletId		query		string	true	"Wallet ID"
//	@Param			transactionType	query		string	false	"Transaction type filter; ALL policies remain included"
//	@Success		200				{object}	policy.GetConfigResponse
//	@Failure		400,404,500		{object}	core.AppError
//	@Router			/v1/policies [get]
func (h *policyHandler) getConfig(c *gin.Context) {
	var request policy.GetConfigRequest
	if err := c.ShouldBindQuery(&request); err != nil {
		handleError(c, badRequest(err, request))
		return
	}
	response, err := h.service.GetConfig(c.Request.Context(), request)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// create godoc
//
//	@Summary	Create a system policy
//	@Tags		Policies
//	@Accept		json
//	@Produce	json
//	@Param		groupId			path		integer						true	"Policy group ID"	minimum(1)
//	@Param		request			body		policy.CreatePolicyRequest	true	"Policy"
//	@Success	201				{object}	policy.PolicyResponse
//	@Failure	400,404,409,500	{object}	core.AppError
//	@Router		/v1/policies/groups/{groupId}/policies [post]
func (h *policyHandler) create(c *gin.Context) {
	groupID, ok := pathID(c, "groupId")
	if !ok {
		return
	}
	var request policy.CreatePolicyRequest
	if !bindJSON(c, &request) {
		return
	}
	response, err := h.service.CreatePolicy(c.Request.Context(), groupID, request)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, response)
}

// get godoc
//
//	@Summary	Get a system policy
//	@Tags		Policies
//	@Produce	json
//	@Param		groupId		path		integer	true	"Policy group ID"	minimum(1)
//	@Param		policyId	path		integer	true	"Policy ID"			minimum(1)
//	@Success	200			{object}	policy.PolicyResponse
//	@Failure	400,404,500	{object}	core.AppError
//	@Router		/v1/policies/groups/{groupId}/policies/{policyId} [get]
func (h *policyHandler) get(c *gin.Context) {
	groupID, ok := pathID(c, "groupId")
	if !ok {
		return
	}
	id, ok := pathID(c, "policyId")
	if !ok {
		return
	}
	response, err := h.service.GetPolicy(c.Request.Context(), groupID, id)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// list godoc
//
//	@Summary	List policies in a group
//	@Tags		Policies
//	@Produce	json
//	@Param		groupId		path		integer	true	"Policy group ID"	minimum(1)
//	@Param		page		query		integer	false	"Page number"		default(1)	minimum(1)
//	@Param		limit		query		integer	false	"Page size"			default(20)	minimum(1)	maximum(100)
//	@Success	200			{object}	policyPageResponse
//	@Failure	400,404,500	{object}	core.AppError
//	@Router		/v1/policies/groups/{groupId}/policies [get]
func (h *policyHandler) list(c *gin.Context) {
	groupID, ok := pathID(c, "groupId")
	if !ok {
		return
	}
	page, ok := bindPage(c)
	if !ok {
		return
	}
	response, err := h.service.ListPolicies(c.Request.Context(), groupID, page)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// update godoc
//
//	@Summary		Update a system policy
//	@Description	Policy group, type, transaction type, channel, and direction are immutable.
//	@Tags			Policies
//	@Accept			json
//	@Produce		json
//	@Param			groupId			path		integer						true	"Policy group ID"	minimum(1)
//	@Param			policyId		path		integer						true	"Policy ID"			minimum(1)
//	@Param			request			body		policy.UpdatePolicyRequest	true	"Mutable policy fields"
//	@Success		200				{object}	policy.PolicyResponse
//	@Failure		400,404,409,500	{object}	core.AppError
//	@Router			/v1/policies/groups/{groupId}/policies/{policyId} [put]
func (h *policyHandler) update(c *gin.Context) {
	groupID, ok := pathID(c, "groupId")
	if !ok {
		return
	}
	id, ok := pathID(c, "policyId")
	if !ok {
		return
	}
	var request policy.UpdatePolicyRequest
	if !bindJSON(c, &request) {
		return
	}
	response, err := h.service.UpdatePolicy(c.Request.Context(), groupID, id, request)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// delete godoc
//
//	@Summary	Delete a system policy
//	@Tags		Policies
//	@Accept		json
//	@Param		groupId		path	integer				true	"Policy group ID"	minimum(1)
//	@Param		policyId	path	integer				true	"Policy ID"			minimum(1)
//	@Param		request		body	core.ActorRequest	true	"Audit actor"
//	@Success	204
//	@Failure	400,404,409,500	{object}	core.AppError
//	@Router		/v1/policies/groups/{groupId}/policies/{policyId} [delete]
func (h *policyHandler) delete(c *gin.Context) {
	groupID, ok := pathID(c, "groupId")
	if !ok {
		return
	}
	id, ok := pathID(c, "policyId")
	if !ok {
		return
	}
	var request core.ActorRequest
	if !bindJSON(c, &request) {
		return
	}
	if err := h.service.DeletePolicy(c.Request.Context(), groupID, id, request); err != nil {
		handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// createCustom godoc
//
//	@Summary	Create a custom wallet policy
//	@Tags		Custom Policies
//	@Accept		json
//	@Produce	json
//	@Param		walletId		path		string								true	"Wallet ID"
//	@Param		request			body		policy.CreateCustomPolicyRequest	true	"Custom policy"
//	@Success	201				{object}	policy.CustomPolicyResponse
//	@Failure	400,404,409,500	{object}	core.AppError
//	@Router		/v1/policies/custom/{walletId} [post]
func (h *policyHandler) createCustom(c *gin.Context) {
	var request policy.CreateCustomPolicyRequest
	if !bindJSON(c, &request) {
		return
	}
	response, err := h.service.CreateCustomPolicy(c.Request.Context(), c.Param("walletId"), request)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, response)
}

// getCustom godoc
//
//	@Summary	Get a custom wallet policy
//	@Tags		Custom Policies
//	@Produce	json
//	@Param		walletId		path		string	true	"Wallet ID"
//	@Param		customPolicyId	path		integer	true	"Custom policy ID"	minimum(1)
//	@Success	200				{object}	policy.CustomPolicyResponse
//	@Failure	400,404,500		{object}	core.AppError
//	@Router		/v1/policies/custom/{walletId}/{customPolicyId} [get]
func (h *policyHandler) getCustom(c *gin.Context) {
	id, ok := pathID(c, "customPolicyId")
	if !ok {
		return
	}
	response, err := h.service.GetCustomPolicy(c.Request.Context(), c.Param("walletId"), id)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// listCustom godoc
//
//	@Summary	List custom wallet policies
//	@Tags		Custom Policies
//	@Produce	json
//	@Param		walletId	path		string	true	"Wallet ID"
//	@Param		page		query		integer	false	"Page number"	default(1)	minimum(1)
//	@Param		limit		query		integer	false	"Page size"		default(20)	minimum(1)	maximum(100)
//	@Success	200			{object}	customPolicyPageResponse
//	@Failure	400,500		{object}	core.AppError
//	@Router		/v1/policies/custom/{walletId} [get]
func (h *policyHandler) listCustom(c *gin.Context) {
	page, ok := bindPage(c)
	if !ok {
		return
	}
	response, err := h.service.ListCustomPolicies(c.Request.Context(), c.Param("walletId"), page)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// updateCustom godoc
//
//	@Summary		Update a custom wallet policy
//	@Description	Wallet, policy type, transaction type, channel, direction, window type, and creator are immutable.
//	@Tags			Custom Policies
//	@Accept			json
//	@Produce		json
//	@Param			walletId		path		string								true	"Wallet ID"
//	@Param			customPolicyId	path		integer								true	"Custom policy ID"	minimum(1)
//	@Param			request			body		policy.UpdateCustomPolicyRequest	true	"Mutable custom policy fields"
//	@Success		200				{object}	policy.CustomPolicyResponse
//	@Failure		400,404,409,500	{object}	core.AppError
//	@Router			/v1/policies/custom/{walletId}/{customPolicyId} [put]
func (h *policyHandler) updateCustom(c *gin.Context) {
	id, ok := pathID(c, "customPolicyId")
	if !ok {
		return
	}
	var request policy.UpdateCustomPolicyRequest
	if !bindJSON(c, &request) {
		return
	}
	response, err := h.service.UpdateCustomPolicy(c.Request.Context(), c.Param("walletId"), id, request)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// deleteCustom godoc
//
//	@Summary	Delete a custom wallet policy
//	@Tags		Custom Policies
//	@Accept		json
//	@Param		walletId		path	string				true	"Wallet ID"
//	@Param		customPolicyId	path	integer				true	"Custom policy ID"	minimum(1)
//	@Param		request			body	core.ActorRequest	true	"Audit actor"
//	@Success	204
//	@Failure	400,404,409,500	{object}	core.AppError
//	@Router		/v1/policies/custom/{walletId}/{customPolicyId} [delete]
func (h *policyHandler) deleteCustom(c *gin.Context) {
	id, ok := pathID(c, "customPolicyId")
	if !ok {
		return
	}
	var request core.ActorRequest
	if !bindJSON(c, &request) {
		return
	}
	if err := h.service.DeleteCustomPolicy(c.Request.Context(), c.Param("walletId"), id, request); err != nil {
		handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
