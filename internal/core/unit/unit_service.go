package unit

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/zeann3th/policy-service/internal/core"
	cachecore "github.com/zeann3th/policy-service/internal/core/cache"
	cacheport "github.com/zeann3th/policy-service/internal/core/port/cache"
)

const cacheTTL = 15 * time.Minute

type service struct {
	logger      *slog.Logger
	store       Store
	cache       cacheport.Cache
	serviceName string
}

func NewService(logger *slog.Logger, store Store, cache cacheport.Cache, serviceName string) Service {
	return &service{logger: logger, store: store, cache: cache, serviceName: serviceName}
}

func (s *service) generation(ctx context.Context, namespace string) string {
	generation, err := s.cache.Get(ctx, cachecore.GenerationKey(s.serviceName, namespace))
	if err == nil {
		return generation
	}
	if !errors.Is(err, cacheport.ErrMiss) {
		s.logger.Warn("get unit cache generation", "namespace", namespace, "error", err)
	}
	return "0"
}

func (s *service) key(ctx context.Context, parts ...any) string {
	args := []any{s.serviceName, cachecore.NamespaceUnits, s.generation(ctx, cachecore.NamespaceUnits)}
	args = append(args, parts...)
	return core.Join(args...)
}

func (s *service) read(ctx context.Context, key string, target any) bool {
	encoded, err := s.cache.Get(ctx, key)
	if err != nil {
		return false
	}
	switch value := target.(type) {
	case *Response:
		decoded, err := core.Decode[Response](encoded)
		if err == nil {
			*value = *decoded
			return true
		}
	case *core.Page[Response]:
		decoded, err := core.Decode[core.Page[Response]](encoded)
		if err == nil {
			*value = *decoded
			return true
		}
	}
	return false
}

func (s *service) write(ctx context.Context, key string, value any) {
	encoded, err := core.Encode(&value)
	if err == nil {
		err = s.cache.SetWithTTL(ctx, key, encoded, cacheTTL)
	}
	if err != nil {
		s.logger.Warn("cache unit response", "key", key, "error", err)
	}
}

func (s *service) invalidate(ctx context.Context, namespaces ...string) {
	for _, namespace := range namespaces {
		if _, err := s.cache.Increment(ctx, cachecore.GenerationKey(s.serviceName, namespace)); err != nil {
			s.logger.Warn("invalidate unit cache", "namespace", namespace, "error", err)
		}
	}
}

func response(value Unit) Response {
	return Response{ID: value.ID, Code: value.Code, Active: value.Active, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func (s *service) Create(ctx context.Context, request CreateRequest) (Response, error) {
	if request.Active == nil {
		active := true
		request.Active = &active
	}
	value, err := s.store.Create(ctx, request)
	if err == nil {
		s.invalidate(ctx, cachecore.NamespaceUnits)
	}
	return response(value), err
}

func (s *service) Get(ctx context.Context, id int64) (Response, error) {
	key := s.key(ctx, "item", id)
	var cached Response
	if s.read(ctx, key, &cached) {
		return cached, nil
	}
	value, err := s.store.Get(ctx, id)
	result := response(value)
	if err == nil {
		s.write(ctx, key, result)
	}
	return result, err
}

func (s *service) List(ctx context.Context, request core.PageRequest) (core.Page[Response], error) {
	page, limit, offset := request.Values()
	key := s.key(ctx, "list", page, limit)
	var cached core.Page[Response]
	if s.read(ctx, key, &cached) {
		return cached, nil
	}
	values, total, err := s.store.List(ctx, limit, offset)
	result := core.Page[Response]{Items: make([]Response, len(values)), Page: page, Limit: limit, Total: total}
	for i, value := range values {
		result.Items[i] = response(value)
	}
	if err == nil {
		s.write(ctx, key, result)
	}
	return result, err
}

func (s *service) Update(ctx context.Context, id int64, request UpdateRequest) (Response, error) {
	if request.Active == nil {
		return Response{}, core.ErrBadRequest.WithMessage("active is required")
	}
	value, err := s.store.Update(ctx, id, *request.Active, request.UpdatedBy)
	if err == nil {
		s.invalidate(ctx, cachecore.NamespaceUnits, cachecore.NamespacePolicyConfig)
	}
	return response(value), err
}

func (s *service) Delete(ctx context.Context, id int64, request core.ActorRequest) error {
	err := s.store.Delete(ctx, id, request.UpdatedBy)
	if err == nil {
		s.invalidate(ctx, cachecore.NamespaceUnits, cachecore.NamespacePolicyConfig)
	}
	return err
}
