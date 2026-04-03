package user

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vibechat/internal/domain"
)

// --- stubs ---

type stubUserRepo struct {
	byID       map[uuid.UUID]*domain.User
	byEmail    map[string]*domain.User
	byUsername map[string]*domain.User
}

func newStubUserRepo(seed ...*domain.User) *stubUserRepo {
	r := &stubUserRepo{
		byID:       make(map[uuid.UUID]*domain.User),
		byEmail:    make(map[string]*domain.User),
		byUsername: make(map[string]*domain.User),
	}
	for _, u := range seed {
		r.byID[u.ID] = u
		r.byEmail[u.Email] = u
		r.byUsername[u.Username] = u
	}
	return r
}

func (r *stubUserRepo) Create(_ context.Context, u *domain.User) error {
	if _, ok := r.byEmail[u.Email]; ok {
		return domain.ErrEmailTaken
	}
	if _, ok := r.byUsername[u.Username]; ok {
		return domain.ErrUsernameTaken
	}
	r.byID[u.ID] = u
	r.byEmail[u.Email] = u
	r.byUsername[u.Username] = u
	return nil
}

func (r *stubUserRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

func (r *stubUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	u, ok := r.byEmail[email]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

func (r *stubUserRepo) GetByUsername(_ context.Context, username string) (*domain.User, error) {
	u, ok := r.byUsername[username]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

func (r *stubUserRepo) Update(_ context.Context, u *domain.User) error {
	r.byID[u.ID] = u
	return nil
}

func (r *stubUserRepo) UpdateStatus(_ context.Context, id uuid.UUID, status domain.UserStatus, _ time.Time) error {
	if u, ok := r.byID[id]; ok {
		u.Status = status
	}
	return nil
}

func (r *stubUserRepo) Search(_ context.Context, _ string, _, _ int) ([]*domain.User, error) {
	return nil, nil
}

func (r *stubUserRepo) Delete(_ context.Context, id uuid.UUID) error {
	u, ok := r.byID[id]
	if !ok {
		return domain.ErrUserNotFound
	}
	delete(r.byID, id)
	delete(r.byEmail, u.Email)
	delete(r.byUsername, u.Username)
	return nil
}

type stubHasher struct{}

func (stubHasher) Hash(pw string) (string, error) { return "h:" + pw, nil }
func (stubHasher) Check(pw, hash string) bool     { return hash == "h:"+pw }

type stubTokens struct{}

func (stubTokens) GenerateAccessToken(id uuid.UUID) (string, error) {
	return "access:" + id.String(), nil
}
func (stubTokens) GenerateRefreshToken(id uuid.UUID) (string, error) {
	return "refresh:" + id.String(), nil
}
func (stubTokens) ValidateRefreshToken(token string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimPrefix(token, "refresh:"))
	if err != nil {
		return uuid.Nil, domain.ErrInvalidToken
	}
	return id, nil
}

func newService(repo *stubUserRepo) UseCase {
	return New(repo, stubHasher{}, stubTokens{})
}

// --- tests ---

func TestRegister(t *testing.T) {
	existingUser := &domain.User{
		ID:           uuid.New(),
		Email:        "taken@example.com",
		Username:     "takenuser",
		PasswordHash: "h:password1",
	}

	tests := []struct {
		name    string
		in      RegisterInput
		wantErr error
	}{
		{
			name:    "valid",
			in:      RegisterInput{Username: "alice", Email: "alice@example.com", Password: "secret123"},
			wantErr: nil,
		},
		{
			name:    "invalid email",
			in:      RegisterInput{Username: "alice", Email: "not-an-email", Password: "secret123"},
			wantErr: domain.ErrValidation,
		},
		{
			name:    "username too short",
			in:      RegisterInput{Username: "ab", Email: "alice@example.com", Password: "secret123"},
			wantErr: domain.ErrValidation,
		},
		{
			name:    "password too short",
			in:      RegisterInput{Username: "alice", Email: "alice@example.com", Password: "short"},
			wantErr: domain.ErrValidation,
		},
		{
			name:    "duplicate email",
			in:      RegisterInput{Username: "newuser", Email: existingUser.Email, Password: "secret123"},
			wantErr: domain.ErrEmailTaken,
		},
		{
			name:    "duplicate username",
			in:      RegisterInput{Username: existingUser.Username, Email: "new@example.com", Password: "secret123"},
			wantErr: domain.ErrUsernameTaken,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newService(newStubUserRepo(existingUser))
			u, tokens, err := svc.Register(context.Background(), tc.in)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("want error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if u.Username != tc.in.Username || u.Email != tc.in.Email {
				t.Fatalf("returned user fields mismatch")
			}
			if tokens.AccessToken == "" || tokens.RefreshToken == "" {
				t.Fatal("expected non-empty tokens")
			}
		})
	}
}

func TestLogin(t *testing.T) {
	const password = "correct-password"
	existing := &domain.User{
		ID:           uuid.New(),
		Email:        "user@example.com",
		Username:     "someuser",
		PasswordHash: "h:" + password,
	}

	tests := []struct {
		name    string
		in      LoginInput
		wantErr error
	}{
		{
			name:    "valid",
			in:      LoginInput{Email: existing.Email, Password: password},
			wantErr: nil,
		},
		{
			name:    "wrong password",
			in:      LoginInput{Email: existing.Email, Password: "wrong"},
			wantErr: domain.ErrInvalidCredentials,
		},
		{
			name:    "unknown email",
			in:      LoginInput{Email: "nobody@example.com", Password: password},
			wantErr: domain.ErrInvalidCredentials,
		},
		{
			name:    "empty email",
			in:      LoginInput{Email: "", Password: password},
			wantErr: domain.ErrValidation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newService(newStubUserRepo(existing))
			u, tokens, err := svc.Login(context.Background(), tc.in)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("want error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if u.ID != existing.ID {
				t.Fatal("returned wrong user")
			}
			if tokens.AccessToken == "" || tokens.RefreshToken == "" {
				t.Fatal("expected non-empty tokens")
			}
		})
	}
}

func ptr(s string) *string { return &s }

func TestUpdateProfile(t *testing.T) {
	existing := &domain.User{ID: uuid.New(), Username: "alice", Email: "alice@example.com", PasswordHash: "h:pass"}
	other := &domain.User{ID: uuid.New(), Username: "taken", Email: "taken@example.com", PasswordHash: "h:pass"}

	tests := []struct {
		name    string
		in      UpdateProfileInput
		check   func(*testing.T, *domain.User)
		wantErr error
	}{
		{
			name: "update bio and avatar",
			in:   UpdateProfileInput{Bio: ptr("new bio"), AvatarURL: ptr("https://example.com/a.png")},
			check: func(t *testing.T, u *domain.User) {
				if u.Bio != "new bio" || u.AvatarURL != "https://example.com/a.png" {
					t.Fatal("fields not updated")
				}
			},
		},
		{
			name: "change username",
			in:   UpdateProfileInput{Username: ptr("alice2")},
			check: func(t *testing.T, u *domain.User) {
				if u.Username != "alice2" {
					t.Fatalf("want alice2, got %q", u.Username)
				}
			},
		},
		{
			name: "nil fields leave values unchanged",
			in:   UpdateProfileInput{},
			check: func(t *testing.T, u *domain.User) {
				if u.Username != existing.Username {
					t.Fatalf("username changed: want %q, got %q", existing.Username, u.Username)
				}
				if u.Email != existing.Email {
					t.Fatalf("email changed: want %q, got %q", existing.Email, u.Email)
				}
			},
		},
		{
			name:    "invalid avatar URL",
			in:      UpdateProfileInput{AvatarURL: ptr("not-a-url")},
			wantErr: domain.ErrValidation,
		},
		{
			name:    "username taken by other user",
			in:      UpdateProfileInput{Username: ptr("taken")},
			wantErr: domain.ErrUsernameTaken,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newService(newStubUserRepo(existing, other))
			u, err := svc.UpdateProfile(context.Background(), existing.ID, tc.in)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("want %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tc.check(t, u)
		})
	}
}

func TestSetOnlineOffline(t *testing.T) {
	u := &domain.User{ID: uuid.New(), Status: domain.UserStatusOffline}
	repo := newStubUserRepo(u)
	svc := newService(repo)

	_ = svc.SetOnline(context.Background(), u.ID)
	if repo.byID[u.ID].Status != domain.UserStatusOnline {
		t.Fatal("expected Online")
	}

	_ = svc.SetOffline(context.Background(), u.ID)
	if repo.byID[u.ID].Status != domain.UserStatusOffline {
		t.Fatal("expected Offline")
	}
}

func TestRefreshToken(t *testing.T) {
	existing := &domain.User{ID: uuid.New(), Email: "u@example.com", Username: "u"}
	svc := newService(newStubUserRepo(existing))

	tokens, err := svc.RefreshToken(context.Background(), "refresh:"+existing.ID.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokens.AccessToken == "" {
		t.Fatal("expected non-empty access token")
	}

	_, err = svc.RefreshToken(context.Background(), "invalid-token")
	if !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}
