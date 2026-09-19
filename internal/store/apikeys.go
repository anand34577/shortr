package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"shortr/internal/ulid"
)

func (s *Store) CreateAPIKey(ctx context.Context, k *APIKey) error {
	if k.ID == "" {
		k.ID = ulid.New()
	}
	k.CreatedAt = time.Now()
	scopes, _ := json.Marshal(k.Scopes)
	_, err := s.execWrite(ctx, `INSERT INTO api_keys (id, user_id, name, prefix, key_hash, scopes, last_used_at, expires_at, revoked_at, created_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		k.ID, k.UserID, k.Name, k.Prefix, k.KeyHash, string(scopes), nullMillis(k.LastUsedAt), nullMillis(k.ExpiresAt), nullMillis(k.RevokedAt), toMillis(k.CreatedAt))
	if err != nil {
		return classifyErr(err)
	}
	return nil
}

func apiKeyFromRow(row interface{ Scan(...any) error }) (*APIKey, error) {
	var k APIKey
	var scopesJSON string
	var lastUsed, expires, revoked sql.NullInt64
	var createdMS int64
	if err := row.Scan(&k.ID, &k.UserID, &k.Name, &k.Prefix, &k.KeyHash, &scopesJSON, &lastUsed, &expires, &revoked, &createdMS); err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(scopesJSON), &k.Scopes) //nolint:errcheck
	k.LastUsedAt = scanNullMillis(lastUsed)
	k.ExpiresAt = scanNullMillis(expires)
	k.RevokedAt = scanNullMillis(revoked)
	k.CreatedAt = fromMillis(createdMS)
	return &k, nil
}

const apiKeyCols = `id, user_id, name, prefix, key_hash, scopes, last_used_at, expires_at, revoked_at, created_at`

func (s *Store) GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error) {
	row := s.queryRow(ctx, `SELECT `+apiKeyCols+` FROM api_keys WHERE key_hash = ?`, hash)
	k, err := apiKeyFromRow(row)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return k, err
}

func (s *Store) ListAPIKeysForUser(ctx context.Context, userID string) ([]*APIKey, error) {
	rows, err := s.query(ctx, `SELECT `+apiKeyCols+` FROM api_keys WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*APIKey
	for rows.Next() {
		k, err := apiKeyFromRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) GetAPIKey(ctx context.Context, id string) (*APIKey, error) {
	row := s.queryRow(ctx, `SELECT `+apiKeyCols+` FROM api_keys WHERE id = ?`, id)
	k, err := apiKeyFromRow(row)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return k, err
}

func (s *Store) TouchAPIKey(ctx context.Context, id string, at time.Time) error {
	_, err := s.execWrite(ctx, `UPDATE api_keys SET last_used_at = ? WHERE id = ?`, toMillis(at), id)
	return err
}

func (s *Store) RevokeAPIKey(ctx context.Context, id string, at time.Time) error {
	_, err := s.execWrite(ctx, `UPDATE api_keys SET revoked_at = ? WHERE id = ?`, toMillis(at), id)
	return err
}
