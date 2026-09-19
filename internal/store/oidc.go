package store

import (
	"context"
	"database/sql"
	"time"

	"shortr/internal/ulid"
)

func (s *Store) CreateOIDCIdentity(ctx context.Context, id *OIDCIdentity) error {
	if id.ID == "" {
		id.ID = ulid.New()
	}
	id.CreatedAt = time.Now()
	_, err := s.execWrite(ctx, `INSERT INTO oidc_identities (id, user_id, issuer, subject, email, name, raw_claims, created_at, last_login_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		id.ID, id.UserID, id.Issuer, id.Subject, id.Email, id.Name, id.RawClaims, toMillis(id.CreatedAt), nullMillis(id.LastLoginAt))
	if err != nil {
		return classifyErr(err)
	}
	return nil
}

const oidcCols = `id, user_id, issuer, subject, email, name, raw_claims, created_at, last_login_at`

func oidcFromRow(row interface{ Scan(...any) error }) (*OIDCIdentity, error) {
	var o OIDCIdentity
	var email, name, raw sql.NullString
	var lastLogin sql.NullInt64
	var createdMS int64
	if err := row.Scan(&o.ID, &o.UserID, &o.Issuer, &o.Subject, &email, &name, &raw, &createdMS, &lastLogin); err != nil {
		return nil, err
	}
	o.Email, o.Name, o.RawClaims = email.String, name.String, raw.String
	o.CreatedAt = fromMillis(createdMS)
	o.LastLoginAt = scanNullMillis(lastLogin)
	return &o, nil
}

func (s *Store) GetOIDCIdentity(ctx context.Context, issuer, subject string) (*OIDCIdentity, error) {
	row := s.queryRow(ctx, `SELECT `+oidcCols+` FROM oidc_identities WHERE issuer = ? AND subject = ?`, issuer, subject)
	o, err := oidcFromRow(row)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return o, err
}

func (s *Store) ListOIDCIdentitiesForUser(ctx context.Context, userID string) ([]*OIDCIdentity, error) {
	rows, err := s.query(ctx, `SELECT `+oidcCols+` FROM oidc_identities WHERE user_id = ? ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*OIDCIdentity
	for rows.Next() {
		o, err := oidcFromRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) CountOIDCIdentitiesForUser(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.queryRow(ctx, `SELECT count(*) FROM oidc_identities WHERE user_id = ?`, userID).Scan(&n)
	return n, err
}

func (s *Store) UpdateOIDCIdentity(ctx context.Context, o *OIDCIdentity, loginAt time.Time) error {
	o.LastLoginAt = &loginAt
	_, err := s.execWrite(ctx, `UPDATE oidc_identities SET email=?, name=?, raw_claims=?, last_login_at=? WHERE id=?`,
		o.Email, o.Name, o.RawClaims, toMillis(loginAt), o.ID)
	return err
}

func (s *Store) DeleteOIDCIdentity(ctx context.Context, id string) error {
	_, err := s.execWrite(ctx, `DELETE FROM oidc_identities WHERE id = ?`, id)
	return err
}

func (s *Store) GetOIDCIdentityByID(ctx context.Context, id string) (*OIDCIdentity, error) {
	row := s.queryRow(ctx, `SELECT `+oidcCols+` FROM oidc_identities WHERE id = ?`, id)
	o, err := oidcFromRow(row)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return o, err
}
