package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/limit"
)

type limitAdminHandler struct{ service limit.LimitAdminService }

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

func (h *limitAdminHandler) unitCreate(c *gin.Context) {
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
func (h *limitAdminHandler) unitGet(c *gin.Context) {
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
func (h *limitAdminHandler) unitList(c *gin.Context) {
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
func (h *limitAdminHandler) unitUpdate(c *gin.Context) {
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
func (h *limitAdminHandler) unitDelete(c *gin.Context) {
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

func (h *limitAdminHandler) typeCreate(c *gin.Context) {
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
func (h *limitAdminHandler) typeGet(c *gin.Context) {
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
func (h *limitAdminHandler) typeList(c *gin.Context) {
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
func (h *limitAdminHandler) typeUpdate(c *gin.Context) {
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
func (h *limitAdminHandler) typeDelete(c *gin.Context) {
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

func (h *limitAdminHandler) groupCreate(c *gin.Context) {
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
func (h *limitAdminHandler) groupGet(c *gin.Context) {
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
func (h *limitAdminHandler) groupDelete(c *gin.Context) {
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

func (h *limitAdminHandler) valueCreate(c *gin.Context) {
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
func (h *limitAdminHandler) valueGet(c *gin.Context) {
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
func (h *limitAdminHandler) valueList(c *gin.Context) {
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
func (h *limitAdminHandler) valueUpdate(c *gin.Context) {
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
func (h *limitAdminHandler) valueDelete(c *gin.Context) {
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

func (h *limitAdminHandler) userCreate(c *gin.Context) {
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
func (h *limitAdminHandler) userGet(c *gin.Context) {
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
func (h *limitAdminHandler) userList(c *gin.Context) {
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
func (h *limitAdminHandler) userUpdate(c *gin.Context) {
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
func (h *limitAdminHandler) userDelete(c *gin.Context) {
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

func RegisterLimitAdminRoutes(r *gin.RouterGroup, service limit.LimitAdminService) {
	h := &limitAdminHandler{service: service}
	r.POST("/v1/units", h.unitCreate)
	r.GET("/v1/units", h.unitList)
	r.GET("/v1/units/:unitId", h.unitGet)
	r.PUT("/v1/units/:unitId", h.unitUpdate)
	r.DELETE("/v1/units/:unitId", h.unitDelete)
	r.POST("/v1/limits/types", h.typeCreate)
	r.GET("/v1/limits/types", h.typeList)
	r.GET("/v1/limits/types/:typeId", h.typeGet)
	r.PUT("/v1/limits/types/:typeId", h.typeUpdate)
	r.DELETE("/v1/limits/types/:typeId", h.typeDelete)
	r.POST("/v1/limits/groups", h.groupCreate)
	r.GET("/v1/limits/groups/:groupId", h.groupGet)
	r.DELETE("/v1/limits/groups/:groupId", h.groupDelete)
	r.POST("/v1/limits/groups/:groupId/values", h.valueCreate)
	r.GET("/v1/limits/groups/:groupId/values", h.valueList)
	r.GET("/v1/limits/groups/:groupId/values/:valueId", h.valueGet)
	r.PUT("/v1/limits/groups/:groupId/values/:valueId", h.valueUpdate)
	r.DELETE("/v1/limits/groups/:groupId/values/:valueId", h.valueDelete)
	r.POST("/v1/limits/users/:walletId", h.userCreate)
	r.GET("/v1/limits/users/:walletId", h.userList)
	r.GET("/v1/limits/users/:walletId/:userLimitId", h.userGet)
	r.PUT("/v1/limits/users/:walletId/:userLimitId", h.userUpdate)
	r.DELETE("/v1/limits/users/:walletId/:userLimitId", h.userDelete)
}
