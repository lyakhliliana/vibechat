package message

import (
	"github.com/google/uuid"

	"vibechat/internal/domain"
	"vibechat/utils/validate"
)

type SendInput struct {
	ChatID    uuid.UUID          `json:"chat_id"`
	Content   string             `json:"content"`
	Type      domain.MessageType `json:"type"`
	ReplyToID *uuid.UUID         `json:"reply_to_id,omitempty"`
}

func (in SendInput) Validate() error {
	return validate.New().
		Check(in.ChatID != uuid.Nil, "chat_id is required").
		Check(validate.MinLen(in.Content, 1), "content must not be empty").
		Check(validate.MaxLen(in.Content, 4096), "content must be at most 4096 characters").
		Check(validate.OneOf(string(in.Type), "text", "image", "file"), "type must be one of: text, image, file").
		Err()
}

type EditInput struct {
	Content string `json:"content"`
}

func (in EditInput) Validate() error {
	return validate.New().
		Check(validate.MinLen(in.Content, 1), "content must not be empty").
		Check(validate.MaxLen(in.Content, 4096), "content must be at most 4096 characters").
		Err()
}

type ListInput struct {
	ChatID uuid.UUID
	Cursor *domain.MessageCursor // nil = start from the newest
	Limit  int
}

type ReactInput struct {
	Emoji string `json:"emoji"`
}

func (in ReactInput) Validate() error {
	return validate.New().
		Check(validate.MinLen(in.Emoji, 1), "emoji is required").
		Check(validate.MaxLen(in.Emoji, 10), "emoji must be at most 10 characters").
		Err()
}

type PageResult struct {
	Messages   []*domain.Message     `json:"messages"`
	NextCursor *domain.MessageCursor `json:"next_cursor,omitempty"`
}

func nextCursor(msgs []*domain.Message, limit int) *domain.MessageCursor {
	if len(msgs) < limit {
		return nil
	}
	last := msgs[len(msgs)-1]
	return &domain.MessageCursor{CreatedAt: last.CreatedAt, ID: last.ID}
}

func buildPageResult(msgs []*domain.Message, limit int) *PageResult {
	return &PageResult{
		Messages:   msgs,
		NextCursor: nextCursor(msgs, limit),
	}
}
