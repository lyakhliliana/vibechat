package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vibechat/internal/domain"
)

type messageRepository struct {
	db *pgxpool.Pool
}

func NewMessageRepository(db *pgxpool.Pool) domain.MessageRepository {
	return &messageRepository{db: db}
}

func (r *messageRepository) Create(ctx context.Context, m *domain.Message) error {
	const q = `
		INSERT INTO messages (id, chat_id, sender_id, content, type, reply_to_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.db.Exec(ctx, q,
		m.ID, m.ChatID, m.SenderID, m.Content, m.Type, m.ReplyToID, m.CreatedAt, m.UpdatedAt,
	)
	return err
}

func (r *messageRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
	const q = `
		SELECT m.id, m.chat_id, m.sender_id, m.content, m.type, m.reply_to_id,
		       m.edited_at, m.deleted_at, m.created_at, m.updated_at
		FROM messages m
		WHERE m.id = $1`

	m, err := scanMessage(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMessageNotFound
		}
		return nil, err
	}
	return m, nil
}

func (r *messageRepository) GetChatMessages(
	ctx context.Context,
	chatID uuid.UUID,
	cursor *domain.MessageCursor,
	limit int,
) ([]*domain.Message, error) {
	var (
		rows pgx.Rows
		err  error
	)

	if cursor == nil {
		const q = `
			SELECT m.id, m.chat_id, m.sender_id, m.content, m.type, m.reply_to_id,
			       m.edited_at, m.deleted_at, m.created_at, m.updated_at,
			       u.id, u.username, u.email, u.avatar_url, u.bio, u.status, u.last_seen, u.created_at, u.updated_at
			FROM messages m
			JOIN users u ON u.id = m.sender_id
			WHERE m.chat_id = $1 AND m.deleted_at IS NULL
			ORDER BY m.created_at DESC, m.id DESC
			LIMIT $2`
		rows, err = r.db.Query(ctx, q, chatID, limit)
	} else {
		const q = `
			SELECT m.id, m.chat_id, m.sender_id, m.content, m.type, m.reply_to_id,
			       m.edited_at, m.deleted_at, m.created_at, m.updated_at,
			       u.id, u.username, u.email, u.avatar_url, u.bio, u.status, u.last_seen, u.created_at, u.updated_at
			FROM messages m
			JOIN users u ON u.id = m.sender_id
			WHERE m.chat_id = $1
			  AND m.deleted_at IS NULL
			  AND (m.created_at, m.id) < ($2, $3)
			ORDER BY m.created_at DESC, m.id DESC
			LIMIT $4`
		rows, err = r.db.Query(ctx, q, chatID, cursor.CreatedAt, cursor.ID, limit)
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
		SET content = $2, edited_at = $3, updated_at = $4
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := r.db.Exec(ctx, q, m.ID, m.Content, m.EditedAt, m.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMessageNotFound
	}
	return nil
}

func (r *messageRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE messages SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMessageNotFound
	}
	return nil
}

func (r *messageRepository) MarkRead(ctx context.Context, chatID, userID uuid.UUID) error {
	const q = `
		INSERT INTO chat_read_receipts (chat_id, user_id, last_read_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (chat_id, user_id) DO UPDATE SET last_read_at = NOW()`

	_, err := r.db.Exec(ctx, q, chatID, userID)
	return err
}

func (r *messageRepository) GetUnreadCount(ctx context.Context, chatID, userID uuid.UUID) (int, error) {
	const q = `
		SELECT COUNT(*)
		FROM messages m
		WHERE m.chat_id = $1
		  AND m.sender_id <> $2
		  AND m.deleted_at IS NULL
		  AND m.created_at > COALESCE(
			  (SELECT last_read_at FROM chat_read_receipts WHERE chat_id = $1 AND user_id = $2),
			  'epoch'
		  )`

	var count int
	if err := r.db.QueryRow(ctx, q, chatID, userID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *messageRepository) AddReaction(ctx context.Context, rx *domain.Reaction) error {
	const q = `
		INSERT INTO message_reactions (message_id, user_id, emoji, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (message_id, user_id, emoji) DO NOTHING`

	_, err := r.db.Exec(ctx, q, rx.MessageID, rx.UserID, rx.Emoji, rx.CreatedAt)
	return err
}

func (r *messageRepository) RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error {
	const q = `DELETE FROM message_reactions WHERE message_id = $1 AND user_id = $2 AND emoji = $3`
	_, err := r.db.Exec(ctx, q, messageID, userID, emoji)
	return err
}

func (r *messageRepository) GetReactions(ctx context.Context, messageID uuid.UUID) ([]*domain.Reaction, error) {
	const q = `
		SELECT mr.message_id, mr.user_id, mr.emoji, mr.created_at,
		       u.id, u.username, u.avatar_url
		FROM message_reactions mr
		JOIN users u ON u.id = mr.user_id AND u.deleted_at IS NULL
		WHERE mr.message_id = $1
		ORDER BY mr.created_at, mr.user_id, mr.emoji`

	rows, err := r.db.Query(ctx, q, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reactions []*domain.Reaction
	for rows.Next() {
		var rx domain.Reaction
		var u domain.User
		if err := rows.Scan(
			&rx.MessageID, &rx.UserID, &rx.Emoji, &rx.CreatedAt,
			&u.ID, &u.Username, &u.AvatarURL,
		); err != nil {
			return nil, err
		}
		rx.User = &u
		reactions = append(reactions, &rx)
	}
	return reactions, rows.Err()
}

func (r *messageRepository) GetReactionsBatch(ctx context.Context, messageIDs []uuid.UUID) (map[uuid.UUID][]*domain.Reaction, error) {
	if len(messageIDs) == 0 {
		return make(map[uuid.UUID][]*domain.Reaction), nil
	}

	const q = `
		SELECT mr.message_id, mr.user_id, mr.emoji, mr.created_at,
		       u.id, u.username, u.avatar_url
		FROM message_reactions mr
		JOIN users u ON u.id = mr.user_id AND u.deleted_at IS NULL
		WHERE mr.message_id = ANY($1)
		ORDER BY mr.message_id, mr.created_at, mr.user_id, mr.emoji`

	rows, err := r.db.Query(ctx, q, messageIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]*domain.Reaction)
	for rows.Next() {
		var rx domain.Reaction
		var u domain.User
		if err := rows.Scan(
			&rx.MessageID, &rx.UserID, &rx.Emoji, &rx.CreatedAt,
			&u.ID, &u.Username, &u.AvatarURL,
		); err != nil {
			return nil, err
		}
		rx.User = &u
		result[rx.MessageID] = append(result[rx.MessageID], &rx)
	}
	return result, rows.Err()
}

func scanMessage(row scanner) (*domain.Message, error) {
	var m domain.Message
	err := row.Scan(
		&m.ID, &m.ChatID, &m.SenderID, &m.Content, &m.Type,
		&m.ReplyToID, &m.EditedAt, &m.DeletedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// scanMessageWithSender scans a message row that includes a JOIN to users
// (id, username, email, avatar_url, bio, status, last_seen, created_at, updated_at).
// password_hash is intentionally excluded from the JOIN select.
func scanMessageWithSender(row scanner) (*domain.Message, error) {
	var m domain.Message
	var u domain.User
	err := row.Scan(
		&m.ID, &m.ChatID, &m.SenderID, &m.Content, &m.Type,
		&m.ReplyToID, &m.EditedAt, &m.DeletedAt, &m.CreatedAt, &m.UpdatedAt,
		&u.ID, &u.Username, &u.Email, &u.AvatarURL, &u.Bio, &u.Status, &u.LastSeen, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	m.Sender = &u
	return &m, nil
}
