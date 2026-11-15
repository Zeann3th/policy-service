package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/limit"
)

type fakeLimitService struct {
	limit.LimitService
	request  limit.GetLimitConfigRequest
	response limit.GetLimitConfigResponse
	err      error
	called   bool
}

func (s *fakeLimitService) GetLimitConfig(_ context.Context, request limit.GetLimitConfigRequest) (limit.GetLimitConfigResponse, error) {
	s.called = true
	s.request = request
	return s.response, s.err
}

func TestGetLimitConfigReturnsResponseDirectly(t *testing.T) {
	service := &fakeLimitService{response: limit.GetLimitConfigResponse{
		Rules: []limit.LimitRule{{LimitType: "TXN_MAX_AMT", Unit: "AMOUNT", Source: limit.SourceSystem}},
	}}
	response := performLimitRequest(service, "/v1/limits?walletType=PERSONAL&currency=USD&tier=TIER_1&walletId=wallet-123&transactionType=TRANSFER_OUT")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body limit.GetLimitConfigResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(body.Rules) != 1 || body.Rules[0].LimitType != "TXN_MAX_AMT" || body.Rules[0].Unit != "AMOUNT" || body.Rules[0].Source != limit.SourceSystem {
		t.Fatalf("response = %s", response.Body.String())
	}
	if service.request.WalletType != "PERSONAL" || service.request.Currency != "USD" || service.request.Tier != "TIER_1" {
		t.Fatalf("request = %#v", service.request)
	}
	if service.request.TransactionType != "TRANSFER_OUT" {
		t.Fatalf("request transaction type = %q", service.request.TransactionType)
	}
}

func TestGetLimitConfigReturnsBadRequest(t *testing.T) {
	service := &fakeLimitService{}
	response := performLimitRequest(service, "/v1/limits?walletType=PERSONAL")

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "currency is required") {
		t.Fatalf("expected actionable 400, got %d: %s", response.Code, response.Body.String())
	}
	if service.called {
		t.Fatal("service called for invalid request")
	}
}

func TestGetLimitConfigReturnsNotFound(t *testing.T) {
	service := &fakeLimitService{err: core.ErrNotFound}
	response := performLimitRequest(service, "/v1/limits?walletType=PERSONAL&currency=USD&tier=TIER_1&walletId=wallet-123")

	assertHTTPError(t, response, http.StatusNotFound, core.ErrNotFound)
}

func TestGetLimitConfigReturnsSanitizedInternalError(t *testing.T) {
	service := &fakeLimitService{err: errors.New("database connection details")}
	response := performLimitRequest(service, "/v1/limits?walletType=PERSONAL&currency=USD&tier=TIER_1&walletId=wallet-123")

	assertHTTPError(t, response, http.StatusInternalServerError, core.ErrInternal)
}

func performLimitRequest(service limit.LimitService, target string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterLimitRoutes(router.Group(""), service)
	request := httptest.NewRequest(http.MethodGet, target, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func assertHTTPError(t *testing.T, response *httptest.ResponseRecorder, status int, want *core.AppError) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, status, response.Body.String())
	}
	var got core.AppError
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.Code != want.Code || got.Message != want.Message {
		t.Fatalf("error = %#v, want %#v", got, want)
	}
}

type limitServiceStub struct {
	limit.LimitService
	page limit.PageRequest
}

func TestCreateLimitValueRequiresDecimalPresence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterLimitRoutes(router.Group(""), &limitServiceStub{})
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
	RegisterLimitRoutes(router.Group(""), &limitServiceStub{})
	request := httptest.NewRequest(http.MethodPut, "/v1/limits/users/W1/1", strings.NewReader(`{"operator":"LTE","value":"0","updatedBy":"admin"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "effectiveFrom is required") {
		t.Fatalf("status/body = %d %s", response.Code, response.Body.String())
	}
}

func (s *limitServiceStub) ListUnits(_ context.Context, page limit.PageRequest) (limit.Page[limit.UnitResponse], error) {
	s.page = page
	return limit.Page[limit.UnitResponse]{Items: []limit.UnitResponse{}, Page: 1, Limit: 20}, nil
}

func TestUnitListDefaultsPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &limitServiceStub{}
	router := gin.New()
	RegisterLimitRoutes(router.Group(""), service)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/units", nil))
	if response.Code != http.StatusOK || service.page.Page != 0 || service.page.Limit != 0 {
		t.Fatalf("status = %d, request = %#v, body = %s", response.Code, service.page, response.Body.String())
	}
}

func TestGroupListAndUpdateRoutesAreAbsent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterLimitRoutes(router.Group(""), &limitServiceStub{})
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(method, "/v1/limits/groups", nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s group collection status = %d", method, response.Code)
		}
	}
}
