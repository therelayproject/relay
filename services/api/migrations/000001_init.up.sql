-- ─────────────────────────────────────────────────────────────────────────────
-- 000001_init — Core schema
-- ─────────────────────────────────────────────────────────────────────────────

-- Snowflake ID generator (time-ordered, k-sortable)
CREATE SEQUENCE IF NOT EXISTS snowflake_seq START 1;

CREATE OR REPLACE FUNCTION next_snowflake()
RETURNS BIGINT AS $$
DECLARE
  epoch      BIGINT := 1704067200000; -- 2024-01-01 UTC in ms
  now_ms     BIGINT;
  seq        BIGINT;
BEGIN
  now_ms := (EXTRACT(EPOCH FROM clock_timestamp()) * 1000)::BIGINT - epoch;
  seq    := nextval('snowflake_seq') % 4096;
  RETURN (now_ms << 22) | seq;
END;
$$ LANGUAGE plpgsql;

-- ── Users ─────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users (
  id           BIGINT PRIMARY KEY DEFAULT next_snowflake(),
  email        TEXT   NOT NULL UNIQUE,
  username     TEXT   NOT NULL UNIQUE,
  display_name TEXT,
  avatar_url   TEXT,
  password_hash TEXT, -- NULL for OAuth-only accounts
  totp_secret  TEXT,  -- NULL if MFA not enabled
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Workspaces ────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS workspaces (
  id           BIGINT PRIMARY KEY DEFAULT next_snowflake(),
  slug         TEXT   NOT NULL UNIQUE,
  name         TEXT   NOT NULL,
  plan         TEXT   NOT NULL DEFAULT 'community',
  icon_url     TEXT,
  -- Federation settings
  federation_enabled             BOOLEAN NOT NULL DEFAULT FALSE,
  require_admin_approval         BOOLEAN NOT NULL DEFAULT TRUE,
  external_can_dm_internal       BOOLEAN NOT NULL DEFAULT FALSE,
  external_can_create_channels   BOOLEAN NOT NULL DEFAULT FALSE,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Member roles ──────────────────────────────────────────────────────────────
CREATE TYPE member_role AS ENUM ('owner', 'admin', 'member', 'guest');

CREATE TABLE IF NOT EXISTS workspace_members (
  workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role         member_role NOT NULL DEFAULT 'member',
  joined_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (workspace_id, user_id)
);

CREATE TABLE IF NOT EXISTS guest_channel_access (
  workspace_id BIGINT NOT NULL,
  user_id      BIGINT NOT NULL,
  channel_id   BIGINT NOT NULL, -- FK added after channels table
  FOREIGN KEY (workspace_id, user_id) REFERENCES workspace_members(workspace_id, user_id) ON DELETE CASCADE,
  PRIMARY KEY (workspace_id, user_id, channel_id)
);

-- ── Channels ──────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS channels (
  id           BIGINT PRIMARY KEY DEFAULT next_snowflake(),
  workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  name         TEXT   NOT NULL,
  topic        TEXT,
  type         TEXT   NOT NULL DEFAULT 'public', -- public | private | dm | announcement
  created_by   BIGINT NOT NULL REFERENCES users(id),
  archived_at  TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS channels_workspace_name_uidx
  ON channels (workspace_id, name) WHERE archived_at IS NULL;

ALTER TABLE guest_channel_access
  ADD CONSTRAINT fk_guest_channel FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE;

CREATE TABLE IF NOT EXISTS channel_members (
  channel_id         BIGINT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  user_id            BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  last_read_id       BIGINT, -- Snowflake cursor for unread tracking
  mention_count      INT NOT NULL DEFAULT 0,
  notification_level TEXT NOT NULL DEFAULT 'all', -- all | mentions | nothing
  joined_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (channel_id, user_id)
);

-- ── Messages ──────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS messages (
  id           BIGINT PRIMARY KEY DEFAULT next_snowflake(),
  channel_id   BIGINT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  parent_id    BIGINT REFERENCES messages(id) ON DELETE SET NULL,
  user_id      BIGINT NOT NULL REFERENCES users(id),
  app_id       BIGINT, -- NULL for human messages; FK added in later migration
  content      TEXT,   -- plain text / markdown
  blocks       JSONB,  -- Block Kit payload (NULL for plain messages)
  content_type TEXT NOT NULL DEFAULT 'markdown', -- markdown | blocks | system
  reply_count  INT NOT NULL DEFAULT 0,
  edited_at    TIMESTAMPTZ,
  deleted_at   TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT must_have_content CHECK (content IS NOT NULL OR blocks IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS messages_channel_id_idx ON messages (channel_id, id DESC);
CREATE INDEX IF NOT EXISTS messages_parent_id_idx  ON messages (parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS messages_blocks_gin_idx ON messages USING GIN (blocks) WHERE blocks IS NOT NULL;

-- ── Reactions ─────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS reactions (
  message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  emoji      TEXT   NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (message_id, user_id, emoji)
);

-- ── Domain policies (federation) ─────────────────────────────────────────────
CREATE TYPE domain_classification AS ENUM ('internal', 'external', 'blocked');

CREATE TABLE IF NOT EXISTS workspace_domain_policies (
  workspace_id   BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  domain         TEXT   NOT NULL, -- 'acme.com', 'partner.com', or '*' catch-all
  classification domain_classification NOT NULL DEFAULT 'external',
  sort_order     INT NOT NULL DEFAULT 0,
  PRIMARY KEY (workspace_id, domain)
);
