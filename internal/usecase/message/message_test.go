package message

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"vibechat/internal/domain"
)

// --- stubs ---

type stubMessageRepo struct {
	messages  map[uuid.UUID]*domain.Message
	reactions map[uuid.UUID][]*domain.Reaction // messageID → reactions
	readMarks map[string]struct{}              // "chatID:userID"
}

func newStubMessageRepo() *stubMessageRepo {
	return &stubMessageRepo{
		messages:  make(map[uuid.UUID]*domain.Message),
		reactions: make(map[uuid.UUID][]*domain.Reaction),
		readMarks: make(map[string]struct{}),
	}
}

func (r *stubMessageRepo) Create(_ context.Context, m *domain.Message) error {
	r.messages[m.ID] = m
	return nil
}

func (r *stubMessageRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Message, error) {
	m, ok := r.messages[id]
	if !ok {
		return nil, domain.ErrMessageNotFound
	}
	return m, nil
}

func (r *stubMessageRepo) GetChatMessages(_ context.Context, chatID uuid.UUID, _ *domain.MessageCursor, limit int) ([]*domain.Message, error) {
	var out []*domain.Message
	for _, m := range r.messages {
		if m.ChatID == chatID && m.DeletedAt == nil {
			out = append(out, m)
		}
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (r *stubMessageRepo) Update(_ context.Context, m *domain.Message) error {
	r.messages[m.ID] = m
	return nil
}

func (r *stubMessageRepo) Delete(_ context.Context, id uuid.UUID) error {
	m, ok := r.messages[id]
	if !ok {
		return domain.ErrMessageNotFound
	}
	now := time.Now()
	m.DeletedAt = &now
	return nil
}

func (r *stubMessageRepo) MarkRead(_ context.Context, chatID, userID uuid.UUID) error {
	r.readMarks[chatID.String()+":"+userID.String()] = struct{}{}
	return nil
}

func (r *stubMessageRepo) GetUnreadCount(_ context.Context, _, _ uuid.UUID) (int, error) {
	return 0, nil
}

func (r *stubMessageRepo) AddReaction(_ context.Context, rx *domain.Reaction) error {
	r.reactions[rx.MessageID] = append(r.reactions[rx.MessageID], rx)
	return nil
}

func (r *stubMessageRepo) RemoveReaction(_ context.Context, msgID, userID uuid.UUID, emoji string) error {
	rxs := r.reactions[msgID]
	out := rxs[:0]
	for _, rx := range rxs {
		if !(rx.UserID == userID && rx.Emoji == emoji) {
			out = append(out, rx)
		}
	}
	r.reactions[msgID] = out
	return nil
}

func (r *stubMessageRepo) GetReactions(_ context.Context, msgID uuid.UUID) ([]*domain.Reaction, error) {
	return r.reactions[msgID], nil
}

func (r *stubMessageRepo) GetReactionsBatch(_ context.Context, ids []uuid.UUID) (map[uuid.UUID][]*domain.Reaction, error) {
	out := make(map[uuid.UUID][]*domain.Reaction, len(ids))
	for _, id := range ids {
		out[id] = r.reactions[id]
	}
	return out, nil
}

// minimal chat repo — only IsMember is used by message use case
type stubChatRepo struct {
	members map[string]struct{} // "chatID:userID"
}

func newStubChatRepo(pairs ...string) *stubChatRepo {
	r := &stubChatRepo{members: make(map[string]struct{})}
	for _, p := range pairs {
		r.members[p] = struct{}{}
	}
	return r
}

func memberKey(chatID, userID uuid.UUID) string {
	return chatID.String() + ":" + userID.String()
}

func (r *stubChatRepo) IsMember(_ context.Context, chatID, userID uuid.UUID) (bool, error) {
	_, ok := r.members[memberKey(chatID, userID)]
	return ok, nil
}

// implement the rest as no-ops to satisfy domain.ChatRepository
func (r *stubChatRepo) CreateWithMembers(_ context.Context, _ *domain.Chat, _ []*domain.ChatMember) error {
	return nil
}
func (r *stubChatRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.Chat, error) {
	return nil, domain.ErrChatNotFound
}
func (r *stubChatRepo) Update(_ context.Context, _ *domain.Chat) error          { return nil }
func (r *stubChatRepo) Delete(_ context.Context, _ uuid.UUID) error             { return nil }
func (r *stubChatRepo) AddMember(_ context.Context, _ *domain.ChatMember) error { return nil }
func (r *stubChatRepo) RemoveMember(_ context.Context, _, _ uuid.UUID) error    { return nil }
func (r *stubChatRepo) GetMember(_ context.Context, _, _ uuid.UUID) (*domain.ChatMember, error) {
	return nil, domain.ErrNotChatMember
}
func (r *stubChatRepo) GetMembers(_ context.Context, _ uuid.UUID) ([]*domain.ChatMember, error) {
	return nil, nil
}
func (r *stubChatRepo) UpdateMemberRole(_ context.Context, _, _ uuid.UUID, _ domain.MemberRole) error {
	return nil
}
func (r *stubChatRepo) GetUserChats(_ context.Context, _ uuid.UUID) ([]*domain.ChatPreview, error) {
	return nil, nil
}
func (r *stubChatRepo) GetDirectChat(_ context.Context, _, _ uuid.UUID) (*domain.Chat, error) {
	return nil, domain.ErrChatNotFound
}

// helpers

func newSvc(msgs *stubMessageRepo, chats *stubChatRepo) UseCase {
	return New(msgs, chats)
}

func asMember(chatID, userID uuid.UUID) *stubChatRepo {
	return newStubChatRepo(memberKey(chatID, userID))
}

// --- tests ---

func TestSend(t *testing.T) {
	chatID := uuid.New()
	senderID := uuid.New()

	t.Run("creates message", func(t *testing.T) {
		msgs := newStubMessageRepo()
		svc := newSvc(msgs, asMember(chatID, senderID))

		msg, err := svc.Send(context.Background(), senderID, SendInput{
			ChatID:  chatID,
			Content: "hello",
			Type:    domain.MessageTypeText,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if msg.ChatID != chatID || msg.SenderID != senderID {
			t.Fatal("message fields mismatch")
		}
		if _, ok := msgs.messages[msg.ID]; !ok {
			t.Fatal("message not stored in repo")
		}
	})

	t.Run("non-member cannot send", func(t *testing.T) {
		svc := newSvc(newStubMessageRepo(), newStubChatRepo())

		_, err := svc.Send(context.Background(), senderID, SendInput{
			ChatID:  chatID,
			Content: "hello",
			Type:    domain.MessageTypeText,
		})
		if !errors.Is(err, domain.ErrNotChatMember) {
			t.Fatalf("want ErrNotChatMember, got %v", err)
		}
	})

	t.Run("empty content fails validation", func(t *testing.T) {
		svc := newSvc(newStubMessageRepo(), asMember(chatID, senderID))

		_, err := svc.Send(context.Background(), senderID, SendInput{
			ChatID:  chatID,
			Content: "",
			Type:    domain.MessageTypeText,
		})
		if !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("want ErrValidation, got %v", err)
		}
	})

	t.Run("invalid type fails validation", func(t *testing.T) {
		svc := newSvc(newStubMessageRepo(), asMember(chatID, senderID))

		_, err := svc.Send(context.Background(), senderID, SendInput{
			ChatID:  chatID,
			Content: "hello",
			Type:    "audio",
		})
		if !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("want ErrValidation, got %v", err)
		}
	})

	t.Run("reply to non-existent message fails", func(t *testing.T) {
		msgs := newStubMessageRepo()
		svc := newSvc(msgs, asMember(chatID, senderID))

		missingID := uuid.New()
		_, err := svc.Send(context.Background(), senderID, SendInput{
			ChatID:    chatID,
			Content:   "reply",
			Type:      domain.MessageTypeText,
			ReplyToID: &missingID,
		})
		if !errors.Is(err, domain.ErrMessageNotFound) {
			t.Fatalf("want ErrMessageNotFound, got %v", err)
		}
	})
}

func TestEdit(t *testing.T) {
	chatID := uuid.New()
	authorID := uuid.New()
	otherID := uuid.New()

	seed := func() (*stubMessageRepo, *domain.Message) {
		msgs := newStubMessageRepo()
		msg := &domain.Message{
			ID:       uuid.New(),
			ChatID:   chatID,
			SenderID: authorID,
			Content:  "original",
			Type:     domain.MessageTypeText,
		}
		msgs.messages[msg.ID] = msg
		return msgs, msg
	}

	t.Run("author can edit", func(t *testing.T) {
		msgs, msg := seed()
		svc := newSvc(msgs, asMember(chatID, authorID))

		updated, err := svc.Edit(context.Background(), msg.ID, authorID, EditInput{Content: "edited"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Content != "edited" {
			t.Fatal("content not updated")
		}
		if updated.EditedAt == nil {
			t.Fatal("EditedAt should be set")
		}
	})

	t.Run("non-author cannot edit", func(t *testing.T) {
		msgs, msg := seed()
		svc := newSvc(msgs, asMember(chatID, otherID))

		_, err := svc.Edit(context.Background(), msg.ID, otherID, EditInput{Content: "hacked"})
		if !errors.Is(err, domain.ErrNotMessageAuthor) {
			t.Fatalf("want ErrNotMessageAuthor, got %v", err)
		}
	})

	t.Run("cannot edit deleted message", func(t *testing.T) {
		msgs, msg := seed()
		now := time.Now()
		msg.DeletedAt = &now
		svc := newSvc(msgs, asMember(chatID, authorID))

		_, err := svc.Edit(context.Background(), msg.ID, authorID, EditInput{Content: "edited"})
		if !errors.Is(err, domain.ErrMessageDeleted) {
			t.Fatalf("want ErrMessageDeleted, got %v", err)
		}
	})
}

func TestDelete(t *testing.T) {
	chatID := uuid.New()
	authorID := uuid.New()
	otherID := uuid.New()

	seed := func() (*stubMessageRepo, *domain.Message) {
		msgs := newStubMessageRepo()
		msg := &domain.Message{
			ID:       uuid.New(),
			ChatID:   chatID,
			SenderID: authorID,
			Type:     domain.MessageTypeText,
			Content:  "to delete",
		}
		msgs.messages[msg.ID] = msg
		return msgs, msg
	}

	t.Run("author can delete", func(t *testing.T) {
		msgs, msg := seed()
		svc := newSvc(msgs, asMember(chatID, authorID))

		if err := svc.Delete(context.Background(), msg.ID, authorID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if msgs.messages[msg.ID].DeletedAt == nil {
			t.Fatal("message should be soft-deleted")
		}
	})

	t.Run("non-author cannot delete", func(t *testing.T) {
		msgs, msg := seed()
		svc := newSvc(msgs, asMember(chatID, otherID))

		err := svc.Delete(context.Background(), msg.ID, otherID)
		if !errors.Is(err, domain.ErrNotMessageAuthor) {
			t.Fatalf("want ErrNotMessageAuthor, got %v", err)
		}
	})

	t.Run("already deleted message returns error", func(t *testing.T) {
		msgs, msg := seed()
		now := time.Now()
		msg.DeletedAt = &now
		svc := newSvc(msgs, asMember(chatID, authorID))

		err := svc.Delete(context.Background(), msg.ID, authorID)
		if !errors.Is(err, domain.ErrMessageDeleted) {
			t.Fatalf("want ErrMessageDeleted, got %v", err)
		}
	})
}

func TestAddRemoveReaction(t *testing.T) {
	chatID := uuid.New()
	userID := uuid.New()

	msgs := newStubMessageRepo()
	msg := &domain.Message{
		ID:       uuid.New(),
		ChatID:   chatID,
		SenderID: uuid.New(),
		Content:  "hi",
		Type:     domain.MessageTypeText,
	}
	msgs.messages[msg.ID] = msg

	svc := newSvc(msgs, asMember(chatID, userID))

	t.Run("add reaction", func(t *testing.T) {
		err := svc.AddReaction(context.Background(), msg.ID, userID, ReactInput{Emoji: "👍"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(msgs.reactions[msg.ID]) != 1 {
			t.Fatal("expected 1 reaction")
		}
	})

	t.Run("remove reaction", func(t *testing.T) {
		err := svc.RemoveReaction(context.Background(), msg.ID, userID, ReactInput{Emoji: "👍"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(msgs.reactions[msg.ID]) != 0 {
			t.Fatal("expected 0 reactions after remove")
		}
	})

	t.Run("cannot react to deleted message", func(t *testing.T) {
		now := time.Now()
		msg.DeletedAt = &now

		err := svc.AddReaction(context.Background(), msg.ID, userID, ReactInput{Emoji: "👍"})
		if !errors.Is(err, domain.ErrMessageDeleted) {
			t.Fatalf("want ErrMessageDeleted, got %v", err)
		}
	})
}

func TestMarkRead(t *testing.T) {
	chatID := uuid.New()
	userID := uuid.New()
	msgs := newStubMessageRepo()
	svc := newSvc(msgs, asMember(chatID, userID))

	if err := svc.MarkRead(context.Background(), chatID, userID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	key := chatID.String() + ":" + userID.String()
	if _, ok := msgs.readMarks[key]; !ok {
		t.Fatal("expected read mark to be stored")
	}
}

func TestMarkRead_NonMember(t *testing.T) {
	chatID := uuid.New()
	userID := uuid.New()
	svc := newSvc(newStubMessageRepo(), newStubChatRepo())

	err := svc.MarkRead(context.Background(), chatID, userID)
	if !errors.Is(err, domain.ErrNotChatMember) {
		t.Fatalf("want ErrNotChatMember, got %v", err)
	}
}

func TestGetHistory_AttachesReactions(t *testing.T) {
	chatID := uuid.New()
	userID := uuid.New()
	msgs := newStubMessageRepo()

	msg := &domain.Message{
		ID:       uuid.New(),
		ChatID:   chatID,
		SenderID: uuid.New(),
		Content:  "hi",
		Type:     domain.MessageTypeText,
	}
	msgs.messages[msg.ID] = msg
	msgs.reactions[msg.ID] = []*domain.Reaction{
		{MessageID: msg.ID, UserID: userID, Emoji: "❤️"},
	}

	svc := newSvc(msgs, asMember(chatID, userID))

	result, err := svc.GetHistory(context.Background(), userID, ListInput{
		ChatID: chatID,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Messages) == 0 {
		t.Fatal("expected at least one message")
	}
	if len(result.Messages[0].Reactions) == 0 {
		t.Fatal("expected reactions to be attached to message")
	}
}
