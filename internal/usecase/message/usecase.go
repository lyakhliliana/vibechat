package message

import (
	"context"

	"github.com/google/uuid"

	"vibechat/internal/domain"
)

type UseCase interface {
	Send(ctx context.Context, senderID uuid.UUID, in SendInput) (*domain.Message, error)
	GetHistory(ctx context.Context, callerID uuid.UUID, in ListInput) (*PageResult, error)
	Edit(ctx context.Context, messageID, callerID uuid.UUID, in EditInput) (*domain.Message, error)
	Delete(ctx context.Context, messageID, callerID uuid.UUID) error
	MarkRead(ctx context.Context, chatID, callerID uuid.UUID) error
	AddReaction(ctx context.Context, messageID, callerID uuid.UUID, in ReactInput) error
	RemoveReaction(ctx context.Context, messageID, callerID uuid.UUID, in ReactInput) error
}
