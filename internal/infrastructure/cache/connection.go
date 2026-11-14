package redis

import (
	"context"
	"time"

	redis "github.com/redis/go-redis/v9"
	cacheport "github.com/zeann3th/policy-service/internal/core/port/cache"
	"github.com/zeann3th/policy-service/internal/infrastructure/config"
)

type redisCache struct {
	client redis.UniversalClient
}

var _ cacheport.Cache = (*redisCache)(nil)

func NewRedisClient(cfg *config.Config) redis.UniversalClient {
	return redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:      cfg.RedisConfig.Addrs,
		Username:   cfg.RedisConfig.Username,
		Password:   cfg.RedisConfig.Password,
		DB:         cfg.RedisConfig.DB,
		MasterName: cfg.RedisConfig.MasterName,
	})
}

func New(client redis.UniversalClient) cacheport.Cache {
	return &redisCache{client: client}
}

func (c *redisCache) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

func (c *redisCache) Set(ctx context.Context, key string, value string) error {
	return c.client.Set(ctx, key, value, 0).Err()
}

func (c *redisCache) SetWithTTL(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *redisCache) TrySet(ctx context.Context, key string, value string) (bool, error) {
	return c.client.SetNX(ctx, key, value, 0).Result()
}

func (c *redisCache) TrySetWithTTL(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return c.client.SetNX(ctx, key, value, ttl).Result()
}

func (c *redisCache) Evict(ctx context.Context, key string) (bool, error) {
	deleted, err := c.client.Del(ctx, key).Result()
	return deleted > 0, err
}

func (c *redisCache) EvictAll(ctx context.Context) error {
	return c.client.FlushDB(ctx).Err()
}
