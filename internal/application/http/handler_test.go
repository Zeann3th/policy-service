package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
	"github.com/zeann3th/policy-service/internal/infrastructure/config"
)

type policyServiceStub struct {
	policy.Service
	request       policy.GetConfigRequest
	response      policy.GetConfigResponse
	customRequest policy.CreateCustomPolicyRequest
	err           error
	called        bool
}

func (s *policyServiceStub) GetConfig(_ context.Context, request policy.GetConfigRequest) (policy.GetConfigResponse, error) {
	s.called = true
	s.request = request
	return s.response, s.err
}

func (s *policyServiceStub) CreateCustomPolicy(_ context.Context, walletID string, request policy.CreateCustomPolicyRequest) (policy.CustomPolicyResponse, error) {
	s.called = true
	s.customRequest = request
	return policy.CustomPolicyResponse{WalletID: walletID, WindowType: request.WindowType}, s.err
}

func TestGetPolicyConfigReturnsResponseDirectly(t *testing.T) {
	service := &policyServiceStub{response: policy.GetConfigResponse{Rules: []policy.Rule{{PolicyType: "TXN_MAX", Kind: policy.KindLimit, Unit: policy.UnitAmount, Direction: policy.DirectionOut, WindowType: policy.WindowDaily, Source: policy.SourceSystem}}}}
	response := performPolicyRequest(service, "/v1/policies?walletType=PERSONAL&currency=USD&tier=TIER_1&walletId=wallet-123&transactionType=TRANSFER_OUT")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body policy.GetConfigResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(body.Rules) != 1 || body.Rules[0].PolicyType != "TXN_MAX" || body.Rules[0].Direction != policy.DirectionOut || body.Rules[0].WindowType != policy.WindowDaily {
		t.Fatalf("response = %s", response.Body.String())
	}
	if service.request.TransactionType != "TRANSFER_OUT" {
		t.Fatalf("transaction type = %q", service.request.TransactionType)
	}
}

func TestGetPolicyConfigReturnsActionableBadRequest(t *testing.T) {
	service := &policyServiceStub{}
	response := performPolicyRequest(service, "/v1/policies?walletType=PERSONAL")

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "currency is required") {
		t.Fatalf("status/body = %d %s", response.Code, response.Body.String())
	}
	if service.called {
		t.Fatal("service called for invalid request")
	}
}

func TestHandleErrorMapsStatusAndMessage(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
		want   *core.AppError
	}{
		{name: "not found", err: core.ErrNotFound, status: http.StatusNotFound, want: core.ErrNotFound},
		{name: "conflict keeps message", err: core.ErrConflict.WithMessage("reservation is RELEASED").WithCause(errors.New("database details")), status: http.StatusConflict, want: core.ErrConflict.WithMessage("reservation is RELEASED")},
		{name: "internal", err: errors.New("database details"), status: http.StatusInternalServerError, want: core.ErrInternal},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := performPolicyRequest(&policyServiceStub{err: test.err}, "/v1/policies?walletType=PERSONAL&currency=USD&tier=TIER_1&walletId=W1")
			if response.Code != test.status {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			var got core.AppError
			if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil || got.Code != test.want.Code || got.Message != test.want.Message {
				t.Fatalf("error = %#v, decode = %v", got, err)
			}
		})
	}
}

func TestCreateCustomPolicyAcceptsWindowType(t *testing.T) {
	service := &policyServiceStub{}
	response := performPolicyJSONRequest(service, http.MethodPost, "/v1/policies/custom/wallet-123", `{"policyTypeId":1,"unitId":1,"windowType":"DAILY","operator":"LTE","value":"1000","updatedBy":"admin"}`)

	if response.Code != http.StatusCreated {
		t.Fatalf("status/body = %d %s", response.Code, response.Body.String())
	}
	if !service.called || service.customRequest.WindowType != policy.WindowDaily {
		t.Fatalf("request = %#v", service.customRequest)
	}
}

func TestCreateCustomPolicyRejectsInvalidWindowType(t *testing.T) {
	service := &policyServiceStub{}
	response := performPolicyJSONRequest(service, http.MethodPost, "/v1/policies/custom/wallet-123", `{"policyTypeId":1,"windowType":"YEARLY","operator":"LTE","value":"1000","updatedBy":"admin"}`)

	if response.Code != http.StatusBadRequest || service.called {
		t.Fatalf("status/body/called = %d %s %t", response.Code, response.Body.String(), service.called)
	}
}

func performPolicyRequest(service policy.Service, target string) *httptest.ResponseRecorder {
	return performPolicyJSONRequest(service, http.MethodGet, target, "")
}

func performPolicyJSONRequest(service policy.Service, method, target, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterPolicyRoutes(router.Group(""), service)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	return response
}

func TestSwaggerDocumentsEveryRoute(t *testing.T) {
	data, err := os.ReadFile("../../../docs/swagger.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		BasePath string                                `json:"basePath"`
		Paths    map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if document.BasePath != "/policy-service/api" {
		t.Fatalf("basePath = %q", document.BasePath)
	}

	expected := map[string][]string{
		"/healthz":                       {http.MethodGet},
		"/v1/cache/reload":               {http.MethodPost},
		"/v1/policies":                   {http.MethodGet},
		"/v1/policies/custom/{walletId}": {http.MethodGet, http.MethodPost},
		"/v1/policies/custom/{walletId}/{customPolicyId}":   {http.MethodGet, http.MethodPut, http.MethodDelete},
		"/v1/policies/groups":                               {http.MethodPost},
		"/v1/policies/groups/{groupId}":                     {http.MethodGet, http.MethodDelete},
		"/v1/policies/groups/{groupId}/policies":            {http.MethodGet, http.MethodPost},
		"/v1/policies/groups/{groupId}/policies/{policyId}": {http.MethodGet, http.MethodPut, http.MethodDelete},
		"/v1/policies/types":                                {http.MethodGet, http.MethodPost},
		"/v1/policies/types/{typeId}":                       {http.MethodGet, http.MethodPut, http.MethodDelete},
		"/v1/units":                                         {http.MethodGet, http.MethodPost},
		"/v1/units/{unitId}":                                {http.MethodGet, http.MethodPut, http.MethodDelete},
	}
	for path, methods := range expected {
		for _, method := range methods {
			if _, ok := document.Paths[path][strings.ToLower(method)]; !ok {
				t.Errorf("Swagger missing %s %s", method, path)
			}
		}
	}
}

func TestSwaggerRouteFollowsConfig(t *testing.T) {
	for enabled, want := range map[bool]int{false: http.StatusNotFound, true: http.StatusOK} {
		cfg := &config.Config{ServiceName: "policy-service", HTTP: config.HTTP{Port: "0", GinMode: gin.TestMode, SwaggerEnabled: enabled}}
		server, err := New(cfg, slog.New(slog.DiscardHandler), nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		server.listener.Close()
		response := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/policy-service/api/swagger/index.html", nil))
		if response.Code != want {
			t.Errorf("SwaggerEnabled=%v: status = %d, want %d", enabled, response.Code, want)
		}
	}
}

func TestCORSFollowsAllowedOrigins(t *testing.T) {
	for _, test := range []struct {
		origins []string
		origin  string
		want    string
	}{
		{nil, "http://localhost:3000", ""},
		{[]string{"http://localhost:3000", "https://cms.example"}, "https://cms.example", "https://cms.example"},
		{[]string{"http://localhost:3000"}, "https://evil.example", ""},
		{[]string{"*"}, "https://anything.example", "*"},
	} {
		cfg := &config.Config{ServiceName: "policy-service", HTTP: config.HTTP{Port: "0", GinMode: gin.TestMode, CORSOrigins: test.origins}}
		server, err := New(cfg, slog.New(slog.DiscardHandler), nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		server.listener.Close()
		request := httptest.NewRequest(http.MethodOptions, "/policy-service/api/healthz", nil)
		request.Header.Set("Origin", test.origin)
		request.Header.Set("Access-Control-Request-Method", http.MethodPost)
		response := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(response, request)
		if got := response.Header().Get("Access-Control-Allow-Origin"); got != test.want {
			t.Errorf("origins %v, origin %s: Access-Control-Allow-Origin = %q, want %q", test.origins, test.origin, got, test.want)
		}
	}
}
