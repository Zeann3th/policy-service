package limit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zeann3th/policy-service/internal/core"
	cacheport "github.com/zeann3th/policy-service/internal/core/port/cache"
)

const adminCacheTTL = 15 * time.Minute

func cacheRead[T any](s *limitServiceImpl, ctx context.Context, key string) (T, bool) {
	var zero T
	encoded, err := s.cache.Get(ctx, key)
	if err != nil {
		if errors.Is(err, cacheport.ErrMiss) {
			s.logger.Debug("limit cache miss", "key", key)
		} else {
			s.logger.Warn("get limit cache", "key", key, "error", err)
		}
		return zero, false
	}
	value, err := core.Decode[T](encoded)
	if err != nil {
		s.logger.Warn("decode limit cache", "key", key, "error", err)
		return zero, false
	}
	return *value, true
}

func cacheWrite[T any](s *limitServiceImpl, ctx context.Context, key string, value T) {
	encoded, err := core.Encode(&value)
	if err == nil {
		err = s.cache.SetWithTTL(ctx, key, encoded, adminCacheTTL)
	}
	if err != nil {
		s.logger.Warn("cache limit admin response", "key", key, "error", err)
	}
}

func (s *limitServiceImpl) generation(ctx context.Context, scope string) string {
	generation, err := s.cache.Get(ctx, core.Join(s.cachePrefix, scope, "generation"))
	if err == nil {
		return generation
	}
	if errors.Is(err, cacheport.ErrMiss) {
		return "0"
	}
	s.logger.Warn("get limit cache generation", "scope", scope, "error", err)
	return fmt.Sprintf("bypass-%d", time.Now().UnixNano())
}

func (s *limitServiceImpl) generatedKey(ctx context.Context, scope string, parts ...any) string {
	args := []any{s.cachePrefix, "admin", s.generation(ctx, "admin"), scope, s.generation(ctx, scope)}
	args = append(args, parts...)
	return core.Join(args...)
}

func (s *limitServiceImpl) itemKey(ctx context.Context, resource string, parts ...any) string {
	args := []any{s.cachePrefix, "admin", s.generation(ctx, "admin"), resource}
	args = append(args, parts...)
	return core.Join(args...)
}

func (s *limitServiceImpl) limitConfigKey(ctx context.Context, parts ...any) string {
	args := []any{s.cachePrefix, "limit-config", s.generation(ctx, "limit-config")}
	args = append(args, parts...)
	return core.Join(args...)
}

func (s *limitServiceImpl) evict(ctx context.Context, key string) {
	if _, err := s.cache.Evict(ctx, key); err != nil {
		s.logger.Warn("evict limit cache", "key", key, "error", err)
	}
}

func (s *limitServiceImpl) evictLimitConfig(ctx context.Context, group LimitGroup) {
	s.evict(ctx, s.limitConfigKey(ctx, group.WalletType, group.Currency, group.Tier))
}

func (s *limitServiceImpl) evictGroupConfig(ctx context.Context, store LimitAdminStore, groupID int64) {
	group, err := store.GetLimitGroup(ctx, groupID)
	if err != nil {
		s.logger.Warn("get group for cache invalidation", "groupId", groupID, "error", err)
		return
	}
	s.evictLimitConfig(ctx, group)
}

func (s *limitServiceImpl) adminStore() (LimitAdminStore, error) {
	store, ok := s.store.(LimitAdminStore)
	if !ok {
		return nil, core.ErrInternal.WithCause(fmt.Errorf("limit admin store unavailable"))
	}
	return store, nil
}

func (s *limitServiceImpl) invalidate(ctx context.Context, keys ...string) {
	for _, key := range keys {
		if _, err := s.cache.Increment(ctx, core.Join(s.cachePrefix, key, "generation")); err != nil {
			s.logger.Warn("invalidate limit cache", "key", key, "error", err)
		}
	}
}

func unitResponse(v Unit) UnitResponse {
	return UnitResponse{v.ID, v.Code, v.Active, v.CreatedAt, v.UpdatedAt}
}
func limitTypeResponse(v LimitType) LimitTypeResponse {
	return LimitTypeResponse{v.ID, v.Code, v.Description, v.UnitID, v.Active, v.CreatedAt, v.UpdatedAt}
}
func limitGroupResponse(v LimitGroup) LimitGroupResponse {
	return LimitGroupResponse{v.ID, v.WalletType, v.Currency, v.Tier, v.CreatedAt, v.UpdatedAt}
}
func limitValueResponse(v LimitValue) LimitValueResponse {
	return LimitValueResponse{v.ID, v.LimitGroupID, v.LimitTypeID, v.TransactionType, v.Channel, v.Operator, v.Value, v.CreatedAt, v.UpdatedAt}
}
func userLimitResponse(v UserLimit) UserLimitResponse {
	return UserLimitResponse{v.ID, v.WalletID, v.LimitTypeID, v.TransactionType, v.Channel, v.Operator, v.Value, v.EffectiveFrom, v.EffectiveTo, v.Reason, v.CreatedBy, v.CreatedAt, v.UpdatedAt}
}

func (s *limitServiceImpl) CreateUnit(ctx context.Context, c CreateUnitRequest) (UnitResponse, error) {
	if c.Active == nil {
		active := true
		c.Active = &active
	}
	st, e := s.adminStore()
	if e != nil {
		return UnitResponse{}, e
	}
	v, e := st.CreateUnit(ctx, c)
	if e == nil {
		s.invalidate(ctx, "units")
	}
	return unitResponse(v), e
}
func (s *limitServiceImpl) GetUnit(ctx context.Context, id int64) (UnitResponse, error) {
	key := s.itemKey(ctx, "unit", id)
	if v, ok := cacheRead[UnitResponse](s, ctx, key); ok {
		return v, nil
	}
	st, e := s.adminStore()
	if e != nil {
		return UnitResponse{}, e
	}
	v, e := st.GetUnit(ctx, id)
	r := unitResponse(v)
	if e == nil {
		cacheWrite(s, ctx, key, r)
	}
	return r, e
}
func (s *limitServiceImpl) ListUnits(ctx context.Context, p PageRequest) (Page[UnitResponse], error) {
	page, limit, off := p.Values()
	key := s.generatedKey(ctx, "units", page, limit)
	if v, ok := cacheRead[Page[UnitResponse]](s, ctx, key); ok {
		return v, nil
	}
	st, e := s.adminStore()
	if e != nil {
		return Page[UnitResponse]{}, e
	}
	vs, total, e := st.ListUnits(ctx, limit, off)
	r := Page[UnitResponse]{Items: make([]UnitResponse, len(vs)), Page: page, Limit: limit, Total: total}
	for i, v := range vs {
		r.Items[i] = unitResponse(v)
	}
	if e == nil {
		cacheWrite(s, ctx, key, r)
	}
	return r, e
}
func (s *limitServiceImpl) UpdateUnit(ctx context.Context, id int64, c UpdateUnitRequest) (UnitResponse, error) {
	st, e := s.adminStore()
	if e != nil {
		return UnitResponse{}, e
	}
	v, e := st.UpdateUnit(ctx, id, *c.Active, c.UpdatedBy)
	if e == nil {
		s.evict(ctx, s.itemKey(ctx, "unit", id))
		s.invalidate(ctx, "units", "limit-config")
	}
	return unitResponse(v), e
}
func (s *limitServiceImpl) DeleteUnit(ctx context.Context, id int64, c ActorRequest) error {
	st, e := s.adminStore()
	if e != nil {
		return e
	}
	e = st.DeleteUnit(ctx, id, c.UpdatedBy)
	if e == nil {
		s.evict(ctx, s.itemKey(ctx, "unit", id))
		s.invalidate(ctx, "units", "limit-config", "admin")
	}
	return e
}

func (s *limitServiceImpl) CreateLimitType(ctx context.Context, c CreateLimitTypeRequest) (LimitTypeResponse, error) {
	if c.Active == nil {
		active := true
		c.Active = &active
	}
	st, e := s.adminStore()
	if e != nil {
		return LimitTypeResponse{}, e
	}
	v, e := st.CreateLimitType(ctx, c)
	if e == nil {
		s.invalidate(ctx, "limit-types")
	}
	return limitTypeResponse(v), e
}
func (s *limitServiceImpl) GetLimitType(ctx context.Context, id int64) (LimitTypeResponse, error) {
	key := s.itemKey(ctx, "limit-type", id)
	if v, ok := cacheRead[LimitTypeResponse](s, ctx, key); ok {
		return v, nil
	}
	st, e := s.adminStore()
	if e != nil {
		return LimitTypeResponse{}, e
	}
	v, e := st.GetLimitType(ctx, id)
	r := limitTypeResponse(v)
	if e == nil {
		cacheWrite(s, ctx, key, r)
	}
	return r, e
}
func (s *limitServiceImpl) ListLimitTypes(ctx context.Context, p PageRequest) (Page[LimitTypeResponse], error) {
	page, l, o := p.Values()
	key := s.generatedKey(ctx, "limit-types", page, l)
	if v, ok := cacheRead[Page[LimitTypeResponse]](s, ctx, key); ok {
		return v, nil
	}
	st, e := s.adminStore()
	if e != nil {
		return Page[LimitTypeResponse]{}, e
	}
	vs, total, e := st.ListLimitTypes(ctx, l, o)
	r := Page[LimitTypeResponse]{Items: make([]LimitTypeResponse, len(vs)), Page: page, Limit: l, Total: total}
	for i, v := range vs {
		r.Items[i] = limitTypeResponse(v)
	}
	if e == nil {
		cacheWrite(s, ctx, key, r)
	}
	return r, e
}
func (s *limitServiceImpl) UpdateLimitType(ctx context.Context, id int64, c UpdateLimitTypeRequest) (LimitTypeResponse, error) {
	st, e := s.adminStore()
	if e != nil {
		return LimitTypeResponse{}, e
	}
	v, e := st.UpdateLimitType(ctx, id, c)
	if e == nil {
		s.evict(ctx, s.itemKey(ctx, "limit-type", id))
		s.invalidate(ctx, "limit-types", "limit-config")
	}
	return limitTypeResponse(v), e
}
func (s *limitServiceImpl) DeleteLimitType(ctx context.Context, id int64, c ActorRequest) error {
	st, e := s.adminStore()
	if e != nil {
		return e
	}
	e = st.DeleteLimitType(ctx, id, c.UpdatedBy)
	if e == nil {
		s.evict(ctx, s.itemKey(ctx, "limit-type", id))
		s.invalidate(ctx, "limit-types", "limit-config", "admin")
	}
	return e
}

func (s *limitServiceImpl) CreateLimitGroup(ctx context.Context, c CreateLimitGroupRequest) (LimitGroupResponse, error) {
	st, e := s.adminStore()
	if e != nil {
		return LimitGroupResponse{}, e
	}
	v, e := st.CreateLimitGroup(ctx, c)
	return limitGroupResponse(v), e
}
func (s *limitServiceImpl) GetLimitGroup(ctx context.Context, id int64) (LimitGroupResponse, error) {
	key := s.itemKey(ctx, "limit-group", id)
	if v, ok := cacheRead[LimitGroupResponse](s, ctx, key); ok {
		return v, nil
	}
	st, e := s.adminStore()
	if e != nil {
		return LimitGroupResponse{}, e
	}
	v, e := st.GetLimitGroup(ctx, id)
	r := limitGroupResponse(v)
	if e == nil {
		cacheWrite(s, ctx, key, r)
	}
	return r, e
}
func (s *limitServiceImpl) DeleteLimitGroup(ctx context.Context, id int64, c ActorRequest) error {
	st, e := s.adminStore()
	if e != nil {
		return e
	}
	group, e := st.GetLimitGroup(ctx, id)
	if e != nil {
		return e
	}
	e = st.DeleteLimitGroup(ctx, id, c.UpdatedBy)
	if e == nil {
		s.evict(ctx, s.itemKey(ctx, "limit-group", id))
		s.evictLimitConfig(ctx, group)
		s.invalidate(ctx, "admin")
	}
	return e
}

func defaults(t, c string) (string, string) {
	if t == "" {
		t = All
	}
	if c == "" {
		c = All
	}
	return t, c
}
func (s *limitServiceImpl) CreateLimitValue(ctx context.Context, g int64, c CreateLimitValueRequest) (LimitValueResponse, error) {
	if c.Value == nil {
		return LimitValueResponse{}, core.ErrBadRequest.WithMessage("value is required")
	}
	c.TransactionType, c.Channel = defaults(c.TransactionType, c.Channel)
	st, e := s.adminStore()
	if e != nil {
		return LimitValueResponse{}, e
	}
	v, e := st.CreateLimitValue(ctx, g, c)
	if e == nil {
		s.invalidate(ctx, fmt.Sprintf("limit-values:%d", g))
		s.evictGroupConfig(ctx, st, g)
	}
	return limitValueResponse(v), e
}
func (s *limitServiceImpl) GetLimitValue(ctx context.Context, g, id int64) (LimitValueResponse, error) {
	key := s.itemKey(ctx, "limit-value", g, id)
	if v, ok := cacheRead[LimitValueResponse](s, ctx, key); ok {
		return v, nil
	}
	st, e := s.adminStore()
	if e != nil {
		return LimitValueResponse{}, e
	}
	v, e := st.GetLimitValue(ctx, g, id)
	r := limitValueResponse(v)
	if e == nil {
		cacheWrite(s, ctx, key, r)
	}
	return r, e
}
func (s *limitServiceImpl) ListLimitValues(ctx context.Context, g int64, p PageRequest) (Page[LimitValueResponse], error) {
	page, l, o := p.Values()
	key := s.generatedKey(ctx, fmt.Sprintf("limit-values:%d", g), page, l)
	if v, ok := cacheRead[Page[LimitValueResponse]](s, ctx, key); ok {
		return v, nil
	}
	st, e := s.adminStore()
	if e != nil {
		return Page[LimitValueResponse]{}, e
	}
	vs, total, e := st.ListLimitValues(ctx, g, l, o)
	r := Page[LimitValueResponse]{Items: make([]LimitValueResponse, len(vs)), Page: page, Limit: l, Total: total}
	for i, v := range vs {
		r.Items[i] = limitValueResponse(v)
	}
	if e == nil {
		cacheWrite(s, ctx, key, r)
	}
	return r, e
}
func (s *limitServiceImpl) UpdateLimitValue(ctx context.Context, g, id int64, c UpdateLimitValueRequest) (LimitValueResponse, error) {
	if c.Value == nil {
		return LimitValueResponse{}, core.ErrBadRequest.WithMessage("value is required")
	}
	st, e := s.adminStore()
	if e != nil {
		return LimitValueResponse{}, e
	}
	v, e := st.UpdateLimitValue(ctx, g, id, c)
	if e == nil {
		s.evict(ctx, s.itemKey(ctx, "limit-value", g, id))
		s.invalidate(ctx, fmt.Sprintf("limit-values:%d", g))
		s.evictGroupConfig(ctx, st, g)
	}
	return limitValueResponse(v), e
}
func (s *limitServiceImpl) DeleteLimitValue(ctx context.Context, g, id int64, c ActorRequest) error {
	st, e := s.adminStore()
	if e != nil {
		return e
	}
	e = st.DeleteLimitValue(ctx, g, id, c.UpdatedBy)
	if e == nil {
		s.evict(ctx, s.itemKey(ctx, "limit-value", g, id))
		s.invalidate(ctx, fmt.Sprintf("limit-values:%d", g))
		s.evictGroupConfig(ctx, st, g)
	}
	return e
}

func validWindow(from time.Time, to *time.Time) bool { return to == nil || from.Before(*to) }

func (s *limitServiceImpl) CreateUserLimit(ctx context.Context, w string, c CreateUserLimitRequest) (UserLimitResponse, error) {
	if c.Value == nil {
		return UserLimitResponse{}, core.ErrBadRequest.WithMessage("value is required")
	}
	if c.EffectiveFrom != nil && !validWindow(*c.EffectiveFrom, c.EffectiveTo) {
		return UserLimitResponse{}, core.ErrBadRequest.WithMessage("effectiveFrom must be before effectiveTo")
	}
	if c.EffectiveFrom == nil && c.EffectiveTo != nil && !time.Now().Before(*c.EffectiveTo) {
		return UserLimitResponse{}, core.ErrBadRequest.WithMessage("effectiveTo must be in the future when effectiveFrom is omitted")
	}
	c.TransactionType, c.Channel = defaults(c.TransactionType, c.Channel)
	st, e := s.adminStore()
	if e != nil {
		return UserLimitResponse{}, e
	}
	v, e := st.CreateUserLimit(ctx, w, c)
	if e == nil {
		s.invalidate(ctx, fmt.Sprintf("user-limits:%s", w))
	}
	return userLimitResponse(v), e
}
func (s *limitServiceImpl) GetUserLimit(ctx context.Context, w string, id int64) (UserLimitResponse, error) {
	key := s.itemKey(ctx, "user-limit", w, id)
	if v, ok := cacheRead[UserLimitResponse](s, ctx, key); ok {
		return v, nil
	}
	st, e := s.adminStore()
	if e != nil {
		return UserLimitResponse{}, e
	}
	v, e := st.GetUserLimit(ctx, w, id)
	r := userLimitResponse(v)
	if e == nil {
		cacheWrite(s, ctx, key, r)
	}
	return r, e
}
func (s *limitServiceImpl) ListUserLimits(ctx context.Context, w string, p PageRequest) (Page[UserLimitResponse], error) {
	page, l, o := p.Values()
	key := s.generatedKey(ctx, fmt.Sprintf("user-limits:%s", w), page, l)
	if v, ok := cacheRead[Page[UserLimitResponse]](s, ctx, key); ok {
		return v, nil
	}
	st, e := s.adminStore()
	if e != nil {
		return Page[UserLimitResponse]{}, e
	}
	vs, total, e := st.ListUserLimits(ctx, w, l, o)
	r := Page[UserLimitResponse]{Items: make([]UserLimitResponse, len(vs)), Page: page, Limit: l, Total: total}
	for i, v := range vs {
		r.Items[i] = userLimitResponse(v)
	}
	if e == nil {
		cacheWrite(s, ctx, key, r)
	}
	return r, e
}
func (s *limitServiceImpl) UpdateUserLimit(ctx context.Context, w string, id int64, c UpdateUserLimitRequest) (UserLimitResponse, error) {
	if c.Value == nil {
		return UserLimitResponse{}, core.ErrBadRequest.WithMessage("value is required")
	}
	if c.EffectiveFrom == nil {
		return UserLimitResponse{}, core.ErrBadRequest.WithMessage("effectiveFrom is required")
	}
	if !validWindow(*c.EffectiveFrom, c.EffectiveTo) {
		return UserLimitResponse{}, core.ErrBadRequest.WithMessage("effectiveFrom must be before effectiveTo")
	}
	st, e := s.adminStore()
	if e != nil {
		return UserLimitResponse{}, e
	}
	v, e := st.UpdateUserLimit(ctx, w, id, c)
	if e == nil {
		s.evict(ctx, s.itemKey(ctx, "user-limit", w, id))
		s.invalidate(ctx, fmt.Sprintf("user-limits:%s", w))
	}
	return userLimitResponse(v), e
}
func (s *limitServiceImpl) DeleteUserLimit(ctx context.Context, w string, id int64, c ActorRequest) error {
	st, e := s.adminStore()
	if e != nil {
		return e
	}
	e = st.DeleteUserLimit(ctx, w, id, c.UpdatedBy)
	if e == nil {
		s.evict(ctx, s.itemKey(ctx, "user-limit", w, id))
		s.invalidate(ctx, fmt.Sprintf("user-limits:%s", w))
	}
	return e
}
