package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	ServiceName      string
	Environment      string
	HttpConfig       HttpConfig
	GrpcConfig       GrpcConfig
	DatabaseConfig   DatabaseConfig
	LocalCacheConfig LocalCacheConfig
	RedisConfig      RedisConfig
}

type HttpConfig struct {
	Port    string
	GinMode string
}

type GrpcConfig struct {
	Port string
}

type DatabaseConfig struct {
	URL string
}

type LocalCacheConfig struct {
	Capacity uint32
}

type RedisConfig struct {
	Addrs      []string
	Username   string
	Password   string
	DB         int
	MasterName string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	dbURL, err := getOrError("DATABASE_URL")
	if err != nil {
		return nil, err
	}

	serviceName, err := getOrError("SERVICE_NAME")
	if err != nil {
		return nil, err
	}

	redisAddrs, err := getListOrDefault("REDIS_ADDRS", []string{"localhost:6379"})
	if err != nil {
		return nil, err
	}

	redisDB, err := getIntOrDefault("REDIS_DB", 0)
	if err != nil {
		return nil, err
	}

	return &Config{
		ServiceName: serviceName,
		Environment: getOrDefault("ENVIRONMENT", "release"),
		HttpConfig: HttpConfig{
			Port:    getOrDefault("HTTP_PORT", "8080"),
			GinMode: getOrDefault("GIN_MODE", "release"),
		},
		GrpcConfig: GrpcConfig{
			Port: getOrDefault("GRPC_PORT", "8081"),
		},
		DatabaseConfig: DatabaseConfig{
			URL: dbURL,
		},
		RedisConfig: RedisConfig{
			Addrs:      redisAddrs,
			Username:   getOrDefault("REDIS_USERNAME", ""),
			Password:   getOrDefault("REDIS_PASSWORD", ""),
			DB:         redisDB,
			MasterName: getOrDefault("REDIS_MASTER_NAME", ""),
		},
	}, nil
}

func getOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getOrError(key string) (string, error) {
	if value := os.Getenv(key); value != "" {
		return value, nil
	}
	return "", fmt.Errorf("missing required environment variable: %s", key)
}

func getIntOrDefault(key string, defaultValue int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("malformed %s: %w", key, err)
	}
	return parsed, nil
}

func getUint32OrDefault(key string, defaultValue uint32) (uint32, error) {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("malformed %s: %w", key, err)
	}
	return uint32(parsed), nil
}

func getListOrDefault(key string, defaultValue []string) ([]string, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue, nil
	}

	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			return nil, fmt.Errorf("malformed %s: contains empty entry", key)
		}
		result = append(result, trimmed)
	}

	return result, nil
}
