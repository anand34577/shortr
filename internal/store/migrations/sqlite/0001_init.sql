CREATE TABLE users (
    id                   TEXT PRIMARY KEY,
    email                TEXT NOT NULL UNIQUE,
    email_verified       INTEGER NOT NULL DEFAULT 0,
    name                 TEXT NOT NULL DEFAULT '',
    password_hash        TEXT,
    role                 TEXT NOT NULL DEFAULT 'user',
    status                TEXT NOT NULL DEFAULT 'active',
    max_links            INTEGER,
    role_locked          INTEGER NOT NULL DEFAULT 0,
    must_change_password INTEGER NOT NULL DEFAULT 0,
    last_login_at        INTEGER,
    password_changed_at  INTEGER,
    created_at           INTEGER NOT NULL,
    updated_at           INTEGER NOT NULL
);

CREATE TABLE oidc_identities (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issuer        TEXT NOT NULL,
    subject       TEXT NOT NULL,
    email         TEXT,
    name          TEXT,
    raw_claims    TEXT,
    created_at    INTEGER NOT NULL,
    last_login_at INTEGER,
    UNIQUE(issuer, subject)
);
CREATE INDEX idx_oidc_identities_user ON oidc_identities(user_id);

CREATE TABLE sessions (
    id            TEXT PRIMARY KEY, -- sha256 hash of the token, hex
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    csrf_token    TEXT NOT NULL,
    ip            TEXT,
    user_agent    TEXT,
    created_at    INTEGER NOT NULL,
    expires_at    INTEGER NOT NULL,
    last_seen_at  INTEGER NOT NULL
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

CREATE TABLE api_keys (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    prefix        TEXT NOT NULL,
    key_hash      TEXT NOT NULL UNIQUE,
    scopes        TEXT NOT NULL DEFAULT '["links:read","links:write","stats:read"]',
    last_used_at  INTEGER,
    expires_at    INTEGER,
    revoked_at    INTEGER,
    created_at    INTEGER NOT NULL
);
CREATE INDEX idx_api_keys_user ON api_keys(user_id);

CREATE TABLE links (
    id                TEXT PRIMARY KEY,
    code              TEXT NOT NULL UNIQUE,
    target_url        TEXT NOT NULL,
    title             TEXT NOT NULL DEFAULT '',
    description       TEXT NOT NULL DEFAULT '',
    user_id           TEXT REFERENCES users(id) ON DELETE SET NULL,
    redirect_status   INTEGER NOT NULL DEFAULT 302,
    password_hash     TEXT,
    expires_at        INTEGER,
    max_clicks        INTEGER,
    click_count       INTEGER NOT NULL DEFAULT 0,
    last_click_at     INTEGER,
    status            TEXT NOT NULL DEFAULT 'active',
    deleted_at        INTEGER,
    utm_source        TEXT NOT NULL DEFAULT '',
    utm_medium        TEXT NOT NULL DEFAULT '',
    utm_campaign      TEXT NOT NULL DEFAULT '',
    utm_term          TEXT NOT NULL DEFAULT '',
    utm_content       TEXT NOT NULL DEFAULT '',
    pass_query        INTEGER NOT NULL DEFAULT 1,
    tags              TEXT NOT NULL DEFAULT '[]',
    created_at        INTEGER NOT NULL,
    updated_at        INTEGER NOT NULL,
    created_by_ip     TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX idx_links_code_lower ON links(LOWER(code));
CREATE INDEX idx_links_user_created ON links(user_id, created_at DESC);
CREATE INDEX idx_links_deleted ON links(deleted_at);
CREATE INDEX idx_links_expires ON links(expires_at) WHERE expires_at IS NOT NULL;

CREATE TABLE clicks (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    link_id        TEXT NOT NULL,
    ts             INTEGER NOT NULL,
    ip             TEXT NOT NULL DEFAULT '',
    ip_version     INTEGER NOT NULL DEFAULT 4,
    country        TEXT NOT NULL DEFAULT '',
    region         TEXT NOT NULL DEFAULT '',
    city           TEXT NOT NULL DEFAULT '',
    referrer       TEXT NOT NULL DEFAULT '',
    referrer_host  TEXT NOT NULL DEFAULT '',
    user_agent     TEXT NOT NULL DEFAULT '',
    device         TEXT NOT NULL DEFAULT '',
    os             TEXT NOT NULL DEFAULT '',
    os_version     TEXT NOT NULL DEFAULT '',
    browser        TEXT NOT NULL DEFAULT '',
    browser_version TEXT NOT NULL DEFAULT '',
    is_bot         INTEGER NOT NULL DEFAULT 0,
    lang           TEXT NOT NULL DEFAULT '',
    utm_source     TEXT NOT NULL DEFAULT '',
    utm_medium     TEXT NOT NULL DEFAULT '',
    utm_campaign   TEXT NOT NULL DEFAULT '',
    utm_term       TEXT NOT NULL DEFAULT '',
    utm_content    TEXT NOT NULL DEFAULT '',
    qs             TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_clicks_link_ts ON clicks(link_id, ts);
CREATE INDEX idx_clicks_ts ON clicks(ts);

CREATE TABLE click_rollups_daily (
    link_id  TEXT NOT NULL,
    day      INTEGER NOT NULL, -- yyyymmdd (UTC)
    dim      TEXT NOT NULL,    -- total|country|device|os|browser|referrer_host
    key      TEXT NOT NULL DEFAULT '',
    clicks   INTEGER NOT NULL DEFAULT 0,
    bots     INTEGER NOT NULL DEFAULT 0,
    uniques  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (link_id, day, dim, key)
);
CREATE INDEX idx_rollups_day ON click_rollups_daily(day);

CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at INTEGER NOT NULL,
    updated_by TEXT NOT NULL DEFAULT ''
);

CREATE TABLE audit_log (
    id             TEXT PRIMARY KEY,
    ts             INTEGER NOT NULL,
    actor_user_id  TEXT NOT NULL DEFAULT '',
    actor_ip       TEXT NOT NULL DEFAULT '',
    action         TEXT NOT NULL,
    target_type    TEXT NOT NULL DEFAULT '',
    target_id      TEXT NOT NULL DEFAULT '',
    meta           TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX idx_audit_ts ON audit_log(ts);

CREATE TABLE notifications (
    id         TEXT PRIMARY KEY,
    user_id    TEXT REFERENCES users(id) ON DELETE CASCADE, -- NULL = broadcast to admins
    kind       TEXT NOT NULL,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL DEFAULT '',
    data       TEXT NOT NULL DEFAULT '{}',
    read_at    INTEGER,
    created_at INTEGER NOT NULL
);
CREATE INDEX idx_notifications_user ON notifications(user_id, created_at DESC);

CREATE TABLE idempotency_keys (
    key          TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL,
    response     TEXT NOT NULL,
    created_at   INTEGER NOT NULL
);
