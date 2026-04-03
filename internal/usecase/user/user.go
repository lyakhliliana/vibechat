package user

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"vibechat/internal/domain"
)

type tokenManager interface {
	GenerateAccessToken(userID uuid.UUID) (string, error)
	GenerateRefreshToken(userID uuid.UUID) (string, error)
	ValidateRefreshToken(token string) (uuid.UUID, error)
}

type passwordHasher interface {
	Hash(password string) (string, error)
	Check(password, hash string) bool
}

type service struct {
	users  domain.UserRepository
	hasher passwordHasher
	jwt    tokenManager
}

func New(users domain.UserRepository, h passwordHasher, j tokenManager) UseCase {
	return &service{users: users, hasher: h, jwt: j}
}

func (s *service) Register(ctx context.Context, in RegisterInput) (*domain.User, *TokenPair, error) {
	if err := in.Validate(); err != nil {
		return nil, nil, err
	}

	if _, err := s.users.GetByEmail(ctx, in.Email); err == nil {
		return nil, nil, domain.ErrEmailTaken
	} else if !errors.Is(err, domain.ErrUserNotFound) {
		return nil, nil, err
	}

	if _, err := s.users.GetByUsername(ctx, in.Username); err == nil {
		return nil, nil, domain.ErrUsernameTaken
	} else if !errors.Is(err, domain.ErrUserNotFound) {
		return nil, nil, err
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, nil, domain.ErrInternal
	}

	now := time.Now().UTC()
	u := &domain.User{
		ID:           uuid.New(),
		Username:     in.Username,
		Email:        in.Email,
		PasswordHash: hash,
		Status:       domain.UserStatusOffline,
		LastSeen:     now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err = s.users.Create(ctx, u); err != nil {
		return nil, nil, err
	}

	tokens, err := s.issueTokens(u.ID)
	if err != nil {
		_ = s.users.Delete(ctx, u.ID)
		return nil, nil, err
	}

	zerolog.Ctx(ctx).Info().
		Str("user_id", u.ID.String()).
		Str("username", u.Username).
		Msg("user registered")

	return u, tokens, nil
}

func (s *service) Login(ctx context.Context, in LoginInput) (*domain.User, *TokenPair, error) {
	if err := in.Validate(); err != nil {
		return nil, nil, err
	}

	u, err := s.users.GetByEmail(ctx, in.Email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, nil, domain.ErrInvalidCredentials
		}
		return nil, nil, err
	}

	if !s.hasher.Check(in.Password, u.PasswordHash) {
		return nil, nil, domain.ErrInvalidCredentials
	}

	tokens, err := s.issueTokens(u.ID)
	if err != nil {
		return nil, nil, err
	}

	zerolog.Ctx(ctx).Info().
		Str("user_id", u.ID.String()).
		Str("username", u.Username).
		Msg("user logged in")

	return u, tokens, nil
}

func (s *service) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	userID, err := s.jwt.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, err // domain.ErrExpiredToken or domain.ErrInvalidToken
	}

	if _, err = s.users.GetByID(ctx, userID); err != nil {
		return nil, err
	}

	tokens, err := s.issueTokens(userID)
	if err != nil {
		return nil, err
	}

	zerolog.Ctx(ctx).Debug().Str("user_id", userID.String()).Msg("token refreshed")
	return tokens, nil
}

func (s *service) GetProfile(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.users.GetByID(ctx, id)
}

func (s *service) UpdateProfile(ctx context.Context, id uuid.UUID, in UpdateProfileInput) (*domain.User, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}

	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// pointer == nil means "leave unchanged"
	if in.Username != nil && *in.Username != u.Username {
		existing, err := s.users.GetByUsername(ctx, *in.Username)
		switch {
		case err == nil && existing.ID != id:
			return nil, domain.ErrUsernameTaken
		case err != nil && !errors.Is(err, domain.ErrUserNotFound):
			return nil, err
		}
		u.Username = *in.Username
	}
	if in.Bio != nil {
		u.Bio = *in.Bio
	}
	if in.AvatarURL != nil {
		u.AvatarURL = *in.AvatarURL
	}
	u.UpdatedAt = time.Now().UTC()

	if err = s.users.Update(ctx, u); err != nil {
		return nil, err
	}

	zerolog.Ctx(ctx).Debug().
		Str("user_id", id.String()).
		Msg("profile updated")

	return u, nil
}

func (s *service) Search(ctx context.Context, in SearchInput) ([]*domain.User, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	return s.users.Search(ctx, in.Query, in.Limit, in.Offset)
}

func (s *service) SetOnline(ctx context.Context, id uuid.UUID) error {
	zerolog.Ctx(ctx).Debug().Str("user_id", id.String()).Msg("user online")
	return s.users.UpdateStatus(ctx, id, domain.UserStatusOnline, time.Now().UTC())
}

func (s *service) SetOffline(ctx context.Context, id uuid.UUID) error {
	zerolog.Ctx(ctx).Debug().Str("user_id", id.String()).Msg("user offline")
	return s.users.UpdateStatus(ctx, id, domain.UserStatusOffline, time.Now().UTC())
}

func (s *service) issueTokens(userID uuid.UUID) (*TokenPair, error) {
	access, err := s.jwt.GenerateAccessToken(userID)
	if err != nil {
		return nil, domain.ErrInternal
	}
	refresh, err := s.jwt.GenerateRefreshToken(userID)
	if err != nil {
		return nil, domain.ErrInternal
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}
