package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/limit"
)

type limitHandler struct {
	service limit.LimitService
}

func (h *limitHandler) getLimitConfig(c *gin.Context) {
	var request limit.GetLimitConfigRequest
	if err := c.ShouldBindQuery(&request); err != nil {
		handleError(c, badRequest(err, request))
		return
	}

	response, err := h.service.GetLimitConfig(c.Request.Context(), request)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func RegisterLimitRoutes(rg *gin.RouterGroup, limitService limit.LimitService) {
	handler := &limitHandler{
		service: limitService,
	}

	rg.GET("/v1/limits", handler.getLimitConfig)
	rg.POST("/v1/units", handler.unitCreate)
	rg.GET("/v1/units", handler.unitList)
	rg.GET("/v1/units/:unitId", handler.unitGet)
	rg.PUT("/v1/units/:unitId", handler.unitUpdate)
	rg.DELETE("/v1/units/:unitId", handler.unitDelete)
	rg.POST("/v1/limits/types", handler.typeCreate)
	rg.GET("/v1/limits/types", handler.typeList)
	rg.GET("/v1/limits/types/:typeId", handler.typeGet)
	rg.PUT("/v1/limits/types/:typeId", handler.typeUpdate)
	rg.DELETE("/v1/limits/types/:typeId", handler.typeDelete)
	rg.POST("/v1/limits/groups", handler.groupCreate)
	rg.GET("/v1/limits/groups/:groupId", handler.groupGet)
	rg.DELETE("/v1/limits/groups/:groupId", handler.groupDelete)
	rg.POST("/v1/limits/groups/:groupId/values", handler.valueCreate)
	rg.GET("/v1/limits/groups/:groupId/values", handler.valueList)
	rg.GET("/v1/limits/groups/:groupId/values/:valueId", handler.valueGet)
	rg.PUT("/v1/limits/groups/:groupId/values/:valueId", handler.valueUpdate)
	rg.DELETE("/v1/limits/groups/:groupId/values/:valueId", handler.valueDelete)
	rg.POST("/v1/limits/users/:walletId", handler.userCreate)
	rg.GET("/v1/limits/users/:walletId", handler.userList)
	rg.GET("/v1/limits/users/:walletId/:userLimitId", handler.userGet)
	rg.PUT("/v1/limits/users/:walletId/:userLimitId", handler.userUpdate)
	rg.DELETE("/v1/limits/users/:walletId/:userLimitId", handler.userDelete)
}

func pathID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id < 1 {
		handleError(c, core.ErrBadRequest.WithMessage(name+" must be a positive integer"))
		return 0, false
	}
	return id, true
}

func bindJSON(c *gin.Context, v any) bool {
	if err := c.ShouldBindJSON(v); err != nil {
		handleError(c, badRequest(err, v))
		return false
	}
	return true
}

func bindPage(c *gin.Context) (limit.PageRequest, bool) {
	var p limit.PageRequest
	if err := c.ShouldBindQuery(&p); err != nil {
		handleError(c, badRequest(err, p))
		return p, false
	}
	return p, true
}

func (h *limitHandler) unitCreate(c *gin.Context) {
	var r limit.CreateUnitRequest
	if !bindJSON(c, &r) {
		return
	}
	v, e := h.service.CreateUnit(c.Request.Context(), r)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusCreated, v)
}

func (h *limitHandler) unitGet(c *gin.Context) {
	id, ok := pathID(c, "unitId")
	if !ok {
		return
	}
	v, e := h.service.GetUnit(c.Request.Context(), id)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *limitHandler) unitList(c *gin.Context) {
	p, ok := bindPage(c)
	if !ok {
		return
	}
	v, e := h.service.ListUnits(c.Request.Context(), p)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *limitHandler) unitUpdate(c *gin.Context) {
	id, ok := pathID(c, "unitId")
	if !ok {
		return
	}
	var r limit.UpdateUnitRequest
	if !bindJSON(c, &r) {
		return
	}
	v, e := h.service.UpdateUnit(c.Request.Context(), id, r)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *limitHandler) unitDelete(c *gin.Context) {
	id, ok := pathID(c, "unitId")
	if !ok {
		return
	}
	var r limit.ActorRequest
	if !bindJSON(c, &r) {
		return
	}
	if e := h.service.DeleteUnit(c.Request.Context(), id, r); e != nil {
		handleError(c, e)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *limitHandler) typeCreate(c *gin.Context) {
	var r limit.CreateLimitTypeRequest
	if !bindJSON(c, &r) {
		return
	}
	v, e := h.service.CreateLimitType(c.Request.Context(), r)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusCreated, v)
}

func (h *limitHandler) typeGet(c *gin.Context) {
	id, ok := pathID(c, "typeId")
	if !ok {
		return
	}
	v, e := h.service.GetLimitType(c.Request.Context(), id)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *limitHandler) typeList(c *gin.Context) {
	p, ok := bindPage(c)
	if !ok {
		return
	}
	v, e := h.service.ListLimitTypes(c.Request.Context(), p)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *limitHandler) typeUpdate(c *gin.Context) {
	id, ok := pathID(c, "typeId")
	if !ok {
		return
	}
	var r limit.UpdateLimitTypeRequest
	if !bindJSON(c, &r) {
		return
	}
	v, e := h.service.UpdateLimitType(c.Request.Context(), id, r)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *limitHandler) typeDelete(c *gin.Context) {
	id, ok := pathID(c, "typeId")
	if !ok {
		return
	}
	var r limit.ActorRequest
	if !bindJSON(c, &r) {
		return
	}
	if e := h.service.DeleteLimitType(c.Request.Context(), id, r); e != nil {
		handleError(c, e)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *limitHandler) groupCreate(c *gin.Context) {
	var r limit.CreateLimitGroupRequest
	if !bindJSON(c, &r) {
		return
	}
	v, e := h.service.CreateLimitGroup(c.Request.Context(), r)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusCreated, v)
}

func (h *limitHandler) groupGet(c *gin.Context) {
	id, ok := pathID(c, "groupId")
	if !ok {
		return
	}
	v, e := h.service.GetLimitGroup(c.Request.Context(), id)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *limitHandler) groupDelete(c *gin.Context) {
	id, ok := pathID(c, "groupId")
	if !ok {
		return
	}
	var r limit.ActorRequest
	if !bindJSON(c, &r) {
		return
	}
	if e := h.service.DeleteLimitGroup(c.Request.Context(), id, r); e != nil {
		handleError(c, e)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *limitHandler) valueCreate(c *gin.Context) {
	g, ok := pathID(c, "groupId")
	if !ok {
		return
	}
	var r limit.CreateLimitValueRequest
	if !bindJSON(c, &r) {
		return
	}
	v, e := h.service.CreateLimitValue(c.Request.Context(), g, r)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusCreated, v)
}

func (h *limitHandler) valueGet(c *gin.Context) {
	g, ok := pathID(c, "groupId")
	if !ok {
		return
	}
	id, ok := pathID(c, "valueId")
	if !ok {
		return
	}
	v, e := h.service.GetLimitValue(c.Request.Context(), g, id)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *limitHandler) valueList(c *gin.Context) {
	g, ok := pathID(c, "groupId")
	if !ok {
		return
	}
	p, ok := bindPage(c)
	if !ok {
		return
	}
	v, e := h.service.ListLimitValues(c.Request.Context(), g, p)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *limitHandler) valueUpdate(c *gin.Context) {
	g, ok := pathID(c, "groupId")
	if !ok {
		return
	}
	id, ok := pathID(c, "valueId")
	if !ok {
		return
	}
	var r limit.UpdateLimitValueRequest
	if !bindJSON(c, &r) {
		return
	}
	v, e := h.service.UpdateLimitValue(c.Request.Context(), g, id, r)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *limitHandler) valueDelete(c *gin.Context) {
	g, ok := pathID(c, "groupId")
	if !ok {
		return
	}
	id, ok := pathID(c, "valueId")
	if !ok {
		return
	}
	var r limit.ActorRequest
	if !bindJSON(c, &r) {
		return
	}
	if e := h.service.DeleteLimitValue(c.Request.Context(), g, id, r); e != nil {
		handleError(c, e)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *limitHandler) userCreate(c *gin.Context) {
	w := c.Param("walletId")
	var r limit.CreateUserLimitRequest
	if !bindJSON(c, &r) {
		return
	}
	v, e := h.service.CreateUserLimit(c.Request.Context(), w, r)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusCreated, v)
}

func (h *limitHandler) userGet(c *gin.Context) {
	w := c.Param("walletId")
	id, ok := pathID(c, "userLimitId")
	if !ok {
		return
	}
	v, e := h.service.GetUserLimit(c.Request.Context(), w, id)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *limitHandler) userList(c *gin.Context) {
	w := c.Param("walletId")
	p, ok := bindPage(c)
	if !ok {
		return
	}
	v, e := h.service.ListUserLimits(c.Request.Context(), w, p)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *limitHandler) userUpdate(c *gin.Context) {
	w := c.Param("walletId")
	id, ok := pathID(c, "userLimitId")
	if !ok {
		return
	}
	var r limit.UpdateUserLimitRequest
	if !bindJSON(c, &r) {
		return
	}
	v, e := h.service.UpdateUserLimit(c.Request.Context(), w, id, r)
	if e != nil {
		handleError(c, e)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *limitHandler) userDelete(c *gin.Context) {
	w := c.Param("walletId")
	id, ok := pathID(c, "userLimitId")
	if !ok {
		return
	}
	var r limit.ActorRequest
	if !bindJSON(c, &r) {
		return
	}
	if e := h.service.DeleteUserLimit(c.Request.Context(), w, id, r); e != nil {
		handleError(c, e)
		return
	}
	c.Status(http.StatusNoContent)
}
