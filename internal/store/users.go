package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"shortr/internal/ulid"
)

func toMillis(t time.Time) int64    { return t.UnixMilli() }
func fromMillis(ms int64) time.Time { return time.UnixMilli(ms) }

func nullMillis(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UnixMilli()
}

func scanNullMillis(ns sql.NullInt64) *time.Time {
	if !ns.Valid {
		return nil
	}
	t := fromMillis(ns.Int64)
	return &t
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

const userCols = `id, email, email_verified, name, password_hash, role, status, max_links, role_locked, must_change_password, last_login_at, password_changed_at, created_at, updated_at`

// CreateUser inserts a new user. email must already be validated/normalised.
func (s *Store) CreateUser(ctx context.Context, u *User) error {
	if u.ID == "" {
		u.ID = ulid.New()
	}
	now := time.Now()
	u.CreatedAt, u.UpdatedAt = now, now
	if u.Role == "" {
		u.Role = "user"
	}
	if u.Status == "" {
		u.Status = "active"
	}
	_, err := s.execWrite(ctx, `INSERT INTO users (id, email, email_verified, name, password_hash, role, status, max_links, role_locked, must_change_password, last_login_at, password_changed_at, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		u.ID, u.Email, boolToInt(u.EmailVerified), u.Name, u.PasswordHash, u.Role, u.Status, u.MaxLinks, boolToInt(u.RoleLocked), boolToInt(u.MustChangePassword),
		nullMillis(u.LastLoginAt), nullMillis(u.PasswordChangedAt), toMillis(u.CreatedAt), toMillis(u.UpdatedAt))
	if err != nil {
		return fmt.Errorf("create user: %w", classifyErr(err))
	}
	return nil
}

func (s *Store) userFromRow(row interface{ Scan(...any) error }) (*User, error) {
	var u User
	var pwHash sql.NullString
	var maxLinks sql.NullInt64
	var lastLogin, pwChanged sql.NullInt64
	var emailVerified, roleLocked, mustChange int
	var createdMS, updatedMS int64
	if err := row.Scan(&u.ID, &u.Email, &emailVerified, &u.Name, &pwHash, &u.Role, &u.Status, &maxLinks, &roleLocked, &mustChange, &lastLogin, &pwChanged, &createdMS, &updatedMS); err != nil {
		return nil, err
	}
	u.EmailVerified = emailVerified != 0
	u.RoleLocked = roleLocked != 0
	u.MustChangePassword = mustChange != 0
	if pwHash.Valid {
		u.PasswordHash = &pwHash.String
	}
	if maxLinks.Valid {
		n := int(maxLinks.Int64)
		u.MaxLinks = &n
	}
	u.LastLoginAt = scanNullMillis(lastLogin)
	u.PasswordChangedAt = scanNullMillis(pwChanged)
	u.CreatedAt = fromMillis(createdMS)
	u.UpdatedAt = fromMillis(updatedMS)
	return &u, nil
}

func (s *Store) GetUserByID(ctx context.Context, id string) (*User, error) {
	row := s.queryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id)
	u, err := s.userFromRow(row)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	row := s.queryRow(ctx, `SELECT `+userCols+` FROM users WHERE email = ?`, email)
	u, err := s.userFromRow(row)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.queryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CountAdmins(ctx context.Context, excludeUserID string) (int, error) {
	var n int
	err := s.queryRow(ctx, `SELECT count(*) FROM users WHERE role = 'admin' AND status = 'active' AND id != ?`, excludeUserID).Scan(&n)
	return n, err
}

// UpdateUser persists mutable fields of u (identified by ID).
func (s *Store) UpdateUser(ctx context.Context, u *User) error {
	u.UpdatedAt = time.Now()
	_, err := s.execWrite(ctx, `UPDATE users SET email=?, email_verified=?, name=?, password_hash=?, role=?, status=?, max_links=?, role_locked=?, must_change_password=?, last_login_at=?, password_changed_at=?, updated_at=? WHERE id=?`,
		u.Email, boolToInt(u.EmailVerified), u.Name, u.PasswordHash, u.Role, u.Status, u.MaxLinks, boolToInt(u.RoleLocked), boolToInt(u.MustChangePassword),
		nullMillis(u.LastLoginAt), nullMillis(u.PasswordChangedAt), toMillis(u.UpdatedAt), u.ID)
	if err != nil {
		return fmt.Errorf("update user: %w", classifyErr(err))
	}
	return nil
}

func (s *Store) DeleteUser(ctx context.Context, id string) error {
	_, err := s.execWrite(ctx, `DELETE FROM users WHERE id = ?`, id)
	return err
}

// ListActiveAdmins returns all active admin users — used to fan out
// broadcast notifications (new user registered, backup failed, etc).
func (s *Store) ListActiveAdmins(ctx context.Context) ([]*User, error) {
	rows, err := s.query(ctx, `SELECT `+userCols+` FROM users WHERE role = 'admin' AND status = 'active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u, err := s.userFromRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) ListUsers(ctx context.Context, q, cursor string, limit int) ([]*User, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	var rows *sql.Rows
	var err error
	where, args := "1=1", []any{}
	if q = strings.ToLower(strings.TrimSpace(q)); q != "" {
		like := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(q) + "%"
		where = `(lower(email) LIKE ? ESCAPE '\' OR lower(name) LIKE ? ESCAPE '\')`
		args = append(args, like, like)
	}
	if cursor != "" {
		createdMS, id, cErr := decodeCursor(cursor)
		if cErr != nil {
			return nil, "", cErr
		}
		where += ` AND ((created_at < ?) OR (created_at = ? AND id < ?))`
		args = append(args, createdMS, createdMS, id)
	}
	args = append(args, limit+1)
	rows, err = s.query(ctx, `SELECT `+userCols+` FROM users WHERE `+where+` ORDER BY created_at DESC, id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u, err := s.userFromRow(rows)
		if err != nil {
			return nil, "", err
		}
		out = append(out, u)
	}
	next := ""
	if len(out) > limit {
		last := out[limit-1]
		next = encodeCursor(toMillis(last.CreatedAt), last.ID)
		out = out[:limit]
	}
	return out, next, rows.Err()
}
