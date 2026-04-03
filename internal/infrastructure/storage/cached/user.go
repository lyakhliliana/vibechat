package cached

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"vibechat/internal/domain"
	"vibechat/internal/infrastructure/cache"
)

type userRepo struct {
	origin domain.UserRepository
	cache  cache.Cache
	ttl    time.Duration
}

// NewUserRepository wraps origin with a read-through / write-invalidate cache.
// If c is nil, origin is returned as-is — caching is fully disabled with zero overhead.
// Only GetByID is cached — it is on the hot path of every authenticated request.
// All writes call through to origin first, then delete the cached entry so the
// next read fetches fresh data (write-invalidate, no dirty cache).
func NewUserRepository(origin domain.UserRepository, c cache.Cache, ttl time.Duration) domain.UserRepository {
	if c == nil {
		return origin
	}
	return &userRepo{origin: origin, cache: c, ttl: ttl}
}

func userKey(id uuid.UUID) string { return "user:" + id.String() }

func (r *userRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	key := userKey(id)
	if raw, err := r.cache.Get(ctx, key); err == nil {
		var u domain.User
		if json.Unmarshal([]byte(raw), &u) == nil {
			zerolog.Ctx(ctx).Trace().Str("user_id", id.String()).Msg("user cache hit")
			return &u, nil
		}
		// corrupted entry — fall through to origin and overwrite
	} else {
		zerolog.Ctx(ctx).Trace().Str("user_id", id.String()).Msg("user cache miss")
	}

	u, err := r.origin.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if b, merr := json.Marshal(u); merr == nil {
		if serr := r.cache.Set(ctx, key, string(b), r.ttl); serr != nil {
			zerolog.Ctx(ctx).Warn().Err(serr).Str("user_id", id.String()).Msg("user cache set failed")
		}
	}
	return u, nil
}

func (r *userRepo) Create(ctx context.Context, user *domain.User) error {
	return r.origin.Create(ctx, user)
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.origin.GetByEmail(ctx, email)
}

func (r *userRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return r.origin.GetByUsername(ctx, username)
}

func (r *userRepo) Update(ctx context.Context, user *domain.User) error {
	if err := r.origin.Update(ctx, user); err != nil {
		return err
	}
	if err := r.cache.Del(ctx, userKey(user.ID)); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Str("user_id", user.ID.String()).Msg("user cache invalidation failed")
	}
	return nil
}

func (r *userRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.UserStatus, lastSeen time.Time) error {
	if err := r.origin.UpdateStatus(ctx, id, status, lastSeen); err != nil {
		return err
	}
	if err := r.cache.Del(ctx, userKey(id)); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Str("user_id", id.String()).Msg("user cache invalidation failed")
	}
	return nil
}

func (r *userRepo) Search(ctx context.Context, query string, limit, offset int) ([]*domain.User, error) {
	return r.origin.Search(ctx, query, limit, offset)
}

func (r *userRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.origin.Delete(ctx, id); err != nil {
		return err
	}
	_ = r.cache.Del(ctx, userKey(id))
	return nil
}
