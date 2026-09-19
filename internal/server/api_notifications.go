package server

import (
	"context"
	"encoding/json"
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
	respondJSON(w, http.StatusOK, map[string]any{"items": out, "unreadCount": unread})
}

func (s *Server) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request, u *store.User, id string) {
	if err := s.store.MarkNotificationRead(r.Context(), id, time.Now()); err != nil {
		respondError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMarkAllNotificationsRead(w http.ResponseWriter, r *http.Request, u *store.User) {
	if err := s.store.MarkAllNotificationsRead(r.Context(), u.ID, u.IsAdmin(), time.Now()); err != nil {
		respondError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// The frontend's NotificationPreferences (web/src/lib/types.ts) is a
// per-kind list of channels ("email"|"gotify"|"browser"), a UI-friendly
// shape distinct from notify.Prefs (three per-kind bool maps, one per
// channel) which internal/notify uses for its own backend-triggered
// notification kinds (notify.Kind — "user.registered" etc). The two
// vocabularies of "kind" don't overlap 1:1, so per-kind toggles set here
// only govern the frontend's own client-side kinds; backend-originated
// notifications (see internal/notify.Kind) keep defaulting to "on" for
// every channel (secure by default). "browser" isn't
// tracked server-side at all — that decision is entirely client-side
// (Notification permission + tab visibility), so it's accepted and echoed
// back for round-tripping but not otherwise interpreted here.
type wireNotificationPrefs struct {
	Channels         map[string][]string `json:"channels"`
	EmailAddress     string              `json:"emailAddress,omitempty"`
	GotifyConfigured bool                `json:"gotifyConfigured"`
}

const notifyChannelsSettingPrefix = notify.ChannelsSettingPrefix

func (s *Server) handleGetNotifyPrefs(w http.ResponseWriter, r *http.Request, u *store.User) {
	channels := loadNotifyChannels(r.Context(), s.store, u.ID)
	respondJSON(w, http.StatusOK, wireNotificationPrefs{
		Channels: channels, EmailAddress: u.Email, GotifyConfigured: s.cfg.GotifyEnabled,
	})
}

func (s *Server) handlePutNotifyPrefs(w http.ResponseWriter, r *http.Request, u *store.User) {
	var req wireNotificationPrefs
	if err := decodeJSON(w, r, 8192, &req); err != nil {
		respondError(w, r, err)
		return
	}
	b, err := json.Marshal(req.Channels)
	if err != nil {
		respondError(w, r, err)
		return
	}
	if err := s.store.SetSetting(r.Context(), notifyChannelsSettingPrefix+u.ID, string(b), u.ID); err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, wireNotificationPrefs{Channels: req.Channels, EmailAddress: u.Email, GotifyConfigured: s.cfg.GotifyEnabled})
}

// loadNotifyChannels returns the saved per-kind channels, filling in the
// default (everything on) for every configurable kind the user never touched
// so the settings page shows what the notifier will actually do.
func loadNotifyChannels(ctx context.Context, st *store.Store, userID string) map[string][]string {
	m := map[string][]string{}
	if v, ok, err := st.GetSetting(ctx, notifyChannelsSettingPrefix+userID); err == nil && ok {
		_ = json.Unmarshal([]byte(v), &m)
	}
	for _, k := range notify.AllKinds {
		if _, set := m[string(k)]; !set {
			m[string(k)] = []string{"browser", "email", "gotify"}
		}
	}
	return m
}
