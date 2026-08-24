package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisLimiter(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Если Redis недоступен — пропускаем (инфраструктурный тест).
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}

	// Cleanup
	client.FlushDB(context.Background())

	limiter := NewRedisLimiter(client, 5, 1*time.Minute)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if !limiter.Allow(ctx, "test_user") {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	if limiter.Allow(ctx, "test_user") {
		t.Error("6th request should be rejected")
	}

	client.FlushDB(context.Background())
}
