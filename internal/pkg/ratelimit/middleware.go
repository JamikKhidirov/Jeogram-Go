package ratelimit

import (
	"net/http"

	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/response"
)

type LimitConfig struct {
	Name   string
	Rate   int
	Window int // seconds
}

func NewRateLimitMiddleware(limiter Limiter, config LimitConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := middleware.UserID(r)
			if userID == "" {
				userID = r.RemoteAddr
			}

			key := userID + ":" + config.Name
			if !limiter.Allow(r.Context(), key) {
				w.Header().Set("Retry-After", "60")
				response.WriteError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "Too many requests")
				return
			}

			w.Header().Set("X-RateLimit-Limit", string(rune(config.Rate)))
			next.ServeHTTP(w, r)
		})
	}
}

// Predefined limits
var (
	AuthLimit = LimitConfig{Name: "auth", Rate: 5, Window: 60}        // 5 per minute
	MessageLimit = LimitConfig{Name: "messages", Rate: 30, Window: 60} // 30 per minute
	SearchLimit = LimitConfig{Name: "search", Rate: 10, Window: 60}    // 10 per minute
	BlockLimit = LimitConfig{Name: "block", Rate: 20, Window: 60}      // 20 per minute
)
