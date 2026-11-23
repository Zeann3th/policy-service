package unit

import (
	"context"

	"github.com/zeann3th/policy-service/internal/core"
	cachecore "github.com/zeann3th/policy-service/internal/core/cache"
)

type service struct {
	store Store
	cache *cachecore.Versioned
}

func NewService(store Store, cache *cachecore.Versioned) Service {
	return &service{store: store, cache: cache}
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
		s.cache.Invalidate(ctx, cachecore.NamespaceUnits)
	}
	return response(value), err
}

func (s *service) Get(ctx context.Context, id int64) (Response, error) {
	key := s.cache.Key(ctx, cachecore.NamespaceUnits, "item", id)
	if cached, ok := cachecore.Read[Response](ctx, s.cache, key); ok {
		return cached, nil
	}
	value, err := s.store.Get(ctx, id)
	result := response(value)
	if err == nil {
		cachecore.Write(ctx, s.cache, key, result)
	}
	return result, err
}

func (s *service) List(ctx context.Context, request core.PageRequest) (core.Page[Response], error) {
	page, limit, offset := request.Values()
	key := s.cache.Key(ctx, cachecore.NamespaceUnits, "list", page, limit)
	if cached, ok := cachecore.Read[core.Page[Response]](ctx, s.cache, key); ok {
		return cached, nil
	}
	values, total, err := s.store.List(ctx, limit, offset)
	result := core.Page[Response]{Items: make([]Response, len(values)), Page: page, Limit: limit, Total: total}
	for i, value := range values {
		result.Items[i] = response(value)
	}
	if err == nil {
		cachecore.Write(ctx, s.cache, key, result)
	}
	return result, err
}

func (s *service) Update(ctx context.Context, id int64, request UpdateRequest) (Response, error) {
	if request.Active == nil {
		return Response{}, core.ErrBadRequest.WithMessage("active is required")
	}
	value, err := s.store.Update(ctx, id, *request.Active, request.UpdatedBy)
	if err == nil {
		s.cache.Invalidate(ctx, cachecore.NamespaceUnits, cachecore.NamespacePolicyConfig)
	}
	return response(value), err
}

func (s *service) Delete(ctx context.Context, id int64, request core.ActorRequest) error {
	err := s.store.Delete(ctx, id, request.UpdatedBy)
	if err == nil {
		s.cache.Invalidate(ctx, cachecore.NamespaceUnits, cachecore.NamespacePolicyConfig)
	}
	return err
}
