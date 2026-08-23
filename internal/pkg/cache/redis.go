package cache

import (
	"context"
	"time"

	"github.com/jeogram/messenger/internal/config"
	"github.com/redis/go-redis/v9"
)

// Redis wraps a go-redis client with helper methods used across modules.
type Redis struct {
	client *redis.Client
}

// NewRedis connects to Redis using the provided configuration.
func NewRedis(cfg config.RedisConfig) (*Redis, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &Redis{client: client}, nil
}

func (r *Redis) Client() *redis.Client { return r.client }

// Set stores a value with a TTL. Use 0 for no expiration.
func (r *Redis) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

// Get returns the value for a key, or ("", false) if missing.
func (r *Redis) Get(ctx context.Context, key string) (string, bool, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

// Del removes one or more keys.
func (r *Redis) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Exists reports whether a key is present.
func (r *Redis) Exists(ctx context.Context, key string) (bool, error) {
	n, err := r.client.Exists(ctx, key).Result()
	return n > 0, err
}

// Close closes the underlying connection.
func (r *Redis) Close() error { return r.client.Close() }
