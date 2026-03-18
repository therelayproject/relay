-- Channel Service schema: relay_channels
-- Run against: relay_channels database

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Core channels table
CREATE TABLE IF NOT EXISTS channels (
  id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID        NOT NULL,
  name         TEXT        NOT NULL,
  slug         TEXT        NOT NULL,          -- URL-safe, unique within workspace
  description  TEXT,
  topic        TEXT,
  type         TEXT        NOT NULL DEFAULT 'public', -- public|private|dm
  is_archived  BOOLEAN     NOT NULL DEFAULT false,
  created_by   UUID        NOT NULL,
  member_count INT         NOT NULL DEFAULT 0,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_channels_workspace ON channels(workspace_id);
CREATE INDEX IF NOT EXISTS idx_channels_workspace_type ON channels(workspace_id, type);

-- Channel membership table
CREATE TABLE IF NOT EXISTS channel_members (
  channel_id   UUID        NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  user_id      UUID        NOT NULL,
  role         TEXT        NOT NULL DEFAULT 'member', -- owner|admin|member
  last_read_at TIMESTAMPTZ,
  joined_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (channel_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_channel_members_user ON channel_members(user_id);
CREATE INDEX IF NOT EXISTS idx_channel_members_channel ON channel_members(channel_id);

-- Trigger function: keep updated_at fresh
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER channels_updated_at
  BEFORE UPDATE ON channels
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Trigger function: increment member_count on insert into channel_members
CREATE OR REPLACE FUNCTION channel_member_count_inc()
RETURNS TRIGGER AS $$
BEGIN
  UPDATE channels SET member_count = member_count + 1 WHERE id = NEW.channel_id;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_channel_member_count_inc
  AFTER INSERT ON channel_members
  FOR EACH ROW EXECUTE FUNCTION channel_member_count_inc();

-- Trigger function: decrement member_count on delete from channel_members
CREATE OR REPLACE FUNCTION channel_member_count_dec()
RETURNS TRIGGER AS $$
BEGIN
  UPDATE channels SET member_count = GREATEST(member_count - 1, 0) WHERE id = OLD.channel_id;
  RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_channel_member_count_dec
  AFTER DELETE ON channel_members
  FOR EACH ROW EXECUTE FUNCTION channel_member_count_dec();
