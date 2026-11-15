package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

type policyServiceStub struct {
	policy.Service
	request  policy.GetConfigRequest
	response policy.GetConfigResponse
	err      error
	called   bool
}

func (s *policyServiceStub) GetConfig(_ context.Context, request policy.GetConfigRequest) (policy.GetConfigResponse, error) {
	s.called = true
	s.request = request
	return s.response, s.err
}

func TestGetPolicyConfigReturnsResponseDirectly(t *testing.T) {
	service := &policyServiceStub{response: policy.GetConfigResponse{Rules: []policy.Rule{{PolicyType: "FEE_AMT", Unit: "AMOUNT", Direction: policy.DirectionOut, Source: policy.SourceSystem}}}}
	response := performPolicyRequest(service, "/v1/policies?walletType=PERSONAL&currency=USD&tier=TIER_1&walletId=wallet-123&transactionType=TRANSFER_OUT")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body policy.GetConfigResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(body.Rules) != 1 || body.Rules[0].PolicyType != "FEE_AMT" || body.Rules[0].Direction != policy.DirectionOut {
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

func TestGetPolicyConfigMapsErrors(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
		want   *core.AppError
	}{
		{name: "not found", err: core.ErrNotFound, status: http.StatusNotFound, want: core.ErrNotFound},
		{name: "internal", err: errors.New("database details"), status: http.StatusInternalServerError, want: core.ErrInternal},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := performPolicyRequest(&policyServiceStub{err: test.err}, "/v1/policies?walletType=PERSONAL&currency=USD&tier=TIER_1&walletId=W1")
			if response.Code != test.status {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			var got core.AppError
			if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil || got.Code != test.want.Code {
				t.Fatalf("error = %#v, decode = %v", got, err)
			}
		})
	}
}

func performPolicyRequest(service policy.Service, target string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterPolicyRoutes(router.Group(""), service)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
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
