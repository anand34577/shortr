// Package validate holds server-side input validation shared by API handlers.
// Every rule here is the source of truth; the frontend's zod schemas mirror it
// for UX only.
package validate

import (
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"strings"
)

var ErrInvalid = errors.New("invalid")

// FieldError is a validation failure attributable to one request field.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }

// Errors collects multiple FieldErrors.
type Errors []*FieldError

func (e Errors) Error() string {
	var sb strings.Builder
	for i, fe := range e {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(fe.Error())
	}
	return sb.String()
}

func (e Errors) Map() map[string]string {
	m := make(map[string]string, len(e))
	for _, fe := range e {
		m[fe.Field] = fe.Message
	}
	return m
}

func (e *Errors) Add(field, msg string, args ...any) {
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	*e = append(*e, &FieldError{Field: field, Message: msg})
}

func (e Errors) HasAny() bool { return len(e) > 0 }

// TargetURL validates and normalises a link target. allowPrivate permits
// loopback/private/link-local hosts (admin override); blocked is a list of
// exact-or-suffix-matched blocked domains.
func TargetURL(raw string, maxLen int, allowPrivate bool, blocked []string, selfHost string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("target_url is required")
	}
	if len(raw) > maxLen {
		return "", fmt.Errorf("target_url must be at most %d characters", maxLen)
	}
	if strings.ContainsAny(raw, " \t\r\n") {
		return "", errors.New("target_url must not contain whitespace (encode it first)")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("target_url is not a valid URL")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", errors.New("target_url must use http or https")
	}
	if u.Host == "" {
		return "", errors.New("target_url must include a host")
	}
	if u.User != nil {
		return "", errors.New("target_url must not contain credentials")
	}
	host := strings.ToLower(u.Hostname())
	if selfHost != "" && host == strings.ToLower(selfHost) {
		return "", errors.New("target_url must not point back at this shortener")
	}
	for _, b := range blocked {
		b = strings.ToLower(strings.TrimSpace(b))
		if b == "" {
			continue
		}
		if host == b || strings.HasSuffix(host, "."+b) {
			return "", errors.New("target_url domain is blocked")
		}
	}
	if !allowPrivate && isPrivateHost(host) {
		return "", errors.New("target_url must not point to a private, loopback, or internal address")
	}
	u.Scheme = scheme
	u.Host = strings.ToLower(u.Host)
	return u.String(), nil
}

func isPrivateHost(host string) bool {
	h := strings.ToLower(host)
	if h == "localhost" || strings.HasSuffix(h, ".localhost") ||
		strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".internal") {
		return true
	}
	ip := net.ParseIP(strings.Trim(h, "[]"))
	if ip == nil {
		return false // not a literal IP; DNS resolution (SSRF check) happens at fetch time, not here
	}
	return IsPrivateIP(ip)
}

// IsPrivateIP reports whether ip is loopback, private, link-local, multicast,
// or unspecified — used both for literal-IP targets and for the title-fetch
// SSRF guard (which re-checks resolved IPs).
func IsPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// carrier-grade NAT 100.64.0.0/10
		if ip4[0] == 100 && ip4[1]&0xC0 == 64 {
			return true
		}
	} else {
		// unique local IPv6 fc00::/7
		if ip[0]&0xFE == 0xFC {
			return true
		}
	}
	return false
}

// Email validates and normalises an address (lowercased, trimmed).
func Email(raw string) (string, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" || len(raw) > 254 {
		return "", errors.New("email is required and must be at most 254 characters")
	}
	addr, err := mail.ParseAddress(raw)
	if err != nil || addr.Address != raw {
		return "", errors.New("email is not valid")
	}
	if !strings.Contains(raw, "@") {
		return "", errors.New("email is not valid")
	}
	domain := raw[strings.LastIndex(raw, "@")+1:]
	if !strings.Contains(domain, ".") {
		return "", errors.New("email domain is not valid")
	}
	return raw, nil
}

// Alias validates a user-chosen short code.
func Alias(s string, isReserved func(string) bool, validChars func(string) bool) error {
	if !validChars(s) {
		return errors.New("must be 1-64 characters: letters, digits, - or _, not starting/ending with - or _")
	}
	if isReserved(s) {
		return errors.New("this alias is reserved")
	}
	return nil
}

func StrLen(s string, min, max int) bool {
	n := len([]rune(s))
	return n >= min && n <= max
}

// NextPath validates a post-login redirect target: must be a relative path
// under /app, no scheme, no protocol-relative "//" prefix (open-redirect guard).
func NextPath(next string) string {
	if next == "" || len(next) > 512 {
		return "/app"
	}
	if !strings.HasPrefix(next, "/app") {
		return "/app"
	}
	if strings.HasPrefix(next, "//") || strings.Contains(next, "\\") {
		return "/app"
	}
	if strings.ContainsAny(next, "\r\n") {
		return "/app"
	}
	return next
}
