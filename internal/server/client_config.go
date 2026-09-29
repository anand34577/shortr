package server

import "net/http"

// clientConfigDTO is what a mobile app needs before it has any credentials:
// proof it's talking to Shortr, the public short-link origin, and how to
// sign in with SSO. Served on the public listener, so it holds nothing secret.
type clientConfigDTO struct {
	Name       string         `json:"name"`
	SiteName   string         `json:"siteName"`
	BaseURL    string         `json:"baseUrl"`
	APIVersion int            `json:"apiVersion"` // bumped on breaking API changes; build version stays private
	OIDC       *clientOIDCDTO `json:"oidc"`
}

type clientOIDCDTO struct {
	Issuer      string   `json:"issuer"`
	ClientID    string   `json:"clientId"`
	DisplayName string   `json:"displayName"`
	Scopes      []string `json:"scopes"`
}

func (s *Server) handleClientConfig(w http.ResponseWriter, r *http.Request) {
	out := clientConfigDTO{Name: "shortr", SiteName: s.siteNameOrDefault(r.Context()), BaseURL: s.cfg.BaseURL, APIVersion: 1}
	// The app signs in as its own public OIDC client: the first entry of
	// SHORTR_OIDC_API_AUDIENCES (see docs/wiki/Android-App.md).
	if s.oidc != nil && len(s.cfg.OIDCAPIAudiences) > 0 {
		out.OIDC = &clientOIDCDTO{
			Issuer: s.cfg.OIDCIssuer, ClientID: s.cfg.OIDCAPIAudiences[0], DisplayName: s.cfg.OIDCDisplayName,
			Scopes: []string{"openid", "profile", "email", "offline_access"},
		}
	}
	w.Header().Set("Cache-Control", "no-cache")
	respondJSON(w, http.StatusOK, out)
}
