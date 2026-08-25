package middleware

import (
	"bufio"
	"context"
	"errors"
	"net"
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
			var token string
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					token = parts[1]
				}
			}
			// Для WebSocket/Socket.IO токен также передаётся в query (?token=),
			// т.к. браузерный WebSocket не может задать заголовок Authorization.
			if token == "" {
				token = r.URL.Query().Get("token")
			}
			if token == "" {
				token = r.URL.Query().Get("access_token")
			}
			if token == "" {
				response.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authorization token")
				return
			}
			claims, err := j.ParseAccess(token)
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

// RequireAdmin allows the request to proceed only if the authenticated user is
// in the provided admin ID list. A list containing "*" grants admin access to
// any authenticated user (intended for personal/self-hosted deployments; set a
// concrete user ID in production). Use it to guard sensitive admin routes.
func RequireAdmin(ids []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id != "" {
			allowed[id] = true
		}
	}
	wildcard := allowed["*"]
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uid := UserID(r)
			if uid == "" || (!wildcard && !allowed[uid]) {
				response.WriteError(w, http.StatusForbidden, "forbidden", "admin access required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
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

// Hijack делегирует вызов нижележащему writer, чтобы поддерживать WebSocket-апгрейды.
func (s *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := s.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, errors.New("response writer does not support hijacking")
}

// Flush делегирует вызов нижележащему writer.
func (s *statusWriter) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Push делегирует вызов нижележащему writer.
func (s *statusWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := s.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}
