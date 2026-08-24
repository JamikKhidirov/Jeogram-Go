package ratelimit

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// MemoryLimiter — in-memory реализация фиксированного окна (без внешних зависимостей).
// Используется, когда Redis недоступен (REDIS_ENABLED=false), чтобы rate limiting
// продолжал работать на одном инстансе.
type MemoryLimiter struct {
	mu      sync.Mutex
	hits    map[string]int
	resetAt map[string]time.Time
	rate    int
	window  time.Duration
}

// NewMemoryLimiter создаёт in-memory лимитер: rate запросов на window.
func NewMemoryLimiter(rate int, window time.Duration) *MemoryLimiter {
	if rate <= 0 {
		rate = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &MemoryLimiter{
		hits:    make(map[string]int),
		resetAt: make(map[string]time.Time),
		rate:    rate,
		window:  window,
	}
}

// Allow возвращает true, если запрос в рамках лимита.
func (l *MemoryLimiter) Allow(_ context.Context, key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if t, ok := l.resetAt[key]; !ok || now.After(t) {
		l.hits[key] = 0
		l.resetAt[key] = now.Add(l.window)
	}

	l.hits[key]++
	return l.hits[key] <= l.rate
}

// Reset сбрасывает счётчик для ключа.
func (l *MemoryLimiter) Reset(_ context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.hits, key)
	delete(l.resetAt, key)
	return nil
}

// NewLimiter возвращает Redis-лимитер при наличии клиента, иначе in-memory.
func NewLimiter(client *redis.Client, rate int, window time.Duration) Limiter {
	if client != nil {
		return NewRedisLimiter(client, rate, window)
	}
	return NewMemoryLimiter(rate, window)
}
