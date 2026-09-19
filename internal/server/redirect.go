package server

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"shortr/internal/auth"
	"shortr/internal/click"
	"shortr/internal/link"
	"shortr/internal/store"
)

// handleRedirect is the hot path: cache lookup, a handful of status checks,
// one header write, a non-blocking click enqueue. No DB write, no template
// render on the success path (PLAN.md §9).
func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimPrefix(r.URL.Path, "/")
	if strings.HasSuffix(code, "/") {
		http.Redirect(w, r, "/"+strings.TrimSuffix(code, "/"), http.StatusMovedPermanently)
		return
	}
	if !link.ValidCodePathSegment(code) {
		s.renderPublic(w, http.StatusNotFound, "not_found", nil)
		s.metrics.IncRedirect("notfound")
		return
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
	case http.MethodPost:
		s.handlePasswordSubmit(w, r, code)
		return
	case http.MethodOptions:
		w.Header().Set("Allow", "GET, HEAD, POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	default:
		w.Header().Set("Allow", "GET, HEAD, POST, OPTIONS")
		respondError(w, r, NewAPIError(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed"))
		return
	}

	l, err := s.links.ResolveForRedirect(r.Context(), code)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.renderPublic(w, http.StatusNotFound, "not_found", nil)
			s.metrics.IncRedirect("notfound")
			return
		}
		respondError(w, r, ErrDBUnavailable)
		return
	}

	switch {
	case l.DeletedAt != nil:
		s.renderPublic(w, http.StatusGone, "gone", nil)
		s.metrics.IncRedirect("gone")
		return
	case l.Status == "disabled":
		s.renderPublic(w, http.StatusNotFound, "disabled", nil)
		s.metrics.IncRedirect("disabled")
		return
	case l.ExpiresAt != nil && l.ExpiresAt.Before(time.Now()):
		s.renderPublic(w, http.StatusGone, "gone", nil)
		s.metrics.IncRedirect("gone")
		return
	case l.MaxClicks != nil && l.ClickCount >= int64(*l.MaxClicks):
		s.renderPublic(w, http.StatusGone, "gone", nil)
		s.metrics.IncRedirect("gone")
		return
	}

	if l.HasPassword() {
		if !s.hasValidPasswordCookie(r, l) {
			s.renderPublic(w, http.StatusOK, "password", map[string]any{"Code": code})
			return
		}
	}

	if r.Method == http.MethodGet && l.MaxClicks != nil && !click.ParseUA(r.UserAgent()).IsBot && !s.links.ConsumeClick(l) {
		s.renderPublic(w, http.StatusGone, "gone", nil)
		s.metrics.IncRedirect("gone")
		return
	}

	target := s.buildTargetURL(l, r)
	w.Header().Set("Location", target)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Robots-Tag", "noindex")
	w.WriteHeader(l.RedirectStatus)

	s.metrics.IncRedirect("ok")
	if r.Method == http.MethodGet {
		s.enqueueClick(r, l, code)
	}
}

func (s *Server) buildTargetURL(l *store.Link, r *http.Request) string {
	target := l.TargetURL
	hasUTM := l.UTMSource != "" || l.UTMMedium != "" || l.UTMCampaign != "" || l.UTMTerm != "" || l.UTMContent != ""
	if !l.PassQuery && !hasUTM {
		return target
	}
	u, err := url.Parse(target)
	if err != nil {
		return target
	}
	q := u.Query()
	if l.PassQuery && r.URL.RawQuery != "" {
		extra, err := url.ParseQuery(r.URL.RawQuery)
		if err == nil {
			total := len(u.RawQuery) + len(r.URL.RawQuery)
			if total <= 2048 {
				for k, vs := range extra {
					for _, v := range vs {
						q.Add(k, v)
					}
				}
			} // else: silently skip merge, redirect to target as-is (PLAN.md §24.1 "query merge > 2KB")
		}
	}
	addIfAbsent := func(key, val string) {
		if val != "" && q.Get(key) == "" {
			q.Set(key, val)
		}
	}
	addIfAbsent("utm_source", l.UTMSource)
	addIfAbsent("utm_medium", l.UTMMedium)
	addIfAbsent("utm_campaign", l.UTMCampaign)
	addIfAbsent("utm_term", l.UTMTerm)
	addIfAbsent("utm_content", l.UTMContent)
	u.RawQuery = q.Encode()
	return u.String()
}

func (s *Server) enqueueClick(r *http.Request, l *store.Link, code string) {
	ip := clientIPFromContext(r.Context())
	ev := click.Event{
		LinkID: l.ID, TS: time.Now(), RawIP: ip,
		UserAgent: r.UserAgent(), Referrer: r.Referer(), AcceptLanguage: r.Header.Get("Accept-Language"),
		QueryString: r.URL.RawQuery,
	}
	if q := r.URL.Query(); len(q) > 0 {
		ev.UTMSource, ev.UTMMedium, ev.UTMCampaign = q.Get("utm_source"), q.Get("utm_medium"), q.Get("utm_campaign")
		ev.UTMTerm, ev.UTMContent = q.Get("utm_term"), q.Get("utm_content")
	}
	if !s.clickWriter.Enqueue(ev) {
		s.log.Warn("click queue full, dropping event", "link_id", l.ID)
	}
}

// --- link-password gate --------------------------------------------------

const linkPasswordCookiePrefix = "lp_"

func (s *Server) hasValidPasswordCookie(r *http.Request, l *store.Link) bool {
	c, err := r.Cookie(linkPasswordCookiePrefix + l.Code)
	if err != nil || c.Value == "" {
		return false
	}
	payload, err := auth.OpenValue(s.cfg.SecretKey, c.Value)
	if err != nil {
		return false
	}
	// payload is "<linkID>|<passwordHash>" — the hash is included so a
	// password change invalidates all outstanding cookies immediately.
	want := l.ID + "|"
	if l.PasswordHash != nil {
		want += *l.PasswordHash
	}
	return payload == want
}

func (s *Server) setPasswordCookie(w http.ResponseWriter, l *store.Link) {
	payload := l.ID + "|"
	if l.PasswordHash != nil {
		payload += *l.PasswordHash
	}
	token := auth.SealValue(s.cfg.SecretKey, payload, time.Hour)
	http.SetCookie(w, &http.Cookie{
		Name: linkPasswordCookiePrefix + l.Code, Value: token, Path: "/",
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: 3600,
	})
}

func (s *Server) handlePasswordSubmit(w http.ResponseWriter, r *http.Request, code string) {
	ip := clientIPFromContext(r.Context())
	rlKey := "pwlink:" + code + ":" + ipStrOrEmpty(ip)
	if !s.rlPassword.Allow(rlKey) {
		s.renderPublic(w, http.StatusTooManyRequests, "password", map[string]any{"Code": code, "Error": "Too many attempts. Please wait a minute and try again."})
		return
	}

	l, err := s.links.ResolveForRedirect(r.Context(), code)
	if err != nil || l.DeletedAt != nil || l.Status == "disabled" {
		s.renderPublic(w, http.StatusNotFound, "not_found", nil)
		return
	}
	if !l.HasPassword() {
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		respondError(w, r, NewAPIError(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed"))
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderPublic(w, http.StatusBadRequest, "password", map[string]any{"Code": code, "Error": "Invalid request."})
		return
	}
	pw := r.FormValue("password")
	ok, _, err := auth.VerifyPassword(pw, *l.PasswordHash)
	if err != nil || !ok {
		s.renderPublic(w, http.StatusOK, "password", map[string]any{"Code": code, "Error": "Incorrect password."})
		return
	}
	s.setPasswordCookie(w, l)
	http.Redirect(w, r, "/"+code, http.StatusSeeOther)
}

func ipStrOrEmpty(ip net.IP) string {
	if ip == nil {
		return ""
	}
	return ip.String()
}
