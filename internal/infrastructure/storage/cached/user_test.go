package cached

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"vibechat/internal/domain"
	"vibechat/internal/infrastructure/cache/mock"
)

// countingUserRepo wraps a map-backed store and counts GetByID calls.
type countingUserRepo struct {
	users      map[uuid.UUID]*domain.User
	getByIDCnt int
}

func newCountingUserRepo(seed ...*domain.User) *countingUserRepo {
	r := &countingUserRepo{users: make(map[uuid.UUID]*domain.User)}
	for _, u := range seed {
		r.users[u.ID] = u
	}
	return r
}

func (r *countingUserRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	r.getByIDCnt++
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

func (r *countingUserRepo) Create(_ context.Context, u *domain.User) error {
	r.users[u.ID] = u
	return nil
}
func (r *countingUserRepo) GetByEmail(_ context.Context, _ string) (*domain.User, error) {
	return nil, domain.ErrUserNotFound
}
func (r *countingUserRepo) GetByUsername(_ context.Context, _ string) (*domain.User, error) {
	return nil, domain.ErrUserNotFound
}
func (r *countingUserRepo) Update(_ context.Context, u *domain.User) error {
	r.users[u.ID] = u
	return nil
}
func (r *countingUserRepo) UpdateStatus(_ context.Context, id uuid.UUID, status domain.UserStatus, _ time.Time) error {
	if u, ok := r.users[id]; ok {
		u.Status = status
	}
	return nil
}
func (r *countingUserRepo) Search(_ context.Context, _ string, _, _ int) ([]*domain.User, error) {
	return nil, nil
}
func (r *countingUserRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(r.users, id)
	return nil
}

const testTTL = 5 * time.Minute

func newCachedUserRepo(origin *countingUserRepo) domain.UserRepository {
	return NewUserRepository(origin, mock.New(), testTTL)
}

func TestCachedUser_GetByID_Miss(t *testing.T) {
	u := &domain.User{ID: uuid.New(), Username: "alice", Email: "alice@example.com"}
	origin := newCountingUserRepo(u)
	repo := newCachedUserRepo(origin)

	got, err := repo.GetByID(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != u.ID {
		t.Fatal("returned wrong user")
	}
	if origin.getByIDCnt != 1 {
		t.Fatalf("expected 1 origin call on miss, got %d", origin.getByIDCnt)
	}
}

func TestCachedUser_GetByID_HitAfterMiss(t *testing.T) {
	u := &domain.User{ID: uuid.New(), Username: "alice", Email: "alice@example.com"}
	origin := newCountingUserRepo(u)
	repo := newCachedUserRepo(origin)

	_, _ = repo.GetByID(context.Background(), u.ID) // miss — populates cache
	_, _ = repo.GetByID(context.Background(), u.ID) // hit — must not call origin again
	_, _ = repo.GetByID(context.Background(), u.ID) // hit

	if origin.getByIDCnt != 1 {
		t.Fatalf("expected exactly 1 origin call for 3 reads, got %d", origin.getByIDCnt)
	}
}

func TestCachedUser_NotFound_NotCached(t *testing.T) {
	origin := newCountingUserRepo() // empty
	repo := newCachedUserRepo(origin)

	id := uuid.New()
	_, err1 := repo.GetByID(context.Background(), id)
	_, err2 := repo.GetByID(context.Background(), id)

	if !errors.Is(err1, domain.ErrUserNotFound) || !errors.Is(err2, domain.ErrUserNotFound) {
		t.Fatal("expected ErrUserNotFound for both calls")
	}
	// not-found results must NOT be cached — origin must be hit both times
	if origin.getByIDCnt != 2 {
		t.Fatalf("not-found should not be cached; expected 2 origin calls, got %d", origin.getByIDCnt)
	}
}

func TestCachedUser_Update_InvalidatesCache(t *testing.T) {
	u := &domain.User{ID: uuid.New(), Username: "alice", Email: "alice@example.com"}
	origin := newCountingUserRepo(u)
	repo := newCachedUserRepo(origin)

	_, _ = repo.GetByID(context.Background(), u.ID) // miss — populates cache

	u.Username = "alice2"
	_ = repo.Update(context.Background(), u) // must delete cache entry

	got, _ := repo.GetByID(context.Background(), u.ID) // must hit origin again
	if origin.getByIDCnt != 2 {
		t.Fatalf("expected 2 origin calls after Update invalidation, got %d", origin.getByIDCnt)
	}
	if got.Username != "alice2" {
		t.Fatal("expected updated username after cache invalidation")
	}
}

func TestCachedUser_UpdateStatus_InvalidatesCache(t *testing.T) {
	u := &domain.User{ID: uuid.New(), Status: domain.UserStatusOffline}
	origin := newCountingUserRepo(u)
	repo := newCachedUserRepo(origin)

	_, _ = repo.GetByID(context.Background(), u.ID) // miss

	_ = repo.UpdateStatus(context.Background(), u.ID, domain.UserStatusOnline, time.Now())

	_, _ = repo.GetByID(context.Background(), u.ID) // must go to origin
	if origin.getByIDCnt != 2 {
		t.Fatalf("expected 2 origin calls after UpdateStatus, got %d", origin.getByIDCnt)
	}
}

func TestCachedUser_Delete_InvalidatesCache(t *testing.T) {
	u := &domain.User{ID: uuid.New()}
	origin := newCountingUserRepo(u)
	repo := newCachedUserRepo(origin)

	_, _ = repo.GetByID(context.Background(), u.ID) // miss — populates cache

	_ = repo.Delete(context.Background(), u.ID) // removes from origin + cache

	_, err := repo.GetByID(context.Background(), u.ID)
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound after delete, got %v", err)
	}
	// origin called on miss + again after delete
	if origin.getByIDCnt != 2 {
		t.Fatalf("expected 2 origin calls, got %d", origin.getByIDCnt)
	}
}

func TestCachedUser_NilCache_PassesThrough(t *testing.T) {
	u := &domain.User{ID: uuid.New()}
	origin := newCountingUserRepo(u)

	// nil cache → must return origin directly, not the wrapper
	repo := NewUserRepository(origin, nil, testTTL)

	got, err := repo.GetByID(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != u.ID {
		t.Fatal("wrong user returned")
	}
}
