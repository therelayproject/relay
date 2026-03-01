-- ─────────────────────────────────────────────────────────────────────────────
-- seed.sql — Local development seed data
-- Idempotent: safe to run multiple times (INSERT ... ON CONFLICT DO NOTHING)
-- DO NOT use in production. All password hashes are intentional placeholders.
-- ─────────────────────────────────────────────────────────────────────────────

-- ── Workspaces ────────────────────────────────────────────────────────────────

INSERT INTO workspaces (id, slug, name, plan, federation_enabled, require_admin_approval, external_can_dm_internal, external_can_create_channels)
VALUES
  (1000000000000000001, 'relay-hq',     'Relay HQ',    'community', FALSE, TRUE,  FALSE, FALSE),
  (1000000000000000002, 'partner-corp',  'Partner Corp', 'community', TRUE,  FALSE, TRUE,  FALSE)
ON CONFLICT DO NOTHING;

-- ── Users ─────────────────────────────────────────────────────────────────────
-- password_hash is a bcrypt placeholder — never use in production

INSERT INTO users (id, email, username, display_name, password_hash)
VALUES
  (1000000000000001001, 'alice@relay.gg',  'alice', 'Alice',       '$2a$12$placeholder.hash.for.dev.only'),
  (1000000000000001002, 'bob@relay.gg',    'bob',   'Bob',         '$2a$12$placeholder.hash.for.dev.only'),
  (1000000000000001003, 'carol@relay.gg',  'carol', 'Carol',       '$2a$12$placeholder.hash.for.dev.only'),
  (1000000000000001004, 'dave@partner.corp','dave',  'Dave',        '$2a$12$placeholder.hash.for.dev.only')
ON CONFLICT DO NOTHING;

-- ── Workspace members ─────────────────────────────────────────────────────────

INSERT INTO workspace_members (workspace_id, user_id, role)
VALUES
  -- Relay HQ
  (1000000000000000001, 1000000000000001001, 'owner'),   -- alice  → owner
  (1000000000000000001, 1000000000000001002, 'member'),  -- bob    → member
  (1000000000000000001, 1000000000000001003, 'admin'),   -- carol  → admin
  -- Partner Corp
  (1000000000000000002, 1000000000000001004, 'member')   -- dave   → member
ON CONFLICT DO NOTHING;

-- ── Channels (Relay HQ) ───────────────────────────────────────────────────────

INSERT INTO channels (id, workspace_id, name, topic, type, created_by)
VALUES
  (1000000000000002001, 1000000000000000001, 'general',       'General discussion',          'public',  1000000000000001001),
  (1000000000000002002, 1000000000000000001, 'announcements', 'Company-wide announcements',  'public',  1000000000000001001),
  (1000000000000002003, 1000000000000000001, 'engineering',   'Engineering team discussion', 'private', 1000000000000001001)
ON CONFLICT DO NOTHING;

-- ── Channel members ───────────────────────────────────────────────────────────
-- alice, bob, carol are in all three Relay HQ channels; dave is not in any

INSERT INTO channel_members (channel_id, user_id)
VALUES
  -- #general
  (1000000000000002001, 1000000000000001001),  -- alice
  (1000000000000002001, 1000000000000001002),  -- bob
  (1000000000000002001, 1000000000000001003),  -- carol
  -- #announcements
  (1000000000000002002, 1000000000000001001),  -- alice
  (1000000000000002002, 1000000000000001002),  -- bob
  (1000000000000002002, 1000000000000001003),  -- carol
  -- #engineering
  (1000000000000002003, 1000000000000001001),  -- alice
  (1000000000000002003, 1000000000000001002),  -- bob
  (1000000000000002003, 1000000000000001003)   -- carol
ON CONFLICT DO NOTHING;

-- ── Messages in #general ──────────────────────────────────────────────────────
-- Five sample messages, alternating between alice and bob.
-- IDs are Snowflake-style bigints starting at 2000000000000000001.

INSERT INTO messages (id, channel_id, user_id, content, content_type)
VALUES
  (2000000000000000001, 1000000000000002001, 1000000000000001001, 'Hey everyone, welcome to Relay HQ! :wave:', 'markdown'),
  (2000000000000000002, 1000000000000002001, 1000000000000001002, 'Thanks Alice! Excited to be here.', 'markdown'),
  (2000000000000000003, 1000000000000002001, 1000000000000001001, 'Feel free to use this channel for general chat and questions.', 'markdown'),
  (2000000000000000004, 1000000000000002001, 1000000000000001002, 'Will do. Is there a doc on how federation works?', 'markdown'),
  (2000000000000000005, 1000000000000002001, 1000000000000001001, 'Yes — check #engineering for the design doc. Carol can give you access.', 'markdown')
ON CONFLICT DO NOTHING;

-- ── Workspace domain policies ─────────────────────────────────────────────────

INSERT INTO workspace_domain_policies (workspace_id, domain, classification, sort_order)
VALUES
  (1000000000000000001, 'relay.gg', 'internal', 1),    -- relay.gg addresses are internal
  (1000000000000000001, '*',        'external', 999)   -- everything else is external
ON CONFLICT DO NOTHING;
