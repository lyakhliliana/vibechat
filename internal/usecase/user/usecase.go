package user

import (
	"context"

	"github.com/google/uuid"

	"vibechat/internal/domain"
)

type UseCase interface {
	Register(ctx context.Context, input RegisterInput) (*domain.User, *TokenPair, error)
	Login(ctx context.Context, input LoginInput) (*domain.User, *TokenPair, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
	GetProfile(ctx context.Context, id uuid.UUID) (*domain.User, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, input UpdateProfileInput) (*domain.User, error)
	Search(ctx context.Context, input SearchInput) ([]*domain.User, error)
	SetOnline(ctx context.Context, id uuid.UUID) error
	SetOffline(ctx context.Context, id uuid.UUID) error
}
