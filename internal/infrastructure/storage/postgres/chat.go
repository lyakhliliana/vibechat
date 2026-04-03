package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vibechat/internal/domain"
)

type chatRepository struct {
	db *pgxpool.Pool
}

func NewChatRepository(db *pgxpool.Pool) domain.ChatRepository {
	return &chatRepository{db: db}
}

// CreateWithMembers atomically inserts the chat and all initial members.
func (r *chatRepository) CreateWithMembers(ctx context.Context, c *domain.Chat, members []*domain.ChatMember) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	const insertChat = `
		INSERT INTO chats (id, type, name, avatar_url, description, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	if _, err = tx.Exec(ctx, insertChat,
		c.ID, c.Type, nullableStr(c.Name), nullableStr(c.AvatarURL), nullableStr(c.Description),
		c.CreatedBy, c.CreatedAt, c.UpdatedAt,
	); err != nil {
		return err
	}

	const insertMember = `INSERT INTO chat_members (chat_id, user_id, role, joined_at) VALUES ($1, $2, $3, $4)`
	for _, m := range members {
		if _, err = tx.Exec(ctx, insertMember, m.ChatID, m.UserID, m.Role, m.JoinedAt); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *chatRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Chat, error) {
	const q = `
		SELECT id, type, name, avatar_url, description, created_by, created_at, updated_at
		FROM chats
		WHERE id = $1 AND deleted_at IS NULL`

	c, err := r.scanChat(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrChatNotFound
		}
		return nil, err
	}
	return c, nil
}

func (r *chatRepository) Update(ctx context.Context, c *domain.Chat) error {
	const q = `
		UPDATE chats
		SET name = $2, avatar_url = $3, description = $4, updated_at = $5
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := r.db.Exec(ctx, q, c.ID, nullableStr(c.Name), nullableStr(c.AvatarURL), nullableStr(c.Description), c.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrChatNotFound
	}
	return nil
}

func (r *chatRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE chats SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.db.Exec(ctx, q, id)
	return err
}

func (r *chatRepository) AddMember(ctx context.Context, m *domain.ChatMember) error {
	const q = `
		INSERT INTO chat_members (chat_id, user_id, role, joined_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (chat_id, user_id) DO NOTHING`

	tag, err := r.db.Exec(ctx, q, m.ChatID, m.UserID, m.Role, m.JoinedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrAlreadyChatMember
	}
	return nil
}

func (r *chatRepository) RemoveMember(ctx context.Context, chatID, userID uuid.UUID) error {
	const q = `DELETE FROM chat_members WHERE chat_id = $1 AND user_id = $2`
	tag, err := r.db.Exec(ctx, q, chatID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotChatMember
	}
	return nil
}

func (r *chatRepository) GetMember(ctx context.Context, chatID, userID uuid.UUID) (*domain.ChatMember, error) {
	const q = `
		SELECT cm.chat_id, cm.user_id, cm.role, cm.joined_at,
		       u.id, u.username, u.email, u.password_hash, u.avatar_url, u.bio, u.status, u.last_seen, u.created_at, u.updated_at
		FROM chat_members cm
		JOIN users u ON u.id = cm.user_id AND u.deleted_at IS NULL
		WHERE cm.chat_id = $1 AND cm.user_id = $2`

	row := r.db.QueryRow(ctx, q, chatID, userID)
	m, err := scanMemberWithUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotChatMember
		}
		return nil, err
	}
	return m, nil
}

func (r *chatRepository) GetMembers(ctx context.Context, chatID uuid.UUID) ([]*domain.ChatMember, error) {
	const q = `
		SELECT cm.chat_id, cm.user_id, cm.role, cm.joined_at,
		       u.id, u.username, u.email, u.password_hash, u.avatar_url, u.bio, u.status, u.last_seen, u.created_at, u.updated_at
		FROM chat_members cm
		JOIN users u ON u.id = cm.user_id AND u.deleted_at IS NULL
		WHERE cm.chat_id = $1
		ORDER BY cm.joined_at, cm.user_id`

	rows, err := r.db.Query(ctx, q, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*domain.ChatMember
	for rows.Next() {
		m, err := scanMemberWithUser(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *chatRepository) UpdateMemberRole(ctx context.Context, chatID, userID uuid.UUID, role domain.MemberRole) error {
	const q = `UPDATE chat_members SET role = $3 WHERE chat_id = $1 AND user_id = $2`
	tag, err := r.db.Exec(ctx, q, chatID, userID, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotChatMember
	}
	return nil
}

func (r *chatRepository) GetUserChats(ctx context.Context, userID uuid.UUID) ([]*domain.ChatPreview, error) {
	const q = `
		WITH user_chats AS (
			SELECT c.id, c.type, c.name, c.avatar_url, c.description, c.created_by, c.created_at, c.updated_at
			FROM chats c
			JOIN chat_members cm ON cm.chat_id = c.id AND cm.user_id = $1
			WHERE c.deleted_at IS NULL
		),
		last_messages AS (
			SELECT DISTINCT ON (m.chat_id)
				m.id, m.chat_id, m.sender_id, m.content, m.type, m.created_at, m.updated_at
			FROM messages m
			WHERE m.chat_id IN (SELECT id FROM user_chats) AND m.deleted_at IS NULL
			ORDER BY m.chat_id, m.created_at DESC, m.id DESC
		),
		unread AS (
			SELECT m.chat_id, COUNT(*) AS cnt
			FROM messages m
			WHERE m.chat_id IN (SELECT id FROM user_chats)
			  AND m.sender_id <> $1
			  AND m.deleted_at IS NULL
			  AND m.created_at > COALESCE(
				  (SELECT crr.last_read_at FROM chat_read_receipts crr
				   WHERE crr.chat_id = m.chat_id AND crr.user_id = $1),
				  'epoch'
			  )
			GROUP BY m.chat_id
		)
		SELECT
			uc.id, uc.type, uc.name, uc.avatar_url, uc.description, uc.created_by, uc.created_at, uc.updated_at,
			lm.id, lm.sender_id, lm.content, lm.type, lm.created_at, lm.updated_at,
			COALESCE(u.cnt, 0) AS unread_count
		FROM user_chats uc
		LEFT JOIN last_messages lm ON lm.chat_id = uc.id
		LEFT JOIN unread u ON u.chat_id = uc.id
		ORDER BY COALESCE(lm.created_at, uc.created_at) DESC`

	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var previews []*domain.ChatPreview
	for rows.Next() {
		var (
			c                                        domain.Chat
			preview                                  domain.ChatPreview
			lastMsg                                  domain.Message
			chatName, chatAvatarURL, chatDescription *string
			lastMsgID                                *uuid.UUID
			lastSenderID                             *uuid.UUID
			lastContent                              *string
			lastType                                 *domain.MessageType
			lastMsgCreatedAt                         *time.Time
			lastMsgUpdatedAt                         *time.Time
		)

		if err := rows.Scan(
			&c.ID, &c.Type, &chatName, &chatAvatarURL, &chatDescription, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
			&lastMsgID, &lastSenderID, &lastContent, &lastType, &lastMsgCreatedAt, &lastMsgUpdatedAt,
			&preview.UnreadCount,
		); err != nil {
			return nil, err
		}

		if chatName != nil {
			c.Name = *chatName
		}
		if chatAvatarURL != nil {
			c.AvatarURL = *chatAvatarURL
		}
		if chatDescription != nil {
			c.Description = *chatDescription
		}

		preview.Chat = &c
		if lastMsgID != nil && lastSenderID != nil && lastContent != nil && lastType != nil {
			lastMsg.ID = *lastMsgID
			lastMsg.ChatID = c.ID
			lastMsg.SenderID = *lastSenderID
			lastMsg.Content = *lastContent
			lastMsg.Type = *lastType
			if lastMsgCreatedAt != nil {
				lastMsg.CreatedAt = *lastMsgCreatedAt
			}
			if lastMsgUpdatedAt != nil {
				lastMsg.UpdatedAt = *lastMsgUpdatedAt
			}
			preview.LastMessage = &lastMsg
		}

		previews = append(previews, &preview)
	}
	return previews, rows.Err()
}

func (r *chatRepository) GetDirectChat(ctx context.Context, user1ID, user2ID uuid.UUID) (*domain.Chat, error) {
	const q = `
		SELECT c.id, c.type, c.name, c.avatar_url, c.description, c.created_by, c.created_at, c.updated_at
		FROM chats c
		WHERE c.type = 'direct' AND c.deleted_at IS NULL
		  AND EXISTS (SELECT 1 FROM chat_members WHERE chat_id = c.id AND user_id = $1)
		  AND EXISTS (SELECT 1 FROM chat_members WHERE chat_id = c.id AND user_id = $2)
		LIMIT 1`

	c, err := r.scanChat(r.db.QueryRow(ctx, q, user1ID, user2ID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrChatNotFound
		}
		return nil, err
	}
	return c, nil
}

func (r *chatRepository) IsMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM chat_members WHERE chat_id = $1 AND user_id = $2)`
	var ok bool
	if err := r.db.QueryRow(ctx, q, chatID, userID).Scan(&ok); err != nil {
		return false, err
	}
	return ok, nil
}

type scanner interface {
	Scan(dest ...any) error
}

// nullableStr converts an empty Go string to nil so pgx sends NULL to postgres
// for optional text columns (name, avatar_url, description).
func nullableStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (r *chatRepository) scanChat(row scanner) (*domain.Chat, error) {
	var c domain.Chat
	var name, avatarURL, description *string
	err := row.Scan(
		&c.ID, &c.Type, &name, &avatarURL, &description,
		&c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if name != nil {
		c.Name = *name
	}
	if avatarURL != nil {
		c.AvatarURL = *avatarURL
	}
	if description != nil {
		c.Description = *description
	}
	return &c, nil
}

func scanMemberWithUser(row scanner) (*domain.ChatMember, error) {
	var m domain.ChatMember
	var u domain.User
	err := row.Scan(
		&m.ChatID, &m.UserID, &m.Role, &m.JoinedAt,
		&u.ID, &u.Username, &u.Email, &u.PasswordHash,
		&u.AvatarURL, &u.Bio, &u.Status, &u.LastSeen, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	m.User = &u
	return &m, nil
}
