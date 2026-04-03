package ws

import (
	"encoding/json"

	"github.com/google/uuid"
)

// Event is the Hub routing envelope. SenderID and ChatID are server-side only
// (json:"-") — clients receive only Type and Payload.
type Event struct {
	Type     string          `json:"type"`
	Payload  json.RawMessage `json:"payload"`
	SenderID uuid.UUID       `json:"-"`
	ChatID   uuid.UUID       `json:"-"`
}

// clientEvent is the client-to-server envelope.
// Clients must set chat_id so the server can route and validate membership.
type clientEvent struct {
	Type    string          `json:"type"`
	ChatID  uuid.UUID       `json:"chat_id"`
	Payload json.RawMessage `json:"payload"`
}
