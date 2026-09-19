package store

import (
	"context"
	"time"

	"shortr/internal/ulid"
)

func (s *Store) AddAudit(ctx context.Context, e *AuditEntry) error {
	if e.ID == "" {
		e.ID = ulid.New()
	}
	e.TS = time.Now()
	if e.Meta == "" {
		e.Meta = "{}"
	}
	_, err := s.execWrite(ctx, `INSERT INTO audit_log (id, ts, actor_user_id, actor_ip, action, target_type, target_id, meta) VALUES (?,?,?,?,?,?,?,?)`,
		e.ID, toMillis(e.TS), e.ActorUserID, e.ActorIP, e.Action, e.TargetType, e.TargetID, e.Meta)
	return err
}

func (s *Store) ListAudit(ctx context.Context, cursor string, limit int) ([]*AuditEntry, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var args []any
	q := `SELECT id, ts, actor_user_id, actor_ip, action, target_type, target_id, meta FROM audit_log`
	if cursor != "" {
		ms, id, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		q += ` WHERE (ts < ?) OR (ts = ? AND id < ?)`
		args = append(args, ms, ms, id)
	}
	q += ` ORDER BY ts DESC, id DESC LIMIT ?`
	args = append(args, limit+1)
	res, err := s.query(ctx, q, args...)
	if err != nil {
		return nil, "", err
	}
	defer res.Close()
	var out []*AuditEntry
	for res.Next() {
		var e AuditEntry
		var tsMS int64
		if err := res.Scan(&e.ID, &tsMS, &e.ActorUserID, &e.ActorIP, &e.Action, &e.TargetType, &e.TargetID, &e.Meta); err != nil {
			return nil, "", err
		}
		e.TS = fromMillis(tsMS)
		out = append(out, &e)
	}
	next := ""
	if len(out) > limit {
		last := out[limit-1]
		next = encodeCursor(toMillis(last.TS), last.ID)
		out = out[:limit]
	}
	return out, next, res.Err()
}
