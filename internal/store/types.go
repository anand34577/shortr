package store

import "time"

type User struct {
	ID                 string
	Email              string
	EmailVerified      bool
	Name               string
	PasswordHash       *string
	Role               string // admin | user
	Status             string // active | disabled
	MaxLinks           *int
	RoleLocked         bool
	MustChangePassword bool
	LastLoginAt        *time.Time
	PasswordChangedAt  *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (u *User) IsAdmin() bool  { return u.Role == "admin" }
func (u *User) IsActive() bool { return u.Status == "active" }

type OIDCIdentity struct {
	ID          string
	UserID      string
	Issuer      string
	Subject     string
	Email       string
	Name        string
	RawClaims   string
	CreatedAt   time.Time
	LastLoginAt *time.Time
}

type Session struct {
	ID         string // hashed token (hex sha256)
	UserID     string
	CSRFToken  string
	IP         string
	UserAgent  string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastSeenAt time.Time
}

type APIKey struct {
	ID         string
	UserID     string
	Name       string
	Prefix     string
	KeyHash    string
	Scopes     []string
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

type Link struct {
	ID             string
	Code           string
	TargetURL      string
	Title          string
	Description    string
	UserID         *string
	RedirectStatus int
	PasswordHash   *string
	ExpiresAt      *time.Time
	MaxClicks      *int
	ClickCount     int64
	LastClickAt    *time.Time
	Status         string // active | disabled
	DeletedAt      *time.Time
	UTMSource      string
	UTMMedium      string
	UTMCampaign    string
	UTMTerm        string
	UTMContent     string
	PassQuery      bool
	Tags           []string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CreatedByIP    string
}

func (l *Link) HasPassword() bool { return l.PasswordHash != nil && *l.PasswordHash != "" }

type Click struct {
	ID             int64
	LinkID         string
	TS             time.Time
	IP             string
	IPVersion      int
	Country        string
	Region         string
	City           string
	Referrer       string
	ReferrerHost   string
	UserAgent      string
	Device         string
	OS             string
	OSVersion      string
	Browser        string
	BrowserVersion string
	IsBot          bool
	Lang           string
	UTMSource      string
	UTMMedium      string
	UTMCampaign    string
	UTMTerm        string
	UTMContent     string
	QS             string
}

type AuditEntry struct {
	ID          string
	TS          time.Time
	ActorUserID string
	ActorIP     string
	Action      string
	TargetType  string
	TargetID    string
	Meta        string // JSON
}

type Notification struct {
	ID        string
	UserID    *string // nil = broadcast to admins
	Kind      string
	Title     string
	Body      string
	Data      string
	ReadAt    *time.Time
	CreatedAt time.Time
}

// Page is a generic cursor-paginated result.
type Page[T any] struct {
	Items      []T
	NextCursor string
}
