package server

import (
	"net/http"
	"strings"
	"time"

	"shortr/internal/auth"
	"shortr/internal/notify"
	"shortr/internal/store"
	"shortr/internal/validate"
)

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, u *store.User) {
	respondJSON(w, http.StatusOK, s.toMeDTO(r, u))
}

// toMeDTO builds the full "me" shape, including the CSRF token the SPA
// needs for its next mutating request. Every endpoint that establishes a
// session (setup/login/register — see auth.go) must return this, not a bare
// userDTO: the frontend caches the response directly as its "me" query
// result rather than re-fetching /api/v1/me, so a bare user object would
// leave the client with no CSRF token until the next unrelated refetch,
// making every write until then fail CSRF validation.
func (s *Server) toMeDTO(r *http.Request, u *store.User) meDTO {
	caps := []string{"links:read", "links:write", "stats:read"}
	if u.IsAdmin() {
		caps = append(caps, "admin")
	}
	csrf := ""
	if sess := sessionFromContext(r.Context()); sess != nil {
		csrf = sess.CSRFToken
	}
	return meDTO{userDTO: toUserDTO(u), CSRFToken: csrf, Capabilities: caps}
}

type patchMeReq struct {
	Name     *string `json:"name"`
	Email    *string `json:"email"`
	Password *string `json:"password"` // required to confirm an email change
}

func (s *Server) handlePatchMe(w http.ResponseWriter, r *http.Request, u *store.User) {
	var req patchMeReq
	if err := decodeJSON(w, r, 4096, &req); err != nil {
		respondError(w, r, err)
		return
	}
	verrs := validate.Errors{}
	if req.Name != nil {
		if !validate.StrLen(*req.Name, 1, 100) {
			verrs.Add("name", "must be 1-100 characters")
		} else {
			u.Name = strings.TrimSpace(*req.Name)
		}
	}
	if req.Email != nil {
		email, err := validate.Email(*req.Email)
		if err != nil {
			verrs.Add("email", err.Error())
		} else if email != u.Email {
			if u.PasswordHash != nil {
				if req.Password == nil || *req.Password == "" {
					verrs.Add("password", "current password is required to change email")
				} else if ok, _, err := auth.VerifyPassword(*req.Password, *u.PasswordHash); err != nil || !ok {
					verrs.Add("password", "incorrect password")
				}
			}
			if !verrs.HasAny() {
				if _, err := s.store.GetUserByEmail(r.Context(), email); err == nil {
					verrs.Add("email", "this email is already in use")
				} else {
					u.Email = email
					u.EmailVerified = false
				}
			}
		}
	}
	if verrs.HasAny() {
		respondError(w, r, ValidationFailed(verrs))
		return
	}
	if err := s.store.UpdateUser(r.Context(), u); err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toUserDTO(u))
}

type putPasswordReq struct {
	Current string `json:"current"`
	New     string `json:"new"`
}

func (s *Server) handlePutPassword(w http.ResponseWriter, r *http.Request, u *store.User) {
	var req putPasswordReq
	if err := decodeJSON(w, r, 2048, &req); err != nil {
		respondError(w, r, err)
		return
	}
	if !validate.StrLen(req.New, 10, 128) {
		respondError(w, r, ValidationFailed(validate.Errors{{Field: "new", Message: "must be 10-128 characters"}}))
		return
	}
	if u.PasswordHash != nil {
		ok, _, err := auth.VerifyPassword(req.Current, *u.PasswordHash)
		if err != nil || !ok {
			respondError(w, r, ValidationFailed(validate.Errors{{Field: "current", Message: "incorrect password"}}))
			return
		}
	}
	hash, err := auth.HashPassword(req.New)
	if err != nil {
		respondError(w, r, err)
		return
	}
	now := time.Now()
	u.PasswordHash = &hash
	u.PasswordChangedAt = &now
	if err := s.store.UpdateUser(r.Context(), u); err != nil {
		respondError(w, r, err)
		return
	}
	// changing password invalidates every other session
	cur := sessionFromContext(r.Context())
	keepID := ""
	if cur != nil {
		keepID = cur.ID
	}
	_ = s.store.DeleteSessionsForUserExcept(r.Context(), u.ID, keepID)
	s.audit(r, u.ID, "user.password_changed", "user", u.ID, nil)
	if s.notifier != nil {
		s.notifier.NotifyUser(r.Context(), u.ID, u.Email, notify.KindPasswordChanged, "Password changed", "Your password was changed. If this wasn't you, contact an administrator.", nil)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListMySessions(w http.ResponseWriter, r *http.Request, u *store.User) {
	sessions, err := s.store.ListSessionsForUser(r.Context(), u.ID)
	if err != nil {
		respondError(w, r, err)
		return
	}
	cur := sessionFromContext(r.Context())
	out := make([]sessionDTO, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, sessionDTO{ID: sess.ID, IP: sess.IP, UserAgent: sess.UserAgent, CreatedAt: sess.CreatedAt, ExpiresAt: sess.ExpiresAt, LastSeenAt: sess.LastSeenAt, Current: cur != nil && cur.ID == sess.ID})
	}
	respondList(w, out, "")
}

func (s *Server) handleDeleteMySession(w http.ResponseWriter, r *http.Request, u *store.User, id string) {
	sess, err := s.store.GetSession(r.Context(), id)
	if err != nil || sess.UserID != u.ID {
		respondError(w, r, ErrNotFound)
		return
	}
	if err := s.store.DeleteSession(r.Context(), id); err != nil {
		respondError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListMyIdentities(w http.ResponseWriter, r *http.Request, u *store.User) {
	ids, err := s.store.ListOIDCIdentitiesForUser(r.Context(), u.ID)
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]identityDTO, 0, len(ids))
	for _, i := range ids {
		out = append(out, toIdentityDTO(i))
	}
	respondList(w, out, "")
}

func (s *Server) handleDeleteMyIdentity(w http.ResponseWriter, r *http.Request, u *store.User, id string) {
	if !s.hasSudo(r) {
		respondError(w, r, ErrSudoRequired)
		return
	}
	identity, err := s.store.GetOIDCIdentityByID(r.Context(), id)
	if err != nil || identity.UserID != u.ID {
		respondError(w, r, ErrNotFound)
		return
	}
	n, err := s.store.CountOIDCIdentitiesForUser(r.Context(), u.ID)
	if err != nil {
		respondError(w, r, err)
		return
	}
	if n <= 1 && u.PasswordHash == nil {
		respondError(w, r, ErrCannotUnlink)
		return
	}
	if err := s.store.DeleteOIDCIdentity(r.Context(), id); err != nil {
		respondError(w, r, err)
		return
	}
	s.audit(r, u.ID, "oidc.unlink", "user", u.ID, map[string]any{"issuer": identity.Issuer})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteMe(w http.ResponseWriter, r *http.Request, u *store.User) {
	if !s.hasSudo(r) {
		respondError(w, r, ErrSudoRequired)
		return
	}
	if u.IsAdmin() {
		n, err := s.store.CountAdmins(r.Context(), u.ID)
		if err != nil {
			respondError(w, r, err)
			return
		}
		if n == 0 {
			respondError(w, r, ErrLastAdmin)
			return
		}
	}
	linksMode := r.URL.Query().Get("links")
	if linksMode == "delete" {
		links, _, err := s.store.ListLinks(r.Context(), store.LinkFilter{UserID: u.ID, Limit: 100})
		if err == nil {
			for _, l := range links {
				_ = s.links.Delete(r.Context(), l.ID, l.Code)
			}
		}
	}
	if err := s.store.DeleteUser(r.Context(), u.ID); err != nil {
		respondError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}
