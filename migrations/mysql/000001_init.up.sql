-- MySQL 8.0+ schema for vibechat.
-- Key differences from the PostgreSQL schema:
--   • ENUM types are inline column definitions (no CREATE TYPE).
--   • UUIDs are stored as CHAR(36) CHARACTER SET ascii (no native UUID type).
--   • DATETIME(6) replaces TIMESTAMPTZ (microsecond precision, UTC enforced by the app).
--   • Partial unique indexes are emulated via a virtual generated column (active_flag).
--     MySQL treats NULLs as distinct in UNIQUE indexes, so deleted rows (active_flag IS NULL)
--     never conflict with each other, while active rows (active_flag = 1) must be unique.

CREATE TABLE users (
    id            CHAR(36)      CHARACTER SET ascii NOT NULL,
    username      VARCHAR(50)   NOT NULL,
    email         VARCHAR(255)  NOT NULL,
    password_hash TEXT          NOT NULL,
    avatar_url    TEXT,
    bio           VARCHAR(500),
    status        ENUM('online','offline','away') NOT NULL DEFAULT 'offline',
    last_seen     DATETIME(6)   NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    created_at    DATETIME(6)   NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at    DATETIME(6)   NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at    DATETIME(6),

    -- Virtual flag: 1 for active rows, NULL for soft-deleted.
    -- MySQL UNIQUE indexes allow many NULLs, so only one active user per email/username.
    active_flag   TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) VIRTUAL,

    PRIMARY KEY (id),
    CONSTRAINT chk_users_username     CHECK (CHAR_LENGTH(username) BETWEEN 3 AND 50),
    CONSTRAINT chk_users_email        CHECK (CHAR_LENGTH(email) > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Partial-index equivalent: active users only (active_flag = 1).
CREATE UNIQUE INDEX uidx_users_email    ON users (email, active_flag);
CREATE UNIQUE INDEX uidx_users_username ON users (username, active_flag);
CREATE        INDEX  idx_users_status   ON users (status, deleted_at);


CREATE TABLE chats (
    id          CHAR(36)     CHARACTER SET ascii NOT NULL,
    type        ENUM('direct','group') NOT NULL,
    name        VARCHAR(100),
    avatar_url  TEXT,
    description VARCHAR(500),
    created_by  CHAR(36)     CHARACTER SET ascii NOT NULL,
    created_at  DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at  DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at  DATETIME(6),

    PRIMARY KEY (id),
    CONSTRAINT fk_chats_created_by FOREIGN KEY (created_by) REFERENCES users (id),
    CONSTRAINT chk_chats_group_name  CHECK (type <> 'group'  OR name IS NOT NULL),
    CONSTRAINT chk_chats_direct_name CHECK (type <> 'direct' OR name IS NULL)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE INDEX idx_chats_created_by ON chats (created_by, deleted_at);


CREATE TABLE chat_members (
    chat_id   CHAR(36)     CHARACTER SET ascii NOT NULL,
    user_id   CHAR(36)     CHARACTER SET ascii NOT NULL,
    role      ENUM('owner','admin','member') NOT NULL DEFAULT 'member',
    joined_at DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (chat_id, user_id),
    CONSTRAINT fk_cm_chat FOREIGN KEY (chat_id) REFERENCES chats (id) ON DELETE CASCADE,
    CONSTRAINT fk_cm_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE INDEX idx_chat_members_user_id ON chat_members (user_id);


CREATE TABLE messages (
    id          CHAR(36)     CHARACTER SET ascii NOT NULL,
    chat_id     CHAR(36)     CHARACTER SET ascii NOT NULL,
    sender_id   CHAR(36)     CHARACTER SET ascii NOT NULL,
    content     TEXT         NOT NULL,
    type        ENUM('text','image','file') NOT NULL DEFAULT 'text',
    reply_to_id CHAR(36)     CHARACTER SET ascii,
    edited_at   DATETIME(6),
    deleted_at  DATETIME(6),
    created_at  DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at  DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),
    CONSTRAINT fk_msg_chat    FOREIGN KEY (chat_id)     REFERENCES chats    (id) ON DELETE CASCADE,
    CONSTRAINT fk_msg_sender  FOREIGN KEY (sender_id)   REFERENCES users    (id),
    CONSTRAINT fk_msg_reply   FOREIGN KEY (reply_to_id) REFERENCES messages (id) ON DELETE SET NULL,
    CONSTRAINT chk_msg_content     CHECK (CHAR_LENGTH(content) > 0),
    CONSTRAINT chk_msg_content_max CHECK (CHAR_LENGTH(content) <= 4096)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Composite index for keyset pagination (chat_id, created_at DESC, id DESC).
CREATE INDEX idx_messages_chat_created ON messages (chat_id, created_at DESC, id DESC);
CREATE INDEX idx_messages_sender_id    ON messages (sender_id);
CREATE INDEX idx_messages_reply_to     ON messages (reply_to_id);


CREATE TABLE chat_read_receipts (
    chat_id      CHAR(36)    CHARACTER SET ascii NOT NULL,
    user_id      CHAR(36)    CHARACTER SET ascii NOT NULL,
    last_read_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (chat_id, user_id),
    CONSTRAINT fk_crr_chat FOREIGN KEY (chat_id) REFERENCES chats (id) ON DELETE CASCADE,
    CONSTRAINT fk_crr_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE INDEX idx_read_receipts_user_id ON chat_read_receipts (user_id);


CREATE TABLE message_reactions (
    message_id CHAR(36)    CHARACTER SET ascii NOT NULL,
    user_id    CHAR(36)    CHARACTER SET ascii NOT NULL,
    emoji      VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (message_id, user_id, emoji),
    CONSTRAINT fk_rxn_message FOREIGN KEY (message_id) REFERENCES messages (id) ON DELETE CASCADE,
    CONSTRAINT fk_rxn_user    FOREIGN KEY (user_id)    REFERENCES users    (id) ON DELETE CASCADE,
    CONSTRAINT chk_rxn_emoji  CHECK (CHAR_LENGTH(emoji) BETWEEN 1 AND 10)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE INDEX idx_message_reactions_message_id ON message_reactions (message_id);
