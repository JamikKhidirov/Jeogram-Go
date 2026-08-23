package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/response"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog/log"
)

type ctxKey string

const (
	UserIDKey ctxKey = "user_id"
	EmailKey  ctxKey = "email"
)

var httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "jeogram",
	Name:      "http_requests_total",
	Help:      "Total HTTP requests by method, path and status.",
}, []string{"method", "path", "status"})

var httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "jeogram",
	Name:      "http_request_duration_seconds",
	Help:      "HTTP request latency in seconds.",
	Buckets:   prometheus.DefBuckets,
}, []string{"method", "path"})

// JWTAuth validates the bearer token and injects the user identity into context.
func JWTAuth(j *auth.JWT) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authorization header")
				return
			}
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				response.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid authorization format")
				return
			}
			claims, err := j.ParseAccess(parts[1])
			if err != nil {
				response.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, EmailKey, claims.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserID returns the authenticated user id from context (empty if absent).
func UserID(r *http.Request) string {
	if v, ok := r.Context().Value(UserIDKey).(string); ok {
		return v
	}
	return ""
}

// Logger logs each request with method, path, status and duration.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		dur := time.Since(start)
		log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", sw.status).
			Dur("duration", dur).
			Str("remote", r.RemoteAddr).
			Msg("http request")
	})
}

// Metrics records Prometheus metrics for each request.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		dur := time.Since(start)
		httpRequests.WithLabelValues(r.Method, r.URL.Path, http.StatusText(sw.status)).Inc()
		httpDuration.WithLabelValues(r.Method, r.URL.Path).Observe(dur.Seconds())
	})
}

// Recover converts panics into 500 responses.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error().Interface("panic", rec).Str("path", r.URL.Path).Msg("panic recovered")
				response.WriteError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
