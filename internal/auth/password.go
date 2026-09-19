// Package auth implements password hashing, sessions, API keys, and OIDC.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters (OWASP baseline: t=1 memory=19MB is minimum; we use a
// stronger profile since login is infrequent). Encoded into the hash string
// so future upgrades can rehash old hashes transparently.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // KB (64 MB)
	argonThreads = 1
	argonKeyLen  = 32
	argonSaltLen = 16
)

var ErrInvalidHash = errors.New("auth: invalid password hash format")
var ErrMismatch = errors.New("auth: password does not match")

// HashPassword returns an encoded argon2id hash: $argon2id$v=19$m=...,t=...,p=...$salt$hash
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads, b64Salt, b64Hash), nil
}

// VerifyPassword reports whether password matches the encoded hash, and
// whether the hash uses outdated parameters and should be rehashed.
func VerifyPassword(password, encoded string) (ok, needsRehash bool, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, false, ErrInvalidHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, false, ErrInvalidHash
	}
	var mem uint32
	var t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &t, &p); err != nil {
		return false, false, ErrInvalidHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, false, ErrInvalidHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, false, ErrInvalidHash
	}
	got := argon2.IDKey([]byte(password), salt, t, mem, p, uint32(len(want)))
	match := subtle.ConstantTimeCompare(got, want) == 1
	rehash := mem != argonMemory || t != argonTime || p != argonThreads || version != argon2.Version
	return match, rehash, nil
}

// dummyHash is verified against on unknown-email login attempts so timing
// doesn't reveal whether an account exists.
var dummyHash string

func init() {
	h, err := HashPassword("dummy-constant-time-comparison-password")
	if err != nil {
		panic(err)
	}
	dummyHash = h
}

// VerifyAgainstDummy performs a wasted argon2 verify with constant shape to
// keep login timing similar for unknown emails.
func VerifyAgainstDummy(password string) {
	VerifyPassword(password, dummyHash) //nolint:errcheck
}
