package ws

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	mw "vibechat/internal/delivery/http/middleware"
	"vibechat/internal/usecase/chat"
	"vibechat/internal/usecase/user"
)

type Handler struct {
	hub      *Hub
	chatUC   chat.UseCase
	userUC   user.UseCase
	cfg      Config
	upgrader *websocket.Upgrader
}

func NewHandler(hub *Hub, chatUC chat.UseCase, userUC user.UseCase, allowedOrigins []string, cfg Config) *Handler {
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[o] = struct{}{}
	}

	checkOrigin := func(r *http.Request) bool {
		if len(originSet) == 0 {
			return true // dev: allow all
		}
		_, ok := originSet[r.Header.Get("Origin")]
		return ok
	}

	return &Handler{
		hub:    hub,
		chatUC: chatUC,
		userUC: userUC,
		cfg:    cfg,
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     checkOrigin,
		},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID := mw.UserID(r)
	logger := zerolog.Ctx(r.Context())

	previews, err := h.chatUC.GetUserChats(r.Context(), userID)
	if err != nil {
		logger.Error().Err(err).Str("user_id", userID.String()).Msg("ws: load user chats failed")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	chatIDs := make([]uuid.UUID, 0, len(previews))
	for _, p := range previews {
		chatIDs = append(chatIDs, p.Chat.ID)
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("ws: upgrade failed")
		return
	}

	if err := h.userUC.SetOnline(r.Context(), userID); err != nil {
		logger.Warn().Err(err).Str("user_id", userID.String()).Msg("ws: set online failed")
	}
	// r.Context() is cancelled when the WS connection closes, so SetOffline needs its own context.
	// Capture the logger now while the request context is still live.
	offlineLog := logger.With().Str("user_id", userID.String()).Logger()
	defer func() {
		if err := h.userUC.SetOffline(context.Background(), userID); err != nil {
			offlineLog.Warn().Err(err).Msg("ws: set offline failed")
		}
	}()

	logger.Info().
		Str("user_id", userID.String()).
		Int("rooms", len(chatIDs)).
		Msg("ws: client connected")

	client := NewClient(h.hub, conn, userID, chatIDs, h.cfg)
	client.Run()

	logger.Info().Str("user_id", userID.String()).Msg("ws: client disconnected")
}
