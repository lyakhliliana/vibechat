package message

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"vibechat/internal/domain"
)

type service struct {
	messages domain.MessageRepository
	chats    domain.ChatRepository
}

func New(messages domain.MessageRepository, chats domain.ChatRepository) UseCase {
	return &service{messages: messages, chats: chats}
}

func (s *service) Send(ctx context.Context, senderID uuid.UUID, in SendInput) (*domain.Message, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	if err := s.assertMember(ctx, in.ChatID, senderID); err != nil {
		return nil, err
	}

	if in.ReplyToID != nil {
		reply, err := s.messages.GetByID(ctx, *in.ReplyToID)
		if err != nil {
			return nil, err // ErrMessageNotFound → 404
		}
		if reply.ChatID != in.ChatID {
			return nil, domain.ErrMessageNotFound
		}
	}

	now := time.Now().UTC()
	msg := &domain.Message{
		ID:        uuid.New(),
		ChatID:    in.ChatID,
		SenderID:  senderID,
		Content:   in.Content,
		Type:      in.Type,
		ReplyToID: in.ReplyToID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.messages.Create(ctx, msg); err != nil {
		return nil, err
	}

	zerolog.Ctx(ctx).Debug().
		Str("message_id", msg.ID.String()).
		Str("chat_id", in.ChatID.String()).
		Str("sender_id", senderID.String()).
		Str("type", string(in.Type)).
		Msg("message sent")

	return msg, nil
}

func (s *service) Edit(ctx context.Context, messageID, callerID uuid.UUID, in EditInput) (*domain.Message, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	msg, err := s.messages.GetByID(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if msg.IsDeleted() {
		return nil, domain.ErrMessageDeleted
	}
	if !msg.IsAuthoredBy(callerID) {
		return nil, domain.ErrNotMessageAuthor
	}
	if err = s.assertMember(ctx, msg.ChatID, callerID); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	msg.Content = in.Content
	msg.EditedAt = &now
	msg.UpdatedAt = now

	if err = s.messages.Update(ctx, msg); err != nil {
		return nil, err
	}

	zerolog.Ctx(ctx).Debug().
		Str("message_id", messageID.String()).
		Str("chat_id", msg.ChatID.String()).
		Str("caller_id", callerID.String()).
		Msg("message edited")

	return msg, nil
}

func (s *service) Delete(ctx context.Context, messageID, callerID uuid.UUID) error {
	msg, err := s.messages.GetByID(ctx, messageID)
	if err != nil {
		return err
	}
	if msg.IsDeleted() {
		return domain.ErrMessageDeleted
	}
	if !msg.IsAuthoredBy(callerID) {
		return domain.ErrNotMessageAuthor
	}
	if err = s.assertMember(ctx, msg.ChatID, callerID); err != nil {
		return err
	}

	if err = s.messages.Delete(ctx, messageID); err != nil {
		return err
	}

	zerolog.Ctx(ctx).Debug().
		Str("message_id", messageID.String()).
		Str("chat_id", msg.ChatID.String()).
		Str("caller_id", callerID.String()).
		Msg("message deleted")

	return nil
}

func (s *service) MarkRead(ctx context.Context, chatID, callerID uuid.UUID) error {
	if err := s.assertMember(ctx, chatID, callerID); err != nil {
		return err
	}

	if err := s.messages.MarkRead(ctx, chatID, callerID); err != nil {
		return err
	}

	zerolog.Ctx(ctx).Trace().
		Str("chat_id", chatID.String()).
		Str("caller_id", callerID.String()).
		Msg("chat marked as read")

	return nil
}

func (s *service) GetHistory(ctx context.Context, callerID uuid.UUID, in ListInput) (*PageResult, error) {
	if err := s.assertMember(ctx, in.ChatID, callerID); err != nil {
		return nil, err
	}

	msgs, err := s.messages.GetChatMessages(ctx, in.ChatID, in.Cursor, in.Limit)
	if err != nil {
		return nil, err
	}

	if len(msgs) > 0 {
		ids := make([]uuid.UUID, len(msgs))
		for i, m := range msgs {
			ids[i] = m.ID
		}
		reactionsByMsg, err := s.messages.GetReactionsBatch(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, m := range msgs {
			m.Reactions = reactionsByMsg[m.ID]
		}
	}

	zerolog.Ctx(ctx).Trace().
		Str("chat_id", in.ChatID.String()).
		Int("count", len(msgs)).
		Msg("history fetched")

	return buildPageResult(msgs, in.Limit), nil
}

func (s *service) AddReaction(ctx context.Context, messageID, callerID uuid.UUID, in ReactInput) error {
	if err := in.Validate(); err != nil {
		return err
	}
	msg, err := s.messages.GetByID(ctx, messageID)
	if err != nil {
		return err
	}
	if msg.IsDeleted() {
		return domain.ErrMessageDeleted
	}
	if err = s.assertMember(ctx, msg.ChatID, callerID); err != nil {
		return err
	}

	if err = s.messages.AddReaction(ctx, &domain.Reaction{
		MessageID: messageID,
		UserID:    callerID,
		Emoji:     in.Emoji,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return err
	}

	zerolog.Ctx(ctx).Debug().
		Str("message_id", messageID.String()).
		Str("caller_id", callerID.String()).
		Str("emoji", in.Emoji).
		Msg("reaction added")

	return nil
}

func (s *service) RemoveReaction(ctx context.Context, messageID, callerID uuid.UUID, in ReactInput) error {
	if err := in.Validate(); err != nil {
		return err
	}
	msg, err := s.messages.GetByID(ctx, messageID)
	if err != nil {
		return err
	}
	if msg.IsDeleted() {
		return domain.ErrMessageDeleted
	}
	if err = s.assertMember(ctx, msg.ChatID, callerID); err != nil {
		return err
	}

	if err = s.messages.RemoveReaction(ctx, messageID, callerID, in.Emoji); err != nil {
		return err
	}

	zerolog.Ctx(ctx).Debug().
		Str("message_id", messageID.String()).
		Str("caller_id", callerID.String()).
		Str("emoji", in.Emoji).
		Msg("reaction removed")

	return nil
}

func (s *service) assertMember(ctx context.Context, chatID, userID uuid.UUID) error {
	ok, err := s.chats.IsMember(ctx, chatID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrNotChatMember
	}
	return nil
}
