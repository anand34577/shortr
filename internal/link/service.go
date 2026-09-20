// Package link implements link creation, resolution, and the redirect-path
// cache. It is the only package that decides whether a target URL / alias is
// acceptable (validation lives in internal/validate; policy lives here).
package link

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"shortr/internal/auth"
	"shortr/internal/store"
	"shortr/internal/validate"
)

type Config struct {
	BaseURL               string
	BaseHost              string
	CodeLength            int
	Alphabet              string
	MaxURLLength          int
	AllowPrivateTargets   bool
	BlockedDomains        []string
	DefaultRedirectStatus int
	MaxLinksPerUser       int
	CacheCapacity         int
}

// CacheMetrics is the subset of internal/metrics.Registry the redirect
// cache reports to; defined here (not imported) to avoid a dependency
// cycle, same pattern as internal/click.Metrics.
type CacheMetrics interface {
	IncCacheHit()
	IncCacheMiss()
}

type noopCacheMetrics struct{}

func (noopCacheMetrics) IncCacheHit()  {}
func (noopCacheMetrics) IncCacheMiss() {}

type Service struct {
	store   *store.Store
	cache   *Cache
	cfg     Config
	metrics CacheMetrics
	hits    sync.Map // link ID -> *hitCounter: live click counter for max_clicks links
}

// ConsumeClick reserves one click against a link's max_clicks limit and
// reports whether the redirect may proceed. The counter is seeded from the
// stored click_count and lives in memory, so the limit holds even before the
// batch writer has flushed the click to the database.
func (s *Service) ConsumeClick(l *store.Link) bool {
	if l.MaxClicks == nil {
		return true
	}
	c := &hitCounter{}
	c.n.Store(l.ClickCount)
	v, _ := s.hits.LoadOrStore(l.ID, c)
	ctr := v.(*hitCounter)
	ctr.last.Store(time.Now().UnixNano())
	if ctr.n.Add(1) > int64(*l.MaxClicks) {
		ctr.n.Add(-1)
		return false
	}
	return true
}

type hitCounter struct {
	n    atomic.Int64
	last atomic.Int64 // unix nanos of the last ConsumeClick
}

// PruneHits drops click counters idle for longer than idle. By then the
// link's cache entry has expired too, so the next ConsumeClick re-seeds from
// the stored click_count instead of a stale value. Bounds memory on
// long-lived instances with many max_clicks links.
func (s *Service) PruneHits(idle time.Duration) {
	cutoff := time.Now().Add(-idle).UnixNano()
	s.hits.Range(func(k, v any) bool {
		if v.(*hitCounter).last.Load() < cutoff {
			s.hits.Delete(k)
		}
		return true
	})
}

// SetMetrics wires a metrics sink after construction (optional — a
// noop is used until this is called, so tests don't need to care).
func (s *Service) SetMetrics(m CacheMetrics) {
	if m != nil {
		s.metrics = m
	}
}

func NewService(st *store.Store, cfg Config) *Service {
	if cfg.CodeLength == 0 {
		cfg.CodeLength = 7
	}
	if cfg.Alphabet == "" || cfg.Alphabet == "base58" {
		cfg.Alphabet = AlphabetBase58
	} else {
		cfg.Alphabet = AlphabetBase62
	}
	if cfg.DefaultRedirectStatus == 0 {
		cfg.DefaultRedirectStatus = 302
	}
	if cfg.MaxURLLength == 0 {
		cfg.MaxURLLength = 2048
	}
	return &Service{store: st, cache: NewCache(cfg.CacheCapacity), cfg: cfg, metrics: noopCacheMetrics{}}
}

func (s *Service) Cache() *Cache { return s.cache }

var (
	ErrCodeTaken     = errors.New("link: code already taken")
	ErrCodeExhausted = errors.New("link: could not generate a unique code")
	ErrLimitReached  = errors.New("link: per-user link limit reached")
	ErrNotFound      = store.ErrNotFound
)

type CreateInput struct {
	TargetURL                                              string
	Code                                                   string // optional custom alias
	Length                                                 int    // optional length override for generated codes
	Title                                                  string
	Description                                            string
	RedirectStatus                                         int
	Password                                               string
	ExpiresAt                                              *time.Time
	MaxClicks                                              *int
	Tags                                                   []string
	PassQuery                                              *bool
	UTMSource, UTMMedium, UTMCampaign, UTMTerm, UTMContent string
}

func (s *Service) Create(ctx context.Context, actorUserID *string, actorIP string, in CreateInput) (*store.Link, error) {
	verrs := validate.Errors{}

	target, err := validate.TargetURL(in.TargetURL, s.cfg.MaxURLLength, s.cfg.AllowPrivateTargets, s.cfg.BlockedDomains, s.cfg.BaseHost)
	if err != nil {
		verrs.Add("target_url", err.Error())
	}

	redirectStatus := in.RedirectStatus
	if redirectStatus == 0 {
		redirectStatus = s.cfg.DefaultRedirectStatus
	}
	if redirectStatus != 301 && redirectStatus != 302 && redirectStatus != 307 && redirectStatus != 308 {
		verrs.Add("redirect_status", "must be one of 301, 302, 307, 308")
	}

	if in.Length != 0 && (in.Length < 4 || in.Length > 16) {
		verrs.Add("length", "must be between 4 and 16")
	}
	for _, u := range []string{in.UTMSource, in.UTMMedium, in.UTMCampaign, in.UTMTerm, in.UTMContent} {
		if len(u) > 255 || strings.ContainsAny(u, "\r\n") {
			verrs.Add("utm", "each field must be at most 255 characters with no line breaks")
			break
		}
	}
	if len(in.Title) > 200 {
		verrs.Add("title", "must be at most 200 characters")
	}
	if len(in.Description) > 1000 {
		verrs.Add("description", "must be at most 1000 characters")
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(time.Now().Add(time.Minute)) {
		verrs.Add("expires_at", "must be at least 1 minute in the future")
	}
	if in.MaxClicks != nil && *in.MaxClicks < 1 {
		verrs.Add("max_clicks", "must be at least 1")
	}
	if len(in.Tags) > 10 {
		verrs.Add("tags", "at most 10 tags allowed")
	}
	for _, t := range in.Tags {
		if !validate.StrLen(t, 1, 32) {
			verrs.Add("tags", "each tag must be 1-32 characters")
			break
		}
	}
	if in.Password != "" && !validate.StrLen(in.Password, 1, 128) {
		verrs.Add("password", "must be at most 128 characters")
	}

	code := strings.TrimSpace(in.Code)
	if code != "" {
		if err := validate.Alias(code, IsReserved, ValidAliasChars); err != nil {
			verrs.Add("code", err.Error())
		} else {
			exists, err := s.store.CodeExists(ctx, code)
			if err != nil {
				return nil, fmt.Errorf("checking alias availability: %w", err)
			}
			if exists {
				verrs.Add("code", "this alias is already taken")
			}
		}
	}

	if verrs.HasAny() {
		return nil, &ValidationError{Errors: verrs}
	}

	if actorUserID != nil && s.cfg.MaxLinksPerUser > 0 {
		n, err := s.store.CountLinksForUser(ctx, *actorUserID)
		if err != nil {
			return nil, err
		}
		if n >= s.cfg.MaxLinksPerUser {
			return nil, ErrLimitReached
		}
	}

	l := &store.Link{
		TargetURL:      target,
		Title:          strings.TrimSpace(in.Title),
		Description:    in.Description,
		UserID:         actorUserID,
		RedirectStatus: redirectStatus,
		ExpiresAt:      in.ExpiresAt,
		MaxClicks:      in.MaxClicks,
		Tags:           normalizeTags(in.Tags),
		PassQuery:      true,
		UTMSource:      in.UTMSource, UTMMedium: in.UTMMedium, UTMCampaign: in.UTMCampaign, UTMTerm: in.UTMTerm, UTMContent: in.UTMContent,
		CreatedByIP: actorIP,
	}
	if in.PassQuery != nil {
		l.PassQuery = *in.PassQuery
	}
	if in.Password != "" {
		h, err := auth.HashPassword(in.Password)
		if err != nil {
			return nil, err
		}
		l.PasswordHash = &h
	}

	length := in.Length
	if length == 0 {
		length = s.cfg.CodeLength
	}

	if code != "" {
		l.Code = code
		if err := s.store.CreateLink(ctx, l); err != nil {
			if errors.Is(err, store.ErrConflict) {
				return nil, ErrCodeTaken
			}
			return nil, err
		}
		s.cache.Invalidate(l.Code) // drop any negative entry from an earlier miss
		return l, nil
	}

	for attempt := 0; attempt < 5; attempt++ {
		genLen := length
		if attempt == 4 {
			genLen++
		}
		gc, err := GenerateCode(s.cfg.Alphabet, genLen)
		if err != nil {
			return nil, err
		}
		if IsReserved(gc) {
			continue
		}
		l.Code = gc
		err = s.store.CreateLink(ctx, l)
		if err == nil {
			s.cache.Invalidate(l.Code)
			return l, nil
		}
		if !errors.Is(err, store.ErrConflict) {
			return nil, err
		}
		// collision: retry with a fresh random code
	}
	return nil, ErrCodeExhausted
}

func normalizeTags(tags []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	return out
}

// ValidationError carries field-level errors up to the API layer.
type ValidationError struct{ Errors validate.Errors }

func (e *ValidationError) Error() string { return e.Errors.Error() }

// ResolveForRedirect is the hot path: cache first, DB on miss, negative
// cache for unknown codes. Never returns a soft-deleted/expired link as if
// it were live — callers must still check Link.Status/DeletedAt/ExpiresAt.
func (s *Service) ResolveForRedirect(ctx context.Context, code string) (*store.Link, error) {
	if e, ok := s.cache.Get(code); ok {
		s.metrics.IncCacheHit()
		if e.Missing {
			return nil, store.ErrNotFound
		}
		return e.Link, nil
	}
	s.metrics.IncCacheMiss()
	l, err := s.store.GetLinkByCode(ctx, code)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.cache.PutMissing(code)
		}
		return nil, err
	}
	s.cache.Put(l.Code, l)
	return l, nil
}

func (s *Service) Get(ctx context.Context, id string) (*store.Link, error) {
	return s.store.GetLinkByID(ctx, id)
}

func (s *Service) GetByCode(ctx context.Context, code string) (*store.Link, error) {
	return s.store.GetLinkByCode(ctx, code)
}

type UpdateInput struct {
	Code                                                   *string
	TargetURL                                              *string
	Title                                                  *string
	Description                                            *string
	RedirectStatus                                         *int
	Password                                               *string // empty string clears
	ExpiresAt                                              **time.Time
	MaxClicks                                              **int
	Status                                                 *string
	Tags                                                   *[]string
	PassQuery                                              *bool
	UTMSource, UTMMedium, UTMCampaign, UTMTerm, UTMContent *string
}

func (s *Service) Update(ctx context.Context, l *store.Link, in UpdateInput) error {
	verrs := validate.Errors{}
	oldCode := l.Code

	if in.Code != nil && *in.Code != l.Code {
		if err := validate.Alias(*in.Code, IsReserved, ValidAliasChars); err != nil {
			verrs.Add("code", err.Error())
		} else {
			exists, err := s.store.CodeExists(ctx, *in.Code)
			if err != nil {
				return err
			}
			if exists {
				verrs.Add("code", "this alias is already taken")
			} else {
				l.Code = *in.Code
			}
		}
	}
	if in.TargetURL != nil {
		target, err := validate.TargetURL(*in.TargetURL, 8192, false, nil, "")
		if err != nil {
			verrs.Add("target_url", err.Error())
		} else {
			l.TargetURL = target
		}
	}
	if in.Title != nil {
		if len(*in.Title) > 200 {
			verrs.Add("title", "must be at most 200 characters")
		} else {
			l.Title = *in.Title
		}
	}
	if in.Description != nil {
		if len(*in.Description) > 1000 {
			verrs.Add("description", "must be at most 1000 characters")
		} else {
			l.Description = *in.Description
		}
	}
	if in.RedirectStatus != nil {
		if *in.RedirectStatus != 301 && *in.RedirectStatus != 302 && *in.RedirectStatus != 307 && *in.RedirectStatus != 308 {
			verrs.Add("redirect_status", "must be one of 301, 302, 307, 308")
		} else {
			l.RedirectStatus = *in.RedirectStatus
		}
	}
	if in.Password != nil {
		if *in.Password == "" {
			l.PasswordHash = nil
		} else {
			h, err := auth.HashPassword(*in.Password)
			if err != nil {
				return err
			}
			l.PasswordHash = &h
		}
	}
	if in.ExpiresAt != nil {
		if *in.ExpiresAt != nil && !(*in.ExpiresAt).After(time.Now()) {
			verrs.Add("expires_at", "must be in the future")
		} else {
			l.ExpiresAt = *in.ExpiresAt
		}
	}
	if in.MaxClicks != nil {
		l.MaxClicks = *in.MaxClicks
	}
	if in.Status != nil {
		if *in.Status != "active" && *in.Status != "disabled" {
			verrs.Add("status", "must be active or disabled")
		} else {
			l.Status = *in.Status
		}
	}
	if in.Tags != nil {
		if len(*in.Tags) > 10 {
			verrs.Add("tags", "at most 10 tags allowed")
		} else {
			l.Tags = normalizeTags(*in.Tags)
		}
	}
	if in.PassQuery != nil {
		l.PassQuery = *in.PassQuery
	}
	if in.UTMSource != nil {
		l.UTMSource = *in.UTMSource
	}
	if in.UTMMedium != nil {
		l.UTMMedium = *in.UTMMedium
	}
	if in.UTMCampaign != nil {
		l.UTMCampaign = *in.UTMCampaign
	}
	if in.UTMTerm != nil {
		l.UTMTerm = *in.UTMTerm
	}
	if in.UTMContent != nil {
		l.UTMContent = *in.UTMContent
	}

	if verrs.HasAny() {
		return &ValidationError{Errors: verrs}
	}

	if err := s.store.UpdateLink(ctx, l); err != nil {
		if errors.Is(err, store.ErrConflict) {
			return ErrCodeTaken
		}
		return err
	}
	s.hits.Delete(l.ID)
	s.cache.Invalidate(oldCode)
	if l.Code != oldCode {
		s.cache.Invalidate(l.Code)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id, code string) error {
	if err := s.store.SoftDeleteLink(ctx, id, time.Now()); err != nil {
		return err
	}
	s.hits.Delete(id)
	s.cache.Invalidate(code)
	return nil
}

func (s *Service) Restore(ctx context.Context, id, code string) error {
	if err := s.store.RestoreLink(ctx, id); err != nil {
		return err
	}
	s.cache.Invalidate(code)
	return nil
}

func (s *Service) InvalidateCache(code string) { s.cache.Invalidate(code) }

func (s *Service) ShortURL(code string) string {
	return strings.TrimRight(s.cfg.BaseURL, "/") + "/" + code
}
