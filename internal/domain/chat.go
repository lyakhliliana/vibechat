package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ChatType string

const (
	ChatTypeDirect ChatType = "direct"
	ChatTypeGroup  ChatType = "group"
)

type MemberRole string

const (
	MemberRoleOwner  MemberRole = "owner"
	MemberRoleAdmin  MemberRole = "admin"
	MemberRoleMember MemberRole = "member"
)

// Chat represents a conversation — either direct (2 users) or group.
type Chat struct {
	ID          uuid.UUID `json:"id"`
	Type        ChatType  `json:"type"`
	Name        string    `json:"name,omitempty"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	Description string    `json:"description,omitempty"`
	CreatedBy   uuid.UUID `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ChatMember struct {
	ChatID   uuid.UUID  `json:"chat_id"`
	UserID   uuid.UUID  `json:"user_id"`
	Role     MemberRole `json:"role"`
	JoinedAt time.Time  `json:"joined_at"`
	User     *User      `json:"user,omitempty"`
}

// ChatPreview is a read-model for the chat list: chat + last message + unread count.
type ChatPreview struct {
	Chat        *Chat         `json:"chat"`
	Members     []*ChatMember `json:"members,omitempty"`
	LastMessage *Message      `json:"last_message,omitempty"`
	UnreadCount int           `json:"unread_count"`
}

func (c *Chat) IsDirect() bool {
	return c.Type == ChatTypeDirect
}

func (r MemberRole) CanManageMembers() bool {
	return r == MemberRoleOwner || r == MemberRoleAdmin
}

type ChatRepository interface {
	// CreateWithMembers atomically creates a chat and its initial members.
	CreateWithMembers(ctx context.Context, chat *Chat, members []*ChatMember) error
	GetByID(ctx context.Context, id uuid.UUID) (*Chat, error)
	Update(ctx context.Context, chat *Chat) error
	Delete(ctx context.Context, id uuid.UUID) error

	AddMember(ctx context.Context, member *ChatMember) error
	RemoveMember(ctx context.Context, chatID, userID uuid.UUID) error
	GetMember(ctx context.Context, chatID, userID uuid.UUID) (*ChatMember, error)
	GetMembers(ctx context.Context, chatID uuid.UUID) ([]*ChatMember, error)
	UpdateMemberRole(ctx context.Context, chatID, userID uuid.UUID, role MemberRole) error

	GetUserChats(ctx context.Context, userID uuid.UUID) ([]*ChatPreview, error)
	GetDirectChat(ctx context.Context, user1ID, user2ID uuid.UUID) (*Chat, error)
	IsMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error)
}
