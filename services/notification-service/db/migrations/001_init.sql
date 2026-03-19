-- notification-service schema

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ── Notification preferences ──────────────────────────────────────────────────

CREATE TABLE relay_notification_preferences (
    id           UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id      UUID        NOT NULL,
    scope        VARCHAR(64) NOT NULL,  -- "global", channel_id, workspace_id
    level        VARCHAR(16) NOT NULL DEFAULT 'mentions', -- all | mentions | nothing
    muted        BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, scope)
);

CREATE INDEX idx_notif_prefs_user ON relay_notification_preferences (user_id);

-- ── Push tokens ───────────────────────────────────────────────────────────────

CREATE TABLE relay_push_tokens (
    id         UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id    UUID        NOT NULL,
    platform   VARCHAR(16) NOT NULL, -- ios | android | web
    token      TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, platform)
);

CREATE INDEX idx_push_tokens_user ON relay_push_tokens (user_id);

-- ── Do Not Disturb ────────────────────────────────────────────────────────────

CREATE TABLE relay_dnd (
    user_id    UUID        NOT NULL PRIMARY KEY,
    until      TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Notifications log (in-app) ────────────────────────────────────────────────

CREATE TABLE relay_notifications (
    id         UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id    UUID        NOT NULL,
    type       VARCHAR(64) NOT NULL,
    title      TEXT        NOT NULL,
    body       TEXT        NOT NULL,
    action_url TEXT,
    read_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user ON relay_notifications (user_id, created_at DESC);
