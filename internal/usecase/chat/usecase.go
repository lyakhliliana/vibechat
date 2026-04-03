package chat

import (
	"context"

	"github.com/google/uuid"

	"vibechat/internal/domain"
)

type UseCase interface {
	// CreateDirect opens a 1-to-1 chat, or returns the existing one.
	// The bool is true when a new chat was created, false when an existing one was returned.
	CreateDirect(ctx context.Context, callerID uuid.UUID, in CreateDirectInput) (*domain.Chat, bool, error)
	CreateGroup(ctx context.Context, callerID uuid.UUID, in CreateGroupInput) (*domain.Chat, error)
	GetChat(ctx context.Context, chatID, callerID uuid.UUID) (*domain.Chat, error)
	GetUserChats(ctx context.Context, callerID uuid.UUID) ([]*domain.ChatPreview, error)
	UpdateGroup(ctx context.Context, chatID, callerID uuid.UUID, in UpdateGroupInput) (*domain.Chat, error)
	AddMember(ctx context.Context, chatID, callerID uuid.UUID, in AddMemberInput) error
	// RemoveMember: owners/admins remove members; only owner can remove admins.
	RemoveMember(ctx context.Context, chatID, callerID, targetID uuid.UUID) error
	// ChangeMemberRole: owner only.
	ChangeMemberRole(ctx context.Context, chatID, callerID uuid.UUID, in ChangeMemberRoleInput) error
	// LeaveChat transfers ownership to the oldest admin/member when the owner leaves.
	LeaveChat(ctx context.Context, chatID, callerID uuid.UUID) error
	GetMembers(ctx context.Context, chatID, callerID uuid.UUID) ([]*domain.ChatMember, error)
}
