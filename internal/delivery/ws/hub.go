package ws

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Hub manages WebSocket rooms (one room per chat). Safe for concurrent use.
// chatClients maps chatID → *sync.Map{ *Client → struct{} }.
type Hub struct {
	chatClients sync.Map
}

func NewHub() *Hub {
	return &Hub{}
}

func (h *Hub) Subscribe(client *Client, chatIDs []uuid.UUID) {
	for _, id := range chatIDs {
		h.room(id).Store(client, struct{}{})
	}
}

func (h *Hub) Unsubscribe(client *Client, chatIDs []uuid.UUID) {
	for _, id := range chatIDs {
		if room, ok := h.chatClients.Load(id); ok {
			room.(*sync.Map).Delete(client)
		}
	}
}

// BroadcastToChat fans out event to every client in the room, skipping excludeUserID (echo prevention).
func (h *Hub) BroadcastToChat(chatID uuid.UUID, event Event, excludeUserID uuid.UUID) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Error().Err(err).Msg("ws: marshal broadcast event")
		return
	}

	room, ok := h.chatClients.Load(chatID)
	if !ok {
		return
	}

	room.(*sync.Map).Range(func(key, _ any) bool {
		client := key.(*Client)
		if client.userID == excludeUserID {
			return true
		}
		select {
		case client.send <- data:
		default:
			// Client too slow — drop so one sluggish reader can't block the room.
			log.Warn().
				Str("client", client.userID.String()).
				Msg("ws: send buffer full, message dropped")
		}
		return true
	})
}

// Dispatch reads events and broadcasts them. Run in a dedicated goroutine.
func (h *Hub) Dispatch(ctx context.Context, events <-chan Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-events:
			if !ok {
				return
			}
			h.BroadcastToChat(e.ChatID, e, e.SenderID)
		}
	}
}

func (h *Hub) room(chatID uuid.UUID) *sync.Map {
	actual, _ := h.chatClients.LoadOrStore(chatID, &sync.Map{})
	return actual.(*sync.Map)
}
