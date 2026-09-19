package store

import (
	"context"
	"time"
)

func (s *Store) CreateSession(ctx context.Context, sess *Session) error {
	now := time.Now()
	sess.CreatedAt, sess.LastSeenAt = now, now
	_, err := s.execWrite(ctx, `INSERT INTO sessions (id, user_id, csrf_token, ip, user_agent, created_at, expires_at, last_seen_at) VALUES (?,?,?,?,?,?,?,?)`,
		sess.ID, sess.UserID, sess.CSRFToken, sess.IP, sess.UserAgent, toMillis(sess.CreatedAt), toMillis(sess.ExpiresAt), toMillis(sess.LastSeenAt))
	return err
}

func (s *Store) GetSession(ctx context.Context, id string) (*Session, error) {
	row := s.queryRow(ctx, `SELECT id, user_id, csrf_token, ip, user_agent, created_at, expires_at, last_seen_at FROM sessions WHERE id = ?`, id)
	var sess Session
	var created, expires, lastSeen int64
	if err := row.Scan(&sess.ID, &sess.UserID, &sess.CSRFToken, &sess.IP, &sess.UserAgent, &created, &expires, &lastSeen); err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	sess.CreatedAt, sess.ExpiresAt, sess.LastSeenAt = fromMillis(created), fromMillis(expires), fromMillis(lastSeen)
	return &sess, nil
}

// TouchSession bumps last_seen_at; callers should throttle calls (e.g. once
// per 5 min) to avoid a write on every request.
func (s *Store) TouchSession(ctx context.Context, id string, at time.Time) error {
	_, err := s.execWrite(ctx, `UPDATE sessions SET last_seen_at = ? WHERE id = ?`, toMillis(at), id)
	return err
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	_, err := s.execWrite(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	return err
}

func (s *Store) DeleteSessionsForUser(ctx context.Context, userID string) error {
	_, err := s.execWrite(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

func (s *Store) DeleteSessionsForUserExcept(ctx context.Context, userID, keepSessionID string) error {
	_, err := s.execWrite(ctx, `DELETE FROM sessions WHERE user_id = ? AND id != ?`, userID, keepSessionID)
	return err
}

func (s *Store) ListSessionsForUser(ctx context.Context, userID string) ([]*Session, error) {
	rows, err := s.query(ctx, `SELECT id, user_id, csrf_token, ip, user_agent, created_at, expires_at, last_seen_at FROM sessions WHERE user_id = ? ORDER BY last_seen_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Session
	for rows.Next() {
		var sess Session
		var created, expires, lastSeen int64
		if err := rows.Scan(&sess.ID, &sess.UserID, &sess.CSRFToken, &sess.IP, &sess.UserAgent, &created, &expires, &lastSeen); err != nil {
			return nil, err
		}
		sess.CreatedAt, sess.ExpiresAt, sess.LastSeenAt = fromMillis(created), fromMillis(expires), fromMillis(lastSeen)
		out = append(out, &sess)
	}
	return out, rows.Err()
}

func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.execWrite(ctx, `DELETE FROM sessions WHERE expires_at < ?`, toMillis(now))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
