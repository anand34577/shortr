package store

import (
	"context"
	"strings"
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

// AuditFilter narrows ListAudit. Action matches as a prefix ("user." finds
// every user event); the rest are exact.
type AuditFilter struct {
	Action   string
	ActorID  string
	TargetID string
	Cursor   string
	Limit    int
}

func (s *Store) ListAudit(ctx context.Context, f AuditFilter) ([]*AuditEntry, string, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var where []string
	var args []any
	if f.Action != "" {
		where = append(where, `substr(action, 1, ?) = ?`)
		args = append(args, len(f.Action), f.Action)
	}
	if f.ActorID != "" {
		where = append(where, `actor_user_id = ?`)
		args = append(args, f.ActorID)
	}
	if f.TargetID != "" {
		where = append(where, `target_id = ?`)
		args = append(args, f.TargetID)
	}
	if f.Cursor != "" {
		ms, id, err := decodeCursor(f.Cursor)
		if err != nil {
			return nil, "", err
		}
		where = append(where, `((ts < ?) OR (ts = ? AND id < ?))`)
		args = append(args, ms, ms, id)
	}
	q := `SELECT id, ts, actor_user_id, actor_ip, action, target_type, target_id, meta FROM audit_log`
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, " AND ")
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
