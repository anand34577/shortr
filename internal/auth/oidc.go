package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	oidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCConfig is the subset of config the manager needs (kept separate from
// internal/config to avoid an import cycle; server wires the values).
type OIDCConfig struct {
	Issuer               string
	ClientID             string
	ClientSecret         string
	Scopes               []string
	RedirectURL          string
	InsecureHTTP         bool
	RequireEmailVerified bool
	AllowedDomains       []string
}

// Manager wraps OIDC discovery + verification. Discovery can fail at
// startup (provider down); Manager retries lazily so the app still boots
// (PLAN.md §24.4 "discovery fails at startup").
type Manager struct {
	cfg OIDCConfig

	mu       sync.RWMutex
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	lastTry  time.Time
	lastErr  error
}

func NewManager(cfg OIDCConfig) *Manager {
	m := &Manager{cfg: cfg}
	ctx := context.Background()
	if cfg.InsecureHTTP {
		ctx = oidc.InsecureIssuerURLContext(ctx, cfg.Issuer)
	}
	_ = m.tryDiscover(ctx)
	return m
}

func (m *Manager) tryDiscover(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.provider != nil {
		return nil
	}
	if time.Since(m.lastTry) < 30*time.Second && m.lastErr != nil {
		return m.lastErr // backoff: don't hammer a down IdP
	}
	m.lastTry = time.Now()
	p, err := oidc.NewProvider(ctx, m.cfg.Issuer)
	if err != nil {
		m.lastErr = err
		return err
	}
	m.provider = p
	m.verifier = p.Verifier(&oidc.Config{ClientID: m.cfg.ClientID, SupportedSigningAlgs: []string{oidc.RS256, oidc.ES256}})
	m.lastErr = nil
	return nil
}

// Ready reports whether discovery has succeeded; used by the "SSO
// temporarily unavailable" UI state.
func (m *Manager) Ready(ctx context.Context) bool {
	m.mu.RLock()
	ok := m.provider != nil
	m.mu.RUnlock()
	if !ok {
		_ = m.tryDiscover(ctx)
		m.mu.RLock()
		ok = m.provider != nil
		m.mu.RUnlock()
	}
	return ok
}

func (m *Manager) oauth2Config() *oauth2.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &oauth2.Config{
		ClientID:     m.cfg.ClientID,
		ClientSecret: m.cfg.ClientSecret,
		RedirectURL:  m.cfg.RedirectURL,
		Endpoint:     m.provider.Endpoint(),
		Scopes:       m.cfg.Scopes,
	}
}

// PKCE returns a random verifier and its S256 challenge.
func PKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return
}

// AuthCodeURL builds the provider authorization URL.
func (m *Manager) AuthCodeURL(ctx context.Context, state, nonce, codeChallenge string) (string, error) {
	if !m.Ready(ctx) {
		return "", fmt.Errorf("oidc: provider unavailable: %w", m.lastErr)
	}
	opts := []oauth2.AuthCodeOption{
		oidc.Nonce(nonce),
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	}
	return m.oauth2Config().AuthCodeURL(state, opts...), nil
}

type Claims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Groups        []string
	Raw           map[string]any
}

// Exchange completes the authorization code flow, verifies the ID token
// (signature, iss, aud, nonce, exp with 60s leeway), and extracts claims —
// calling userinfo as a fallback if the ID token has no email.
func (m *Manager) Exchange(ctx context.Context, code, codeVerifier, nonce, groupsClaim string) (*Claims, error) {
	if !m.Ready(ctx) {
		return nil, fmt.Errorf("oidc: provider unavailable: %w", m.lastErr)
	}
	tok, err := m.oauth2Config().Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return nil, fmt.Errorf("oidc: token exchange failed: %w", err)
	}
	rawIDToken, ok := tok.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, errors.New("oidc: no id_token in response")
	}
	m.mu.RLock()
	verifier := m.verifier
	provider := m.provider
	m.mu.RUnlock()
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("oidc: id_token verification failed: %w", err)
	}
	if idToken.Nonce != nonce {
		return nil, errors.New("oidc: nonce mismatch")
	}

	var raw map[string]any
	if err := idToken.Claims(&raw); err != nil {
		return nil, fmt.Errorf("oidc: parsing claims: %w", err)
	}

	c := &Claims{Subject: idToken.Subject, Raw: raw}
	if v, ok := raw["email"].(string); ok {
		c.Email = strings.ToLower(strings.TrimSpace(v))
	}
	if v, ok := raw["email_verified"].(bool); ok {
		c.EmailVerified = v
	}
	if v, ok := raw["name"].(string); ok {
		c.Name = v
	} else if v, ok := raw["preferred_username"].(string); ok {
		c.Name = v
	}
	c.Groups = extractGroups(raw, groupsClaim)

	// fallback to userinfo if email is missing (some providers omit it from the ID token)
	if c.Email == "" {
		userInfo, err := provider.UserInfo(ctx, oauth2.StaticTokenSource(tok))
		if err == nil {
			var ui map[string]any
			if err := userInfo.Claims(&ui); err == nil {
				if v, ok := ui["email"].(string); ok {
					c.Email = strings.ToLower(strings.TrimSpace(v))
				}
				if v, ok := ui["email_verified"].(bool); ok {
					c.EmailVerified = v
				}
			}
		}
	}

	if m.cfg.RequireEmailVerified && c.Email != "" && !c.EmailVerified {
		return nil, ErrEmailUnverified
	}
	if len(m.cfg.AllowedDomains) > 0 && c.Email != "" {
		domain := c.Email[strings.LastIndex(c.Email, "@")+1:]
		allowed := false
		for _, d := range m.cfg.AllowedDomains {
			if strings.EqualFold(d, domain) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, ErrDomainNotAllowed
		}
	}
	return c, nil
}

var (
	ErrEmailUnverified = errors.New("oidc: email not verified")
	ErrDomainNotAllowed = errors.New("oidc: email domain not allowed")
)

func extractGroups(raw map[string]any, claim string) []string {
	v, ok := raw[claim]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, g := range t {
			if s, ok := g.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		return []string{t}
	default:
		b, _ := json.Marshal(v)
		if len(b) > 32*1024 {
			return nil // guard against absurd token sizes (PLAN.md §24.4)
		}
		return nil
	}
}

// EndSessionURL returns the RP-initiated logout URL if the provider
// advertises one, else "".
func (m *Manager) EndSessionURL(idTokenHint, postLogoutRedirect string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.provider == nil {
		return ""
	}
	var claims struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := m.provider.Claims(&claims); err != nil || claims.EndSessionEndpoint == "" {
		return ""
	}
	u := claims.EndSessionEndpoint + "?post_logout_redirect_uri=" + postLogoutRedirect
	if idTokenHint != "" {
		u += "&id_token_hint=" + idTokenHint
	}
	return u
}
