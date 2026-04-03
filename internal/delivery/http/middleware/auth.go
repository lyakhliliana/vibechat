package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"vibechat/internal/delivery/http/response"
)

type contextKey int

const ctxKeyUserID contextKey = iota

// TokenValidator is the interface owned by the auth middleware (defined at point of consumption).
type TokenValidator interface {
	ValidateAccessToken(token string) (uuid.UUID, error)
}

func Auth(tv TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				response.JSON(w, http.StatusUnauthorized, response.Body{Error: "unauthorized"})
				return
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				response.JSON(w, http.StatusUnauthorized, response.Body{Error: "unauthorized"})
				return
			}

			userID, err := tv.ValidateAccessToken(parts[1])
			if err != nil {
				zerolog.Ctx(r.Context()).Debug().Err(err).Msg("auth: invalid token")
				response.JSON(w, http.StatusUnauthorized, response.Body{Error: "unauthorized"})
				return
			}

			// Enrich the per-request logger (seeded by RequestID middleware) with user_id.
			logger := zerolog.Ctx(r.Context()).With().Str("user_id", userID.String()).Logger()
			ctx := logger.WithContext(r.Context())
			ctx = context.WithValue(ctx, ctxKeyUserID, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserID extracts the caller's UUID from context. Panics outside Auth-protected routes.
func UserID(r *http.Request) uuid.UUID {
	return r.Context().Value(ctxKeyUserID).(uuid.UUID)
}
