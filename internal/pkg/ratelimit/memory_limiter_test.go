package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestMemoryLimiter(t *testing.T) {
	l := NewMemoryLimiter(3, time.Minute)
	ctx := context.Background()
	key := "u1"

	for i := 0; i < 3; i++ {
		if !l.Allow(ctx, key) {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if l.Allow(ctx, key) {
		t.Fatal("4th request should be rejected")
	}

	if err := l.Reset(ctx, key); err != nil {
		t.Fatalf("reset error: %v", err)
	}
	if !l.Allow(ctx, key) {
		t.Fatal("after reset request should be allowed again")
	}
}
