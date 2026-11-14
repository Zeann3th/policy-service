package redis

import (
	redis "github.com/redis/go-redis/v9"
	"github.com/zeann3th/haphaestus/internal/infrastructure/config"
)

func NewRedisClient(cfg *config.Config) redis.UniversalClient {
	return redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:      cfg.RedisConfig.Addrs,
		Username:   cfg.RedisConfig.Username,
		Password:   cfg.RedisConfig.Password,
		DB:         cfg.RedisConfig.DB,
		MasterName: cfg.RedisConfig.MasterName,
	})
}
