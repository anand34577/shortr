package store

import (
	"context"
	"time"
)

// GetIdempotentResponse returns a previously stored response body for
// (key, userID), if one exists and hasn't expired (24h, enforced by caller
// via CreatedAt). single table instead of a separate cache; the
// row is tiny and self-cleans via PruneIdempotencyKeys.
func (s *Store) GetIdempotentResponse(ctx context.Context, key, userID string) (string, bool, error) {
	var resp, uid string
	err := s.queryRow(ctx, `SELECT response, user_id FROM idempotency_keys WHERE key = ?`, key).Scan(&resp, &uid)
	if isNoRows(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if uid != userID {
		return "", false, nil // key collision across users: treat as miss, don't leak
	}
	return resp, true, nil
}

func (s *Store) SaveIdempotentResponse(ctx context.Context, key, userID, response string) error {
	if s.Driver == "postgres" {
		_, err := s.execWrite(ctx, `INSERT INTO idempotency_keys (key, user_id, response, created_at) VALUES (?,?,?,?) ON CONFLICT (key) DO NOTHING`,
			key, userID, response, toMillis(time.Now()))
		return err
	}
	_, err := s.execWrite(ctx, `INSERT OR IGNORE INTO idempotency_keys (key, user_id, response, created_at) VALUES (?,?,?,?)`,
		key, userID, response, toMillis(time.Now()))
	return err
}

func (s *Store) PruneIdempotencyKeys(ctx context.Context, before time.Time) error {
	_, err := s.execWrite(ctx, `DELETE FROM idempotency_keys WHERE created_at < ?`, toMillis(before))
	return err
}
