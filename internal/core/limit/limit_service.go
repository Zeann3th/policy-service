package limit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/zeann3th/policy-service/internal/core"
	cacheport "github.com/zeann3th/policy-service/internal/core/port/cache"
)

const unitsCacheKeyPart = "units"

type LimitService interface {
	GetLimitConfig(context.Context, GetLimitConfigRequest) (GetLimitConfigResponse, error)
	CreateUnit(context.Context, CreateUnitRequest) (UnitResponse, error)
	GetUnit(context.Context, int64) (UnitResponse, error)
	ListUnits(context.Context, PageRequest) (Page[UnitResponse], error)
	UpdateUnit(context.Context, int64, UpdateUnitRequest) (UnitResponse, error)
	DeleteUnit(context.Context, int64, ActorRequest) error
	CreateLimitType(context.Context, CreateLimitTypeRequest) (LimitTypeResponse, error)
	GetLimitType(context.Context, int64) (LimitTypeResponse, error)
	ListLimitTypes(context.Context, PageRequest) (Page[LimitTypeResponse], error)
	UpdateLimitType(context.Context, int64, UpdateLimitTypeRequest) (LimitTypeResponse, error)
	DeleteLimitType(context.Context, int64, ActorRequest) error
	CreateLimitGroup(context.Context, CreateLimitGroupRequest) (LimitGroupResponse, error)
	GetLimitGroup(context.Context, int64) (LimitGroupResponse, error)
	DeleteLimitGroup(context.Context, int64, ActorRequest) error
	CreateLimitValue(context.Context, int64, CreateLimitValueRequest) (LimitValueResponse, error)
	GetLimitValue(context.Context, int64, int64) (LimitValueResponse, error)
	ListLimitValues(context.Context, int64, PageRequest) (Page[LimitValueResponse], error)
	UpdateLimitValue(context.Context, int64, int64, UpdateLimitValueRequest) (LimitValueResponse, error)
	DeleteLimitValue(context.Context, int64, int64, ActorRequest) error
	CreateUserLimit(context.Context, string, CreateUserLimitRequest) (UserLimitResponse, error)
	GetUserLimit(context.Context, string, int64) (UserLimitResponse, error)
	ListUserLimits(context.Context, string, PageRequest) (Page[UserLimitResponse], error)
	UpdateUserLimit(context.Context, string, int64, UpdateUserLimitRequest) (UserLimitResponse, error)
	DeleteUserLimit(context.Context, string, int64, ActorRequest) error
}

type limitServiceImpl struct {
	logger      *slog.Logger
	store       LimitStore
	cache       cacheport.Cache
	cachePrefix string
}

func NewService(
	logger *slog.Logger,
	store LimitStore,
	cache cacheport.Cache,
	serviceName string,
) LimitService {
	return &limitServiceImpl{
		logger:      logger,
		store:       store,
		cache:       cache,
		cachePrefix: core.Join(serviceName, "limit"),
	}
}

func (s *limitServiceImpl) GetLimitConfig(ctx context.Context, command GetLimitConfigRequest) (GetLimitConfigResponse, error) {
	values, err := s.getSystemLimitConfigs(ctx, command)
	if err != nil {
		return GetLimitConfigResponse{}, err
	}
	userLimits, err := s.store.GetActiveUserLimits(ctx, command.WalletID)
	if err != nil {
		return GetLimitConfigResponse{}, fmt.Errorf("get active user limits: %w", err)
	}
	values = mergeLimitValues(values, userLimits)
	values = filterLimitValuesByTransactionType(values, command.TransactionType)

	response := GetLimitConfigResponse{Rules: make([]LimitRule, len(values))}
	if len(values) == 0 {
		return response, nil
	}
	units, err := s.getUnits(ctx)
	if err != nil {
		return GetLimitConfigResponse{}, err
	}
	for i, value := range values {
		unit, ok := units[value.UnitID]
		if !ok {
			return GetLimitConfigResponse{}, fmt.Errorf("unit %d not found", value.UnitID)
		}
		response.Rules[i] = LimitRule{
			LimitType:       value.LimitType,
			Unit:            unit,
			TransactionType: value.TransactionType,
			Channel:         value.Channel,
			Operator:        value.Operator,
			Value:           value.Value,
			Source:          value.Source,
		}
	}
	return response, nil
}

func filterLimitValuesByTransactionType(values []LimitValue, transactionType string) []LimitValue {
	if transactionType == "" {
		return values
	}
	filtered := make([]LimitValue, 0, len(values))
	for _, value := range values {
		if value.TransactionType == All || value.TransactionType == transactionType {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func mergeLimitValues(systemValues []LimitValue, userLimits []UserLimit) []LimitValue {
	values := append([]LimitValue(nil), systemValues...)
	indexes := make(map[LimitKey]int, len(values))
	for i, value := range values {
		values[i].Source = SourceSystem
		indexes[value.Key()] = i
	}
	for _, userLimit := range userLimits {
		value := LimitValue{
			LimitTypeID:     userLimit.LimitTypeID,
			LimitType:       userLimit.LimitType,
			UnitID:          userLimit.UnitID,
			TransactionType: userLimit.TransactionType,
			Channel:         userLimit.Channel,
			Operator:        userLimit.Operator,
			Value:           userLimit.Value,
			Source:          SourceUser,
		}
		if i, ok := indexes[userLimit.Key()]; ok {
			values[i] = value
			continue
		}
		indexes[userLimit.Key()] = len(values)
		values = append(values, value)
	}
	return values
}

func (s *limitServiceImpl) getUnits(ctx context.Context) (map[int64]string, error) {
	cacheKey := s.limitConfigKey(ctx, unitsCacheKeyPart)
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
		units, err := core.Decode[map[int64]string](cached)
		if err == nil {
			return *units, nil
		}
	}

	items, err := s.store.GetUnits(ctx)
	if err != nil {
		return nil, fmt.Errorf("get units: %w", err)
	}
	units := make(map[int64]string, len(items))
	for _, item := range items {
		units[item.ID] = item.Code
	}
	encoded, err := core.Encode(&units)
	if err != nil {
		s.logger.Warn("encode units for cache", "key", cacheKey, "error", err)
		return units, nil
	}
	if err := s.cache.SetWithTTL(ctx, cacheKey, encoded, limitCacheTTL); err != nil {
		s.logger.Warn("cache units", "key", cacheKey, "error", err)
	}
	return units, nil
}

func (s *limitServiceImpl) getSystemLimitConfigs(ctx context.Context, command GetLimitConfigRequest) ([]LimitValue, error) {
	cacheKey := s.limitConfigKey(ctx,
		command.WalletType,
		command.Currency,
		command.Tier,
	)

	if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
		results, err := core.Decode[[]LimitValue](cached)
		if err == nil {
			return *results, nil
		}
	}

	groupID, err := s.store.GetLimitGroupID(
		ctx,
		command.WalletType,
		command.Currency,
		command.Tier,
	)
	if err != nil {
		return nil, fmt.Errorf("get limit group: %w", err)
	}

	values, err := s.store.GetLimitValuesByGroupID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("get limit values: %w", err)
	}

	encoded, err := core.Encode(&values)
	if err != nil {
		s.logger.Warn("encode limit config for cache", "key", cacheKey, "error", err)
		return values, nil
	}
	if err := s.cache.SetWithTTL(ctx, cacheKey, encoded, limitCacheTTL); err != nil {
		s.logger.Warn("cache limit config", "key", cacheKey, "error", err)
	}
	return values, nil
}

const limitCacheTTL = 15 * time.Minute

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
		err = s.cache.SetWithTTL(ctx, key, encoded, limitCacheTTL)
	}
	if err != nil {
		s.logger.Warn("cache limit response", "key", key, "error", err)
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
	args := []any{s.cachePrefix, "resources", s.generation(ctx, "resources"), scope, s.generation(ctx, scope)}
	args = append(args, parts...)
	return core.Join(args...)
}

func (s *limitServiceImpl) itemKey(ctx context.Context, resource string, parts ...any) string {
	args := []any{s.cachePrefix, "resources", s.generation(ctx, "resources"), resource}
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

func (s *limitServiceImpl) evictGroupConfig(ctx context.Context, store LimitStore, groupID int64) {
	group, err := store.GetLimitGroup(ctx, groupID)
	if err != nil {
		s.logger.Warn("get group for cache invalidation", "groupId", groupID, "error", err)
		return
	}
	s.evictLimitConfig(ctx, group)
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
	st := s.store
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
	st := s.store
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
	st := s.store
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
	st := s.store
	v, e := st.UpdateUnit(ctx, id, *c.Active, c.UpdatedBy)
	if e == nil {
		s.evict(ctx, s.itemKey(ctx, "unit", id))
		s.invalidate(ctx, "units", "limit-config")
	}
	return unitResponse(v), e
}

func (s *limitServiceImpl) DeleteUnit(ctx context.Context, id int64, c ActorRequest) error {
	st := s.store
	e := st.DeleteUnit(ctx, id, c.UpdatedBy)
	if e == nil {
		s.evict(ctx, s.itemKey(ctx, "unit", id))
		s.invalidate(ctx, "units", "limit-config", "resources")
	}
	return e
}

func (s *limitServiceImpl) CreateLimitType(ctx context.Context, c CreateLimitTypeRequest) (LimitTypeResponse, error) {
	if c.Active == nil {
		active := true
		c.Active = &active
	}
	st := s.store
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
	st := s.store
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
	st := s.store
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
	st := s.store
	v, e := st.UpdateLimitType(ctx, id, c)
	if e == nil {
		s.evict(ctx, s.itemKey(ctx, "limit-type", id))
		s.invalidate(ctx, "limit-types", "limit-config")
	}
	return limitTypeResponse(v), e
}

func (s *limitServiceImpl) DeleteLimitType(ctx context.Context, id int64, c ActorRequest) error {
	st := s.store
	e := st.DeleteLimitType(ctx, id, c.UpdatedBy)
	if e == nil {
		s.evict(ctx, s.itemKey(ctx, "limit-type", id))
		s.invalidate(ctx, "limit-types", "limit-config", "resources")
	}
	return e
}

func (s *limitServiceImpl) CreateLimitGroup(ctx context.Context, c CreateLimitGroupRequest) (LimitGroupResponse, error) {
	st := s.store
	v, e := st.CreateLimitGroup(ctx, c)
	return limitGroupResponse(v), e
}

func (s *limitServiceImpl) GetLimitGroup(ctx context.Context, id int64) (LimitGroupResponse, error) {
	key := s.itemKey(ctx, "limit-group", id)
	if v, ok := cacheRead[LimitGroupResponse](s, ctx, key); ok {
		return v, nil
	}
	st := s.store
	v, e := st.GetLimitGroup(ctx, id)
	r := limitGroupResponse(v)
	if e == nil {
		cacheWrite(s, ctx, key, r)
	}
	return r, e
}

func (s *limitServiceImpl) DeleteLimitGroup(ctx context.Context, id int64, c ActorRequest) error {
	st := s.store
	group, e := st.GetLimitGroup(ctx, id)
	if e != nil {
		return e
	}
	e = st.DeleteLimitGroup(ctx, id, c.UpdatedBy)
	if e == nil {
		s.evict(ctx, s.itemKey(ctx, "limit-group", id))
		s.evictLimitConfig(ctx, group)
		s.invalidate(ctx, "resources")
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
	st := s.store
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
	st := s.store
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
	st := s.store
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
	st := s.store
	v, e := st.UpdateLimitValue(ctx, g, id, c)
	if e == nil {
		s.evict(ctx, s.itemKey(ctx, "limit-value", g, id))
		s.invalidate(ctx, fmt.Sprintf("limit-values:%d", g))
		s.evictGroupConfig(ctx, st, g)
	}
	return limitValueResponse(v), e
}

func (s *limitServiceImpl) DeleteLimitValue(ctx context.Context, g, id int64, c ActorRequest) error {
	st := s.store
	e := st.DeleteLimitValue(ctx, g, id, c.UpdatedBy)
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
	st := s.store
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
	st := s.store
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
	st := s.store
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
	st := s.store
	v, e := st.UpdateUserLimit(ctx, w, id, c)
	if e == nil {
		s.evict(ctx, s.itemKey(ctx, "user-limit", w, id))
		s.invalidate(ctx, fmt.Sprintf("user-limits:%s", w))
	}
	return userLimitResponse(v), e
}

func (s *limitServiceImpl) DeleteUserLimit(ctx context.Context, w string, id int64, c ActorRequest) error {
	st := s.store
	e := st.DeleteUserLimit(ctx, w, id, c.UpdatedBy)
	if e == nil {
		s.evict(ctx, s.itemKey(ctx, "user-limit", w, id))
		s.invalidate(ctx, fmt.Sprintf("user-limits:%s", w))
	}
	return e
}
