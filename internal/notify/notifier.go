package notify

import (
	"context"
	"encoding/json"
	"log/slog"

	"shortr/internal/store"
)

// Kind identifies a notification type; used both as the DB `kind` column
// and as the per-user preference key.
type Kind string

const (
	KindUserRegistered  Kind = "user.registered"
	KindLinkExpiring    Kind = "link.expiring_soon"
	KindLoginLockout    Kind = "auth.lockout"
	KindNewLogin        Kind = "security.new_login"
	KindOIDCLinked      Kind = "security.oidc_linked"
	KindPasswordChanged Kind = "security.password_changed"
	KindBackupCompleted Kind = "system.backup_completed"
	KindBackupFailed    Kind = "system.backup_failed"
	KindAdminBroadcast  Kind = "admin.broadcast"
)

// AllKinds lists every kind a user can configure in Settings → Notifications.
var AllKinds = []Kind{
	KindUserRegistered, KindLinkExpiring, KindPasswordChanged, KindBackupFailed,
}

// ChannelsSettingPrefix is the settings key (plus user ID) holding the
// per-kind channel list edited in the UI: {"<kind>": ["email","gotify","browser"]}.
const ChannelsSettingPrefix = "notify_channels:"

// channelOn reports whether the user's UI-managed preferences allow channel
// for kind. A kind the user never touched defaults to on for every channel.
func channelOn(ctx context.Context, st *store.Store, userID string, kind Kind, channel string) bool {
	v, ok, err := st.GetSetting(ctx, ChannelsSettingPrefix+userID)
	if err != nil || !ok {
		return true
	}
	var m map[string][]string
	if json.Unmarshal([]byte(v), &m) != nil {
		return true
	}
	chans, set := m[string(kind)]
	if !set {
		return true
	}
	for _, c := range chans {
		if c == channel {
			return true
		}
	}
	return false
}

// DefaultKindPriority maps a kind to a Gotify priority / email urgency.
func (k Kind) priority() gotifyPriority {
	switch k {
	case KindBackupFailed, KindLoginLockout:
		return PriorityHigh
	case KindLinkExpiring, KindBackupCompleted:
		return PriorityLow
	default:
		return PriorityNormal
	}
}

// Prefs is a user's per-kind channel opt-ins, stored as JSON in the
// settings table under key "notify_prefs:<userID>" (reuse the
// existing settings table instead of a new one-row-per-user table).
type Prefs struct {
	Email  map[string]bool `json:"email"`
	Gotify map[string]bool `json:"gotify"`
	InApp  map[string]bool `json:"inApp"`
}

func DefaultPrefs() Prefs {
	return Prefs{Email: map[string]bool{}, Gotify: map[string]bool{}, InApp: map[string]bool{}}
}

// enabled defaults to true unless the user explicitly turned a kind off.
func (p Prefs) enabled(m map[string]bool, kind Kind) bool {
	if v, ok := m[string(kind)]; ok {
		return v
	}
	return true
}

func LoadPrefs(ctx context.Context, st *store.Store, userID string) Prefs {
	v, ok, err := st.GetSetting(ctx, "notify_prefs:"+userID)
	if err != nil || !ok {
		return DefaultPrefs()
	}
	var p Prefs
	if err := json.Unmarshal([]byte(v), &p); err != nil {
		return DefaultPrefs()
	}
	if p.Email == nil {
		p.Email = map[string]bool{}
	}
	if p.Gotify == nil {
		p.Gotify = map[string]bool{}
	}
	if p.InApp == nil {
		p.InApp = map[string]bool{}
	}
	return p
}

func SavePrefs(ctx context.Context, st *store.Store, userID string, p Prefs) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return st.SetSetting(ctx, "notify_prefs:"+userID, string(b), userID)
}

// Notifier fans a single event out to in-app storage, email, and Gotify per
// the recipient's preferences and the admin's global toggles. Every send is
// best-effort and logged; a channel failure never propagates to the caller
//.
type Notifier struct {
	store  *store.Store
	smtp   *SMTPSender
	gotify *GotifySender
	log    *slog.Logger
}

func New(st *store.Store, smtp *SMTPSender, gotify *GotifySender, log *slog.Logger) *Notifier {
	if log == nil {
		log = slog.Default()
	}
	return &Notifier{store: st, smtp: smtp, gotify: gotify, log: log}
}

// NotifyUser sends to one user (in-app always; email/gotify per their prefs
// and gotifyTarget, which is looked up by the caller since Gotify tokens are
// per-application, not per-user, in a typical single-app Gotify setup —
// v1 uses one shared admin-configured Gotify app for all users, simplest
// useful default. userEmail may be "" for OIDC-only accounts without a
// verified email on file; email delivery is skipped then).
func (n *Notifier) NotifyUser(ctx context.Context, userID, userEmail string, kind Kind, title, body string, data map[string]any) {
	dataJSON, _ := json.Marshal(data)
	rec := &store.Notification{UserID: &userID, Kind: string(kind), Title: title, Body: body, Data: string(dataJSON)}
	if err := n.store.CreateNotification(ctx, rec); err != nil {
		n.log.Warn("failed to store notification", "error", err)
	}

	prefs := LoadPrefs(ctx, n.store, userID)

	if n.smtp != nil && n.smtp.Enabled() && userEmail != "" && prefs.enabled(prefs.Email, kind) && channelOn(ctx, n.store, userID, kind, "email") {
		go func() {
			if err := n.smtp.Send(userEmail, "[Shortr] "+title, body); err != nil {
				n.log.Warn("email notification failed", "kind", kind, "error", err)
			}
		}()
	}
	if n.gotify != nil && n.gotify.Enabled() && prefs.enabled(prefs.Gotify, kind) && channelOn(ctx, n.store, userID, kind, "gotify") {
		go func() {
			if err := n.gotify.Send(context.Background(), title, body, kind.priority()); err != nil {
				n.log.Warn("gotify notification failed", "kind", kind, "error", err)
			}
		}()
	}
}

// NotifyAdmins broadcasts to every active admin (in-app row with
// user_id=NULL, one email/gotify send per admin who opted in).
func (n *Notifier) NotifyAdmins(ctx context.Context, kind Kind, title, body string, data map[string]any) {
	dataJSON, _ := json.Marshal(data)
	rec := &store.Notification{UserID: nil, Kind: string(kind), Title: title, Body: body, Data: string(dataJSON)}
	if err := n.store.CreateNotification(ctx, rec); err != nil {
		n.log.Warn("failed to store admin notification", "error", err)
	}

	admins, err := n.store.ListActiveAdmins(ctx)
	if err != nil {
		n.log.Warn("failed to list admins for notification fan-out", "error", err)
		return
	}
	for _, a := range admins {
		prefs := LoadPrefs(ctx, n.store, a.ID)
		if n.smtp != nil && n.smtp.Enabled() && prefs.enabled(prefs.Email, kind) && channelOn(ctx, n.store, a.ID, kind, "email") {
			go func(email string) {
				if err := n.smtp.Send(email, "[Shortr] "+title, body); err != nil {
					n.log.Warn("admin email notification failed", "error", err)
				}
			}(a.Email)
		}
	}
	if n.gotify != nil && n.gotify.Enabled() {
		go func() {
			if err := n.gotify.Send(context.Background(), title, body, kind.priority()); err != nil {
				n.log.Warn("admin gotify notification failed", "error", err)
			}
		}()
	}
}
