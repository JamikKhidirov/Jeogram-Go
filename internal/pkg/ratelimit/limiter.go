package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter interface {
	Allow(ctx context.Context, key string) bool
	Reset(ctx context.Context, key string) error
}

type RedisLimiter struct {
	client *redis.Client
	rate   int           // requests
	window time.Duration // time window
}

func NewRedisLimiter(client *redis.Client, rate int, window time.Duration) *RedisLimiter {
	return &RedisLimiter{
		client: client,
		rate:   rate,
		window: window,
	}
}

func (l *RedisLimiter) Allow(ctx context.Context, key string) bool {
	redisKey := fmt.Sprintf("ratelimit:%s", key)
	current, err := l.client.Incr(ctx, redisKey).Result()
	if err != nil {
		return true // fail open
	}

	if current == 1 {
		_ = l.client.Expire(ctx, redisKey, l.window).Err()
	}

	return current <= int64(l.rate)
}

func (l *RedisLimiter) Reset(ctx context.Context, key string) error {
	redisKey := fmt.Sprintf("ratelimit:%s", key)
	return l.client.Del(ctx, redisKey).Err()
}
