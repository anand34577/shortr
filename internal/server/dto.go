package server

import (
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
	ShortURL       string     `json:"short_url"`
	TargetURL      string     `json:"target_url"`
	Title          string     `json:"title"`
	Description    string     `json:"description,omitempty"`
	UserID         *string    `json:"user_id"`
	RedirectStatus int        `json:"redirect_status"`
	HasPassword    bool       `json:"has_password"`
	ExpiresAt      *time.Time `json:"expires_at"`
	MaxClicks      *int       `json:"max_clicks"`
	ClickCount     int64      `json:"click_count"`
	LastClickAt    *time.Time `json:"last_click_at"`
	Status         string     `json:"status"`
	Tags           []string   `json:"tags"`
	PassQuery      bool       `json:"pass_query"`
	UTM            utmDTO     `json:"utm"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
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
		CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
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
	EmailVerified bool       `json:"email_verified"`
	Name          string     `json:"name"`
	Role          string     `json:"role"`
	Status        string     `json:"status"`
	HasPassword   bool       `json:"has_password"`
	LastLoginAt   *time.Time `json:"last_login_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

func toUserDTO(u *store.User) userDTO {
	return userDTO{
		ID: u.ID, Email: u.Email, EmailVerified: u.EmailVerified, Name: u.Name, Role: u.Role, Status: u.Status,
		HasPassword: u.PasswordHash != nil, LastLoginAt: u.LastLoginAt, CreatedAt: u.CreatedAt,
	}
}

type meDTO struct {
	userDTO
	CSRFToken    string   `json:"csrf_token"`
	Capabilities []string `json:"capabilities"`
}

type apiKeyDTO struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

func toAPIKeyDTO(k *store.APIKey) apiKeyDTO {
	return apiKeyDTO{ID: k.ID, Name: k.Name, Prefix: k.Prefix, Scopes: k.Scopes, LastUsedAt: k.LastUsedAt, ExpiresAt: k.ExpiresAt, CreatedAt: k.CreatedAt}
}

type sessionDTO struct {
	ID         string    `json:"id"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	Current    bool      `json:"current"`
}

type identityDTO struct {
	ID          string     `json:"id"`
	Issuer      string     `json:"issuer"`
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at"`
}

func toIdentityDTO(o *store.OIDCIdentity) identityDTO {
	return identityDTO{ID: o.ID, Issuer: o.Issuer, Email: o.Email, Name: o.Name, CreatedAt: o.CreatedAt, LastLoginAt: o.LastLoginAt}
}

type notificationDTO struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Data      string     `json:"data"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func toNotificationDTO(n *store.Notification) notificationDTO {
	return notificationDTO{ID: n.ID, Kind: n.Kind, Title: n.Title, Body: n.Body, Data: n.Data, ReadAt: n.ReadAt, CreatedAt: n.CreatedAt}
}

type auditDTO struct {
	ID          string    `json:"id"`
	TS          time.Time `json:"ts"`
	ActorUserID string    `json:"actor_user_id"`
	ActorIP     string    `json:"actor_ip"`
	Action      string    `json:"action"`
	TargetType  string    `json:"target_type"`
	TargetID    string    `json:"target_id"`
	Meta        string    `json:"meta"`
}

func toAuditDTO(e *store.AuditEntry) auditDTO {
	return auditDTO{ID: e.ID, TS: e.TS, ActorUserID: e.ActorUserID, ActorIP: e.ActorIP, Action: e.Action, TargetType: e.TargetType, TargetID: e.TargetID, Meta: e.Meta}
}
