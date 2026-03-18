-- Auth Service schema: relay_auth
-- Run against: relay_auth database

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Core users table (identity only; profile data lives in user-service)
CREATE TABLE IF NOT EXISTS users (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email            TEXT UNIQUE NOT NULL,
  password_hash    TEXT,              -- bcrypt, nullable for OAuth-only accounts
  totp_secret      TEXT,              -- AES-256 encrypted at rest, NULL until setup
  totp_enabled     BOOLEAN NOT NULL DEFAULT false,
  email_verified   BOOLEAN NOT NULL DEFAULT false,
  is_active        BOOLEAN NOT NULL DEFAULT true,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- OAuth provider links
CREATE TABLE IF NOT EXISTS oauth_providers (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider         TEXT NOT NULL,     -- 'google', 'github'
  provider_user_id TEXT NOT NULL,
  access_token     TEXT,              -- encrypted
  refresh_token    TEXT,              -- encrypted
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(provider, provider_user_id)
);

CREATE INDEX IF NOT EXISTS idx_oauth_user ON oauth_providers(user_id);

-- Active device sessions
CREATE TABLE IF NOT EXISTS sessions (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_name  TEXT,
  user_agent   TEXT,
  ip_address   INET,
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);

-- Password reset tokens (short-lived, single-use)
CREATE TABLE IF NOT EXISTS password_reset_tokens (
  token        TEXT PRIMARY KEY,      -- cryptographically random hex
  user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at   TIMESTAMPTZ NOT NULL,
  used_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_prt_user ON password_reset_tokens(user_id);

-- Email verification tokens
CREATE TABLE IF NOT EXISTS email_verification_tokens (
  token        TEXT PRIMARY KEY,
  user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at   TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_evt_user ON email_verification_tokens(user_id);

-- TOTP backup codes (hashed, single-use)
CREATE TABLE IF NOT EXISTS totp_backup_codes (
  id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code_hash TEXT NOT NULL,
  used_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_tbc_user ON totp_backup_codes(user_id);

-- Trigger: keep updated_at fresh on users
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_updated_at
  BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
