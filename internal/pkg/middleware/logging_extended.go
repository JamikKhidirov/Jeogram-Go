package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		method := r.Method
		path := r.RequestURI
		userID := UserID(r)

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Milliseconds()
		logger := log.Info()

		if wrapped.statusCode >= 500 {
			logger = log.Error()
		} else if wrapped.statusCode >= 400 {
			logger = log.Warn()
		}

		logger.
			Str("method", method).
			Str("path", path).
			Str("user_id", userID).
			Int("status", wrapped.statusCode).
			Int64("duration_ms", duration).
			Msg("request completed")
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func UserID(r *http.Request) string {
	userID := r.Context().Value("user_id")
	if uid, ok := userID.(string); ok {
		return uid
	}
	return ""
}

func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, "user_id", userID)
}
