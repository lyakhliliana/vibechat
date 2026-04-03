package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"vibechat/internal/domain"
)

type userRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) domain.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, u *domain.User) error {
	const q = `
		INSERT INTO users (id, username, email, password_hash, avatar_url, bio, status, last_seen, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.db.Exec(ctx, q,
		u.ID, u.Username, u.Email, u.PasswordHash,
		u.AvatarURL, u.Bio, u.Status, u.LastSeen, u.CreatedAt, u.UpdatedAt,
	)
	return mapUserConstraintErr(err)
}

// mapUserConstraintErr maps postgres unique-constraint violations to domain errors.
func mapUserConstraintErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "uidx_users_email":
			return domain.ErrEmailTaken
		case "uidx_users_username":
			return domain.ErrUsernameTaken
		}
	}
	return err
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	const q = `
		SELECT id, username, email, password_hash, avatar_url, bio, status, last_seen, created_at, updated_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL`

	return r.scanOne(ctx, q, id)
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	const q = `
		SELECT id, username, email, password_hash, avatar_url, bio, status, last_seen, created_at, updated_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL`

	return r.scanOne(ctx, q, email)
}

func (r *userRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	const q = `
		SELECT id, username, email, password_hash, avatar_url, bio, status, last_seen, created_at, updated_at
		FROM users
		WHERE username = $1 AND deleted_at IS NULL`

	return r.scanOne(ctx, q, username)
}

func (r *userRepository) Update(ctx context.Context, u *domain.User) error {
	const q = `
		UPDATE users
		SET username = $2, email = $3, avatar_url = $4, bio = $5, updated_at = $6
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := r.db.Exec(ctx, q, u.ID, u.Username, u.Email, u.AvatarURL, u.Bio, u.UpdatedAt)
	if err != nil {
		return mapUserConstraintErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *userRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.UserStatus, lastSeen time.Time) error {
	const q = `UPDATE users SET status = $2, last_seen = $3, updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.db.Exec(ctx, q, id, status, lastSeen)
	return err
}

func (r *userRepository) Search(ctx context.Context, query string, limit, offset int) ([]*domain.User, error) {
	const q = `
		SELECT id, username, email, avatar_url, bio, status, last_seen, created_at, updated_at
		FROM users
		WHERE (username ILIKE '%' || $1 || '%' OR email ILIKE '%' || $1 || '%')
		  AND deleted_at IS NULL
		ORDER BY username
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, q, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanUsersPublic(rows)
}

func (r *userRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE users SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.db.Exec(ctx, q, id)
	return err
}

func (r *userRepository) scanOne(ctx context.Context, query string, arg any) (*domain.User, error) {
	row := r.db.QueryRow(ctx, query, arg)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return u, nil
}

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	err := row.Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash,
		&u.AvatarURL, &u.Bio, &u.Status, &u.LastSeen,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// scanUsersPublic scans rows that do NOT include password_hash (e.g. search results).
func scanUsersPublic(rows pgx.Rows) ([]*domain.User, error) {
	var users []*domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(
			&u.ID, &u.Username, &u.Email,
			&u.AvatarURL, &u.Bio, &u.Status, &u.LastSeen,
			&u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, rows.Err()
}
