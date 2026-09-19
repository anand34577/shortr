package store

import (
	"context"
	"time"
)

func (s *Store) GetSetting(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := s.queryRow(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if isNoRows(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func (s *Store) SetSetting(ctx context.Context, key, value, updatedBy string) error {
	now := toMillis(time.Now())
	if s.Driver == "postgres" {
		_, err := s.execWrite(ctx, `INSERT INTO settings (key, value, updated_at, updated_by) VALUES (?,?,?,?)
			ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at, updated_by = excluded.updated_by`,
			key, value, now, updatedBy)
		return err
	}
	_, err := s.execWrite(ctx, `INSERT INTO settings (key, value, updated_at, updated_by) VALUES (?,?,?,?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at, updated_by = excluded.updated_by`,
		key, value, now, updatedBy)
	return err
}

func (s *Store) AllSettings(ctx context.Context) (map[string]string, error) {
	rows, err := s.query(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}
