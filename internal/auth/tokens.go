package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// NewOpaqueToken returns a URL-safe random token and its sha256 hex digest.
// The raw token is shown to the caller once (cookie / API key display); only
// the hash is ever stored, so a DB leak doesn't hand out live credentials.
func NewOpaqueToken(nBytes int) (raw, hash string, err error) {
	b := make([]byte, nBytes)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	hash = HashToken(raw)
	return raw, hash, nil
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func ConstantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

const apiKeyPrefixLen = 8
const apiKeyBodyAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// NewAPIKey returns a display key like "sk_live_ab12cd34_<32 random chars>",
// its stable prefix (safe to show in lists), and its sha256 hash (stored).
func NewAPIKey() (full, prefix, hash string, err error) {
	const bodyLen = 32
	var sb strings.Builder
	// Rejection sampling against the largest multiple of len(alphabet) that
	// fits in a byte, so every character is uniformly distributed (256 % 62
	// != 0, so a plain modulo would introduce a slight bias).
	alphabetLen := len(apiKeyBodyAlphabet)
	limit := byte(256 - (256 % alphabetLen))
	buf := make([]byte, 1)
	for sb.Len() < bodyLen {
		if _, err = rand.Read(buf); err != nil {
			return "", "", "", err
		}
		if buf[0] >= limit {
			continue
		}
		sb.WriteByte(apiKeyBodyAlphabet[int(buf[0])%alphabetLen])
	}
	body := sb.String()
	prefix = body[:apiKeyPrefixLen]
	full = "sk_" + body
	hash = HashToken(full)
	return full, prefix, hash, nil
}
