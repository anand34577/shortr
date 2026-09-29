package store

import (
	"context"
	"encoding/json"
	"time"
)

// UserMFA is a user's TOTP enrollment. Secret is stored sealed; the store
// never sees the plaintext.
type UserMFA struct {
	UserID        string
	SealedSecret  string
	EnabledAt     *time.Time // nil = enrollment started but not confirmed
	LastStep      int64
	RecoveryCodes []string // sha256 hex of each unused code
}

func (m *UserMFA) Enabled() bool { return m != nil && m.EnabledAt != nil }

func (s *Store) GetUserMFA(ctx context.Context, userID string) (*UserMFA, error) {
	var m UserMFA
	var enabled *int64
	var codes string
	err := s.queryRow(ctx, `SELECT user_id, totp_secret, enabled_at, last_step, recovery_codes FROM user_mfa WHERE user_id = ?`, userID).
		Scan(&m.UserID, &m.SealedSecret, &enabled, &m.LastStep, &codes)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if enabled != nil {
		t := fromMillis(*enabled)
		m.EnabledAt = &t
	}
	_ = json.Unmarshal([]byte(codes), &m.RecoveryCodes)
	return &m, nil
}

// PutUserMFA inserts or replaces a user's enrollment.
func (s *Store) PutUserMFA(ctx context.Context, m *UserMFA) error {
	codes, _ := json.Marshal(m.RecoveryCodes)
	if m.RecoveryCodes == nil {
		codes = []byte("[]")
	}
	var enabled *int64
	if m.EnabledAt != nil {
		v := toMillis(*m.EnabledAt)
		enabled = &v
	}
	_, err := s.execWrite(ctx, `INSERT INTO user_mfa (user_id, totp_secret, enabled_at, last_step, recovery_codes, updated_at) VALUES (?,?,?,?,?,?)
		ON CONFLICT (user_id) DO UPDATE SET totp_secret = excluded.totp_secret, enabled_at = excluded.enabled_at,
		last_step = excluded.last_step, recovery_codes = excluded.recovery_codes, updated_at = excluded.updated_at`,
		m.UserID, m.SealedSecret, enabled, m.LastStep, string(codes), toMillis(time.Now()))
	return err
}

// AdvanceMFAStep records an accepted TOTP step. It only succeeds if step is
// newer than the stored one, so a code can't be replayed even by two
// concurrent requests.
func (s *Store) AdvanceMFAStep(ctx context.Context, userID string, step int64) (bool, error) {
	res, err := s.execWrite(ctx, `UPDATE user_mfa SET last_step = ?, updated_at = ? WHERE user_id = ? AND last_step < ?`,
		step, toMillis(time.Now()), userID, step)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// ConsumeRecoveryCode removes one code hash; false if it wasn't present.
// The compare-and-swap on the old JSON keeps a code single-use under races.
func (s *Store) ConsumeRecoveryCode(ctx context.Context, userID, codeHash string) (bool, error) {
	m, err := s.GetUserMFA(ctx, userID)
	if err != nil {
		return false, err
	}
	old, _ := json.Marshal(m.RecoveryCodes)
	left := make([]string, 0, len(m.RecoveryCodes))
	found := false
	for _, h := range m.RecoveryCodes {
		if !found && h == codeHash {
			found = true
			continue
		}
		left = append(left, h)
	}
	if !found {
		return false, nil
	}
	next, _ := json.Marshal(left)
	res, err := s.execWrite(ctx, `UPDATE user_mfa SET recovery_codes = ?, updated_at = ? WHERE user_id = ? AND recovery_codes = ?`,
		string(next), toMillis(time.Now()), userID, string(old))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (s *Store) DeleteUserMFA(ctx context.Context, userID string) error {
	_, err := s.execWrite(ctx, `DELETE FROM user_mfa WHERE user_id = ?`, userID)
	return err
}

// MFAEnabledUserIDs returns which of ids have confirmed MFA, for list views.
func (s *Store) MFAEnabledUserIDs(ctx context.Context) (map[string]bool, error) {
	rows, err := s.query(ctx, `SELECT user_id FROM user_mfa WHERE enabled_at IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
