package cached

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"vibechat/internal/domain"
	"vibechat/internal/infrastructure/cache"
)

type chatRepo struct {
	origin    domain.ChatRepository
	cache     cache.Cache
	chatTTL   time.Duration
	memberTTL time.Duration
}

func NewChatRepository(origin domain.ChatRepository, c cache.Cache, chatTTL, memberTTL time.Duration) domain.ChatRepository {
	if c == nil {
		return origin
	}
	return &chatRepo{origin: origin, cache: c, chatTTL: chatTTL, memberTTL: memberTTL}
}

func chatKey(id uuid.UUID) string { return "chat:" + id.String() }

func memberKey(chatID, userID uuid.UUID) string {
	return fmt.Sprintf("chatmember:%s:%s", chatID, userID)
}

func isMemberKey(chatID, userID uuid.UUID) string {
	return fmt.Sprintf("ismember:%s:%s", chatID, userID)
}

func (r *chatRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Chat, error) {
	key := chatKey(id)
	if raw, err := r.cache.Get(ctx, key); err == nil {
		var chat domain.Chat
		if json.Unmarshal([]byte(raw), &chat) == nil {
			zerolog.Ctx(ctx).Trace().Str("chat_id", id.String()).Msg("chat cache hit")
			return &chat, nil
		}
	} else {
		zerolog.Ctx(ctx).Trace().Str("chat_id", id.String()).Msg("chat cache miss")
	}

	chat, err := r.origin.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if b, merr := json.Marshal(chat); merr == nil {
		if serr := r.cache.Set(ctx, key, string(b), r.chatTTL); serr != nil {
			zerolog.Ctx(ctx).Warn().Err(serr).Str("chat_id", id.String()).Msg("chat cache set failed")
		}
	}
	return chat, nil
}

func (r *chatRepo) IsMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error) {
	key := isMemberKey(chatID, userID)
	if raw, err := r.cache.Get(ctx, key); err == nil {
		zerolog.Ctx(ctx).Trace().
			Str("chat_id", chatID.String()).
			Str("user_id", userID.String()).
			Msg("ismember cache hit")
		return raw == "1", nil
	} else {
		zerolog.Ctx(ctx).Trace().
			Str("chat_id", chatID.String()).
			Str("user_id", userID.String()).
			Msg("ismember cache miss")
	}

	ok, err := r.origin.IsMember(ctx, chatID, userID)
	if err != nil {
		return false, err
	}
	val := "0"
	if ok {
		val = "1"
	}
	if serr := r.cache.Set(ctx, key, val, r.memberTTL); serr != nil {
		zerolog.Ctx(ctx).Warn().Err(serr).
			Str("chat_id", chatID.String()).
			Str("user_id", userID.String()).
			Msg("ismember cache set failed")
	}
	return ok, nil
}

func (r *chatRepo) GetMember(ctx context.Context, chatID, userID uuid.UUID) (*domain.ChatMember, error) {
	key := memberKey(chatID, userID)
	if raw, err := r.cache.Get(ctx, key); err == nil {
		var m domain.ChatMember
		if json.Unmarshal([]byte(raw), &m) == nil {
			zerolog.Ctx(ctx).Trace().
				Str("chat_id", chatID.String()).
				Str("user_id", userID.String()).
				Msg("chatmember cache hit")
			return &m, nil
		}
	} else {
		zerolog.Ctx(ctx).Trace().
			Str("chat_id", chatID.String()).
			Str("user_id", userID.String()).
			Msg("chatmember cache miss")
	}

	m, err := r.origin.GetMember(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}
	if b, merr := json.Marshal(m); merr == nil {
		if serr := r.cache.Set(ctx, key, string(b), r.memberTTL); serr != nil {
			zerolog.Ctx(ctx).Warn().Err(serr).
				Str("chat_id", chatID.String()).
				Str("user_id", userID.String()).
				Msg("chatmember cache set failed")
		}
	}
	return m, nil
}

func (r *chatRepo) GetUserChats(ctx context.Context, userID uuid.UUID) ([]*domain.ChatPreview, error) {
	return r.origin.GetUserChats(ctx, userID)
}

func (r *chatRepo) GetDirectChat(ctx context.Context, user1ID, user2ID uuid.UUID) (*domain.Chat, error) {
	return r.origin.GetDirectChat(ctx, user1ID, user2ID)
}

func (r *chatRepo) GetMembers(ctx context.Context, chatID uuid.UUID) ([]*domain.ChatMember, error) {
	return r.origin.GetMembers(ctx, chatID)
}

func (r *chatRepo) CreateWithMembers(ctx context.Context, chat *domain.Chat, members []*domain.ChatMember) error {
	if err := r.origin.CreateWithMembers(ctx, chat, members); err != nil {
		return err
	}
	// seed isMember cache for all initial members so the first IsMember check is a hit
	for _, m := range members {
		_ = r.cache.Set(ctx, isMemberKey(chat.ID, m.UserID), "1", r.memberTTL)
		if b, err := json.Marshal(m); err == nil {
			_ = r.cache.Set(ctx, memberKey(chat.ID, m.UserID), string(b), r.memberTTL)
		}
	}
	return nil
}

func (r *chatRepo) Update(ctx context.Context, chat *domain.Chat) error {
	// Invalidate before writing so concurrent readers go to the DB and get fresh data
	// rather than re-caching the old value during the update window.
	_ = r.cache.Del(ctx, chatKey(chat.ID))
	return r.origin.Update(ctx, chat)
}

func (r *chatRepo) Delete(ctx context.Context, id uuid.UUID) error {
	members, _ := r.origin.GetMembers(ctx, id)
	if err := r.origin.Delete(ctx, id); err != nil {
		return err
	}
	_ = r.cache.Del(ctx, chatKey(id))
	for _, m := range members {
		_ = r.cache.Del(ctx, isMemberKey(id, m.UserID))
		_ = r.cache.Del(ctx, memberKey(id, m.UserID))
	}
	return nil
}

func (r *chatRepo) AddMember(ctx context.Context, member *domain.ChatMember) error {
	if err := r.origin.AddMember(ctx, member); err != nil {
		return err
	}
	_ = r.cache.Set(ctx, isMemberKey(member.ChatID, member.UserID), "1", r.memberTTL)
	if b, err := json.Marshal(member); err == nil {
		_ = r.cache.Set(ctx, memberKey(member.ChatID, member.UserID), string(b), r.memberTTL)
	}
	return nil
}

func (r *chatRepo) RemoveMember(ctx context.Context, chatID, userID uuid.UUID) error {
	if err := r.origin.RemoveMember(ctx, chatID, userID); err != nil {
		return err
	}
	_ = r.cache.Del(ctx, isMemberKey(chatID, userID))
	_ = r.cache.Del(ctx, memberKey(chatID, userID))
	return nil
}

func (r *chatRepo) UpdateMemberRole(ctx context.Context, chatID, userID uuid.UUID, role domain.MemberRole) error {
	if err := r.origin.UpdateMemberRole(ctx, chatID, userID, role); err != nil {
		return err
	}
	// role changed — cached GetMember entry is stale
	_ = r.cache.Del(ctx, memberKey(chatID, userID))
	return nil
}
