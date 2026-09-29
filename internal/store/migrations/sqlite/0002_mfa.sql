-- TOTP two-factor sign-in for local (password) accounts.
CREATE TABLE user_mfa (
    user_id        TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    totp_secret    TEXT NOT NULL,              -- AES-GCM sealed with the instance secret key
    enabled_at     INTEGER,                    -- NULL while enrollment is pending
    last_step      INTEGER NOT NULL DEFAULT 0, -- last accepted 30s step, blocks code replay
    recovery_codes TEXT NOT NULL DEFAULT '[]', -- JSON array of sha256(code) hex
    updated_at     INTEGER NOT NULL
);

-- how a session was established: password | oidc
ALTER TABLE sessions ADD COLUMN auth_method TEXT NOT NULL DEFAULT 'password';
