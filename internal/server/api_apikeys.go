package server

import (
	"net/http"
	"time"

	"shortr/internal/auth"
	"shortr/internal/store"
	"shortr/internal/validate"
)

var allScopes = map[string]bool{"links:read": true, "links:write": true, "stats:read": true, "admin:*": true}

type createAPIKeyReq struct {
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	ExpiresAt *string  `json:"expiresAt"`
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request, u *store.User) {
	var req createAPIKeyReq
	if err := decodeJSON(w, r, 2048, &req); err != nil {
		respondError(w, r, err)
		return
	}
	verrs := validate.Errors{}
	if !validate.StrLen(req.Name, 1, 64) {
		verrs.Add("name", "must be 1-64 characters")
	}
	scopes := req.Scopes
	if len(scopes) == 0 {
		scopes = []string{"links:read", "links:write", "stats:read"}
	}
	for _, sc := range scopes {
		if !allScopes[sc] {
			verrs.Add("scopes", "unknown scope %q", sc)
			break
		}
		if sc == "admin:*" && !u.IsAdmin() {
			verrs.Add("scopes", "only admins may create admin-scoped keys")
			break
		}
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			verrs.Add("expiresAt", "must be RFC 3339")
		} else if !t.After(time.Now()) {
			verrs.Add("expiresAt", "must be in the future")
		} else {
			expiresAt = &t
		}
	}
	if verrs.HasAny() {
		respondError(w, r, ValidationFailed(verrs))
		return
	}

	full, prefix, hash, err := auth.NewAPIKey()
	if err != nil {
		respondError(w, r, err)
		return
	}
	key := &store.APIKey{UserID: u.ID, Name: req.Name, Prefix: prefix, KeyHash: hash, Scopes: scopes, ExpiresAt: expiresAt}
	if err := s.store.CreateAPIKey(r.Context(), key); err != nil {
		respondError(w, r, err)
		return
	}
	s.audit(r, u.ID, "apikey.create", "apikey", key.ID, map[string]any{"name": key.Name})

	resp := toAPIKeyDTO(key)
	respondJSON(w, http.StatusCreated, struct {
		apiKeyDTO
		Key string `json:"key"` // shown once
	}{resp, full})
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request, u *store.User) {
	keys, err := s.store.ListAPIKeysForUser(r.Context(), u.ID)
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]apiKeyDTO, 0, len(keys))
	for _, k := range keys {
		if k.RevokedAt != nil {
			continue
		}
		out = append(out, toAPIKeyDTO(k))
	}
	respondList(w, out, "")
}

func (s *Server) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request, u *store.User, id string) {
	key, err := s.store.GetAPIKey(r.Context(), id)
	if err != nil || key.UserID != u.ID {
		respondError(w, r, ErrNotFound)
		return
	}
	if err := s.store.RevokeAPIKey(r.Context(), id, time.Now()); err != nil {
		respondError(w, r, err)
		return
	}
	s.audit(r, u.ID, "apikey.revoke", "apikey", id, nil)
	w.WriteHeader(http.StatusNoContent)
}
