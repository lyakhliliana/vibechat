package chat

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"vibechat/internal/domain"
)

// --- stubs ---

type stubChatRepo struct {
	chats   map[uuid.UUID]*domain.Chat
	members map[uuid.UUID][]*domain.ChatMember // chatID → members
}

func newStubChatRepo() *stubChatRepo {
	return &stubChatRepo{
		chats:   make(map[uuid.UUID]*domain.Chat),
		members: make(map[uuid.UUID][]*domain.ChatMember),
	}
}

func (r *stubChatRepo) CreateWithMembers(_ context.Context, c *domain.Chat, ms []*domain.ChatMember) error {
	r.chats[c.ID] = c
	r.members[c.ID] = ms
	return nil
}

func (r *stubChatRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Chat, error) {
	c, ok := r.chats[id]
	if !ok {
		return nil, domain.ErrChatNotFound
	}
	return c, nil
}

func (r *stubChatRepo) Update(_ context.Context, c *domain.Chat) error {
	r.chats[c.ID] = c
	return nil
}

func (r *stubChatRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(r.chats, id)
	delete(r.members, id)
	return nil
}

func (r *stubChatRepo) AddMember(_ context.Context, m *domain.ChatMember) error {
	r.members[m.ChatID] = append(r.members[m.ChatID], m)
	return nil
}

func (r *stubChatRepo) RemoveMember(_ context.Context, chatID, userID uuid.UUID) error {
	ms := r.members[chatID]
	filtered := ms[:0]
	for _, m := range ms {
		if m.UserID != userID {
			filtered = append(filtered, m)
		}
	}
	r.members[chatID] = filtered
	return nil
}

func (r *stubChatRepo) GetMember(_ context.Context, chatID, userID uuid.UUID) (*domain.ChatMember, error) {
	for _, m := range r.members[chatID] {
		if m.UserID == userID {
			return m, nil
		}
	}
	return nil, domain.ErrNotChatMember
}

func (r *stubChatRepo) GetMembers(_ context.Context, chatID uuid.UUID) ([]*domain.ChatMember, error) {
	return r.members[chatID], nil
}

func (r *stubChatRepo) UpdateMemberRole(_ context.Context, chatID, userID uuid.UUID, role domain.MemberRole) error {
	for _, m := range r.members[chatID] {
		if m.UserID == userID {
			m.Role = role
			return nil
		}
	}
	return domain.ErrNotChatMember
}

func (r *stubChatRepo) GetUserChats(_ context.Context, _ uuid.UUID) ([]*domain.ChatPreview, error) {
	return nil, nil
}

func (r *stubChatRepo) GetDirectChat(_ context.Context, u1, u2 uuid.UUID) (*domain.Chat, error) {
	for _, c := range r.chats {
		if c.Type != domain.ChatTypeDirect {
			continue
		}
		ms := r.members[c.ID]
		has1, has2 := false, false
		for _, m := range ms {
			if m.UserID == u1 {
				has1 = true
			}
			if m.UserID == u2 {
				has2 = true
			}
		}
		if has1 && has2 {
			return c, nil
		}
	}
	return nil, domain.ErrChatNotFound
}

func (r *stubChatRepo) IsMember(_ context.Context, chatID, userID uuid.UUID) (bool, error) {
	for _, m := range r.members[chatID] {
		if m.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

// reuse user stub from user package — define a minimal one locally
type stubUserRepo struct {
	users map[uuid.UUID]*domain.User
}

func newStubUserRepo(seed ...*domain.User) *stubUserRepo {
	r := &stubUserRepo{users: make(map[uuid.UUID]*domain.User)}
	for _, u := range seed {
		r.users[u.ID] = u
	}
	return r
}

func (r *stubUserRepo) Create(_ context.Context, u *domain.User) error {
	r.users[u.ID] = u
	return nil
}
func (r *stubUserRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}
func (r *stubUserRepo) GetByEmail(_ context.Context, _ string) (*domain.User, error) {
	return nil, domain.ErrUserNotFound
}
func (r *stubUserRepo) GetByUsername(_ context.Context, _ string) (*domain.User, error) {
	return nil, domain.ErrUserNotFound
}
func (r *stubUserRepo) Update(_ context.Context, u *domain.User) error { return nil }
func (r *stubUserRepo) UpdateStatus(_ context.Context, _ uuid.UUID, _ domain.UserStatus, _ time.Time) error {
	return nil
}
func (r *stubUserRepo) Search(_ context.Context, _ string, _, _ int) ([]*domain.User, error) {
	return nil, nil
}
func (r *stubUserRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }

func newSvc(chats *stubChatRepo, users *stubUserRepo) UseCase {
	return New(chats, users)
}

// --- tests ---

func TestCreateDirect(t *testing.T) {
	caller := &domain.User{ID: uuid.New()}
	target := &domain.User{ID: uuid.New()}

	t.Run("creates chat with two members", func(t *testing.T) {
		chats := newStubChatRepo()
		svc := newSvc(chats, newStubUserRepo(caller, target))

		chat, created, err := svc.CreateDirect(context.Background(), caller.ID, CreateDirectInput{TargetUserID: target.ID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !created {
			t.Fatal("expected created=true for new chat")
		}
		if chat.Type != domain.ChatTypeDirect {
			t.Fatal("expected direct chat type")
		}
		members := chats.members[chat.ID]
		if len(members) != 2 {
			t.Fatalf("expected 2 members, got %d", len(members))
		}
	})

	t.Run("returns existing chat on duplicate call", func(t *testing.T) {
		chats := newStubChatRepo()
		svc := newSvc(chats, newStubUserRepo(caller, target))
		in := CreateDirectInput{TargetUserID: target.ID}

		first, _, _ := svc.CreateDirect(context.Background(), caller.ID, in)
		second, created, err := svc.CreateDirect(context.Background(), caller.ID, in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created {
			t.Fatal("expected created=false for existing chat")
		}
		if first.ID != second.ID {
			t.Fatal("expected same chat ID on duplicate create")
		}
		if len(chats.chats) != 1 {
			t.Fatal("expected exactly one chat in store")
		}
	})

	t.Run("forbidden when caller == target", func(t *testing.T) {
		svc := newSvc(newStubChatRepo(), newStubUserRepo(caller))
		_, _, err := svc.CreateDirect(context.Background(), caller.ID, CreateDirectInput{TargetUserID: caller.ID})
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("want ErrForbidden, got %v", err)
		}
	})

	t.Run("error when target user does not exist", func(t *testing.T) {
		svc := newSvc(newStubChatRepo(), newStubUserRepo(caller))
		_, _, err := svc.CreateDirect(context.Background(), caller.ID, CreateDirectInput{TargetUserID: uuid.New()})
		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("want ErrUserNotFound, got %v", err)
		}
	})
}

func TestCreateGroup(t *testing.T) {
	owner := &domain.User{ID: uuid.New()}
	member1 := &domain.User{ID: uuid.New()}
	member2 := &domain.User{ID: uuid.New()}

	t.Run("owner gets owner role", func(t *testing.T) {
		chats := newStubChatRepo()
		svc := newSvc(chats, newStubUserRepo(owner, member1))

		chat, err := svc.CreateGroup(context.Background(), owner.ID, CreateGroupInput{
			Name:      "test group",
			MemberIDs: []uuid.UUID{member1.ID},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ownerMember, err := chats.GetMember(context.Background(), chat.ID, owner.ID)
		if err != nil {
			t.Fatalf("owner not found in members: %v", err)
		}
		if ownerMember.Role != domain.MemberRoleOwner {
			t.Fatalf("expected owner role, got %s", ownerMember.Role)
		}
	})

	t.Run("duplicate member IDs are deduplicated", func(t *testing.T) {
		chats := newStubChatRepo()
		svc := newSvc(chats, newStubUserRepo(owner, member1, member2))

		chat, err := svc.CreateGroup(context.Background(), owner.ID, CreateGroupInput{
			Name:      "test",
			MemberIDs: []uuid.UUID{member1.ID, member1.ID, member2.ID},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(chats.members[chat.ID]) != 3 { // owner + member1 + member2
			t.Fatalf("expected 3 members after dedup, got %d", len(chats.members[chat.ID]))
		}
	})

	t.Run("caller in member list is not duplicated", func(t *testing.T) {
		chats := newStubChatRepo()
		svc := newSvc(chats, newStubUserRepo(owner, member1))

		chat, err := svc.CreateGroup(context.Background(), owner.ID, CreateGroupInput{
			Name:      "test",
			MemberIDs: []uuid.UUID{owner.ID, member1.ID}, // owner listed twice
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(chats.members[chat.ID]) != 2 { // owner + member1
			t.Fatalf("expected 2 members, got %d", len(chats.members[chat.ID]))
		}
	})

	t.Run("empty name fails validation", func(t *testing.T) {
		svc := newSvc(newStubChatRepo(), newStubUserRepo(owner))
		_, err := svc.CreateGroup(context.Background(), owner.ID, CreateGroupInput{Name: ""})
		if !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("want ErrValidation, got %v", err)
		}
	})
}

func TestLeaveChat(t *testing.T) {
	t.Run("last member leaving deletes the chat", func(t *testing.T) {
		owner := &domain.User{ID: uuid.New()}
		chats := newStubChatRepo()
		svc := newSvc(chats, newStubUserRepo(owner))

		chat, _ := svc.CreateGroup(context.Background(), owner.ID, CreateGroupInput{Name: "solo"})

		err := svc.LeaveChat(context.Background(), chat.ID, owner.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, exists := chats.chats[chat.ID]; exists {
			t.Fatal("expected chat to be deleted when last member leaves")
		}
	})

	t.Run("ownership transfers to admin when owner leaves", func(t *testing.T) {
		owner := &domain.User{ID: uuid.New()}
		admin := &domain.User{ID: uuid.New()}
		chats := newStubChatRepo()
		svc := newSvc(chats, newStubUserRepo(owner, admin))

		chat, _ := svc.CreateGroup(context.Background(), owner.ID, CreateGroupInput{
			Name:      "group",
			MemberIDs: []uuid.UUID{admin.ID},
		})
		// promote admin
		_ = chats.UpdateMemberRole(context.Background(), chat.ID, admin.ID, domain.MemberRoleAdmin)

		err := svc.LeaveChat(context.Background(), chat.ID, owner.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		newOwner, err := chats.GetMember(context.Background(), chat.ID, admin.ID)
		if err != nil {
			t.Fatalf("admin not found after owner left: %v", err)
		}
		if newOwner.Role != domain.MemberRoleOwner {
			t.Fatalf("expected admin to become owner, got role %s", newOwner.Role)
		}

		// old owner must be gone
		_, err = chats.GetMember(context.Background(), chat.ID, owner.ID)
		if !errors.Is(err, domain.ErrNotChatMember) {
			t.Fatalf("expected old owner to be removed, got %v", err)
		}
	})

	t.Run("direct chat cannot be left", func(t *testing.T) {
		caller := &domain.User{ID: uuid.New()}
		target := &domain.User{ID: uuid.New()}
		chats := newStubChatRepo()
		svc := newSvc(chats, newStubUserRepo(caller, target))

		chat, _, _ := svc.CreateDirect(context.Background(), caller.ID, CreateDirectInput{TargetUserID: target.ID})

		err := svc.LeaveChat(context.Background(), chat.ID, caller.ID)
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("want ErrForbidden, got %v", err)
		}
	})

	t.Run("non-member cannot leave", func(t *testing.T) {
		owner := &domain.User{ID: uuid.New()}
		outsider := &domain.User{ID: uuid.New()}
		chats := newStubChatRepo()
		svc := newSvc(chats, newStubUserRepo(owner, outsider))

		chat, _ := svc.CreateGroup(context.Background(), owner.ID, CreateGroupInput{Name: "g"})

		// LeaveChat calls GetMember which returns ErrNotChatMember
		err := svc.LeaveChat(context.Background(), chat.ID, outsider.ID)
		if !errors.Is(err, domain.ErrNotChatMember) {
			t.Fatalf("want ErrNotChatMember, got %v", err)
		}
	})
}

func TestRemoveMember(t *testing.T) {
	t.Run("owner removes a plain member", func(t *testing.T) {
		owner := &domain.User{ID: uuid.New()}
		member := &domain.User{ID: uuid.New()}
		chats := newStubChatRepo()
		svc := newSvc(chats, newStubUserRepo(owner, member))

		chat, _ := svc.CreateGroup(context.Background(), owner.ID, CreateGroupInput{
			Name: "g", MemberIDs: []uuid.UUID{member.ID},
		})

		err := svc.RemoveMember(context.Background(), chat.ID, owner.ID, member.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ok, _ := chats.IsMember(context.Background(), chat.ID, member.ID)
		if ok {
			t.Fatal("expected member to be removed")
		}
	})

	t.Run("owner cannot be removed", func(t *testing.T) {
		owner := &domain.User{ID: uuid.New()}
		admin := &domain.User{ID: uuid.New()}
		chats := newStubChatRepo()
		svc := newSvc(chats, newStubUserRepo(owner, admin))

		chat, _ := svc.CreateGroup(context.Background(), owner.ID, CreateGroupInput{
			Name: "g", MemberIDs: []uuid.UUID{admin.ID},
		})
		_ = chats.UpdateMemberRole(context.Background(), chat.ID, admin.ID, domain.MemberRoleAdmin)

		// admin tries to remove owner
		err := svc.RemoveMember(context.Background(), chat.ID, admin.ID, owner.ID)
		if !errors.Is(err, domain.ErrCannotRemoveOwner) {
			t.Fatalf("want ErrCannotRemoveOwner, got %v", err)
		}
	})

	t.Run("member without rights cannot remove others", func(t *testing.T) {
		owner := &domain.User{ID: uuid.New()}
		member := &domain.User{ID: uuid.New()}
		target := &domain.User{ID: uuid.New()}
		chats := newStubChatRepo()
		svc := newSvc(chats, newStubUserRepo(owner, member, target))

		chat, _ := svc.CreateGroup(context.Background(), owner.ID, CreateGroupInput{
			Name: "g", MemberIDs: []uuid.UUID{member.ID, target.ID},
		})

		err := svc.RemoveMember(context.Background(), chat.ID, member.ID, target.ID)
		if !errors.Is(err, domain.ErrInsufficientRights) {
			t.Fatalf("want ErrInsufficientRights, got %v", err)
		}
	})
}

func TestLeaveChat_OwnershipTransferToMember(t *testing.T) {
	owner := &domain.User{ID: uuid.New()}
	member := &domain.User{ID: uuid.New()}
	chats := newStubChatRepo()
	svc := newSvc(chats, newStubUserRepo(owner, member))

	chat, _ := svc.CreateGroup(context.Background(), owner.ID, CreateGroupInput{
		Name: "g", MemberIDs: []uuid.UUID{member.ID},
	})

	// member has no admin role — ownership should still transfer
	err := svc.LeaveChat(context.Background(), chat.ID, owner.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newOwner, _ := chats.GetMember(context.Background(), chat.ID, member.ID)
	if newOwner.Role != domain.MemberRoleOwner {
		t.Fatalf("expected member to become owner, got %s", newOwner.Role)
	}
}

// Verify nextOwner picks admin over plain member.
func TestNextOwner(t *testing.T) {
	ownerID := uuid.New()
	adminID := uuid.New()
	memberID := uuid.New()

	members := []*domain.ChatMember{
		{UserID: ownerID, Role: domain.MemberRoleOwner},
		{UserID: memberID, Role: domain.MemberRoleMember},
		{UserID: adminID, Role: domain.MemberRoleAdmin},
	}

	next := nextOwner(members, ownerID)
	if next == nil {
		t.Fatal("expected a next owner")
	}
	if next.UserID != adminID {
		t.Fatalf("expected admin to be picked as next owner, got %s", next.Role)
	}
}

// Regression: LeaveChat with a non-existent chat returns ErrChatNotFound.
func TestLeaveChat_ChatNotFound(t *testing.T) {
	user := &domain.User{ID: uuid.New()}
	svc := newSvc(newStubChatRepo(), newStubUserRepo(user))

	err := svc.LeaveChat(context.Background(), uuid.New(), user.ID)
	if !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("want ErrChatNotFound, got %v", err)
	}
}

func TestUpdateGroup(t *testing.T) {
	owner := &domain.User{ID: uuid.New()}
	member := &domain.User{ID: uuid.New()}

	setup := func() (*stubChatRepo, UseCase, *domain.Chat) {
		chats := newStubChatRepo()
		svc := newSvc(chats, newStubUserRepo(owner, member))
		chat, _ := svc.CreateGroup(context.Background(), owner.ID, CreateGroupInput{
			Name:      "original",
			MemberIDs: []uuid.UUID{member.ID},
		})
		return chats, svc, chat
	}

	t.Run("owner can update name", func(t *testing.T) {
		_, svc, chat := setup()
		updated, err := svc.UpdateGroup(context.Background(), chat.ID, owner.ID, UpdateGroupInput{
			Name: strPtr("renamed"),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Name != "renamed" {
			t.Fatalf("want renamed, got %q", updated.Name)
		}
	})

	t.Run("plain member cannot update", func(t *testing.T) {
		_, svc, chat := setup()
		_, err := svc.UpdateGroup(context.Background(), chat.ID, member.ID, UpdateGroupInput{
			Name: strPtr("hacked"),
		})
		if !errors.Is(err, domain.ErrInsufficientRights) {
			t.Fatalf("want ErrInsufficientRights, got %v", err)
		}
	})

	t.Run("direct chat cannot be updated", func(t *testing.T) {
		target := &domain.User{ID: uuid.New()}
		chats := newStubChatRepo()
		svc := newSvc(chats, newStubUserRepo(owner, target))
		direct, _, _ := svc.CreateDirect(context.Background(), owner.ID, CreateDirectInput{TargetUserID: target.ID})

		_, err := svc.UpdateGroup(context.Background(), direct.ID, owner.ID, UpdateGroupInput{
			Name: strPtr("nope"),
		})
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("want ErrForbidden, got %v", err)
		}
	})

	t.Run("invalid avatar URL fails validation", func(t *testing.T) {
		_, svc, chat := setup()
		bad := "not-a-url"
		_, err := svc.UpdateGroup(context.Background(), chat.ID, owner.ID, UpdateGroupInput{AvatarURL: &bad})
		if !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("want ErrValidation, got %v", err)
		}
	})
}

func strPtr(s string) *string { return &s }

func TestAddMember(t *testing.T) {
	owner := &domain.User{ID: uuid.New()}
	newMember := &domain.User{ID: uuid.New()}

	setup := func() (*stubChatRepo, UseCase, *domain.Chat) {
		chats := newStubChatRepo()
		svc := newSvc(chats, newStubUserRepo(owner, newMember))
		chat, _ := svc.CreateGroup(context.Background(), owner.ID, CreateGroupInput{Name: "g"})
		return chats, svc, chat
	}

	t.Run("owner adds new member", func(t *testing.T) {
		chats, svc, chat := setup()
		err := svc.AddMember(context.Background(), chat.ID, owner.ID, AddMemberInput{UserID: newMember.ID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ok, _ := chats.IsMember(context.Background(), chat.ID, newMember.ID)
		if !ok {
			t.Fatal("expected new member to be in chat")
		}
	})

	t.Run("cannot add already-present member", func(t *testing.T) {
		_, svc, chat := setup()
		_ = svc.AddMember(context.Background(), chat.ID, owner.ID, AddMemberInput{UserID: newMember.ID})

		err := svc.AddMember(context.Background(), chat.ID, owner.ID, AddMemberInput{UserID: newMember.ID})
		if !errors.Is(err, domain.ErrAlreadyChatMember) {
			t.Fatalf("want ErrAlreadyChatMember, got %v", err)
		}
	})

	t.Run("plain member cannot add others", func(t *testing.T) {
		chats := newStubChatRepo()
		plain := &domain.User{ID: uuid.New()}
		svc := newSvc(chats, newStubUserRepo(owner, plain, newMember))
		chat, _ := svc.CreateGroup(context.Background(), owner.ID, CreateGroupInput{
			Name:      "g",
			MemberIDs: []uuid.UUID{plain.ID},
		})

		err := svc.AddMember(context.Background(), chat.ID, plain.ID, AddMemberInput{UserID: newMember.ID})
		if !errors.Is(err, domain.ErrInsufficientRights) {
			t.Fatalf("want ErrInsufficientRights, got %v", err)
		}
	})

	t.Run("cannot add to direct chat", func(t *testing.T) {
		target := &domain.User{ID: uuid.New()}
		chats := newStubChatRepo()
		svc := newSvc(chats, newStubUserRepo(owner, target, newMember))
		direct, _, _ := svc.CreateDirect(context.Background(), owner.ID, CreateDirectInput{TargetUserID: target.ID})

		err := svc.AddMember(context.Background(), direct.ID, owner.ID, AddMemberInput{UserID: newMember.ID})
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("want ErrForbidden, got %v", err)
		}
	})
}

func TestChangeMemberRole(t *testing.T) {
	owner := &domain.User{ID: uuid.New()}
	member := &domain.User{ID: uuid.New()}

	setup := func() (*stubChatRepo, UseCase, *domain.Chat) {
		chats := newStubChatRepo()
		svc := newSvc(chats, newStubUserRepo(owner, member))
		chat, _ := svc.CreateGroup(context.Background(), owner.ID, CreateGroupInput{
			Name:      "g",
			MemberIDs: []uuid.UUID{member.ID},
		})
		return chats, svc, chat
	}

	t.Run("owner promotes member to admin", func(t *testing.T) {
		chats, svc, chat := setup()
		err := svc.ChangeMemberRole(context.Background(), chat.ID, owner.ID, ChangeMemberRoleInput{
			UserID: member.ID,
			Role:   "admin",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		m, _ := chats.GetMember(context.Background(), chat.ID, member.ID)
		if m.Role != domain.MemberRoleAdmin {
			t.Fatalf("expected admin role, got %s", m.Role)
		}
	})

	t.Run("non-owner cannot change roles", func(t *testing.T) {
		_, svc, chat := setup()
		err := svc.ChangeMemberRole(context.Background(), chat.ID, member.ID, ChangeMemberRoleInput{
			UserID: owner.ID,
			Role:   "member",
		})
		if !errors.Is(err, domain.ErrInsufficientRights) {
			t.Fatalf("want ErrInsufficientRights, got %v", err)
		}
	})

	t.Run("cannot change own role", func(t *testing.T) {
		_, svc, chat := setup()
		err := svc.ChangeMemberRole(context.Background(), chat.ID, owner.ID, ChangeMemberRoleInput{
			UserID: owner.ID,
			Role:   "member",
		})
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("want ErrForbidden, got %v", err)
		}
	})

	t.Run("invalid role fails validation", func(t *testing.T) {
		_, svc, chat := setup()
		err := svc.ChangeMemberRole(context.Background(), chat.ID, owner.ID, ChangeMemberRoleInput{
			UserID: member.ID,
			Role:   "superadmin",
		})
		if !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("want ErrValidation, got %v", err)
		}
	})
}

// Verify created_at / updated_at are populated on chat creation.
func TestCreateGroup_TimestampsSet(t *testing.T) {
	owner := &domain.User{ID: uuid.New()}
	chats := newStubChatRepo()
	svc := newSvc(chats, newStubUserRepo(owner))

	before := time.Now().Add(-time.Second)
	chat, _ := svc.CreateGroup(context.Background(), owner.ID, CreateGroupInput{Name: "g"})
	after := time.Now().Add(time.Second)

	if chat.CreatedAt.Before(before) || chat.CreatedAt.After(after) {
		t.Fatal("CreatedAt not set correctly")
	}
	if chat.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt should be set")
	}
}
