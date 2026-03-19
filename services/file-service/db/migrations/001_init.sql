-- file-service schema

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE relay_files (
    id             UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    workspace_id   UUID        NOT NULL,
    channel_id     UUID,
    uploader_id    UUID        NOT NULL,
    filename       TEXT        NOT NULL,
    content_type   TEXT        NOT NULL,
    size_bytes     BIGINT      NOT NULL DEFAULT 0,
    storage_key    TEXT        NOT NULL,
    thumbnail_key  TEXT,
    is_image       BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_relay_files_workspace ON relay_files (workspace_id, created_at DESC);
CREATE INDEX idx_relay_files_channel   ON relay_files (channel_id,   created_at DESC) WHERE channel_id IS NOT NULL;
CREATE INDEX idx_relay_files_uploader  ON relay_files (uploader_id,  created_at DESC);
