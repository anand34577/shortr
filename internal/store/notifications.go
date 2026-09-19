package store

import (
	"context"
	"database/sql"
	"time"

	"shortr/internal/ulid"
)

// CreateNotification inserts an in-app notification. userID == nil means it
// broadcasts to all admins (rendered by the API layer per-viewer).
func (s *Store) CreateNotification(ctx context.Context, n *Notification) error {
	if n.ID == "" {
		n.ID = ulid.New()
	}
	n.CreatedAt = time.Now()
	if n.Data == "" {
		n.Data = "{}"
	}
	_, err := s.execWrite(ctx, `INSERT INTO notifications (id, user_id, kind, title, body, data, read_at, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		n.ID, n.UserID, n.Kind, n.Title, n.Body, n.Data, nullMillis(n.ReadAt), toMillis(n.CreatedAt))
	return err
}

func (s *Store) ListNotifications(ctx context.Context, userID string, isAdmin bool, limit int) ([]*Notification, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	q := `SELECT id, user_id, kind, title, body, data, read_at, created_at FROM notifications WHERE user_id = ?`
	args := []any{userID}
	if isAdmin {
		q = `SELECT id, user_id, kind, title, body, data, read_at, created_at FROM notifications WHERE user_id = ? OR user_id IS NULL`
	}
	q += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func scanNotification(row interface{ Scan(...any) error }) (*Notification, error) {
	var n Notification
	var userID sql.NullString
	var createdAt int64
	var readAtN sql.NullInt64
	if err := row.Scan(&n.ID, &userID, &n.Kind, &n.Title, &n.Body, &n.Data, &readAtN, &createdAt); err != nil {
		return nil, err
	}
	if userID.Valid {
		n.UserID = &userID.String
	}
	n.ReadAt = scanNullMillis(readAtN)
	n.CreatedAt = fromMillis(createdAt)
	return &n, nil
}

func (s *Store) MarkNotificationRead(ctx context.Context, id string, at time.Time) error {
	_, err := s.execWrite(ctx, `UPDATE notifications SET read_at = ? WHERE id = ?`, toMillis(at), id)
	return err
}

func (s *Store) MarkAllNotificationsRead(ctx context.Context, userID string, isAdmin bool, at time.Time) error {
	q := `UPDATE notifications SET read_at = ? WHERE read_at IS NULL AND user_id = ?`
	args := []any{toMillis(at), userID}
	if isAdmin {
		q = `UPDATE notifications SET read_at = ? WHERE read_at IS NULL AND (user_id = ? OR user_id IS NULL)`
	}
	_, err := s.execWrite(ctx, q, args...)
	return err
}

func (s *Store) UnreadNotificationCount(ctx context.Context, userID string, isAdmin bool) (int, error) {
	q := `SELECT count(*) FROM notifications WHERE read_at IS NULL AND user_id = ?`
	args := []any{userID}
	if isAdmin {
		q = `SELECT count(*) FROM notifications WHERE read_at IS NULL AND (user_id = ? OR user_id IS NULL)`
	}
	var n int
	err := s.queryRow(ctx, q, args...).Scan(&n)
	return n, err
}
