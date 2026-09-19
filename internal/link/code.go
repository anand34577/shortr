package link

import (
	"crypto/rand"
	"strings"
)

const (
	AlphabetBase58 = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	AlphabetBase62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// reserved path segments that can never be a link code (routes, or confusing).
var Reserved = map[string]bool{
	"api": true, "app": true, "auth": true, "admin": true, "assets": true,
	"static": true, "metrics": true, "health": true, "healthz": true, "readyz": true,
	"ready": true, "robots.txt": true, "favicon.ico": true, "sitemap.xml": true,
	"login": true, "logout": true, "setup": true, "oidc": true, "qr": true,
	"s": true, "u": true, "l": true, "p": true, "docs": true, "version": true,
	"debug": true, ".well-known": true,
}

// GenerateCode returns a cryptographically random code of the given length
// drawn from alphabet, using rejection sampling to avoid modulo bias.
func GenerateCode(alphabet string, length int) (string, error) {
	if length < 4 {
		length = 4
	}
	n := len(alphabet)
	// largest multiple of n that fits in a byte, for rejection sampling
	limit := byte(256 - (256 % n))
	var sb strings.Builder
	sb.Grow(length)
	buf := make([]byte, 1)
	for sb.Len() < length {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		if buf[0] >= limit {
			continue
		}
		sb.WriteByte(alphabet[int(buf[0])%n])
	}
	return sb.String(), nil
}

// ValidAliasChars reports whether s only contains [A-Za-z0-9_-], 1..64 long,
// and does not start/end with '-' or '_'.
func ValidAliasChars(s string) bool {
	if len(s) < 1 || len(s) > 64 {
		return false
	}
	if s[0] == '-' || s[0] == '_' || s[len(s)-1] == '-' || s[len(s)-1] == '_' {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '-' || c == '_':
		default:
			return false
		}
	}
	return true
}

// IsReserved reports whether code (case-insensitively) collides with a route
// or starts with a char that would break routing.
func IsReserved(code string) bool {
	if code == "" || code[0] == '_' || code[0] == '.' {
		return true
	}
	return Reserved[strings.ToLower(code)]
}

// ValidCodePathSegment is a cheap pre-DB filter for the redirect hot path:
// reject anything that could not possibly be a stored code before touching
// the cache or DB.
func ValidCodePathSegment(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '-' || c == '_' || c == '.':
		default:
			return false
		}
	}
	return true
}
