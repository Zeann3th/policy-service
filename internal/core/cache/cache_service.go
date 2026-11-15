package cache

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/zeann3th/policy-service/internal/core"
	cacheport "github.com/zeann3th/policy-service/internal/core/port/cache"
)

const (
	NamespaceUnits          = "units"
	NamespacePolicyTypes    = "policy:types"
	NamespacePolicyGroups   = "policy:groups"
	NamespacePolicies       = "policies"
	NamespaceCustomPolicies = "policy:custom"
	NamespacePolicyConfig   = "policy:config"
)

var namespaces = map[string]struct{}{
	NamespaceUnits:          {},
	NamespacePolicyTypes:    {},
	NamespacePolicyGroups:   {},
	NamespacePolicies:       {},
	NamespaceCustomPolicies: {},
	NamespacePolicyConfig:   {},
}

type ReloadRequest struct {
	Key        string   `example:"policy:config:0:CU0001:VND:1" json:"key"`
	Namespaces []string `enums:"units,policy:types,policy:groups,policies,policy:custom,policy:config" json:"namespaces"`
	UpdatedBy  string   `binding:"required" example:"admin" json:"updatedBy"`
}

type ReloadResponse struct {
	Evicted            bool     `json:"evicted"`
	ReloadedNamespaces []string `json:"reloadedNamespaces,omitempty"`
}

type Service interface {
	Reload(context.Context, ReloadRequest) (ReloadResponse, error)
}

type service struct {
	logger      *slog.Logger
	cache       cacheport.Cache
	serviceName string
}

func NewService(logger *slog.Logger, cache cacheport.Cache, serviceName string) Service {
	return &service{logger: logger, cache: cache, serviceName: serviceName}
}

func Key(serviceName string, parts ...any) string {
	args := append([]any{serviceName}, parts...)
	return core.Join(args...)
}

func GenerationKey(serviceName, namespace string) string {
	return Key(serviceName, namespace, "generation")
}

func (s *service) Reload(ctx context.Context, request ReloadRequest) (ReloadResponse, error) {
	request.Key = strings.TrimSpace(request.Key)
	request.UpdatedBy = strings.TrimSpace(request.UpdatedBy)
	if request.UpdatedBy == "" {
		return ReloadResponse{}, core.ErrBadRequest.WithMessage("updatedBy is required")
	}
	if (request.Key == "") == (len(request.Namespaces) == 0) {
		return ReloadResponse{}, core.ErrBadRequest.WithMessage("provide either key or namespaces")
	}

	if request.Key != "" {
		if strings.HasPrefix(request.Key, s.serviceName+":") || strings.HasPrefix(request.Key, ":") {
			return ReloadResponse{}, core.ErrBadRequest.WithMessage("key must be relative to the service namespace")
		}
		evicted, err := s.cache.Evict(ctx, Key(s.serviceName, request.Key))
		if err != nil {
			return ReloadResponse{}, fmt.Errorf("evict cache key: %w", err)
		}
		s.logger.Info("reloaded cache key", "key", request.Key, "updatedBy", request.UpdatedBy)
		return ReloadResponse{Evicted: evicted}, nil
	}

	seen := make(map[string]struct{}, len(request.Namespaces))
	reloaded := make([]string, 0, len(request.Namespaces))
	for _, namespace := range request.Namespaces {
		namespace = strings.TrimSpace(namespace)
		if _, ok := namespaces[namespace]; !ok {
			return ReloadResponse{}, core.ErrBadRequest.WithMessage("unknown cache namespace: " + namespace)
		}
		if _, ok := seen[namespace]; ok {
			continue
		}
		if _, err := s.cache.Increment(ctx, GenerationKey(s.serviceName, namespace)); err != nil {
			return ReloadResponse{}, fmt.Errorf("reload cache namespace %s: %w", namespace, err)
		}
		seen[namespace] = struct{}{}
		reloaded = append(reloaded, namespace)
	}

	s.logger.Info("reloaded cache namespaces", "namespaces", reloaded, "updatedBy", request.UpdatedBy)
	return ReloadResponse{ReloadedNamespaces: reloaded}, nil
}
