package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type requestIDKey struct{}

// RequestID propagates X-Request-ID from the incoming header or generates a fresh UUID.
// It also seeds a per-request zerolog logger on the context so downstream handlers and
// use-cases can call zerolog.Ctx(ctx) to get a logger pre-populated with request_id.
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = uuid.New().String()
			}
			w.Header().Set("X-Request-ID", id)

			logger := log.Logger.With().Str("request_id", id).Logger()
			ctx := logger.WithContext(r.Context())
			ctx = context.WithValue(ctx, requestIDKey{}, id)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetRequestID(r *http.Request) string {
	if id, ok := r.Context().Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}
