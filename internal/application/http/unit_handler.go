package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/unit"
)

type unitHandler struct {
	service unit.Service
}

func RegisterUnitRoutes(routes *gin.RouterGroup, service unit.Service) {
	handler := &unitHandler{service: service}
	routes.POST("/v1/units", handler.create)
	routes.GET("/v1/units", handler.list)
	routes.GET("/v1/units/:unitId", handler.get)
	routes.PUT("/v1/units/:unitId", handler.update)
	routes.DELETE("/v1/units/:unitId", handler.delete)
}

// create godoc
//
//	@Summary	Create a unit
//	@Tags		Units
//	@Accept		json
//	@Produce	json
//	@Param		request		body		unit.CreateRequest	true	"Unit"
//	@Success	201			{object}	unit.Response
//	@Failure	400,409,500	{object}	core.AppError
//	@Router		/v1/units [post]
func (h *unitHandler) create(c *gin.Context) {
	var request unit.CreateRequest
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
//	@Summary	Get a unit
//	@Tags		Units
//	@Produce	json
//	@Param		unitId		path		integer	true	"Unit ID"	minimum(1)
//	@Success	200			{object}	unit.Response
//	@Failure	400,404,500	{object}	core.AppError
//	@Router		/v1/units/{unitId} [get]
func (h *unitHandler) get(c *gin.Context) {
	id, ok := pathID(c, "unitId")
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
//	@Summary	List units
//	@Tags		Units
//	@Produce	json
//	@Param		page	query		integer	false	"Page number"	default(1)	minimum(1)
//	@Param		limit	query		integer	false	"Page size"		default(20)	minimum(1)	maximum(100)
//	@Success	200		{object}	unitPageResponse
//	@Failure	400,500	{object}	core.AppError
//	@Router		/v1/units [get]
func (h *unitHandler) list(c *gin.Context) {
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
//	@Summary		Update a unit
//	@Description	The unit code is immutable.
//	@Tags			Units
//	@Accept			json
//	@Produce		json
//	@Param			unitId			path		integer				true	"Unit ID"	minimum(1)
//	@Param			request			body		unit.UpdateRequest	true	"Mutable unit fields"
//	@Success		200				{object}	unit.Response
//	@Failure		400,404,409,500	{object}	core.AppError
//	@Router			/v1/units/{unitId} [put]
func (h *unitHandler) update(c *gin.Context) {
	id, ok := pathID(c, "unitId")
	if !ok {
		return
	}
	var request unit.UpdateRequest
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
//	@Summary	Delete a unit
//	@Tags		Units
//	@Accept		json
//	@Param		unitId	path	integer				true	"Unit ID"	minimum(1)
//	@Param		request	body	core.ActorRequest	true	"Audit actor"
//	@Success	204
//	@Failure	400,404,409,500	{object}	core.AppError
//	@Router		/v1/units/{unitId} [delete]
func (h *unitHandler) delete(c *gin.Context) {
	id, ok := pathID(c, "unitId")
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
