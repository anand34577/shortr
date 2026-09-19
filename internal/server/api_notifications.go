package server

import (
	"net/http"
	"time"

	"shortr/internal/notify"
	"shortr/internal/store"
)

func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request, u *store.User) {
	list, err := s.store.ListNotifications(r.Context(), u.ID, u.IsAdmin(), 30)
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]notificationDTO, 0, len(list))
	for _, n := range list {
		out = append(out, toNotificationDTO(n))
	}
	unread, _ := s.store.UnreadNotificationCount(r.Context(), u.ID, u.IsAdmin())
	respondJSON(w, http.StatusOK, map[string]any{"items": out, "unread_count": unread})
}

func (s *Server) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request, u *store.User, id string) {
	if err := s.store.MarkNotificationRead(r.Context(), id, time.Now()); err != nil {
		respondError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type notifyPrefsResp struct {
	Email  map[string]bool `json:"email"`
	Gotify map[string]bool `json:"gotify"`
	InApp  map[string]bool `json:"in_app"`
	SMTPGloballyEnabled   bool `json:"smtp_globally_enabled"`
	GotifyGloballyEnabled bool `json:"gotify_globally_enabled"`
	Kinds []string `json:"kinds"`
}

var notifyKinds = []string{
	string(notify.KindUserRegistered), string(notify.KindLinkExpiring), string(notify.KindLoginLockout),
	string(notify.KindNewLogin), string(notify.KindOIDCLinked), string(notify.KindPasswordChanged),
	string(notify.KindBackupCompleted), string(notify.KindBackupFailed), string(notify.KindAdminBroadcast),
}

func (s *Server) handleGetNotifyPrefs(w http.ResponseWriter, r *http.Request, u *store.User) {
	p := notify.LoadPrefs(r.Context(), s.store, u.ID)
	respondJSON(w, http.StatusOK, notifyPrefsResp{
		Email: p.Email, Gotify: p.Gotify, InApp: p.InApp,
		SMTPGloballyEnabled: s.cfg.SMTPEnabled, GotifyGloballyEnabled: s.cfg.GotifyEnabled, Kinds: notifyKinds,
	})
}

func (s *Server) handlePutNotifyPrefs(w http.ResponseWriter, r *http.Request, u *store.User) {
	var req notify.Prefs
	if err := decodeJSON(w, r, 8192, &req); err != nil {
		respondError(w, r, err)
		return
	}
	if err := notify.SavePrefs(r.Context(), s.store, u.ID, req); err != nil {
		respondError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
