package server

import (
	"context"
	"net/http"
	"time"
)

// handleHealthz is liveness only: 200 as long as the process can respond,
// even mid-DB-outage — so a proxy routing on this keeps sending traffic and
// cached redirects keep working (PLAN.md §20 "health semantics").
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// handleReadyz is readiness: DB reachable and migrations current. A proxy
// or orchestrator can use this to pull an unhealthy instance from rotation
// for *mutations*, while healthz keeps it in rotation for redirects.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	reasons := []string{}
	if err := s.store.Ping(ctx); err != nil {
		reasons = append(reasons, "database unreachable")
	}
	if len(reasons) > 0 {
		respondJSON(w, http.StatusServiceUnavailable, map[string]any{"ready": false, "reasons": reasons})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ready": true})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if s.cfg.MetricsToken != "" {
		if r.URL.Query().Get("token") != s.cfg.MetricsToken && r.Header.Get("Authorization") != "Bearer "+s.cfg.MetricsToken {
			respondError(w, r, ErrForbidden)
			return
		}
	} else {
		ip := clientIPFromContext(r.Context())
		if ip == nil || !ipInAny(ip, s.cfg.TrustedProxies) {
			if ip == nil || (ip.String() != "127.0.0.1" && ip.String() != "::1") {
				respondError(w, r, ErrForbidden)
				return
			}
		}
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(s.metrics.Render()))
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{"version": s.version})
}
