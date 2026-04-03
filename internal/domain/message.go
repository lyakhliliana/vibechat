package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type MessageType string

const (
	MessageTypeText  MessageType = "text"
	MessageTypeImage MessageType = "image"
	MessageTypeFile  MessageType = "file"
)

type Message struct {
	ID        uuid.UUID   `json:"id"`
	ChatID    uuid.UUID   `json:"chat_id"`
	SenderID  uuid.UUID   `json:"sender_id"`
	Content   string      `json:"content"`
	Type      MessageType `json:"type"`
	ReplyToID *uuid.UUID  `json:"reply_to_id,omitempty"`
	EditedAt  *time.Time  `json:"edited_at,omitempty"`
	DeletedAt *time.Time  `json:"deleted_at,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`

	Sender    *User       `json:"sender,omitempty"`
	Reactions []*Reaction `json:"reactions,omitempty"`
	ReplyTo   *Message    `json:"reply_to,omitempty"`
}

type Reaction struct {
	MessageID uuid.UUID `json:"message_id"`
	UserID    uuid.UUID `json:"user_id"`
	Emoji     string    `json:"emoji"`
	CreatedAt time.Time `json:"created_at"`
	User      *User     `json:"user,omitempty"`
}

func (m *Message) IsDeleted() bool {
	return m.DeletedAt != nil
}

func (m *Message) IsAuthoredBy(userID uuid.UUID) bool {
	return m.SenderID == userID
}

// MessageCursor enables stable keyset pagination via (CreatedAt, ID) of the oldest visible message.
type MessageCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

type MessageRepository interface {
	Create(ctx context.Context, msg *Message) error
	GetByID(ctx context.Context, id uuid.UUID) (*Message, error)
	// GetChatMessages returns up to limit messages older than cursor (nil = from latest).
	GetChatMessages(ctx context.Context, chatID uuid.UUID, cursor *MessageCursor, limit int) ([]*Message, error)
	Update(ctx context.Context, msg *Message) error
	Delete(ctx context.Context, id uuid.UUID) error // soft-delete

	// Read receipts — per-chat, per-user last-read timestamp.
	MarkRead(ctx context.Context, chatID, userID uuid.UUID) error
	GetUnreadCount(ctx context.Context, chatID, userID uuid.UUID) (int, error)

	AddReaction(ctx context.Context, reaction *Reaction) error
	RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error
	GetReactions(ctx context.Context, messageID uuid.UUID) ([]*Reaction, error)
	// GetReactionsBatch returns reactions for multiple messages keyed by message ID.
	// Used to avoid N+1 when loading a page of messages.
	GetReactionsBatch(ctx context.Context, messageIDs []uuid.UUID) (map[uuid.UUID][]*Reaction, error)
}
