CREATE TYPE user_status  AS ENUM ('online', 'offline', 'away');
CREATE TYPE chat_type    AS ENUM ('direct', 'group');
CREATE TYPE member_role  AS ENUM ('owner', 'admin', 'member');
CREATE TYPE message_type AS ENUM ('text', 'image', 'file');

-- Users
CREATE TABLE users (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    username      VARCHAR(50)  NOT NULL,
    email         VARCHAR(255) NOT NULL,
    password_hash TEXT         NOT NULL,
    avatar_url    TEXT,
    bio           VARCHAR(500),
    status        user_status  NOT NULL DEFAULT 'offline',
    last_seen     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,

    CONSTRAINT chk_users_username         CHECK (char_length(username) BETWEEN 3 AND 50),
    CONSTRAINT chk_users_username_pattern CHECK (username ~ '^[a-zA-Z0-9]+$'),
    CONSTRAINT chk_users_email            CHECK (char_length(email) > 0)
);

-- Partial unique indexes exclude soft-deleted rows so an email/username can be re-registered after deletion.
CREATE UNIQUE INDEX uidx_users_email    ON users (email)    WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uidx_users_username ON users (username) WHERE deleted_at IS NULL;
CREATE        INDEX  idx_users_status   ON users (status)   WHERE deleted_at IS NULL;

-- Chats
CREATE TABLE chats (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    type        chat_type    NOT NULL,
    name        VARCHAR(100),
    avatar_url  TEXT,
    description VARCHAR(500),
    created_by  UUID         NOT NULL REFERENCES users (id),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,

    -- enforce name rules at the DB level: group chats must have a name, direct chats must not
    CONSTRAINT chk_chats_group_name  CHECK (type <> 'group'  OR name IS NOT NULL),
    CONSTRAINT chk_chats_direct_name CHECK (type <> 'direct' OR name IS NULL)
);

CREATE INDEX idx_chats_created_by ON chats (created_by) WHERE deleted_at IS NULL;

-- Chat members
CREATE TABLE chat_members (
    chat_id   UUID        NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
    user_id   UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role      member_role NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (chat_id, user_id)
);

CREATE INDEX idx_chat_members_user_id ON chat_members (user_id);

-- Messages
CREATE TABLE messages (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id     UUID         NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
    sender_id   UUID         NOT NULL REFERENCES users (id),
    content     TEXT         NOT NULL,
    type        message_type NOT NULL DEFAULT 'text',
    reply_to_id UUID         REFERENCES messages (id) ON DELETE SET NULL,
    edited_at   TIMESTAMPTZ,
    deleted_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_messages_content     CHECK (char_length(content) > 0),
    CONSTRAINT chk_messages_content_max CHECK (char_length(content) <= 4096)
);

-- Composite index for keyset pagination (chat_id, created_at DESC, id DESC).
CREATE INDEX idx_messages_chat_created ON messages (chat_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_messages_sender_id    ON messages (sender_id);
CREATE INDEX idx_messages_reply_to     ON messages (reply_to_id) WHERE reply_to_id IS NOT NULL;

-- Chat read receipts: tracks the last-read timestamp per (chat, user).
CREATE TABLE chat_read_receipts (
    chat_id      UUID        NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
    user_id      UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    last_read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (chat_id, user_id)
);

CREATE INDEX idx_read_receipts_user_id ON chat_read_receipts (user_id);

-- Message reactions
CREATE TABLE message_reactions (
    message_id UUID        NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users (id)    ON DELETE CASCADE,
    emoji      TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (message_id, user_id, emoji),

    CONSTRAINT chk_reactions_emoji CHECK (char_length(emoji) BETWEEN 1 AND 10)
);

CREATE INDEX idx_message_reactions_message_id ON message_reactions (message_id);
