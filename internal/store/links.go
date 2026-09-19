package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"shortr/internal/ulid"
)

const linkCols = `id, code, target_url, title, description, user_id, redirect_status, password_hash, expires_at, max_clicks, click_count, last_click_at, status, deleted_at, utm_source, utm_medium, utm_campaign, utm_term, utm_content, pass_query, tags, created_at, updated_at, created_by_ip`

func linkFromRow(row interface{ Scan(...any) error }) (*Link, error) {
	var l Link
	var userID sql.NullString
	var pwHash sql.NullString
	var expires, lastClick, deleted sql.NullInt64
	var maxClicks sql.NullInt64
	var passQuery int
	var tagsJSON string
	var createdMS, updatedMS int64
	err := row.Scan(&l.ID, &l.Code, &l.TargetURL, &l.Title, &l.Description, &userID, &l.RedirectStatus, &pwHash,
		&expires, &maxClicks, &l.ClickCount, &lastClick, &l.Status, &deleted,
		&l.UTMSource, &l.UTMMedium, &l.UTMCampaign, &l.UTMTerm, &l.UTMContent, &passQuery, &tagsJSON,
		&createdMS, &updatedMS, &l.CreatedByIP)
	if err != nil {
		return nil, err
	}
	if userID.Valid {
		l.UserID = &userID.String
	}
	if pwHash.Valid {
		l.PasswordHash = &pwHash.String
	}
	l.ExpiresAt = scanNullMillis(expires)
	l.LastClickAt = scanNullMillis(lastClick)
	l.DeletedAt = scanNullMillis(deleted)
	if maxClicks.Valid {
		n := int(maxClicks.Int64)
		l.MaxClicks = &n
	}
	l.PassQuery = passQuery != 0
	json.Unmarshal([]byte(tagsJSON), &l.Tags) //nolint:errcheck
	l.CreatedAt = fromMillis(createdMS)
	l.UpdatedAt = fromMillis(updatedMS)
	return &l, nil
}

func (s *Store) CreateLink(ctx context.Context, l *Link) error {
	if l.ID == "" {
		l.ID = ulid.New()
	}
	now := time.Now()
	l.CreatedAt, l.UpdatedAt = now, now
	if l.Status == "" {
		l.Status = "active"
	}
	if l.Tags == nil {
		l.Tags = []string{}
	}
	tagsJSON, _ := json.Marshal(l.Tags)
	_, err := s.execWrite(ctx, `INSERT INTO links (`+linkCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		l.ID, l.Code, l.TargetURL, l.Title, l.Description, l.UserID, l.RedirectStatus, l.PasswordHash,
		nullMillis(l.ExpiresAt), l.MaxClicks, l.ClickCount, nullMillis(l.LastClickAt), l.Status, nullMillis(l.DeletedAt),
		l.UTMSource, l.UTMMedium, l.UTMCampaign, l.UTMTerm, l.UTMContent, boolToInt(l.PassQuery), string(tagsJSON),
		toMillis(l.CreatedAt), toMillis(l.UpdatedAt), l.CreatedByIP)
	if err != nil {
		return classifyErr(err)
	}
	return nil
}

func (s *Store) GetLinkByID(ctx context.Context, id string) (*Link, error) {
	row := s.queryRow(ctx, `SELECT `+linkCols+` FROM links WHERE id = ?`, id)
	l, err := linkFromRow(row)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return l, err
}

// GetLinkByCode looks up an exact match first (the common, hot-path case),
// falling back to a case-insensitive match so `/ABC` resolves the same link
// as `/abc`.
func (s *Store) GetLinkByCode(ctx context.Context, code string) (*Link, error) {
	row := s.queryRow(ctx, `SELECT `+linkCols+` FROM links WHERE code = ?`, code)
	l, err := linkFromRow(row)
	if err == nil {
		return l, nil
	}
	if !isNoRows(err) {
		return nil, err
	}
	row = s.queryRow(ctx, `SELECT `+linkCols+` FROM links WHERE LOWER(code) = LOWER(?)`, code)
	l, err = linkFromRow(row)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return l, err
}

func (s *Store) CodeExists(ctx context.Context, code string) (bool, error) {
	var n int
	err := s.queryRow(ctx, `SELECT count(*) FROM links WHERE LOWER(code) = LOWER(?)`, code).Scan(&n)
	return n > 0, err
}

func (s *Store) UpdateLink(ctx context.Context, l *Link) error {
	l.UpdatedAt = time.Now()
	if l.Tags == nil {
		l.Tags = []string{}
	}
	tagsJSON, _ := json.Marshal(l.Tags)
	_, err := s.execWrite(ctx, `UPDATE links SET code=?, target_url=?, title=?, description=?, user_id=?, redirect_status=?, password_hash=?, expires_at=?, max_clicks=?, status=?, utm_source=?, utm_medium=?, utm_campaign=?, utm_term=?, utm_content=?, pass_query=?, tags=?, updated_at=? WHERE id=?`,
		l.Code, l.TargetURL, l.Title, l.Description, l.UserID, l.RedirectStatus, l.PasswordHash,
		nullMillis(l.ExpiresAt), l.MaxClicks, l.Status, l.UTMSource, l.UTMMedium, l.UTMCampaign, l.UTMTerm, l.UTMContent,
		boolToInt(l.PassQuery), string(tagsJSON), toMillis(l.UpdatedAt), l.ID)
	if err != nil {
		return classifyErr(err)
	}
	return nil
}

func (s *Store) SoftDeleteLink(ctx context.Context, id string, at time.Time) error {
	_, err := s.execWrite(ctx, `UPDATE links SET deleted_at = ?, updated_at = ? WHERE id = ?`, toMillis(at), toMillis(at), id)
	return err
}

func (s *Store) RestoreLink(ctx context.Context, id string) error {
	_, err := s.execWrite(ctx, `UPDATE links SET deleted_at = NULL, updated_at = ? WHERE id = ?`, toMillis(time.Now()), id)
	return err
}

func (s *Store) PurgeLink(ctx context.Context, id string) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := s.txExec(tx, ctx, `DELETE FROM links WHERE id = ?`, id); err != nil {
			return err
		}
		if _, err := s.txExec(tx, ctx, `DELETE FROM clicks WHERE link_id = ?`, id); err != nil {
			return err
		}
		if _, err := s.txExec(tx, ctx, `DELETE FROM click_rollups_daily WHERE link_id = ?`, id); err != nil {
			return err
		}
		return nil
	})
}

// PurgeSoftDeletedBefore hard-deletes links (and their clicks) that were
// soft-deleted before cutoff — the 30-day reservation window.
func (s *Store) PurgeSoftDeletedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	rows, err := s.query(ctx, `SELECT id FROM links WHERE deleted_at IS NOT NULL AND deleted_at < ?`, toMillis(cutoff))
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if err := s.PurgeLink(ctx, id); err != nil {
			return int64(len(ids)), err
		}
	}
	return int64(len(ids)), nil
}

func (s *Store) IncrementClickCount(ctx context.Context, linkID string, n int64, at time.Time) error {
	_, err := s.execWrite(ctx, `UPDATE links SET click_count = click_count + ?, last_click_at = ? WHERE id = ?`, n, toMillis(at), linkID)
	return err
}

type LinkFilter struct {
	UserID string // "" = all users (admin)
	Query  string
	Tag    string
	Status string // "", active, disabled, deleted
	Sort   string // created_at | clicks | title
	Order  string // asc | desc
	Cursor string
	Limit  int
}

func (s *Store) ListLinks(ctx context.Context, f LinkFilter) ([]*Link, string, error) {
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	var where []string
	var args []any
	if f.UserID != "" {
		where = append(where, "user_id = ?")
		args = append(args, f.UserID)
	}
	switch f.Status {
	case "deleted":
		where = append(where, "deleted_at IS NOT NULL")
	case "active", "disabled":
		where = append(where, "deleted_at IS NULL AND status = ?")
		args = append(args, f.Status)
	default:
		where = append(where, "deleted_at IS NULL")
	}
	if f.Query != "" {
		where = append(where, "(code LIKE ? OR title LIKE ? OR target_url LIKE ?)")
		q := "%" + escapeLike(f.Query) + "%"
		args = append(args, q, q, q)
	}
	if f.Tag != "" {
		where = append(where, "tags LIKE ?")
		args = append(args, "%\""+f.Tag+"\"%")
	}

	sortCol := "created_at"
	switch f.Sort {
	case "clicks":
		sortCol = "click_count"
	case "title":
		sortCol = "title"
	}
	order := "DESC"
	if strings.ToLower(f.Order) == "asc" {
		order = "ASC"
	}

	if f.Cursor != "" {
		ms, id, err := decodeCursor(f.Cursor)
		if err != nil {
			return nil, "", err
		}
		cmp := "<"
		if order == "ASC" {
			cmp = ">"
		}
		where = append(where, "((created_at "+cmp+" ?) OR (created_at = ? AND id "+cmp+" ?))")
		args = append(args, ms, ms, id)
	}

	sql := "SELECT " + linkCols + " FROM links"
	if len(where) > 0 {
		sql += " WHERE " + strings.Join(where, " AND ")
	}
	sql += " ORDER BY " + sortCol + " " + order + ", id " + order + " LIMIT ?"
	args = append(args, limit+1)

	rows, err := s.query(ctx, sql, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []*Link
	for rows.Next() {
		l, err := linkFromRow(rows)
		if err != nil {
			return nil, "", err
		}
		out = append(out, l)
	}
	next := ""
	if len(out) > limit {
		last := out[limit-1]
		next = encodeCursor(toMillis(last.CreatedAt), last.ID)
		out = out[:limit]
	}
	return out, next, rows.Err()
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

func (s *Store) CountLinksForUser(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.queryRow(ctx, `SELECT count(*) FROM links WHERE user_id = ? AND deleted_at IS NULL`, userID).Scan(&n)
	return n, err
}

// CountActiveLinks reports active, non-deleted links — scoped to userID if
// non-empty, else across all users (admin "all" view). Feeds the dashboard
// "Active links" stat tile.
func (s *Store) CountActiveLinks(ctx context.Context, userID string) (int, error) {
	var n int
	var err error
	if userID != "" {
		err = s.queryRow(ctx, `SELECT count(*) FROM links WHERE user_id = ? AND deleted_at IS NULL AND status = 'active'`, userID).Scan(&n)
	} else {
		err = s.queryRow(ctx, `SELECT count(*) FROM links WHERE deleted_at IS NULL AND status = 'active'`).Scan(&n)
	}
	return n, err
}

func (s *Store) ListExpiringLinks(ctx context.Context, before time.Time) ([]*Link, error) {
	rows, err := s.query(ctx, `SELECT `+linkCols+` FROM links WHERE deleted_at IS NULL AND status='active' AND expires_at IS NOT NULL AND expires_at < ? AND expires_at > ?`,
		toMillis(before), toMillis(time.Now()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Link
	for rows.Next() {
		l, err := linkFromRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
