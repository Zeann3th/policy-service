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
	return Response{ID: value.ID, Code: value.Code, Description: value.Description, Active: value.Active, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func (s *service) Create(ctx context.Context, request CreateRequest) (Response, error) {
	if request.Active == nil {
		active := true
		request.Active = &active
	}
	code, ok := core.Code(request.Code)
	if !ok {
		return Response{}, core.ErrBadRequest.WithMessage("code must be letters, digits and underscores, starting with a letter")
	}
	request.Code = code
	value, err := s.store.Create(ctx, request)
	if err == nil {
		s.cache.Invalidate(ctx, cachecore.NamespaceUnits)
	}
	return response(value), err
}

func (s *service) Get(ctx context.Context, id int64) (Response, error) {
	key := s.cache.Key(ctx, cachecore.NamespaceUnits, "item", id)
	return cachecore.Fetch(ctx, s.cache, key, func() (Response, error) {
		value, err := s.store.Get(ctx, id)
		return response(value), err
	})
}

func (s *service) List(ctx context.Context, request core.PageRequest) (core.Page[Response], error) {
	page, limit, offset := request.Values()
	key := s.cache.Key(ctx, cachecore.NamespaceUnits, "list", page, limit)
	return cachecore.Fetch(ctx, s.cache, key, func() (core.Page[Response], error) {
		values, total, err := s.store.List(ctx, limit, offset)
		return core.MapPage(values, page, limit, total, response), err
	})
}

func (s *service) Update(ctx context.Context, id int64, request UpdateRequest) (Response, error) {
	if request.Active == nil {
		return Response{}, core.ErrBadRequest.WithMessage("active is required")
	}
	value, err := s.store.Update(ctx, id, request)
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
