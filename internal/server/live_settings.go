package server

import (
	"context"
	"encoding/json"
	"strings"

	"shortr/internal/link"
	"shortr/internal/validate"
)

// liveSettings are the admin-editable policies (Admin → Settings). The env
// value is the default; a saved setting overrides it. They are held in an
// atomic snapshot so request handlers never race with a save.
type liveSettings struct {
	Registration          string
	OIDCAutoCreate        bool
	OIDCAutoLinkByEmail   bool
	FetchTitles           bool
	CountBots             bool
	DefaultRedirectStatus int
	MaxLinksPerUser       int
	BlockedDomains        []string
	MFARequired           bool
}

func (s *Server) live() *liveSettings {
	if l := s.liveSet.Load(); l != nil {
		return l
	}
	return s.envSettings()
}

func (s *Server) envSettings() *liveSettings {
	return &liveSettings{
		Registration: s.cfg.Registration, OIDCAutoCreate: s.cfg.OIDCAutoCreate, OIDCAutoLinkByEmail: s.cfg.OIDCAutoLinkByEmail,
		FetchTitles: s.cfg.FetchTitles, CountBots: s.cfg.CountBots, DefaultRedirectStatus: s.cfg.DefaultRedirectCode,
		MaxLinksPerUser: s.cfg.MaxLinksPerUser, BlockedDomains: s.cfg.BlockedDomains, MFARequired: s.cfg.MFARequired,
	}
}

// parseLiveSettings overlays stored JSON values (keyed by store key) onto the
// env defaults, reporting invalid ones by wire key.
func (s *Server) parseLiveSettings(stored map[string]string) (*liveSettings, validate.Errors) {
	ls := s.envSettings()
	errs := validate.Errors{}
	get := func(storeKey, wireKey string, dst any) bool {
		v, ok := stored[storeKey]
		if !ok || v == "" || v == "null" {
			return false
		}
		if err := json.Unmarshal([]byte(v), dst); err != nil {
			errs.Add(wireKey, "has the wrong type")
			return false
		}
		return true
	}
	var reg string
	if get("registration", "registration", &reg) {
		if reg == "closed" || reg == "open" || reg == "invite" {
			ls.Registration = reg
		} else {
			errs.Add("registration", "must be closed, open or invite")
		}
	}
	get("oidc_auto_create", "oidcAutoCreate", &ls.OIDCAutoCreate)
	get("oidc_auto_link_by_email", "oidcAutoLinkByEmail", &ls.OIDCAutoLinkByEmail)
	get("fetch_titles", "fetchTitles", &ls.FetchTitles)
	get("count_bots", "countBots", &ls.CountBots)
	get("mfa_required", "mfaRequired", &ls.MFARequired)
	var status int
	if get("default_redirect_status", "defaultRedirectStatus", &status) {
		if status == 301 || status == 302 || status == 307 || status == 308 {
			ls.DefaultRedirectStatus = status
		} else {
			errs.Add("defaultRedirectStatus", "must be 301, 302, 307 or 308")
		}
	}
	var maxLinks int
	if get("max_links_per_user", "maxLinksPerUser", &maxLinks) {
		if maxLinks >= 0 {
			ls.MaxLinksPerUser = maxLinks
		} else {
			errs.Add("maxLinksPerUser", "must be 0 (unlimited) or more")
		}
	}
	var blocked []string
	if get("blocked_domains", "blockedDomains", &blocked) {
		ls.BlockedDomains = ls.BlockedDomains[:0:0]
		for _, d := range blocked {
			if d = strings.ToLower(strings.TrimSpace(d)); d != "" {
				ls.BlockedDomains = append(ls.BlockedDomains, d)
			}
		}
		// env-level blocks always apply; the UI can add to them, not lift them
		ls.BlockedDomains = append(ls.BlockedDomains, s.cfg.BlockedDomains...)
	}
	return ls, errs
}

// reloadSettings applies saved settings to the running server. Invalid
// stored values (e.g. from an older version) fall back to env defaults.
func (s *Server) reloadSettings(ctx context.Context) {
	all, err := s.store.AllSettings(ctx)
	if err != nil {
		s.log.Warn("could not load settings; using environment defaults", "error", err)
		return
	}
	ls, errs := s.parseLiveSettings(all)
	if errs.HasAny() {
		s.log.Warn("ignoring invalid saved settings", "errors", errs.Error())
	}
	s.applySettings(ls)
}

func (s *Server) applySettings(ls *liveSettings) {
	s.liveSet.Store(ls)
	if s.links != nil {
		s.links.SetPolicy(link.Policy{
			BlockedDomains: ls.BlockedDomains, DefaultRedirectStatus: ls.DefaultRedirectStatus, MaxLinksPerUser: ls.MaxLinksPerUser,
		})
	}
	if s.clickWriter != nil {
		s.clickWriter.SetCountBots(ls.CountBots)
	}
}
