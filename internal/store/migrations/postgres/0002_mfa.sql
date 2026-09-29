-- TOTP two-factor sign-in for local (password) accounts.
CREATE TABLE user_mfa (
    user_id        TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    totp_secret    TEXT NOT NULL,
    enabled_at     BIGINT,
    last_step      BIGINT NOT NULL DEFAULT 0,
    recovery_codes TEXT NOT NULL DEFAULT '[]',
    updated_at     BIGINT NOT NULL
);

ALTER TABLE sessions ADD COLUMN auth_method TEXT NOT NULL DEFAULT 'password';
