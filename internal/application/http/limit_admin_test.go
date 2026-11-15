package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core/limit"
)

type adminServiceStub struct {
	limit.LimitAdminService
	page limit.PageRequest
}

func TestCreateLimitValueRequiresDecimalPresence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterLimitAdminRoutes(router.Group(""), &adminServiceStub{})
	request := httptest.NewRequest(http.MethodPost, "/v1/limits/groups/1/values", strings.NewReader(`{"limitTypeId":1,"operator":"LTE","updatedBy":"admin"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "value is required") {
		t.Fatalf("status/body = %d %s", response.Code, response.Body.String())
	}
}

func TestUpdateUserLimitRequiresEffectiveFromPresence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterLimitAdminRoutes(router.Group(""), &adminServiceStub{})
	request := httptest.NewRequest(http.MethodPut, "/v1/limits/users/W1/1", strings.NewReader(`{"operator":"LTE","value":"0","updatedBy":"admin"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "effectiveFrom is required") {
		t.Fatalf("status/body = %d %s", response.Code, response.Body.String())
	}
}

func (s *adminServiceStub) ListUnits(_ context.Context, page limit.PageRequest) (limit.Page[limit.UnitResponse], error) {
	s.page = page
	return limit.Page[limit.UnitResponse]{Items: []limit.UnitResponse{}, Page: 1, Limit: 20}, nil
}

func TestUnitListDefaultsPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &adminServiceStub{}
	router := gin.New()
	RegisterLimitAdminRoutes(router.Group(""), service)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/units", nil))
	if response.Code != http.StatusOK || service.page.Page != 0 || service.page.Limit != 0 {
		t.Fatalf("status = %d, request = %#v, body = %s", response.Code, service.page, response.Body.String())
	}
}

func TestGroupListAndUpdateRoutesAreAbsent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterLimitAdminRoutes(router.Group(""), &adminServiceStub{})
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(method, "/v1/limits/groups", nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s group collection status = %d", method, response.Code)
		}
	}
}
