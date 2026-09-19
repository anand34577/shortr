package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"shortr/internal/link"
	"shortr/internal/store"
	"shortr/internal/validate"
)

type createLinkReq struct {
	TargetURL      string   `json:"target_url"`
	Code           string   `json:"code"`
	Length         int      `json:"length"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	RedirectStatus int      `json:"redirect_status"`
	Password       string   `json:"password"`
	ExpiresAt      *string  `json:"expires_at"`
	MaxClicks      *int     `json:"max_clicks"`
	Tags           []string `json:"tags"`
	PassQuery      *bool    `json:"pass_query"`
	UTM            *utmDTO  `json:"utm"`
}

func (s *Server) handleCreateLink(w http.ResponseWriter, r *http.Request, u *store.User) {
	idemKey := r.Header.Get("Idempotency-Key")
	if idemKey != "" {
		if cached, ok, _ := s.store.GetIdempotentResponse(r.Context(), idemKey, u.ID); ok {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Idempotency-Replayed", "true")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(cached)) //nolint:errcheck
			return
		}
	}

	var req createLinkReq
	if err := decodeJSON(w, r, 8192, &req); err != nil {
		respondError(w, r, err)
		return
	}

	in := link.CreateInput{
		TargetURL: req.TargetURL, Code: req.Code, Length: req.Length, Title: req.Title, Description: req.Description,
		RedirectStatus: req.RedirectStatus, Password: req.Password, MaxClicks: req.MaxClicks, Tags: req.Tags, PassQuery: req.PassQuery,
	}
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			respondError(w, r, ValidationFailed(validate.Errors{{Field: "expires_at", Message: "must be RFC 3339"}}))
			return
		}
		in.ExpiresAt = &t
	}
	if req.UTM != nil {
		in.UTMSource, in.UTMMedium, in.UTMCampaign, in.UTMTerm, in.UTMContent = req.UTM.Source, req.UTM.Medium, req.UTM.Campaign, req.UTM.Term, req.UTM.Content
	}

	ip := clientIPFromContext(r.Context())
	uid := u.ID
	l, err := s.links.Create(r.Context(), &uid, ipStrOrEmpty(ip), in)
	if err != nil {
		s.respondLinkErr(w, r, err)
		return
	}
	s.audit(r, u.ID, "link.create", "link", l.ID, map[string]any{"code": l.Code})

	dto := s.toLinkDTO(l)
	respondJSON(w, http.StatusCreated, dto)
	if idemKey != "" {
		if b, err := json.Marshal(dto); err == nil {
			_ = s.store.SaveIdempotentResponse(r.Context(), idemKey, u.ID, string(b))
		}
	}
}

func (s *Server) respondLinkErr(w http.ResponseWriter, r *http.Request, err error) {
	var ve *link.ValidationError
	switch {
	case errors.As(err, &ve):
		respondError(w, r, ValidationFailed(ve.Errors))
	case errors.Is(err, link.ErrCodeTaken):
		respondError(w, r, ErrCodeTaken)
	case errors.Is(err, link.ErrCodeExhausted):
		respondError(w, r, ErrCodeExhausted)
	case errors.Is(err, link.ErrLimitReached):
		respondError(w, r, ErrLinkLimitReached)
	default:
		respondError(w, r, err)
	}
}

func (s *Server) handleListLinks(w http.ResponseWriter, r *http.Request, u *store.User) {
	q := r.URL.Query()
	f := store.LinkFilter{
		Query: q.Get("q"), Tag: q.Get("tag"), Status: q.Get("status"), Sort: q.Get("sort"), Order: q.Get("order"),
		Cursor: q.Get("cursor"),
	}
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			f.Limit = n
		}
	}
	if !u.IsAdmin() || q.Get("scope") != "all" {
		f.UserID = u.ID
	}
	links, next, err := s.store.ListLinks(r.Context(), f)
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]linkDTO, 0, len(links))
	for _, l := range links {
		out = append(out, s.toLinkDTO(l))
	}
	respondList(w, out, next)
}

func (s *Server) getOwnedLink(w http.ResponseWriter, r *http.Request, u *store.User, id string) *store.Link {
	l, err := s.links.Get(r.Context(), id)
	if err != nil {
		respondError(w, r, ErrNotFound)
		return nil
	}
	if !u.IsAdmin() && (l.UserID == nil || *l.UserID != u.ID) {
		respondError(w, r, ErrForbidden)
		return nil
	}
	return l
}

func (s *Server) handleGetLink(w http.ResponseWriter, r *http.Request, u *store.User, id string) {
	l := s.getOwnedLink(w, r, u, id)
	if l == nil {
		return
	}
	respondJSON(w, http.StatusOK, s.toLinkDTO(l))
}

type patchLinkReq struct {
	Code           *string  `json:"code"`
	TargetURL      *string  `json:"target_url"`
	Title          *string  `json:"title"`
	Description    *string  `json:"description"`
	RedirectStatus *int     `json:"redirect_status"`
	Password       *string  `json:"password"`
	ExpiresAt      **string `json:"expires_at"`
	MaxClicks      **int    `json:"max_clicks"`
	Status         *string  `json:"status"`
	Tags           *[]string `json:"tags"`
	PassQuery      *bool    `json:"pass_query"`
	UTM            *utmDTO  `json:"utm"`
}

func (s *Server) handlePatchLink(w http.ResponseWriter, r *http.Request, u *store.User, id string) {
	l := s.getOwnedLink(w, r, u, id)
	if l == nil {
		return
	}
	if inm := r.Header.Get("If-Match"); inm != "" {
		if inm != weakETag(l.UpdatedAt) {
			respondError(w, r, ErrPreconditionFail)
			return
		}
	}

	var req patchLinkReq
	if err := decodeJSON(w, r, 8192, &req); err != nil {
		respondError(w, r, err)
		return
	}
	in := link.UpdateInput{
		Code: req.Code, TargetURL: req.TargetURL, Title: req.Title, Description: req.Description,
		RedirectStatus: req.RedirectStatus, Password: req.Password, Status: req.Status, Tags: req.Tags, PassQuery: req.PassQuery,
	}
	if req.ExpiresAt != nil {
		var t *time.Time
		if *req.ExpiresAt != nil && **req.ExpiresAt != "" {
			parsed, err := time.Parse(time.RFC3339, **req.ExpiresAt)
			if err != nil {
				respondError(w, r, ValidationFailed(validate.Errors{{Field: "expires_at", Message: "must be RFC 3339"}}))
				return
			}
			t = &parsed
		}
		in.ExpiresAt = &t
	}
	if req.MaxClicks != nil {
		in.MaxClicks = req.MaxClicks
	}
	if req.UTM != nil {
		in.UTMSource, in.UTMMedium, in.UTMCampaign, in.UTMTerm, in.UTMContent = &req.UTM.Source, &req.UTM.Medium, &req.UTM.Campaign, &req.UTM.Term, &req.UTM.Content
	}
	if !u.IsAdmin() && req.Status != nil {
		// non-admin owners may still disable/enable their own links
	}

	if err := s.links.Update(r.Context(), l, in); err != nil {
		s.respondLinkErr(w, r, err)
		return
	}
	s.audit(r, u.ID, "link.update", "link", l.ID, nil)
	respondJSON(w, http.StatusOK, s.toLinkDTO(l))
}

func weakETag(t time.Time) string {
	return fmt.Sprintf(`W/"%d"`, t.UnixNano())
}

func (s *Server) handleDeleteLink(w http.ResponseWriter, r *http.Request, u *store.User, id string) {
	l := s.getOwnedLink(w, r, u, id)
	if l == nil {
		return
	}
	if err := s.links.Delete(r.Context(), l.ID, l.Code); err != nil {
		respondError(w, r, err)
		return
	}
	s.audit(r, u.ID, "link.delete", "link", l.ID, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRestoreLink(w http.ResponseWriter, r *http.Request, u *store.User, id string) {
	l := s.getOwnedLink(w, r, u, id)
	if l == nil {
		return
	}
	if l.DeletedAt == nil {
		respondError(w, r, NewAPIError(http.StatusBadRequest, "BAD_REQUEST", "link is not deleted"))
		return
	}
	if time.Since(*l.DeletedAt) > 30*24*time.Hour {
		respondError(w, r, NewAPIError(http.StatusBadRequest, "BAD_REQUEST", "restore window (30 days) has passed"))
		return
	}
	if err := s.links.Restore(r.Context(), l.ID, l.Code); err != nil {
		respondError(w, r, err)
		return
	}
	s.audit(r, u.ID, "link.restore", "link", l.ID, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCheckCode(w http.ResponseWriter, r *http.Request, u *store.User) {
	code := r.URL.Query().Get("code")
	if code == "" {
		respondError(w, r, ErrBadRequest)
		return
	}
	if !link.ValidAliasChars(code) || link.IsReserved(code) {
		respondJSON(w, http.StatusOK, map[string]any{"available": false, "reason": "invalid or reserved"})
		return
	}
	exists, err := s.store.CodeExists(r.Context(), code)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"available": !exists})
}

func (s *Server) handleLinkQR(w http.ResponseWriter, r *http.Request, u *store.User, id string) {
	l := s.getOwnedLink(w, r, u, id)
	if l == nil {
		return
	}
	size := 256
	if v := r.URL.Query().Get("size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 64 && n <= 2048 {
			size = n
		}
	}
	png, err := qrcode.Encode(s.links.ShortURL(l.Code), qrcode.Medium, size)
	if err != nil {
		respondError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Write(png) //nolint:errcheck
}

func (s *Server) handleLinkPreview(w http.ResponseWriter, r *http.Request, u *store.User) {
	if !s.cfg.FetchTitles {
		respondJSON(w, http.StatusOK, map[string]any{"title": "", "final_url": ""})
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := decodeJSON(w, r, 2048, &req); err != nil {
		respondError(w, r, err)
		return
	}
	title, finalURL := fetchTitle(r.Context(), req.URL, s.cfg.AllowPrivateTargets)
	respondJSON(w, http.StatusOK, map[string]any{"title": title, "final_url": finalURL})
}

// bulk create/update

type bulkItem struct {
	Op    string          `json:"op"` // create | delete
	ID    string          `json:"id,omitempty"`
	Input createLinkReq   `json:"input,omitempty"`
}
type bulkResult struct {
	OK    bool   `json:"ok"`
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

func (s *Server) handleBulkLinks(w http.ResponseWriter, r *http.Request, u *store.User) {
	var req struct {
		Items []bulkItem `json:"items"`
	}
	if err := decodeJSON(w, r, 1<<20, &req); err != nil {
		respondError(w, r, err)
		return
	}
	if len(req.Items) > 100 {
		respondError(w, r, NewAPIError(http.StatusBadRequest, "BAD_REQUEST", "at most 100 items per bulk request"))
		return
	}
	results := make([]bulkResult, 0, len(req.Items))
	uid := u.ID
	ip := clientIPFromContext(r.Context())
	for _, item := range req.Items {
		switch item.Op {
		case "create":
			in := link.CreateInput{TargetURL: item.Input.TargetURL, Code: item.Input.Code, Title: item.Input.Title}
			l, err := s.links.Create(r.Context(), &uid, ipStrOrEmpty(ip), in)
			if err != nil {
				results = append(results, bulkResult{OK: false, Error: err.Error()})
				continue
			}
			results = append(results, bulkResult{OK: true, ID: l.ID})
		case "delete":
			l, err := s.links.Get(r.Context(), item.ID)
			if err != nil || (!u.IsAdmin() && (l.UserID == nil || *l.UserID != uid)) {
				results = append(results, bulkResult{OK: false, ID: item.ID, Error: "not found or forbidden"})
				continue
			}
			if err := s.links.Delete(r.Context(), l.ID, l.Code); err != nil {
				results = append(results, bulkResult{OK: false, ID: item.ID, Error: err.Error()})
				continue
			}
			results = append(results, bulkResult{OK: true, ID: item.ID})
		default:
			results = append(results, bulkResult{OK: false, Error: "unknown op " + item.Op})
		}
	}
	respondJSON(w, http.StatusOK, map[string]any{"results": results})
}
