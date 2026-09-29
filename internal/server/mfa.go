package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"shortr/internal/auth"
	"shortr/internal/notify"
	"shortr/internal/store"
	"shortr/internal/validate"
)

// Two-factor sign-in (TOTP) for accounts that sign in with a password.
// SSO users get MFA from their identity provider instead.
//
// Login is two steps: POST /auth/login answers {mfaRequired, mfaToken} for
// an enrolled user, and POST /auth/login/mfa trades that short-lived token
// plus a 6-digit code (or a recovery code) for a session.

const (
	mfaTokenTTL       = 5 * time.Minute
	mfaTokenPrefix    = "mfa-login|"
	recoveryCodeCount = 10
	mfaIssuerFallback = "Shortr"
)

var (
	ErrMFAInvalid        = NewAPIError(http.StatusUnauthorized, "MFA_INVALID", "that code didn't work, try the next one from your app")
	ErrMFAEnrollRequired = NewAPIError(http.StatusForbidden, "MFA_ENROLLMENT_REQUIRED", "set up two-factor authentication to continue")
	ErrMFANoPassword     = NewAPIError(http.StatusConflict, "MFA_NOT_AVAILABLE", "two-factor sign-in is for password accounts; your SSO provider handles it for SSO sign-ins")
)

func (s *Server) userMFA(ctx context.Context, userID string) *store.UserMFA {
	m, err := s.store.GetUserMFA(ctx, userID)
	if err != nil {
		return nil
	}
	return m
}

// mfaEnrollRequired: admin policy says password sign-ins need MFA and this
// session is a password session of a user who hasn't enrolled yet.
func (s *Server) mfaEnrollRequired(r *http.Request, u *store.User) bool {
	if !s.live().MFARequired || u.PasswordHash == nil {
		return false
	}
	sess := sessionFromContext(r.Context())
	if sess == nil || sess.AuthMethod == "oidc" {
		return false
	}
	return !s.userMFA(r.Context(), u.ID).Enabled()
}

// mfaGateAllows lists what an un-enrolled user may still do while the
// policy blocks them: read who they are, enroll, re-authenticate.
func mfaGateAllows(r *http.Request) bool {
	p := r.URL.Path
	// their own account (incl. the security page's sessions list), not links or admin
	return p == "/api/v1/me" || strings.HasPrefix(p, "/api/v1/me/") ||
		strings.HasPrefix(p, "/api/v1/notifications") || strings.HasPrefix(p, "/auth/")
}

// --- login second step ----------------------------------------------------

type mfaLoginReq struct {
	MFAToken     string `json:"mfaToken"`
	Code         string `json:"code"`
	RecoveryCode string `json:"recoveryCode"`
}

func (s *Server) handleLoginMFA(w http.ResponseWriter, r *http.Request) {
	var req mfaLoginReq
	if err := decodeJSON(w, r, 2048, &req); err != nil {
		respondError(w, r, err)
		return
	}
	payload, err := auth.OpenValue(s.cfg.SecretKey, req.MFAToken)
	if err != nil || !strings.HasPrefix(payload, mfaTokenPrefix) {
		respondError(w, r, NewAPIError(http.StatusUnauthorized, "MFA_EXPIRED", "your sign-in timed out, enter your password again"))
		return
	}
	userID := strings.TrimPrefix(payload, mfaTokenPrefix)
	lockKey := "mfa-fail:" + userID
	if !s.rlAuth.Peek(lockKey) {
		respondError(w, r, ErrLockedOut)
		return
	}
	u, err := s.store.GetUserByID(r.Context(), userID)
	if err != nil || !u.IsActive() {
		respondError(w, r, ErrUnauthenticated)
		return
	}
	method, err := s.checkSecondFactor(r.Context(), u.ID, req.Code, req.RecoveryCode)
	if err != nil {
		s.rlAuth.Allow(lockKey)
		s.audit(r, u.ID, "user.mfa_failed", "user", u.ID, nil)
		respondError(w, r, err)
		return
	}
	s.rlAuth.Reset(lockKey)
	now := time.Now()
	u.LastLoginAt = &now
	_ = s.store.UpdateUser(r.Context(), u)
	meta := map[string]any{"mfa": method}
	if method == "recovery_code" {
		if m := s.userMFA(r.Context(), u.ID); m != nil {
			meta["recovery_codes_left"] = len(m.RecoveryCodes)
		}
		if s.notifier != nil {
			s.notifier.NotifyUser(r.Context(), u.ID, u.Email, notify.KindMFAChanged, "Recovery code used",
				"A recovery code was used to sign in to your account. If this wasn't you, change your password and reset two-factor sign-in.", nil)
		}
	}
	s.audit(r, u.ID, "user.login", "user", u.ID, meta)
	sess, ok := s.startSession(w, r, u, "password")
	if !ok {
		return
	}
	respondJSON(w, http.StatusOK, s.meDTOWithSession(r, u, sess))
}

// checkSecondFactor accepts either a current TOTP code (each step once) or
// an unused recovery code, and says which one it was.
func (s *Server) checkSecondFactor(ctx context.Context, userID, code, recovery string) (string, error) {
	m := s.userMFA(ctx, userID)
	if !m.Enabled() {
		return "", ErrMFAInvalid
	}
	if strings.TrimSpace(recovery) != "" {
		ok, err := s.store.ConsumeRecoveryCode(ctx, userID, auth.HashRecoveryCode(recovery))
		if err != nil || !ok {
			return "", ErrMFAInvalid
		}
		return "recovery_code", nil
	}
	secret, err := auth.OpenSecret(s.cfg.SecretKey, m.SealedSecret)
	if err != nil {
		s.log.Error("cannot open MFA secret", "user_id", userID, "error", err)
		return "", ErrInternal
	}
	step, ok := auth.VerifyTOTP(secret, code, time.Now())
	if !ok {
		return "", ErrMFAInvalid
	}
	if fresh, err := s.store.AdvanceMFAStep(ctx, userID, step); err != nil || !fresh {
		return "", ErrMFAInvalid // replayed code
	}
	return "totp", nil
}

// --- self-service ---------------------------------------------------------

type mfaStatusDTO struct {
	Available         bool `json:"available"`
	Enabled           bool `json:"enabled"`
	Required          bool `json:"required"`
	RecoveryCodesLeft int  `json:"recoveryCodesLeft"`
}

func (s *Server) handleGetMFA(w http.ResponseWriter, r *http.Request, u *store.User) {
	m := s.userMFA(r.Context(), u.ID)
	out := mfaStatusDTO{Available: u.PasswordHash != nil, Enabled: m.Enabled(), Required: s.live().MFARequired && u.PasswordHash != nil}
	if m.Enabled() {
		out.RecoveryCodesLeft = len(m.RecoveryCodes)
	}
	respondJSON(w, http.StatusOK, out)
}

// handleMFASetup starts (or restarts) enrollment with a fresh secret. It is
// not active until confirmed with a code, so a half-finished setup never
// locks anyone out.
func (s *Server) handleMFASetup(w http.ResponseWriter, r *http.Request, u *store.User) {
	if u.PasswordHash == nil {
		respondError(w, r, ErrMFANoPassword)
		return
	}
	if !s.hasSudo(r) {
		respondError(w, r, ErrSudoRequired)
		return
	}
	if s.userMFA(r.Context(), u.ID).Enabled() {
		respondError(w, r, NewAPIError(http.StatusConflict, "MFA_ALREADY_ENABLED", "two-factor sign-in is already on; turn it off first to move it to a new device"))
		return
	}
	secret, err := auth.NewTOTPSecret()
	if err != nil {
		respondError(w, r, err)
		return
	}
	sealed, err := auth.SealSecret(s.cfg.SecretKey, secret)
	if err != nil {
		respondError(w, r, err)
		return
	}
	if err := s.store.PutUserMFA(r.Context(), &store.UserMFA{UserID: u.ID, SealedSecret: sealed}); err != nil {
		respondError(w, r, err)
		return
	}
	issuer := s.siteNameOrDefault(r.Context())
	if issuer == "" {
		issuer = mfaIssuerFallback
	}
	respondJSON(w, http.StatusOK, map[string]string{"secret": secret, "otpauthUrl": auth.TOTPURI(issuer, u.Email, secret)})
}

func (s *Server) handleMFAEnable(w http.ResponseWriter, r *http.Request, u *store.User) {
	var req struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(w, r, 512, &req); err != nil {
		respondError(w, r, err)
		return
	}
	m := s.userMFA(r.Context(), u.ID)
	if m == nil || m.Enabled() {
		respondError(w, r, NewAPIError(http.StatusConflict, "MFA_NOT_PENDING", "start the setup again"))
		return
	}
	secret, err := auth.OpenSecret(s.cfg.SecretKey, m.SealedSecret)
	if err != nil {
		respondError(w, r, err)
		return
	}
	step, ok := auth.VerifyTOTP(secret, req.Code, time.Now())
	if !ok {
		verrs := validate.Errors{}
		verrs.Add("code", "that code doesn't match; check the time on your phone and try the newest code")
		respondError(w, r, ValidationFailed(verrs))
		return
	}
	codes, hashes, err := auth.NewRecoveryCodes(recoveryCodeCount)
	if err != nil {
		respondError(w, r, err)
		return
	}
	now := time.Now()
	m.EnabledAt, m.LastStep, m.RecoveryCodes = &now, step, hashes
	if err := s.store.PutUserMFA(r.Context(), m); err != nil {
		respondError(w, r, err)
		return
	}
	s.audit(r, u.ID, "user.mfa_enabled", "user", u.ID, nil)
	if s.notifier != nil {
		s.notifier.NotifyUser(r.Context(), u.ID, u.Email, notify.KindMFAChanged, "Two-factor sign-in turned on",
			"Two-factor sign-in is now on for your account. Keep your recovery codes somewhere safe.", nil)
	}
	respondJSON(w, http.StatusOK, map[string]any{"recoveryCodes": codes})
}

func (s *Server) handleMFARegenerateCodes(w http.ResponseWriter, r *http.Request, u *store.User) {
	if !s.hasSudo(r) {
		respondError(w, r, ErrSudoRequired)
		return
	}
	m := s.userMFA(r.Context(), u.ID)
	if !m.Enabled() {
		respondError(w, r, NewAPIError(http.StatusConflict, "MFA_NOT_ENABLED", "two-factor sign-in is off"))
		return
	}
	codes, hashes, err := auth.NewRecoveryCodes(recoveryCodeCount)
	if err != nil {
		respondError(w, r, err)
		return
	}
	m.RecoveryCodes = hashes
	if err := s.store.PutUserMFA(r.Context(), m); err != nil {
		respondError(w, r, err)
		return
	}
	s.audit(r, u.ID, "user.mfa_recovery_regenerated", "user", u.ID, nil)
	respondJSON(w, http.StatusOK, map[string]any{"recoveryCodes": codes})
}

func (s *Server) handleMFADisable(w http.ResponseWriter, r *http.Request, u *store.User) {
	if !s.hasSudo(r) {
		respondError(w, r, ErrSudoRequired)
		return
	}
	if err := s.store.DeleteUserMFA(r.Context(), u.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
		respondError(w, r, err)
		return
	}
	s.audit(r, u.ID, "user.mfa_disabled", "user", u.ID, nil)
	if s.notifier != nil {
		s.notifier.NotifyUser(r.Context(), u.ID, u.Email, notify.KindMFAChanged, "Two-factor sign-in turned off",
			"Two-factor sign-in was turned off for your account. If this wasn't you, change your password now.", nil)
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleAdminResetMFA helps a user who lost their phone and recovery codes.
// Their sessions are revoked so the next sign-in starts clean.
func (s *Server) handleAdminResetMFA(w http.ResponseWriter, r *http.Request, actor *store.User, id string) {
	u, err := s.store.GetUserByID(r.Context(), id)
	if err != nil {
		respondError(w, r, ErrNotFound)
		return
	}
	if err := s.store.DeleteUserMFA(r.Context(), u.ID); err != nil {
		respondError(w, r, err)
		return
	}
	_ = s.store.DeleteSessionsForUser(r.Context(), u.ID)
	s.audit(r, actor.ID, "user.admin_reset_mfa", "user", u.ID, map[string]any{"email": u.Email})
	w.WriteHeader(http.StatusNoContent)
}
