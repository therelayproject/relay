-- Migration 001: initial schema for relay_workspaces

-- Trigger function to keep updated_at current
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE workspaces (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,
    slug        TEXT        UNIQUE NOT NULL,   -- URL-safe identifier
    description TEXT,
    icon_url    TEXT,
    owner_id    UUID        NOT NULL,          -- user_id of the creator
    allow_guest_invites BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_workspaces_slug ON workspaces (slug);

CREATE TRIGGER trg_workspaces_updated_at
    BEFORE UPDATE ON workspaces
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE workspace_members (
    workspace_id UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      UUID        NOT NULL,
    role         TEXT        NOT NULL DEFAULT 'member', -- owner|admin|member|guest
    invited_by   UUID,
    joined_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, user_id)
);

CREATE INDEX idx_workspace_members_workspace_id ON workspace_members (workspace_id);
CREATE INDEX idx_workspace_members_user_id      ON workspace_members (user_id);

CREATE TABLE workspace_invitations (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email        TEXT,                               -- if email-based invite
    token        TEXT        UNIQUE NOT NULL,        -- link-based invite token
    role         TEXT        NOT NULL DEFAULT 'member',
    invited_by   UUID        NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    accepted_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_workspace_invitations_token ON workspace_invitations (token);
CREATE INDEX idx_workspace_invitations_workspace_id ON workspace_invitations (workspace_id);
