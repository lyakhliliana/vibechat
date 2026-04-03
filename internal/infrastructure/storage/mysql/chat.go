package mysql

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"

	"vibechat/internal/domain"
)

type chatRepository struct {
	db *sql.DB
}

func NewChatRepository(db *sql.DB) domain.ChatRepository {
	return &chatRepository{db: db}
}

// CreateWithMembers atomically inserts the chat and all initial members.
func (r *chatRepository) CreateWithMembers(ctx context.Context, c *domain.Chat, members []*domain.ChatMember) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const insertChat = `
		INSERT INTO chats (id, type, name, avatar_url, description, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err = tx.ExecContext(ctx, insertChat,
		c.ID.String(), c.Type,
		nullableStr(c.Name), nullableStr(c.AvatarURL), nullableStr(c.Description),
		c.CreatedBy.String(), c.CreatedAt, c.UpdatedAt,
	); err != nil {
		return err
	}

	const insertMember = `INSERT INTO chat_members (chat_id, user_id, role, joined_at) VALUES (?, ?, ?, ?)`
	for _, m := range members {
		if _, err = tx.ExecContext(ctx, insertMember,
			m.ChatID.String(), m.UserID.String(), m.Role, m.JoinedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *chatRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Chat, error) {
	const q = `
		SELECT id, type, name, avatar_url, description, created_by, created_at, updated_at
		FROM chats
		WHERE id = ? AND deleted_at IS NULL`

	c, err := scanChat(r.db.QueryRowContext(ctx, q, id.String()))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrChatNotFound
	}
	return c, err
}

func (r *chatRepository) Update(ctx context.Context, c *domain.Chat) error {
	const q = `
		UPDATE chats
		SET name = ?, avatar_url = ?, description = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL`

	res, err := r.db.ExecContext(ctx, q,
		nullableStr(c.Name), nullableStr(c.AvatarURL), nullableStr(c.Description),
		c.UpdatedAt, c.ID.String(),
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrChatNotFound
	}
	return nil
}

func (r *chatRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE chats SET deleted_at = NOW(6) WHERE id = ? AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, q, id.String())
	return err
}

func (r *chatRepository) AddMember(ctx context.Context, m *domain.ChatMember) error {
	// INSERT IGNORE silently skips the insert if the PK already exists.
	// We then check RowsAffected to distinguish "inserted" from "already a member".
	const q = `INSERT IGNORE INTO chat_members (chat_id, user_id, role, joined_at) VALUES (?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, q, m.ChatID.String(), m.UserID.String(), m.Role, m.JoinedAt)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrAlreadyChatMember
	}
	return nil
}

func (r *chatRepository) RemoveMember(ctx context.Context, chatID, userID uuid.UUID) error {
	const q = `DELETE FROM chat_members WHERE chat_id = ? AND user_id = ?`
	res, err := r.db.ExecContext(ctx, q, chatID.String(), userID.String())
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotChatMember
	}
	return nil
}

func (r *chatRepository) GetMember(ctx context.Context, chatID, userID uuid.UUID) (*domain.ChatMember, error) {
	const q = `
		SELECT cm.chat_id, cm.user_id, cm.role, cm.joined_at,
		       u.id, u.username, u.email, u.password_hash,
		       u.avatar_url, u.bio, u.status, u.last_seen, u.created_at, u.updated_at
		FROM chat_members cm
		JOIN users u ON u.id = cm.user_id AND u.deleted_at IS NULL
		WHERE cm.chat_id = ? AND cm.user_id = ?`

	m, err := scanMemberWithUser(r.db.QueryRowContext(ctx, q, chatID.String(), userID.String()))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotChatMember
	}
	return m, err
}

func (r *chatRepository) GetMembers(ctx context.Context, chatID uuid.UUID) ([]*domain.ChatMember, error) {
	const q = `
		SELECT cm.chat_id, cm.user_id, cm.role, cm.joined_at,
		       u.id, u.username, u.email, u.password_hash,
		       u.avatar_url, u.bio, u.status, u.last_seen, u.created_at, u.updated_at
		FROM chat_members cm
		JOIN users u ON u.id = cm.user_id AND u.deleted_at IS NULL
		WHERE cm.chat_id = ?
		ORDER BY cm.joined_at, cm.user_id`

	rows, err := r.db.QueryContext(ctx, q, chatID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*domain.ChatMember
	for rows.Next() {
		m, err := scanMemberWithUserRows(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *chatRepository) UpdateMemberRole(ctx context.Context, chatID, userID uuid.UUID, role domain.MemberRole) error {
	const q = `UPDATE chat_members SET role = ? WHERE chat_id = ? AND user_id = ?`
	res, err := r.db.ExecContext(ctx, q, role, chatID.String(), userID.String())
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotChatMember
	}
	return nil
}

// GetUserChats uses ROW_NUMBER() instead of PostgreSQL's DISTINCT ON.
func (r *chatRepository) GetUserChats(ctx context.Context, userID uuid.UUID) ([]*domain.ChatPreview, error) {
	const q = `
		WITH user_chats AS (
			SELECT c.id, c.type, c.name, c.avatar_url, c.description, c.created_by, c.created_at, c.updated_at
			FROM chats c
			JOIN chat_members cm ON cm.chat_id = c.id AND cm.user_id = ?
			WHERE c.deleted_at IS NULL
		),
		last_messages AS (
			SELECT id, chat_id, sender_id, content, type, created_at, updated_at
			FROM (
				SELECT m.id, m.chat_id, m.sender_id, m.content, m.type, m.created_at, m.updated_at,
				       ROW_NUMBER() OVER (PARTITION BY m.chat_id ORDER BY m.created_at DESC, m.id DESC) AS rn
				FROM messages m
				WHERE m.chat_id IN (SELECT id FROM user_chats) AND m.deleted_at IS NULL
			) ranked
			WHERE rn = 1
		),
		unread AS (
			SELECT m.chat_id, COUNT(*) AS cnt
			FROM messages m
			WHERE m.chat_id IN (SELECT id FROM user_chats)
			  AND m.sender_id <> ?
			  AND m.deleted_at IS NULL
			  AND m.created_at > COALESCE(
			      (SELECT crr.last_read_at FROM chat_read_receipts crr
			       WHERE crr.chat_id = m.chat_id AND crr.user_id = ?),
			      '1970-01-01 00:00:00'
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

	rows, err := r.db.QueryContext(ctx, q, userID.String(), userID.String(), userID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var previews []*domain.ChatPreview
	for rows.Next() {
		var (
			c                                  domain.Chat
			preview                            domain.ChatPreview
			lastMsg                            domain.Message
			chatName, chatAvatarURL, chatDesc  sql.NullString
			lastMsgID, lastSenderID            sql.NullString
			lastContent                        sql.NullString
			lastType                           sql.NullString
			lastMsgCreatedAt, lastMsgUpdatedAt sql.NullTime
		)

		if err := rows.Scan(
			&c.ID, &c.Type, &chatName, &chatAvatarURL, &chatDesc, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
			&lastMsgID, &lastSenderID, &lastContent, &lastType, &lastMsgCreatedAt, &lastMsgUpdatedAt,
			&preview.UnreadCount,
		); err != nil {
			return nil, err
		}

		c.Name = chatName.String
		c.AvatarURL = chatAvatarURL.String
		c.Description = chatDesc.String
		preview.Chat = &c

		if lastMsgID.Valid && lastSenderID.Valid && lastContent.Valid && lastType.Valid {
			lastMsg.ID = uuid.MustParse(lastMsgID.String)
			lastMsg.ChatID = c.ID
			lastMsg.SenderID = uuid.MustParse(lastSenderID.String)
			lastMsg.Content = lastContent.String
			lastMsg.Type = domain.MessageType(lastType.String)
			if lastMsgCreatedAt.Valid {
				lastMsg.CreatedAt = lastMsgCreatedAt.Time
			}
			if lastMsgUpdatedAt.Valid {
				lastMsg.UpdatedAt = lastMsgUpdatedAt.Time
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
		  AND EXISTS (SELECT 1 FROM chat_members WHERE chat_id = c.id AND user_id = ?)
		  AND EXISTS (SELECT 1 FROM chat_members WHERE chat_id = c.id AND user_id = ?)
		LIMIT 1`

	c, err := scanChat(r.db.QueryRowContext(ctx, q, user1ID.String(), user2ID.String()))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrChatNotFound
	}
	return c, err
}

func (r *chatRepository) IsMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM chat_members WHERE chat_id = ? AND user_id = ?)`
	var exists int64
	if err := r.db.QueryRowContext(ctx, q, chatID.String(), userID.String()).Scan(&exists); err != nil {
		return false, err
	}
	return exists == 1, nil
}

// ---------------------------------------------------------------------------
// Scan helpers
// ---------------------------------------------------------------------------

func scanChat(row *sql.Row) (*domain.Chat, error) {
	var c domain.Chat
	var name, avatarURL, description sql.NullString
	err := row.Scan(
		&c.ID, &c.Type, &name, &avatarURL, &description,
		&c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	c.Name = name.String
	c.AvatarURL = avatarURL.String
	c.Description = description.String
	return &c, nil
}

func scanMemberWithUser(row *sql.Row) (*domain.ChatMember, error) {
	var m domain.ChatMember
	var u domain.User
	var avatarURL, bio sql.NullString
	err := row.Scan(
		&m.ChatID, &m.UserID, &m.Role, &m.JoinedAt,
		&u.ID, &u.Username, &u.Email, &u.PasswordHash,
		&avatarURL, &bio, &u.Status, &u.LastSeen, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	u.AvatarURL = avatarURL.String
	u.Bio = bio.String
	m.User = &u
	return &m, nil
}

func scanMemberWithUserRows(rows *sql.Rows) (*domain.ChatMember, error) {
	var m domain.ChatMember
	var u domain.User
	var avatarURL, bio sql.NullString
	err := rows.Scan(
		&m.ChatID, &m.UserID, &m.Role, &m.JoinedAt,
		&u.ID, &u.Username, &u.Email, &u.PasswordHash,
		&avatarURL, &bio, &u.Status, &u.LastSeen, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	u.AvatarURL = avatarURL.String
	u.Bio = bio.String
	m.User = &u
	return &m, nil
}
