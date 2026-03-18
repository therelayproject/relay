-- message-service schema
-- Messages are partitioned by month (created_at) for efficient retention.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ── Messages ─────────────────────────────────────────────────────────────────

CREATE TABLE relay_messages (
    id              UUID        NOT NULL DEFAULT gen_random_uuid(),
    channel_id      UUID        NOT NULL,
    author_id       UUID        NOT NULL,
    body            TEXT        NOT NULL,
    body_parsed     JSONB,
    thread_id       UUID,                    -- root message of thread (NULL = top-level)
    parent_id       UUID,                    -- immediate parent (same as thread_id for first reply)
    idempotency_key UUID        UNIQUE,
    is_edited       BOOLEAN     NOT NULL DEFAULT FALSE,
    is_deleted      BOOLEAN     NOT NULL DEFAULT FALSE,
    reply_count     INTEGER     NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Seed two partitions; migrations script creates future ones via cron.
CREATE TABLE relay_messages_2026_03 PARTITION OF relay_messages
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE TABLE relay_messages_2026_04 PARTITION OF relay_messages
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');

CREATE TABLE relay_messages_2026_05 PARTITION OF relay_messages
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');

CREATE INDEX idx_relay_messages_channel ON relay_messages (channel_id, created_at DESC);
CREATE INDEX idx_relay_messages_thread  ON relay_messages (thread_id, created_at ASC) WHERE thread_id IS NOT NULL;
CREATE INDEX idx_relay_messages_author  ON relay_messages (author_id, created_at DESC);

-- ── Reactions ────────────────────────────────────────────────────────────────

CREATE TABLE relay_reactions (
    id         UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    message_id UUID        NOT NULL,
    channel_id UUID        NOT NULL,
    user_id    UUID        NOT NULL,
    emoji      VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (message_id, user_id, emoji)
);

CREATE INDEX idx_relay_reactions_message ON relay_reactions (message_id);

-- ── Pins ─────────────────────────────────────────────────────────────────────

CREATE TABLE relay_pins (
    id         UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    channel_id UUID        NOT NULL,
    message_id UUID        NOT NULL,
    pinned_by  UUID        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (channel_id, message_id)
);

CREATE INDEX idx_relay_pins_channel ON relay_pins (channel_id, created_at DESC);

-- ── Scheduled messages ───────────────────────────────────────────────────────

CREATE TABLE relay_scheduled_messages (
    id         UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    channel_id UUID        NOT NULL,
    author_id  UUID        NOT NULL,
    body       TEXT        NOT NULL,
    send_at    TIMESTAMPTZ NOT NULL,
    sent_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_relay_scheduled_pending ON relay_scheduled_messages (send_at) WHERE sent_at IS NULL;
