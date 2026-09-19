package server

import (
	"net"
	"net/http"

	"shortr/internal/iploc"
	"shortr/internal/store"
)

// handleIPLookup is the on-demand "IP location checker" tool: an optional,
// admin-configured HTTP service the admin points at their own instance (or
// any compatible one) — never called automatically, only on request here.
func (s *Server) handleIPLookup(w http.ResponseWriter, r *http.Request, u *store.User, ip string) {
	if net.ParseIP(ip) == nil {
		respondError(w, r, NewAPIError(http.StatusBadRequest, "BAD_REQUEST", "invalid IP address"))
		return
	}
	enabled, baseURL := s.ipLocationSettings(r.Context())
	if !enabled || baseURL == "" {
		respondError(w, r, NewAPIError(http.StatusBadRequest, "NOT_CONFIGURED", "IP location checker is not enabled — configure it in Admin > Settings"))
		return
	}
	res, err := iploc.Lookup(r.Context(), baseURL, ip)
	if err != nil {
		respondError(w, r, NewAPIError(http.StatusBadGateway, "UPSTREAM_ERROR", err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, res)
}
