package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"vibechat/internal/domain"
)

type messageRepository struct {
	db *sql.DB
}

func NewMessageRepository(db *sql.DB) domain.MessageRepository {
	return &messageRepository{db: db}
}

func (r *messageRepository) Create(ctx context.Context, m *domain.Message) error {
	const q = `
		INSERT INTO messages (id, chat_id, sender_id, content, type, reply_to_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	var replyToID any
	if m.ReplyToID != nil {
		replyToID = m.ReplyToID.String()
	}

	_, err := r.db.ExecContext(ctx, q,
		m.ID.String(), m.ChatID.String(), m.SenderID.String(),
		m.Content, m.Type, replyToID, m.CreatedAt, m.UpdatedAt,
	)
	return err
}

func (r *messageRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
	const q = `
		SELECT m.id, m.chat_id, m.sender_id, m.content, m.type, m.reply_to_id,
		       m.edited_at, m.deleted_at, m.created_at, m.updated_at
		FROM messages m
		WHERE m.id = ?`

	m, err := scanMessage(r.db.QueryRowContext(ctx, q, id.String()))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrMessageNotFound
	}
	return m, err
}

func (r *messageRepository) GetChatMessages(
	ctx context.Context,
	chatID uuid.UUID,
	cursor *domain.MessageCursor,
	limit int,
) ([]*domain.Message, error) {
	var (
		rows *sql.Rows
		err  error
	)

	if cursor == nil {
		const q = `
			SELECT m.id, m.chat_id, m.sender_id, m.content, m.type, m.reply_to_id,
			       m.edited_at, m.deleted_at, m.created_at, m.updated_at,
			       u.id, u.username, u.email, u.avatar_url, u.bio, u.status, u.last_seen, u.created_at, u.updated_at
			FROM messages m
			JOIN users u ON u.id = m.sender_id
			WHERE m.chat_id = ? AND m.deleted_at IS NULL
			ORDER BY m.created_at DESC, m.id DESC
			LIMIT ?`
		rows, err = r.db.QueryContext(ctx, q, chatID.String(), limit)
	} else {
		// MySQL 8.0+ supports row constructor comparisons: (a, b) < (x, y).
		const q = `
			SELECT m.id, m.chat_id, m.sender_id, m.content, m.type, m.reply_to_id,
			       m.edited_at, m.deleted_at, m.created_at, m.updated_at,
			       u.id, u.username, u.email, u.avatar_url, u.bio, u.status, u.last_seen, u.created_at, u.updated_at
			FROM messages m
			JOIN users u ON u.id = m.sender_id
			WHERE m.chat_id = ?
			  AND m.deleted_at IS NULL
			  AND (m.created_at, m.id) < (?, ?)
			ORDER BY m.created_at DESC, m.id DESC
			LIMIT ?`
		rows, err = r.db.QueryContext(ctx, q,
			chatID.String(), cursor.CreatedAt, cursor.ID.String(), limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*domain.Message
	for rows.Next() {
		m, err := scanMessageWithSender(rows)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (r *messageRepository) Update(ctx context.Context, m *domain.Message) error {
	const q = `
		UPDATE messages
		SET content = ?, edited_at = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL`

	res, err := r.db.ExecContext(ctx, q, m.Content, m.EditedAt, m.UpdatedAt, m.ID.String())
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrMessageNotFound
	}
	return nil
}

func (r *messageRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE messages SET deleted_at = NOW(6), updated_at = NOW(6) WHERE id = ? AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, q, id.String())
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrMessageNotFound
	}
	return nil
}

func (r *messageRepository) MarkRead(ctx context.Context, chatID, userID uuid.UUID) error {
	const q = `
		INSERT INTO chat_read_receipts (chat_id, user_id, last_read_at)
		VALUES (?, ?, NOW(6))
		ON DUPLICATE KEY UPDATE last_read_at = NOW(6)`

	_, err := r.db.ExecContext(ctx, q, chatID.String(), userID.String())
	return err
}

func (r *messageRepository) GetUnreadCount(ctx context.Context, chatID, userID uuid.UUID) (int, error) {
	const q = `
		SELECT COUNT(*)
		FROM messages m
		WHERE m.chat_id = ?
		  AND m.sender_id <> ?
		  AND m.deleted_at IS NULL
		  AND m.created_at > COALESCE(
		      (SELECT last_read_at FROM chat_read_receipts WHERE chat_id = ? AND user_id = ?),
		      '1970-01-01 00:00:00'
		  )`

	var count int
	if err := r.db.QueryRowContext(ctx, q, chatID.String(), userID.String(),
		chatID.String(), userID.String()).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *messageRepository) AddReaction(ctx context.Context, rx *domain.Reaction) error {
	// INSERT IGNORE skips silently if (message_id, user_id, emoji) already exists.
	const q = `
		INSERT IGNORE INTO message_reactions (message_id, user_id, emoji, created_at)
		VALUES (?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, q, rx.MessageID.String(), rx.UserID.String(), rx.Emoji, rx.CreatedAt)
	return err
}

func (r *messageRepository) RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error {
	const q = `DELETE FROM message_reactions WHERE message_id = ? AND user_id = ? AND emoji = ?`
	_, err := r.db.ExecContext(ctx, q, messageID.String(), userID.String(), emoji)
	return err
}

func (r *messageRepository) GetReactions(ctx context.Context, messageID uuid.UUID) ([]*domain.Reaction, error) {
	const q = `
		SELECT mr.message_id, mr.user_id, mr.emoji, mr.created_at,
		       u.id, u.username, u.avatar_url
		FROM message_reactions mr
		JOIN users u ON u.id = mr.user_id AND u.deleted_at IS NULL
		WHERE mr.message_id = ?
		ORDER BY mr.created_at, mr.user_id, mr.emoji`

	rows, err := r.db.QueryContext(ctx, q, messageID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reactions []*domain.Reaction
	for rows.Next() {
		var rx domain.Reaction
		var u domain.User
		var avatarURL sql.NullString
		if err := rows.Scan(
			&rx.MessageID, &rx.UserID, &rx.Emoji, &rx.CreatedAt,
			&u.ID, &u.Username, &avatarURL,
		); err != nil {
			return nil, err
		}
		u.AvatarURL = avatarURL.String
		rx.User = &u
		reactions = append(reactions, &rx)
	}
	return reactions, rows.Err()
}

// GetReactionsBatch builds a dynamic IN clause because MySQL does not support
// the PostgreSQL ANY($1 ::uuid[]) syntax.
func (r *messageRepository) GetReactionsBatch(
	ctx context.Context,
	messageIDs []uuid.UUID,
) (map[uuid.UUID][]*domain.Reaction, error) {
	if len(messageIDs) == 0 {
		return make(map[uuid.UUID][]*domain.Reaction), nil
	}

	placeholders := strings.Repeat("?,", len(messageIDs))
	placeholders = placeholders[:len(placeholders)-1]

	q := fmt.Sprintf(`
		SELECT mr.message_id, mr.user_id, mr.emoji, mr.created_at,
		       u.id, u.username, u.avatar_url
		FROM message_reactions mr
		JOIN users u ON u.id = mr.user_id AND u.deleted_at IS NULL
		WHERE mr.message_id IN (%s)
		ORDER BY mr.message_id, mr.created_at, mr.user_id, mr.emoji`, placeholders)

	args := make([]any, len(messageIDs))
	for i, id := range messageIDs {
		args[i] = id.String()
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]*domain.Reaction)
	for rows.Next() {
		var rx domain.Reaction
		var u domain.User
		var avatarURL sql.NullString
		if err := rows.Scan(
			&rx.MessageID, &rx.UserID, &rx.Emoji, &rx.CreatedAt,
			&u.ID, &u.Username, &avatarURL,
		); err != nil {
			return nil, err
		}
		u.AvatarURL = avatarURL.String
		rx.User = &u
		result[rx.MessageID] = append(result[rx.MessageID], &rx)
	}
	return result, rows.Err()
}

// ---------------------------------------------------------------------------
// Scan helpers
// ---------------------------------------------------------------------------

func scanMessage(row *sql.Row) (*domain.Message, error) {
	var m domain.Message
	var replyToID sql.NullString
	var editedAt, deletedAt sql.NullTime
	err := row.Scan(
		&m.ID, &m.ChatID, &m.SenderID, &m.Content, &m.Type,
		&replyToID, &editedAt, &deletedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if replyToID.Valid {
		id := uuid.MustParse(replyToID.String)
		m.ReplyToID = &id
	}
	if editedAt.Valid {
		m.EditedAt = &editedAt.Time
	}
	if deletedAt.Valid {
		m.DeletedAt = &deletedAt.Time
	}
	return &m, nil
}

// scanMessageWithSender scans a message row that includes a JOIN to users.
// password_hash is intentionally excluded from the SELECT.
func scanMessageWithSender(rows *sql.Rows) (*domain.Message, error) {
	var m domain.Message
	var u domain.User
	var replyToID sql.NullString
	var editedAt, deletedAt sql.NullTime
	var avatarURL, bio sql.NullString
	err := rows.Scan(
		&m.ID, &m.ChatID, &m.SenderID, &m.Content, &m.Type,
		&replyToID, &editedAt, &deletedAt, &m.CreatedAt, &m.UpdatedAt,
		&u.ID, &u.Username, &u.Email, &avatarURL, &bio,
		&u.Status, &u.LastSeen, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if replyToID.Valid {
		id := uuid.MustParse(replyToID.String)
		m.ReplyToID = &id
	}
	if editedAt.Valid {
		m.EditedAt = &editedAt.Time
	}
	if deletedAt.Valid {
		m.DeletedAt = &deletedAt.Time
	}
	u.AvatarURL = avatarURL.String
	u.Bio = bio.String
	m.Sender = &u
	return &m, nil
}
