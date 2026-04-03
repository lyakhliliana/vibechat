package mysql

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"vibechat/internal/domain"
)

type userRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) domain.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, u *domain.User) error {
	const q = `
		INSERT INTO users
			(id, username, email, password_hash, avatar_url, bio, status, last_seen, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, q,
		u.ID.String(), u.Username, u.Email, u.PasswordHash,
		nullableStr(u.AvatarURL), nullableStr(u.Bio),
		u.Status, u.LastSeen, u.CreatedAt, u.UpdatedAt,
	)
	return mapUserConstraintErr(err)
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	const q = `
		SELECT id, username, email, password_hash, avatar_url, bio, status, last_seen, created_at, updated_at
		FROM users
		WHERE id = ? AND deleted_at IS NULL`

	return r.scanOne(ctx, q, id.String())
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	const q = `
		SELECT id, username, email, password_hash, avatar_url, bio, status, last_seen, created_at, updated_at
		FROM users
		WHERE email = ? AND deleted_at IS NULL`

	return r.scanOne(ctx, q, email)
}

func (r *userRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	const q = `
		SELECT id, username, email, password_hash, avatar_url, bio, status, last_seen, created_at, updated_at
		FROM users
		WHERE username = ? AND deleted_at IS NULL`

	return r.scanOne(ctx, q, username)
}

func (r *userRepository) Update(ctx context.Context, u *domain.User) error {
	const q = `
		UPDATE users
		SET username = ?, email = ?, avatar_url = ?, bio = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL`

	res, err := r.db.ExecContext(ctx, q,
		u.Username, u.Email,
		nullableStr(u.AvatarURL), nullableStr(u.Bio),
		u.UpdatedAt, u.ID.String(),
	)
	if err != nil {
		return mapUserConstraintErr(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *userRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.UserStatus, lastSeen time.Time) error {
	const q = `UPDATE users SET status = ?, last_seen = ?, updated_at = NOW(6) WHERE id = ? AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, q, status, lastSeen, id.String())
	return err
}

func (r *userRepository) Search(ctx context.Context, query string, limit, offset int) ([]*domain.User, error) {
	// MySQL LIKE is case-insensitive with utf8mb4_unicode_ci — no ILIKE needed.
	const q = `
		SELECT id, username, email, avatar_url, bio, status, last_seen, created_at, updated_at
		FROM users
		WHERE (username LIKE ? OR email LIKE ?)
		  AND deleted_at IS NULL
		ORDER BY username
		LIMIT ? OFFSET ?`

	pattern := "%" + query + "%"
	rows, err := r.db.QueryContext(ctx, q, pattern, pattern, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var u domain.User
		var avatarURL, bio sql.NullString
		if err := rows.Scan(
			&u.ID, &u.Username, &u.Email,
			&avatarURL, &bio,
			&u.Status, &u.LastSeen, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		u.AvatarURL = avatarURL.String
		u.Bio = bio.String
		users = append(users, &u)
	}
	return users, rows.Err()
}

func (r *userRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE users SET deleted_at = NOW(6) WHERE id = ? AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, q, id.String())
	return err
}

// scanOne runs q with a single arg and maps sql.ErrNoRows to ErrUserNotFound.
func (r *userRepository) scanOne(ctx context.Context, q string, arg any) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx, q, arg)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	return u, err
}

func scanUser(row *sql.Row) (*domain.User, error) {
	var u domain.User
	var avatarURL, bio sql.NullString
	err := row.Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash,
		&avatarURL, &bio,
		&u.Status, &u.LastSeen, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	u.AvatarURL = avatarURL.String
	u.Bio = bio.String
	return &u, nil
}

// mapUserConstraintErr maps MySQL duplicate-key errors to domain errors.
func mapUserConstraintErr(err error) error {
	if err == nil {
		return nil
	}
	var me *mysqldrv.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		msg := me.Message
		if strings.Contains(msg, "uidx_users_email") {
			return domain.ErrEmailTaken
		}
		if strings.Contains(msg, "uidx_users_username") {
			return domain.ErrUsernameTaken
		}
	}
	return err
}

// nullableStr converts an empty Go string to nil so the driver sends NULL
// for optional columns (avatar_url, bio, name, description).
func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
