package cached

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"vibechat/internal/domain"
	"vibechat/internal/infrastructure/cache/mock"
)

// countingChatRepo wraps a map-backed store and counts hot-path calls.
type countingChatRepo struct {
	chats        map[uuid.UUID]*domain.Chat
	members      map[uuid.UUID][]*domain.ChatMember
	getByIDCnt   int
	isMemberCnt  int
	getMemberCnt int
}

func newCountingChatRepo() *countingChatRepo {
	return &countingChatRepo{
		chats:   make(map[uuid.UUID]*domain.Chat),
		members: make(map[uuid.UUID][]*domain.ChatMember),
	}
}

func (r *countingChatRepo) CreateWithMembers(_ context.Context, c *domain.Chat, ms []*domain.ChatMember) error {
	r.chats[c.ID] = c
	r.members[c.ID] = ms
	return nil
}

func (r *countingChatRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Chat, error) {
	r.getByIDCnt++
	c, ok := r.chats[id]
	if !ok {
		return nil, domain.ErrChatNotFound
	}
	return c, nil
}

func (r *countingChatRepo) Update(_ context.Context, c *domain.Chat) error {
	r.chats[c.ID] = c
	return nil
}

func (r *countingChatRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(r.chats, id)
	return nil
}

func (r *countingChatRepo) AddMember(_ context.Context, m *domain.ChatMember) error {
	r.members[m.ChatID] = append(r.members[m.ChatID], m)
	return nil
}

func (r *countingChatRepo) RemoveMember(_ context.Context, chatID, userID uuid.UUID) error {
	ms := r.members[chatID]
	out := ms[:0]
	for _, m := range ms {
		if m.UserID != userID {
			out = append(out, m)
		}
	}
	r.members[chatID] = out
	return nil
}

func (r *countingChatRepo) GetMember(_ context.Context, chatID, userID uuid.UUID) (*domain.ChatMember, error) {
	r.getMemberCnt++
	for _, m := range r.members[chatID] {
		if m.UserID == userID {
			return m, nil
		}
	}
	return nil, domain.ErrNotChatMember
}

func (r *countingChatRepo) GetMembers(_ context.Context, chatID uuid.UUID) ([]*domain.ChatMember, error) {
	return r.members[chatID], nil
}

func (r *countingChatRepo) UpdateMemberRole(_ context.Context, chatID, userID uuid.UUID, role domain.MemberRole) error {
	for _, m := range r.members[chatID] {
		if m.UserID == userID {
			m.Role = role
			return nil
		}
	}
	return domain.ErrNotChatMember
}

func (r *countingChatRepo) GetUserChats(_ context.Context, _ uuid.UUID) ([]*domain.ChatPreview, error) {
	return nil, nil
}

func (r *countingChatRepo) GetDirectChat(_ context.Context, _, _ uuid.UUID) (*domain.Chat, error) {
	return nil, domain.ErrChatNotFound
}

func (r *countingChatRepo) IsMember(_ context.Context, chatID, userID uuid.UUID) (bool, error) {
	r.isMemberCnt++
	for _, m := range r.members[chatID] {
		if m.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

func newCachedChatRepo(origin *countingChatRepo) domain.ChatRepository {
	return NewChatRepository(origin, mock.New(), 10*time.Minute, 2*time.Minute)
}

func seedChat(origin *countingChatRepo) (*domain.Chat, uuid.UUID) {
	chatID := uuid.New()
	userID := uuid.New()
	chat := &domain.Chat{ID: chatID, Type: domain.ChatTypeGroup}
	member := &domain.ChatMember{ChatID: chatID, UserID: userID, Role: domain.MemberRoleMember}
	origin.chats[chatID] = chat
	origin.members[chatID] = []*domain.ChatMember{member}
	return chat, userID
}

func TestCachedChat_GetByID_HitAfterMiss(t *testing.T) {
	origin := newCountingChatRepo()
	chat, _ := seedChat(origin)
	repo := newCachedChatRepo(origin)

	_, _ = repo.GetByID(context.Background(), chat.ID) // miss
	_, _ = repo.GetByID(context.Background(), chat.ID) // hit
	_, _ = repo.GetByID(context.Background(), chat.ID) // hit

	if origin.getByIDCnt != 1 {
		t.Fatalf("expected 1 origin call, got %d", origin.getByIDCnt)
	}
}

func TestCachedChat_GetByID_InvalidatedOnUpdate(t *testing.T) {
	origin := newCountingChatRepo()
	chat, _ := seedChat(origin)
	repo := newCachedChatRepo(origin)

	_, _ = repo.GetByID(context.Background(), chat.ID) // miss

	chat.Name = "updated"
	_ = repo.Update(context.Background(), chat)

	_, _ = repo.GetByID(context.Background(), chat.ID) // must hit origin

	if origin.getByIDCnt != 2 {
		t.Fatalf("expected 2 origin calls after Update, got %d", origin.getByIDCnt)
	}
}

func TestCachedChat_GetByID_InvalidatedOnDelete(t *testing.T) {
	origin := newCountingChatRepo()
	chat, _ := seedChat(origin)
	repo := newCachedChatRepo(origin)

	_, _ = repo.GetByID(context.Background(), chat.ID) // miss

	_ = repo.Delete(context.Background(), chat.ID)

	_, err := repo.GetByID(context.Background(), chat.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

// --- IsMember ---

func TestCachedChat_IsMember_HitAfterMiss(t *testing.T) {
	origin := newCountingChatRepo()
	chat, userID := seedChat(origin)
	repo := newCachedChatRepo(origin)

	_, _ = repo.IsMember(context.Background(), chat.ID, userID) // miss
	_, _ = repo.IsMember(context.Background(), chat.ID, userID) // hit
	_, _ = repo.IsMember(context.Background(), chat.ID, userID) // hit

	if origin.isMemberCnt != 1 {
		t.Fatalf("expected 1 origin call, got %d", origin.isMemberCnt)
	}
}

func TestCachedChat_IsMember_FalseNotCachedAsTrue(t *testing.T) {
	origin := newCountingChatRepo()
	chat, _ := seedChat(origin)
	repo := newCachedChatRepo(origin)

	outsider := uuid.New()
	ok1, _ := repo.IsMember(context.Background(), chat.ID, outsider) // miss → false
	ok2, _ := repo.IsMember(context.Background(), chat.ID, outsider) // should be cached as false

	if ok1 || ok2 {
		t.Fatal("outsider should not be a member")
	}
	// false results ARE cached (so the second call should be a cache hit)
	if origin.isMemberCnt != 1 {
		t.Fatalf("expected 1 origin call (false cached), got %d", origin.isMemberCnt)
	}
}

func TestCachedChat_IsMember_InvalidatedOnRemoveMember(t *testing.T) {
	origin := newCountingChatRepo()
	chat, userID := seedChat(origin)
	repo := newCachedChatRepo(origin)

	ok, _ := repo.IsMember(context.Background(), chat.ID, userID) // miss → true
	if !ok {
		t.Fatal("expected member to be present")
	}

	_ = repo.RemoveMember(context.Background(), chat.ID, userID) // deletes cache + origin

	ok, _ = repo.IsMember(context.Background(), chat.ID, userID) // miss → false
	if ok {
		t.Fatal("expected member to be gone after removal")
	}
	if origin.isMemberCnt != 2 {
		t.Fatalf("expected 2 origin calls after removal, got %d", origin.isMemberCnt)
	}
}

func TestCachedChat_IsMember_SetOnAddMember(t *testing.T) {
	origin := newCountingChatRepo()
	chat, _ := seedChat(origin)
	repo := newCachedChatRepo(origin)

	newUserID := uuid.New()
	_ = repo.AddMember(context.Background(), &domain.ChatMember{
		ChatID: chat.ID, UserID: newUserID, Role: domain.MemberRoleMember, JoinedAt: time.Now(),
	})

	// cache was seeded by AddMember — origin must NOT be called
	ok, _ := repo.IsMember(context.Background(), chat.ID, newUserID)
	if !ok {
		t.Fatal("expected new member to be found")
	}
	if origin.isMemberCnt != 0 {
		t.Fatalf("expected 0 origin calls (seeded by AddMember), got %d", origin.isMemberCnt)
	}
}

// --- CreateWithMembers seeds cache ---

func TestCachedChat_CreateWithMembers_SeedsCache(t *testing.T) {
	origin := newCountingChatRepo()
	repo := newCachedChatRepo(origin)

	chatID := uuid.New()
	user1, user2 := uuid.New(), uuid.New()
	chat := &domain.Chat{ID: chatID, Type: domain.ChatTypeDirect}
	members := []*domain.ChatMember{
		{ChatID: chatID, UserID: user1, Role: domain.MemberRoleMember},
		{ChatID: chatID, UserID: user2, Role: domain.MemberRoleMember},
	}

	_ = repo.CreateWithMembers(context.Background(), chat, members)

	// Both members should be cache hits — origin should not be called
	ok1, _ := repo.IsMember(context.Background(), chatID, user1)
	ok2, _ := repo.IsMember(context.Background(), chatID, user2)

	if !ok1 || !ok2 {
		t.Fatal("expected both members to be in cache after CreateWithMembers")
	}
	if origin.isMemberCnt != 0 {
		t.Fatalf("expected 0 origin calls (cache seeded), got %d", origin.isMemberCnt)
	}
}

// --- GetMember ---

func TestCachedChat_GetMember_HitAfterMiss(t *testing.T) {
	origin := newCountingChatRepo()
	chat, userID := seedChat(origin)
	repo := newCachedChatRepo(origin)

	_, _ = repo.GetMember(context.Background(), chat.ID, userID) // miss
	_, _ = repo.GetMember(context.Background(), chat.ID, userID) // hit

	if origin.getMemberCnt != 1 {
		t.Fatalf("expected 1 origin call, got %d", origin.getMemberCnt)
	}
}

func TestCachedChat_GetMember_InvalidatedOnUpdateRole(t *testing.T) {
	origin := newCountingChatRepo()
	chat, userID := seedChat(origin)
	repo := newCachedChatRepo(origin)

	m, _ := repo.GetMember(context.Background(), chat.ID, userID) // miss
	if m.Role != domain.MemberRoleMember {
		t.Fatalf("unexpected initial role: %s", m.Role)
	}

	// promote in origin first so the next read picks it up
	_ = origin.UpdateMemberRole(context.Background(), chat.ID, userID, domain.MemberRoleAdmin)
	_ = repo.UpdateMemberRole(context.Background(), chat.ID, userID, domain.MemberRoleAdmin) // invalidates

	m, _ = repo.GetMember(context.Background(), chat.ID, userID) // miss → fresh
	if m.Role != domain.MemberRoleAdmin {
		t.Fatalf("expected admin role after invalidation, got %s", m.Role)
	}
	if origin.getMemberCnt != 2 {
		t.Fatalf("expected 2 origin calls, got %d", origin.getMemberCnt)
	}
}

func TestCachedChat_NilCache_PassesThrough(t *testing.T) {
	origin := newCountingChatRepo()
	chat, userID := seedChat(origin)

	repo := NewChatRepository(origin, nil, 10*time.Minute, 2*time.Minute)

	ok, err := repo.IsMember(context.Background(), chat.ID, userID)
	if err != nil || !ok {
		t.Fatalf("expected member to be found via pass-through, err=%v ok=%v", err, ok)
	}
}
