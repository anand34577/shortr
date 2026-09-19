package server

import (
	"encoding/json"
	"time"

	"shortr/internal/store"
)

type utmDTO struct {
	Source   string `json:"source,omitempty"`
	Medium   string `json:"medium,omitempty"`
	Campaign string `json:"campaign,omitempty"`
	Term     string `json:"term,omitempty"`
	Content  string `json:"content,omitempty"`
}

type linkDTO struct {
	ID             string     `json:"id"`
	Code           string     `json:"code"`
	ShortURL       string     `json:"shortUrl"`
	TargetURL      string     `json:"targetUrl"`
	Title          string     `json:"title"`
	Description    string     `json:"description,omitempty"`
	UserID         *string    `json:"userId"`
	RedirectStatus int        `json:"redirectStatus"`
	HasPassword    bool       `json:"hasPassword"`
	ExpiresAt      *time.Time `json:"expiresAt"`
	MaxClicks      *int       `json:"maxClicks"`
	ClickCount     int64      `json:"clickCount"`
	LastClickAt    *time.Time `json:"lastClickAt"`
	Status         string     `json:"status"`
	Tags           []string   `json:"tags"`
	PassQuery      bool       `json:"passQuery"`
	UTM            utmDTO     `json:"utm"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	DeletedAt      *time.Time `json:"deletedAt"`
}

func (s *Server) toLinkDTO(l *store.Link) linkDTO {
	tags := l.Tags
	if tags == nil {
		tags = []string{}
	}
	return linkDTO{
		ID: l.ID, Code: l.Code, ShortURL: s.links.ShortURL(l.Code), TargetURL: l.TargetURL,
		Title: l.Title, Description: l.Description, UserID: l.UserID, RedirectStatus: l.RedirectStatus,
		HasPassword: l.HasPassword(), ExpiresAt: l.ExpiresAt, MaxClicks: l.MaxClicks, ClickCount: l.ClickCount,
		LastClickAt: l.LastClickAt, Status: statusOf(l), Tags: tags, PassQuery: l.PassQuery,
		UTM:       utmDTO{l.UTMSource, l.UTMMedium, l.UTMCampaign, l.UTMTerm, l.UTMContent},
		CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt, DeletedAt: l.DeletedAt,
	}
}

func statusOf(l *store.Link) string {
	if l.DeletedAt != nil {
		return "deleted"
	}
	return l.Status
}

type userDTO struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	EmailVerified bool       `json:"emailVerified"`
	Name          string     `json:"name"`
	Role          string     `json:"role"`
	Status        string     `json:"status"`
	HasPassword   bool       `json:"hasPassword"`
	LastLoginAt   *time.Time `json:"lastLoginAt"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`

	MaxLinks           *int `json:"maxLinks"`
	RoleLocked         bool `json:"roleLocked"`
	MustChangePassword bool `json:"mustChangePassword"`
	LinksCount         *int `json:"linksCount,omitempty"`
}

func toUserDTO(u *store.User) userDTO {
	return userDTO{
		ID: u.ID, Email: u.Email, EmailVerified: u.EmailVerified, Name: u.Name, Role: u.Role, Status: u.Status,
		HasPassword: u.PasswordHash != nil, LastLoginAt: u.LastLoginAt, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
		MaxLinks: u.MaxLinks, RoleLocked: u.RoleLocked, MustChangePassword: u.MustChangePassword,
	}
}

type meDTO struct {
	userDTO
	CSRFToken    string   `json:"csrfToken"`
	Capabilities []string `json:"capabilities"`
}

type apiKeyDTO struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	CreatedAt  time.Time  `json:"createdAt"`
}

func toAPIKeyDTO(k *store.APIKey) apiKeyDTO {
	return apiKeyDTO{ID: k.ID, Name: k.Name, Prefix: k.Prefix, Scopes: k.Scopes, LastUsedAt: k.LastUsedAt, ExpiresAt: k.ExpiresAt, CreatedAt: k.CreatedAt}
}

type sessionDTO struct {
	ID         string    `json:"id"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"userAgent"`
	CreatedAt  time.Time `json:"createdAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	LastSeenAt time.Time `json:"lastSeenAt"`
	Current    bool      `json:"current"`
}

type identityDTO struct {
	ID          string     `json:"id"`
	Issuer      string     `json:"issuer"`
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	CreatedAt   time.Time  `json:"createdAt"`
	LastLoginAt *time.Time `json:"lastLoginAt"`
}

func toIdentityDTO(o *store.OIDCIdentity) identityDTO {
	return identityDTO{ID: o.ID, Issuer: o.Issuer, Email: o.Email, Name: o.Name, CreatedAt: o.CreatedAt, LastLoginAt: o.LastLoginAt}
}

type notificationDTO struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Title     string          `json:"title"`
	Body      string          `json:"body"`
	Data      json.RawMessage `json:"data"`
	Priority  string          `json:"priority"`
	ReadAt    *time.Time      `json:"readAt"`
	CreatedAt time.Time       `json:"createdAt"`
}

func toNotificationDTO(n *store.Notification) notificationDTO {
	return notificationDTO{ID: n.ID, Kind: n.Kind, Title: n.Title, Body: n.Body, Data: rawJSONOrNull(n.Data), Priority: notificationPriority(n.Kind), ReadAt: n.ReadAt, CreatedAt: n.CreatedAt}
}

type auditDTO struct {
	ID          string          `json:"id"`
	TS          time.Time       `json:"ts"`
	ActorUserID string          `json:"actorUserId"`
	ActorEmail  string          `json:"actorEmail,omitempty"`
	ActorIP     string          `json:"actorIp"`
	Action      string          `json:"action"`
	TargetType  string          `json:"targetType"`
	TargetID    string          `json:"targetId"`
	Meta        json.RawMessage `json:"meta"`
}

func toAuditDTO(e *store.AuditEntry) auditDTO {
	return auditDTO{ID: e.ID, TS: e.TS, ActorUserID: e.ActorUserID, ActorIP: e.ActorIP, Action: e.Action, TargetType: e.TargetType, TargetID: e.TargetID, Meta: rawJSONOrNull(e.Meta)}
}

// rawJSONOrNull passes stored JSON text through as a JSON value; empty or
// malformed text becomes null so the response is always valid JSON.
func rawJSONOrNull(v string) json.RawMessage {
	if v != "" && json.Valid([]byte(v)) {
		return json.RawMessage(v)
	}
	return json.RawMessage("null")
}

func notificationPriority(kind string) string {
	switch kind {
	case "auth.lockout", "system.backup_failed", "security.password_changed":
		return "high"
	}
	return "normal"
}
