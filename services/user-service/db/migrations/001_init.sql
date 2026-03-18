-- User Service schema: relay_users
-- Run against: relay_users database

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- User profiles (cross-service: user_id is auth-service's users.id)
CREATE TABLE IF NOT EXISTS user_profiles (
  user_id           UUID PRIMARY KEY,
  display_name      TEXT NOT NULL,
  avatar_url        TEXT,
  timezone          TEXT NOT NULL DEFAULT 'UTC',
  locale            TEXT NOT NULL DEFAULT 'en',
  status_emoji      TEXT,
  status_text       TEXT,
  status_expires_at TIMESTAMPTZ,
  is_active         BOOLEAN NOT NULL DEFAULT true,
  deactivated_at    TIMESTAMPTZ,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_profiles_active ON user_profiles(is_active);

-- Notification preferences (global + per-workspace/channel)
CREATE TABLE IF NOT EXISTS notification_preferences (
  user_id  UUID NOT NULL,
  scope    TEXT NOT NULL,    -- 'global', 'workspace:{id}', 'channel:{id}'
  level    TEXT NOT NULL DEFAULT 'mentions', -- 'all', 'mentions', 'nothing'
  muted    BOOLEAN NOT NULL DEFAULT false,
  PRIMARY KEY (user_id, scope)
);

-- Push notification tokens
CREATE TABLE IF NOT EXISTS push_tokens (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL,
  platform   TEXT NOT NULL,   -- 'ios', 'android', 'web'
  token      TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(platform, token)
);

CREATE INDEX IF NOT EXISTS idx_push_tokens_user ON push_tokens(user_id);

-- DND (Do Not Disturb) schedules
CREATE TABLE IF NOT EXISTS dnd_schedules (
  user_id    UUID PRIMARY KEY,
  until      TIMESTAMPTZ,    -- NULL = indefinite
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Trigger: keep updated_at fresh
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER profiles_updated_at
  BEFORE UPDATE ON user_profiles
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
