package http

import (
	"context"
	"encoding/json"
	nethttp "net/http"

	"vibechat/internal/delivery/http/handler"
	mw "vibechat/internal/delivery/http/middleware"
)

// HealthFunc is called by the health endpoint to verify all dependencies.
// It should return a non-nil error if any dependency is unhealthy.
type HealthFunc func(ctx context.Context) error

// Deps groups all handler dependencies so the router has a single wiring point.
type Deps struct {
	User        *handler.UserHandler
	Chat        *handler.ChatHandler
	Message     *handler.MessageHandler
	JWT         mw.TokenValidator
	Health      HealthFunc                            // optional; nil means always healthy
	RateLimiter func(nethttp.Handler) nethttp.Handler // optional auth rate limiter
}

// SetupRoutes registers all API routes on the given ServeMux.
func SetupRoutes(mux *nethttp.ServeMux, d Deps) {
	protect := mw.Auth(d.JWT)

	h := func(pattern string, fn nethttp.HandlerFunc) {
		mux.Handle(pattern, protect(fn))
	}

	// Public auth routes with optional rate limiting.
	authRoute := func(pattern string, fn nethttp.HandlerFunc) {
		var hdlr nethttp.Handler = fn
		if d.RateLimiter != nil {
			hdlr = d.RateLimiter(hdlr)
		}
		mux.Handle(pattern, hdlr)
	}

	mux.HandleFunc("GET /health", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		w.Header().Set("Content-Type", "application/json")
		if d.Health != nil {
			if err := d.Health(r.Context()); err != nil {
				w.WriteHeader(nethttp.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy", "error": err.Error()})
				return
			}
		}
		w.WriteHeader(nethttp.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Auth (public, rate-limited)
	authRoute("POST /api/v1/auth/register", d.User.Register)
	authRoute("POST /api/v1/auth/login", d.User.Login)
	authRoute("POST /api/v1/auth/refresh", d.User.RefreshToken)

	// Users
	h("GET /api/v1/users/me", d.User.GetMe)
	h("PUT /api/v1/users/me", d.User.UpdateMe)
	h("GET /api/v1/users/search", d.User.Search)
	h("GET /api/v1/users/{user_id}", d.User.GetProfile)

	// Chats
	h("GET /api/v1/chats", d.Chat.GetUserChats)
	h("POST /api/v1/chats/direct", d.Chat.CreateDirect)
	h("POST /api/v1/chats/group", d.Chat.CreateGroup)
	h("GET /api/v1/chats/{chat_id}", d.Chat.GetChat)
	h("PUT /api/v1/chats/{chat_id}", d.Chat.UpdateGroup)

	// Members — "me" is a literal segment and takes precedence over {user_id}
	h("GET /api/v1/chats/{chat_id}/members", d.Chat.GetMembers)
	h("POST /api/v1/chats/{chat_id}/members", d.Chat.AddMember)
	h("DELETE /api/v1/chats/{chat_id}/members/me", d.Chat.LeaveChat)
	h("DELETE /api/v1/chats/{chat_id}/members/{user_id}", d.Chat.RemoveMember)
	h("PATCH /api/v1/chats/{chat_id}/members/{user_id}/role", d.Chat.ChangeMemberRole)

	// Messages
	h("POST /api/v1/chats/{chat_id}/messages", d.Message.Send)
	h("GET /api/v1/chats/{chat_id}/messages", d.Message.GetHistory)
	h("POST /api/v1/chats/{chat_id}/read", d.Message.MarkRead)
	h("PUT /api/v1/chats/{chat_id}/messages/{msg_id}", d.Message.Edit)
	h("DELETE /api/v1/chats/{chat_id}/messages/{msg_id}", d.Message.Delete)
	h("POST /api/v1/chats/{chat_id}/messages/{msg_id}/reactions", d.Message.AddReaction)
	h("DELETE /api/v1/chats/{chat_id}/messages/{msg_id}/reactions", d.Message.RemoveReaction)
}
