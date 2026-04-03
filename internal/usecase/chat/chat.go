package chat

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"vibechat/internal/domain"
)

type service struct {
	chats domain.ChatRepository
	users domain.UserRepository
}

func New(chats domain.ChatRepository, users domain.UserRepository) UseCase {
	return &service{chats: chats, users: users}
}

func (s *service) CreateDirect(ctx context.Context, callerID uuid.UUID, in CreateDirectInput) (*domain.Chat, bool, error) {
	if err := in.Validate(); err != nil {
		return nil, false, err
	}
	if in.TargetUserID == callerID {
		return nil, false, domain.ErrForbidden
	}
	if _, err := s.users.GetByID(ctx, in.TargetUserID); err != nil {
		return nil, false, err
	}

	if existing, err := s.chats.GetDirectChat(ctx, callerID, in.TargetUserID); err == nil {
		return existing, false, nil
	} else if !errors.Is(err, domain.ErrChatNotFound) {
		return nil, false, err
	}

	now := time.Now().UTC()
	chat := &domain.Chat{
		ID:        uuid.New(),
		Type:      domain.ChatTypeDirect,
		CreatedBy: callerID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	members := []*domain.ChatMember{
		{ChatID: chat.ID, UserID: callerID, Role: domain.MemberRoleMember, JoinedAt: now},
		{ChatID: chat.ID, UserID: in.TargetUserID, Role: domain.MemberRoleMember, JoinedAt: now},
	}
	if err := s.chats.CreateWithMembers(ctx, chat, members); err != nil {
		return nil, false, err
	}

	zerolog.Ctx(ctx).Debug().
		Str("chat_id", chat.ID.String()).
		Str("caller_id", callerID.String()).
		Str("target_id", in.TargetUserID.String()).
		Msg("direct chat created")

	return chat, true, nil
}

func (s *service) CreateGroup(ctx context.Context, callerID uuid.UUID, in CreateGroupInput) (*domain.Chat, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	chat := &domain.Chat{
		ID:          uuid.New(),
		Type:        domain.ChatTypeGroup,
		Name:        in.Name,
		Description: in.Description,
		CreatedBy:   callerID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	for _, uid := range in.MemberIDs {
		if uid == callerID {
			continue
		}
		if _, err := s.users.GetByID(ctx, uid); err != nil {
			return nil, err // ErrUserNotFound → 404
		}
	}

	seen := map[uuid.UUID]struct{}{callerID: {}}
	members := make([]*domain.ChatMember, 0, 1+len(in.MemberIDs))
	members = append(members, &domain.ChatMember{
		ChatID:   chat.ID,
		UserID:   callerID,
		Role:     domain.MemberRoleOwner,
		JoinedAt: now,
	})
	for _, uid := range in.MemberIDs {
		if _, dup := seen[uid]; dup {
			continue
		}
		seen[uid] = struct{}{}
		members = append(members, &domain.ChatMember{
			ChatID:   chat.ID,
			UserID:   uid,
			Role:     domain.MemberRoleMember,
			JoinedAt: now,
		})
	}

	if err := s.chats.CreateWithMembers(ctx, chat, members); err != nil {
		return nil, err
	}

	zerolog.Ctx(ctx).Debug().
		Str("chat_id", chat.ID.String()).
		Str("caller_id", callerID.String()).
		Str("name", in.Name).
		Int("members", len(members)).
		Msg("group chat created")

	return chat, nil
}

func (s *service) GetChat(ctx context.Context, chatID, callerID uuid.UUID) (*domain.Chat, error) {
	if err := s.assertMember(ctx, chatID, callerID); err != nil {
		return nil, err
	}
	return s.chats.GetByID(ctx, chatID)
}

func (s *service) GetUserChats(ctx context.Context, callerID uuid.UUID) ([]*domain.ChatPreview, error) {
	return s.chats.GetUserChats(ctx, callerID)
}

func (s *service) GetMembers(ctx context.Context, chatID, callerID uuid.UUID) ([]*domain.ChatMember, error) {
	if err := s.assertMember(ctx, chatID, callerID); err != nil {
		return nil, err
	}
	return s.chats.GetMembers(ctx, chatID)
}

func (s *service) UpdateGroup(ctx context.Context, chatID, callerID uuid.UUID, in UpdateGroupInput) (*domain.Chat, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	chat, err := s.chats.GetByID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if chat.IsDirect() {
		return nil, domain.ErrForbidden
	}

	caller, err := s.chats.GetMember(ctx, chatID, callerID)
	if err != nil {
		return nil, err
	}
	if !caller.Role.CanManageMembers() {
		return nil, domain.ErrInsufficientRights
	}

	if in.Name != nil {
		chat.Name = *in.Name
	}
	if in.Description != nil {
		chat.Description = *in.Description
	}
	if in.AvatarURL != nil {
		chat.AvatarURL = *in.AvatarURL
	}
	chat.UpdatedAt = time.Now().UTC()

	if err = s.chats.Update(ctx, chat); err != nil {
		return nil, err
	}

	zerolog.Ctx(ctx).Debug().
		Str("chat_id", chatID.String()).
		Str("caller_id", callerID.String()).
		Msg("group updated")

	return chat, nil
}

func (s *service) AddMember(ctx context.Context, chatID, callerID uuid.UUID, in AddMemberInput) error {
	if err := in.Validate(); err != nil {
		return err
	}
	chat, err := s.chats.GetByID(ctx, chatID)
	if err != nil {
		return err
	}
	if chat.IsDirect() {
		return domain.ErrForbidden
	}
	caller, err := s.chats.GetMember(ctx, chatID, callerID)
	if err != nil {
		return err
	}
	if !caller.Role.CanManageMembers() {
		return domain.ErrInsufficientRights
	}

	if _, err = s.users.GetByID(ctx, in.UserID); err != nil {
		return err
	}

	already, err := s.chats.IsMember(ctx, chatID, in.UserID)
	if err != nil {
		return err
	}
	if already {
		return domain.ErrAlreadyChatMember
	}

	if err = s.chats.AddMember(ctx, &domain.ChatMember{
		ChatID:   chatID,
		UserID:   in.UserID,
		Role:     domain.MemberRoleMember,
		JoinedAt: time.Now().UTC(),
	}); err != nil {
		return err
	}

	zerolog.Ctx(ctx).Debug().
		Str("chat_id", chatID.String()).
		Str("caller_id", callerID.String()).
		Str("added_user_id", in.UserID.String()).
		Msg("member added to chat")

	return nil
}

func (s *service) RemoveMember(ctx context.Context, chatID, callerID, targetID uuid.UUID) error {
	chat, err := s.chats.GetByID(ctx, chatID)
	if err != nil {
		return err
	}
	if chat.IsDirect() {
		return domain.ErrForbidden
	}
	caller, err := s.chats.GetMember(ctx, chatID, callerID)
	if err != nil {
		return err
	}
	if !caller.Role.CanManageMembers() {
		return domain.ErrInsufficientRights
	}

	target, err := s.chats.GetMember(ctx, chatID, targetID)
	if err != nil {
		return err
	}
	if target.Role == domain.MemberRoleOwner {
		return domain.ErrCannotRemoveOwner
	}
	if target.Role == domain.MemberRoleAdmin && caller.Role != domain.MemberRoleOwner {
		return domain.ErrInsufficientRights
	}

	if err = s.chats.RemoveMember(ctx, chatID, targetID); err != nil {
		return err
	}

	zerolog.Ctx(ctx).Debug().
		Str("chat_id", chatID.String()).
		Str("caller_id", callerID.String()).
		Str("removed_user_id", targetID.String()).
		Msg("member removed from chat")

	return nil
}

func (s *service) ChangeMemberRole(ctx context.Context, chatID, callerID uuid.UUID, in ChangeMemberRoleInput) error {
	if err := in.Validate(); err != nil {
		return err
	}
	if in.UserID == callerID {
		return domain.ErrForbidden
	}
	caller, err := s.chats.GetMember(ctx, chatID, callerID)
	if err != nil {
		return err
	}
	if caller.Role != domain.MemberRoleOwner {
		return domain.ErrInsufficientRights
	}

	if _, err = s.chats.GetMember(ctx, chatID, in.UserID); err != nil {
		return err
	}

	role := domain.MemberRole(in.Role)
	if err = s.chats.UpdateMemberRole(ctx, chatID, in.UserID, role); err != nil {
		return err
	}

	zerolog.Ctx(ctx).Debug().
		Str("chat_id", chatID.String()).
		Str("caller_id", callerID.String()).
		Str("target_user_id", in.UserID.String()).
		Str("new_role", in.Role).
		Msg("member role changed")

	return nil
}

func (s *service) LeaveChat(ctx context.Context, chatID, callerID uuid.UUID) error {
	chat, err := s.chats.GetByID(ctx, chatID)
	if err != nil {
		return err
	}
	if chat.IsDirect() {
		return domain.ErrForbidden
	}

	caller, err := s.chats.GetMember(ctx, chatID, callerID)
	if err != nil {
		return err
	}

	if caller.Role == domain.MemberRoleOwner {
		members, err := s.chats.GetMembers(ctx, chatID)
		if err != nil {
			return err
		}
		next := nextOwner(members, callerID)
		if next == nil {
			// caller is the last member — delete the group instead of leaving an empty shell
			return s.chats.Delete(ctx, chatID)
		}
		if err = s.chats.UpdateMemberRole(ctx, chatID, next.UserID, domain.MemberRoleOwner); err != nil {
			return err
		}
	}

	if err = s.chats.RemoveMember(ctx, chatID, callerID); err != nil {
		return err
	}

	zerolog.Ctx(ctx).Debug().
		Str("chat_id", chatID.String()).
		Str("user_id", callerID.String()).
		Msg("user left chat")

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

// nextOwner picks the next admin (or any member) to receive ownership when the owner leaves.
func nextOwner(members []*domain.ChatMember, excludeID uuid.UUID) *domain.ChatMember {
	var fallback *domain.ChatMember
	for _, m := range members {
		if m.UserID == excludeID {
			continue
		}
		if m.Role == domain.MemberRoleAdmin {
			return m
		}
		if fallback == nil {
			fallback = m
		}
	}
	return fallback
}
